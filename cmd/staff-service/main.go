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
	// Load configuration
	cfg, err := config.Load("staff-service")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
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

	// Initialize repository
	employeeRepo := repository.NewEmployeeRepository(db)
	userCacheRepo := repository.NewUserCacheRepository(db)

	// Initialize validators
	germanValidator := validation.NewGermanValidator()

	// Initialize service
	staffService := service.NewStaffService(employeeRepo, publisher, germanValidator, log)

	// Initialize handlers
	employeeHandler := handler.NewEmployeeHandler(staffService, log)
	validationHandler := handler.NewValidationHandler(germanValidator, log)

	// Start user event consumer
	userConsumer, err := consumers.NewUserEventConsumer(rmq, userCacheRepo, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create user event consumer")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := userConsumer.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start user event consumer")
	}

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
		})

		// Validation routes
		r.Post("/validate/iban", validationHandler.ValidateIBAN)
		r.Post("/validate/tax-id", validationHandler.ValidateTaxID)
		r.Post("/validate/sv-number", validationHandler.ValidateSVNumber)
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
