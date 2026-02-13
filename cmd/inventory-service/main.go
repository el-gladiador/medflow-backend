package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/medflow/medflow-backend/internal/inventory/consumers"
	"github.com/medflow/medflow-backend/internal/inventory/events"
	"github.com/medflow/medflow-backend/internal/inventory/handler"
	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/internal/inventory/service"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

func main() {
	// Load configuration with validation (fails fast in production if required config is missing)
	cfg, err := config.LoadWithValidation("inventory-service")
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("inventory-service", cfg.Server.Environment)
	log.Info().Msg("starting Inventory Service")

	// Connect to database (single Supabase DB, search_path = inventory, public)
	db, err := database.NewWithSearchPath(&cfg.Database, cfg.Database.SearchPath, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Connect to RabbitMQ (optional — nil if unavailable)
	rmq := messaging.NewOptional(&cfg.RabbitMQ, log)
	if rmq != nil {
		defer rmq.Close()
	}

	// Initialize event publisher (nil-safe if RabbitMQ is unavailable)
	var publisher *events.InventoryEventPublisher
	if rmq != nil {
		var err error
		publisher, err = events.NewInventoryEventPublisher(rmq, log)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create event publisher")
		}
	} else {
		log.Warn().Msg("event publishing disabled (no RabbitMQ)")
	}

	// Initialize repositories
	locationRepo := repository.NewLocationRepository(db)
	itemRepo := repository.NewItemRepository(db)
	batchRepo := repository.NewBatchRepository(db)
	alertRepo := repository.NewAlertRepository(db)
	userCacheRepo := repository.NewUserCacheRepository(db)
	hazardousRepo := repository.NewHazardousRepository(db)
	documentRepo := repository.NewDocumentRepository(db)
	temperatureRepo := repository.NewTemperatureRepository(db)
	inspectionRepo := repository.NewInspectionRepository(db)
	trainingRepo := repository.NewTrainingRepository(db)
	incidentRepo := repository.NewIncidentRepository(db)

	// Regulatory compliance repositories
	auditRepo := repository.NewAuditTrailRepository(db)
	btmRepo := repository.NewBtmRepository(db)
	btmAuthRepo := repository.NewBtmAuthRepository(db)
	safetyOfficerRepo := repository.NewSafetyOfficerRepository(db)
	recallRepo := repository.NewRecallRepository(db)
	biosafetyRepo := repository.NewBioSafetyRepository(db)
	retentionRepo := repository.NewRetentionRepository(db)
	reprocessingRepo := repository.NewReprocessingRepository(db)
	hygieneRepo := repository.NewHygieneRepository(db)
	radiationRepo := repository.NewRadiationRepository(db)

	// Initialize service
	inventoryService := service.NewInventoryService(locationRepo, itemRepo, batchRepo, alertRepo, hazardousRepo, documentRepo, temperatureRepo, inspectionRepo, trainingRepo, incidentRepo, publisher, log)

	// Regulatory compliance services
	auditService := service.NewAuditService(auditRepo, log)
	btmService := service.NewBtmService(btmRepo, btmAuthRepo, itemRepo, auditService, log)
	recallService := service.NewRecallService(recallRepo, safetyOfficerRepo, itemRepo, batchRepo, alertRepo, auditService, log)
	biosafetyService := service.NewBioSafetyService(biosafetyRepo, auditService, log)
	retentionService := service.NewRetentionService(retentionRepo, auditService, log)
	reprocessingService := service.NewReprocessingService(reprocessingRepo, auditService, log)
	hygieneService := service.NewHygieneService(hygieneRepo, auditService, log)
	radiationService := service.NewRadiationService(radiationRepo, auditService, log)

	// Initialize handlers
	locationHandler := handler.NewLocationHandler(locationRepo, log)
	itemHandler := handler.NewItemHandler(inventoryService, log)
	batchHandler := handler.NewBatchHandler(inventoryService, log)
	alertHandler := handler.NewAlertHandler(alertRepo, log)
	dashboardHandler := handler.NewDashboardHandler(inventoryService, log)
	complianceHandler := handler.NewComplianceHandler(inventoryService, log)
	exportHandler := handler.NewExportHandler(inventoryService, log)
	temperatureHandler := handler.NewTemperatureHandler(inventoryService, log)
	deviceBookHandler := handler.NewDeviceBookHandler(inventoryService, log)

	// Regulatory compliance handlers
	auditHandler := handler.NewAuditHandler(auditService, log)
	btmHandler := handler.NewBtmHandler(btmService, log)
	recallHandler := handler.NewRecallHandler(recallService, safetyOfficerRepo, log)
	biosafetyHandler := handler.NewBioSafetyHandler(biosafetyService, log)
	retentionHandler := handler.NewRetentionHandler(retentionService, log)
	reprocessingHandler := handler.NewReprocessingHandler(reprocessingService, log)
	hygieneHandler := handler.NewHygieneHandler(hygieneService, log)
	radiationHandler := handler.NewRadiationHandler(radiationService, log)
	dataPortabilityHandler := handler.NewDataPortabilityHandler(inventoryService, biosafetyService, retentionService, reprocessingService, hygieneService, radiationService, log)

	// Initialize alert scanner and scheduler
	alertScanner := service.NewAlertScanner(itemRepo, batchRepo, alertRepo, temperatureRepo, locationRepo, log)
	alertScheduler := service.NewAlertScheduler(alertScanner, db, 15*time.Minute, log)

	// Start user event consumer (if RabbitMQ is available)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if rmq != nil {
		userConsumer, err := consumers.NewUserEventConsumer(rmq, userCacheRepo, log)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create user event consumer")
		}
		if err := userConsumer.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("failed to start user event consumer")
		}
	} else {
		log.Warn().Msg("user event consumer disabled (no RabbitMQ)")
	}

	// Start alert scheduler (runs periodic alert scans across all tenants)
	alertScheduler.Start(ctx)

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))
	r.Use(httputil.TenantMiddleware) // Extract tenant context from headers

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":   "healthy",
			"service":  "inventory-service",
			"database": db.Health(r.Context()),
		}
		if rmq != nil {
			health["rabbitmq"] = rmq.Health()
		} else {
			health["rabbitmq"] = map[string]string{"status": "disabled"}
		}
		httputil.JSON(w, http.StatusOK, health)
	})

	// API routes
	r.Route("/api/v1/inventory", func(r chi.Router) {
		// Location routes
		r.Route("/locations", func(r chi.Router) {
			r.Get("/tree", locationHandler.GetTree)
			r.Route("/rooms", func(r chi.Router) {
				r.Get("/", locationHandler.ListRooms)
				r.Post("/", locationHandler.CreateRoom)
				r.Get("/{id}", locationHandler.GetRoom)
				r.Put("/{id}", locationHandler.UpdateRoom)
				r.Delete("/{id}", locationHandler.DeleteRoom)
			})
			r.Route("/cabinets", func(r chi.Router) {
				r.Get("/", locationHandler.ListCabinets)
				r.Post("/", locationHandler.CreateCabinet)
				r.Get("/{id}", locationHandler.GetCabinet)
				r.Put("/{id}", locationHandler.UpdateCabinet)
				r.Delete("/{id}", locationHandler.DeleteCabinet)
				// Temperature monitoring
				r.Post("/{id}/temperature", temperatureHandler.RecordTemperature)
				r.Get("/{id}/temperature", temperatureHandler.ListReadings)
			})
			r.Route("/shelves", func(r chi.Router) {
				r.Get("/", locationHandler.ListShelves)
				r.Post("/", locationHandler.CreateShelf)
				r.Get("/{id}", locationHandler.GetShelf)
				r.Put("/{id}", locationHandler.UpdateShelf)
				r.Delete("/{id}", locationHandler.DeleteShelf)
			})
		})

		// Temperature webhook
		r.Post("/temperature/webhook", temperatureHandler.Webhook)

		// Item routes
		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemHandler.List)
			r.Post("/", itemHandler.Create)
			r.Get("/{id}", itemHandler.Get)
			r.Put("/{id}", itemHandler.Update)
			r.Delete("/{id}", itemHandler.Delete)
			r.Get("/{id}/batches", batchHandler.ListByItem)
			r.Post("/{id}/batches", batchHandler.Create)
			// Compliance: hazardous substance details
			r.Get("/{id}/hazardous", complianceHandler.GetHazardousDetails)
			r.Put("/{id}/hazardous", complianceHandler.UpsertHazardousDetails)
			r.Delete("/{id}/hazardous", complianceHandler.DeleteHazardousDetails)
			// Compliance: item documents
			r.Get("/{id}/documents", complianceHandler.ListDocuments)
			r.Post("/{id}/documents", complianceHandler.UploadDocument)
			// Device book (Medizinproduktebuch)
			r.Route("/{id}/device-book", func(r chi.Router) {
				r.Get("/inspections", deviceBookHandler.ListInspections)
				r.Post("/inspections", deviceBookHandler.CreateInspection)
				r.Put("/inspections/{inspId}", deviceBookHandler.UpdateInspection)
				r.Delete("/inspections/{inspId}", deviceBookHandler.DeleteInspection)
				r.Get("/trainings", deviceBookHandler.ListTrainings)
				r.Post("/trainings", deviceBookHandler.CreateTraining)
				r.Put("/trainings/{trId}", deviceBookHandler.UpdateTraining)
				r.Delete("/trainings/{trId}", deviceBookHandler.DeleteTraining)
				r.Get("/incidents", deviceBookHandler.ListIncidents)
				r.Post("/incidents", deviceBookHandler.CreateIncident)
				r.Put("/incidents/{incId}", deviceBookHandler.UpdateIncident)
				r.Delete("/incidents/{incId}", deviceBookHandler.DeleteIncident)
			})
		})

		// Batch routes
		r.Route("/batches", func(r chi.Router) {
			r.Get("/{id}", batchHandler.Get)
			r.Put("/{id}", batchHandler.Update)
			r.Delete("/{id}", batchHandler.Delete)
			r.Post("/{id}/adjust", batchHandler.AdjustStock)
			r.Post("/{id}/open", batchHandler.OpenBatch)
		})

		// Document routes (outside items for direct access by doc ID)
		r.Route("/documents", func(r chi.Router) {
			r.Delete("/{id}", complianceHandler.DeleteDocument)
			r.Get("/{id}/download", complianceHandler.DownloadDocument)
		})

		// Export routes
		r.Get("/export/inventory-register", exportHandler.ExportInventoryRegister)
		r.Get("/export/gefahrstoffverzeichnis", exportHandler.ExportGefahrstoffverzeichnis)
		r.Get("/export/bestandsverzeichnis", exportHandler.ExportBestandsverzeichnis)

		// Alert routes
		r.Get("/alerts", alertHandler.List)
		r.Put("/alerts/{id}/acknowledge", alertHandler.Acknowledge)

		// Dashboard
		r.Get("/dashboard/stats", dashboardHandler.GetStats)

		// --- Regulatory Compliance Routes ---

		// Audit trail routes (GoBD compliance)
		r.Get("/items/{id}/audit", auditHandler.GetItemAudit)
		r.Get("/audit", auditHandler.ListAudit)

		// BtM (controlled substance) routes
		r.Route("/btm/{itemId}", func(r chi.Router) {
			r.Get("/register", btmHandler.GetRegister)
			r.Get("/balance", btmHandler.GetBalance)
			r.Post("/receipt", btmHandler.ReceiveSubstance)
			r.Post("/dispense", btmHandler.DispenseSubstance)
			r.Post("/disposal", btmHandler.DisposeSubstance)
			r.Post("/correction", btmHandler.CorrectEntry)
			r.Post("/check", btmHandler.InventoryCheck)
		})
		r.Get("/btm/authorized-personnel", btmHandler.ListAuthorizedPersonnel)
		r.Post("/btm/authorized-personnel", btmHandler.CreateAuthorizedPerson)
		r.Put("/btm/authorized-personnel/{id}/revoke", btmHandler.RevokeAuthorization)

		// Recall / Field Safety Notice routes
		r.Route("/recalls", func(r chi.Router) {
			r.Get("/", recallHandler.ListNotices)
			r.Post("/", recallHandler.CreateNotice)
			r.Get("/{id}", recallHandler.GetNotice)
			r.Put("/{id}/status", recallHandler.UpdateNoticeStatus)
			r.Get("/{id}/matches", recallHandler.ListMatchesByNotice)
			r.Put("/matches/{id}/resolve", recallHandler.ResolveMatch)
			r.Get("/matches/pending", recallHandler.ListPendingMatches)
		})
		r.Get("/safety-officers", recallHandler.ListSafetyOfficers)
		r.Post("/safety-officers", recallHandler.CreateSafetyOfficer)
		r.Put("/safety-officers/{id}", recallHandler.UpdateSafetyOfficer)
		r.Delete("/safety-officers/{id}", recallHandler.DeleteSafetyOfficer)

		// Bio-safety routes (BioStoffV compliance)
		r.Route("/bio-safety", func(r chi.Router) {
			r.Post("/assessments", biosafetyHandler.CreateAssessment)
			r.Get("/items/{itemId}/assessments", biosafetyHandler.ListAssessmentsByItem)
			r.Put("/assessments/{id}", biosafetyHandler.UpdateAssessment)
			r.Delete("/assessments/{id}", biosafetyHandler.DeleteAssessment)
			r.Get("/trainings", biosafetyHandler.ListTrainings)
			r.Post("/trainings", biosafetyHandler.CreateTraining)
			r.Put("/trainings/{id}", biosafetyHandler.UpdateTraining)
			r.Delete("/trainings/{id}", biosafetyHandler.DeleteTraining)
		})

		// Retention policy routes
		r.Get("/retention-policies", retentionHandler.List)
		r.Post("/retention-policies", retentionHandler.Create)
		r.Put("/retention-policies/{id}", retentionHandler.Update)
		r.Delete("/retention-policies/{id}", retentionHandler.Delete)

		// Reprocessing / Sterilization routes (KRINKO compliance)
		r.Route("/sterilization/batches", func(r chi.Router) {
			r.Get("/", reprocessingHandler.ListBatches)
			r.Post("/", reprocessingHandler.CreateBatch)
			r.Get("/{id}", reprocessingHandler.GetBatch)
			r.Put("/{id}", reprocessingHandler.UpdateBatch)
			r.Delete("/{id}", reprocessingHandler.DeleteBatch)
		})
		r.Route("/reprocessing/items/{itemId}/cycles", func(r chi.Router) {
			r.Get("/", reprocessingHandler.ListCyclesByItem)
			r.Post("/", reprocessingHandler.CreateCycle)
		})
		r.Put("/reprocessing/cycles/{id}", reprocessingHandler.UpdateCycle)
		r.Delete("/reprocessing/cycles/{id}", reprocessingHandler.DeleteCycle)

		// Hygiene routes (IfSG compliance)
		r.Route("/hygiene", func(r chi.Router) {
			r.Route("/plans", func(r chi.Router) {
				r.Get("/", hygieneHandler.ListPlans)
				r.Post("/", hygieneHandler.CreatePlan)
				r.Get("/{id}", hygieneHandler.GetPlan)
				r.Put("/{id}", hygieneHandler.UpdatePlan)
				r.Delete("/{id}", hygieneHandler.DeletePlan)
			})
			r.Route("/inspections", func(r chi.Router) {
				r.Get("/", hygieneHandler.ListInspections)
				r.Post("/", hygieneHandler.CreateInspection)
				r.Get("/{id}", hygieneHandler.GetInspection)
				r.Put("/{id}", hygieneHandler.UpdateInspection)
				r.Delete("/{id}", hygieneHandler.DeleteInspection)
			})
		})

		// Radiation protection routes (StrlSchV/RoV compliance)
		r.Route("/radiation", func(r chi.Router) {
			r.Route("/devices", func(r chi.Router) {
				r.Get("/", radiationHandler.ListDevices)
				r.Post("/", radiationHandler.CreateDevice)
				r.Get("/{id}", radiationHandler.GetDevice)
				r.Put("/{id}", radiationHandler.UpdateDevice)
				r.Delete("/{id}", radiationHandler.DeleteDevice)
				r.Get("/{deviceId}/constancy-tests", radiationHandler.ListTests)
				r.Post("/{deviceId}/constancy-tests", radiationHandler.CreateTest)
				r.Get("/{deviceId}/expert-inspections", radiationHandler.ListExpertInspections)
				r.Post("/{deviceId}/expert-inspections", radiationHandler.CreateExpertInspection)
			})
			r.Get("/certifications", radiationHandler.ListCertifications)
			r.Post("/certifications", radiationHandler.CreateCertification)
			r.Put("/certifications/{id}", radiationHandler.UpdateCertification)
			r.Delete("/certifications/{id}", radiationHandler.DeleteCertification)
			r.Get("/dosimetry", radiationHandler.ListAllDosimetry)
			r.Post("/dosimetry", radiationHandler.CreateDosimetryRecord)
			r.Get("/dosimetry/employee/{employeeId}", radiationHandler.ListDosimetryByEmployee)
		})

		// Compliance export routes
		r.Get("/export/gobd", auditHandler.ExportGoBD)
		r.Get("/export/data-portability", dataPortabilityHandler.ExportDataPortability)
	})

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server
	go func() {
		log.Info().Str("addr", addr).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server")

	// Cancel context to stop consumers and scheduler
	cancel()

	// Stop the alert scheduler
	alertScheduler.Stop()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("server stopped")
}
