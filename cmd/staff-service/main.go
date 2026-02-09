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
	"github.com/medflow/medflow-backend/internal/staff/client"
	"github.com/medflow/medflow-backend/internal/staff/consumers"
	"github.com/medflow/medflow-backend/internal/staff/events"
	"github.com/medflow/medflow-backend/internal/staff/handler"
	"github.com/medflow/medflow-backend/internal/staff/repository"
	"github.com/medflow/medflow-backend/internal/staff/service"
	"github.com/medflow/medflow-backend/internal/staff/validation"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

func main() {
	// Load configuration with validation (fails fast in production if required config is missing)
	cfg, err := config.LoadWithValidation("staff-service")
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("staff-service", cfg.Server.Environment)
	log.Info().Msg("starting Staff Service")

	// Connect to database
	db, err := database.New(&cfg.Database, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Connect to RabbitMQ
	rmq, err := messaging.New(&cfg.RabbitMQ, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to RabbitMQ")
	}
	defer rmq.Close()

	// Initialize event publisher
	publisher, err := events.NewStaffEventPublisher(rmq, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create event publisher")
	}

	// Initialize repositories
	employeeRepo := repository.NewEmployeeRepository(db)
	userCacheRepo := repository.NewUserCacheRepository(db)
	shiftRepo := repository.NewShiftRepository(db)
	absenceRepo := repository.NewAbsenceRepository(db)
	timeTrackingRepo := repository.NewTimeTrackingRepository(db)
	complianceRepo := repository.NewComplianceRepository(db)

	// Initialize validators
	germanValidator := validation.NewGermanValidator()

	// Initialize services
	staffService := service.NewStaffService(employeeRepo, publisher, germanValidator, log)
	shiftService := service.NewShiftService(shiftRepo, publisher, log)
	absenceService := service.NewAbsenceService(absenceRepo, publisher, log)
	complianceService := service.NewComplianceService(complianceRepo, timeTrackingRepo, shiftRepo, log)
	timeTrackingService := service.NewTimeTrackingService(timeTrackingRepo, complianceService, publisher, log)

	// Initialize user service client for creating user accounts
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8082" // Default to user service port
	}
	userClient := client.NewUserClient(userServiceURL, log)

	// Set user client on staff service for credential management
	staffService.SetUserClient(userClient)

	// Initialize handlers
	employeeHandler := handler.NewEmployeeHandler(staffService, userClient, log)
	validationHandler := handler.NewValidationHandler(germanValidator, log)
	shiftHandler := handler.NewShiftHandler(shiftService, log)
	absenceHandler := handler.NewAbsenceHandler(absenceService, log)
	timeTrackingHandler := handler.NewTimeTrackingHandler(timeTrackingService, staffService, log)
	complianceHandler := handler.NewComplianceHandler(complianceService, staffService, log)

	// Start user event consumer
	userConsumer, err := consumers.NewUserEventConsumer(rmq, userCacheRepo, employeeRepo, staffService, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create user event consumer")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := userConsumer.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start user event consumer")
	}

	// Start periodic compliance checker (ArbZG monitoring)
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := complianceService.CheckAllActiveEmployees(ctx); err != nil {
					log.Error().Err(err).Msg("periodic compliance check failed")
				}
			}
		}
	}()

	// Create router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))
	r.Use(httputil.TenantMiddleware) // Tenant middleware with /health exception

	// Health check (no tenant required - handled by middleware)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"status":   "healthy",
			"service":  "staff-service",
			"database": db.Health(r.Context()),
			"rabbitmq": rmq.Health(),
		})
	})

	// API routes (tenant required)
	r.Route("/api/v1/staff", func(r chi.Router) {
		// Employee routes
		r.Route("/employees", func(r chi.Router) {
			r.Get("/", employeeHandler.List)
			r.Post("/", employeeHandler.Create)
			r.Get("/{id}", employeeHandler.Get)
			r.Put("/{id}", employeeHandler.Update)
			r.Delete("/{id}", employeeHandler.Delete)
			r.Put("/{id}/personal", employeeHandler.UpdatePersonal)
			r.Put("/{id}/contact", employeeHandler.UpdateContact)
			r.Put("/{id}/financials", employeeHandler.UpdateFinancials)
			r.Get("/{id}/files", employeeHandler.ListFiles)
			r.Post("/{id}/files", employeeHandler.UploadFile)
			r.Delete("/{id}/files/{fileId}", employeeHandler.DeleteFile)

			// Credential management endpoints
			r.Route("/{id}/credentials", func(r chi.Router) {
				r.Post("/", employeeHandler.AddCredentials)
				r.Delete("/", employeeHandler.RemoveCredentials)
				r.Get("/", employeeHandler.GetCredentialStatus)
			})
		})

		// Shift Template routes
		r.Route("/templates", func(r chi.Router) {
			r.Get("/", shiftHandler.ListTemplates)
			r.Post("/", shiftHandler.CreateTemplate)
			r.Get("/{id}", shiftHandler.GetTemplate)
			r.Put("/{id}", shiftHandler.UpdateTemplate)
			r.Delete("/{id}", shiftHandler.DeleteTemplate)
		})

		// Shift Assignment routes
		r.Route("/shifts", func(r chi.Router) {
			r.Get("/", shiftHandler.List)
			r.Post("/", shiftHandler.Create)
			r.Post("/bulk", shiftHandler.BulkCreate)
			r.Get("/{id}", shiftHandler.Get)
			r.Put("/{id}", shiftHandler.Update)
			r.Delete("/{id}", shiftHandler.Delete)
		})

		// Absence routes
		r.Route("/absences", func(r chi.Router) {
			r.Get("/", absenceHandler.List)
			r.Post("/", absenceHandler.Create)
			r.Get("/{id}", absenceHandler.Get)
			r.Put("/{id}", absenceHandler.Update)
			r.Delete("/{id}", absenceHandler.Delete)
			r.Put("/{id}/approve", absenceHandler.Approve)
			r.Put("/{id}/reject", absenceHandler.Reject)
		})

		// Vacation info routes
		r.Get("/vacation-info", absenceHandler.ListVacationBalances)

		// Employee-specific scheduling routes (nested under employee ID)
		r.Route("/{employeeId}", func(r chi.Router) {
			r.Get("/shifts", shiftHandler.GetEmployeeShifts)
			r.Get("/absences", absenceHandler.GetEmployeeAbsences)
			r.Get("/vacation-info", absenceHandler.GetEmployeeVacationBalance)
			r.Put("/vacation-info", absenceHandler.SetVacationEntitlement)
		})

		// Validation routes
		r.Post("/validate/iban", validationHandler.ValidateIBAN)
		r.Post("/validate/tax-id", validationHandler.ValidateTaxID)
		r.Post("/validate/sv-number", validationHandler.ValidateSVNumber)
	})

	// Time Tracking routes (under /api/v1/time-tracking)
	r.Route("/api/v1/time-tracking", func(r chi.Router) {
		// Current user's status (for PersonalClockBar)
		r.Get("/my-status", timeTrackingHandler.GetMyStatus)

		// Status and entries
		r.Get("/statuses", timeTrackingHandler.GetAllStatuses)
		r.Get("/entries", timeTrackingHandler.GetEntriesByDate)
		r.Patch("/entries/{id}", timeTrackingHandler.UpdateEntry)
		r.Patch("/entries/{id}/breaks", timeTrackingHandler.UpdateEntryBreaks)
		r.Delete("/entries/{id}", timeTrackingHandler.DeleteEntry)

		// Corrections
		r.Post("/corrections", timeTrackingHandler.CreateCorrection)

		// Employee-specific time tracking
		r.Route("/employees/{id}", func(r chi.Router) {
			r.Post("/clock-in", timeTrackingHandler.ClockIn)
			r.Post("/clock-out", timeTrackingHandler.ClockOut)
			r.Post("/break/start", timeTrackingHandler.StartBreak)
			r.Post("/break/end", timeTrackingHandler.EndBreak)
			r.Post("/manual-clock-in", timeTrackingHandler.ManualClockIn)
			r.Post("/manual-clock-out", timeTrackingHandler.ManualClockOut)
			r.Get("/history", timeTrackingHandler.GetEmployeeHistory)
			r.Get("/corrections", timeTrackingHandler.GetEmployeeCorrections)
		})
	})

	// Compliance routes (ArbZG - German Labor Law)
	r.Route("/api/v1/compliance", func(r chi.Router) {
		// Break validation
		r.Get("/break/check", complianceHandler.CheckBreakEnd)
		r.Get("/employees/{id}/break/check", complianceHandler.CheckBreakEndForEmployee)

		// Clock out compliance check
		r.Get("/clock-out/check", complianceHandler.CheckClockOut)

		// Shift validation
		r.Post("/shifts/validate", complianceHandler.ValidateShift)

		// Alerts (manager view)
		r.Get("/alerts", complianceHandler.GetActiveAlerts)
		r.Post("/alerts/{id}/dismiss", complianceHandler.DismissAlert)

		// Violations (manager view)
		r.Get("/violations", complianceHandler.GetViolations)
		r.Post("/violations/{id}/acknowledge", complianceHandler.AcknowledgeViolation)

		// Time correction requests
		r.Post("/correction-requests", complianceHandler.CreateCorrectionRequest)
		r.Get("/correction-requests/my", complianceHandler.GetMyCorrectionRequests)
		r.Get("/correction-requests/pending", complianceHandler.GetPendingCorrectionRequests)
		r.Get("/correction-requests/{id}", complianceHandler.GetCorrectionRequest)
		r.Post("/correction-requests/{id}/approve", complianceHandler.ApproveCorrectionRequest)
		r.Post("/correction-requests/{id}/reject", complianceHandler.RejectCorrectionRequest)

		// Settings (admin)
		r.Get("/settings", complianceHandler.GetSettings)
		r.Put("/settings", complianceHandler.UpdateSettings)

		// Manual compliance check trigger
		r.Post("/check-all", complianceHandler.RunComplianceCheck)
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

	// Cancel context to stop consumers
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("server stopped")
}
