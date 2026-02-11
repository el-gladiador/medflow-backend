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
	"github.com/medflow/medflow-backend/internal/user/events"
	"github.com/medflow/medflow-backend/internal/user/handler"
	"github.com/medflow/medflow-backend/internal/user/repository"
	"github.com/medflow/medflow-backend/internal/user/service"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

func main() {
	// Load configuration with validation (fails fast in production if required config is missing)
	cfg, err := config.LoadWithValidation("user-service")
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("user-service", cfg.Server.Environment)
	log.Info().Msg("starting User Service")

	// Connect to database (single Supabase DB, search_path = users, public)
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
	var publisher *events.UserEventPublisher
	if rmq != nil {
		var err error
		publisher, err = events.NewUserEventPublisher(rmq, log)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create event publisher")
		}
	} else {
		log.Warn().Msg("event publishing disabled (no RabbitMQ)")
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	// Initialize services
	userService := service.NewUserService(userRepo, roleRepo, auditRepo, publisher, log)

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService, log)
	roleHandler := handler.NewRoleHandler(roleRepo, log)
	auditHandler := handler.NewAuditHandler(auditRepo, log)

	// Create router
	r := chi.NewRouter()

	// Global middleware (no tenant required)
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))

	// Health check (no tenant required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":   "healthy",
			"service":  "user-service",
			"database": db.Health(r.Context()),
		}
		if rmq != nil {
			health["rabbitmq"] = rmq.Health()
		} else {
			health["rabbitmq"] = map[string]string{"status": "disabled"}
		}
		httputil.JSON(w, http.StatusOK, health)
	})

	// Internal endpoints (no tenant required - used during login to find tenant)
	r.Route("/api/v1/internal", func(r chi.Router) {
		r.Post("/validate-credentials", userHandler.ValidateCredentials)
		r.Get("/users/{id}", userHandler.GetUserInternal)
	})

	// Protected API endpoints (tenant required)
	r.Route("/api/v1", func(r chi.Router) {
		// Apply tenant middleware to all protected routes
		r.Use(httputil.TenantMiddleware)
		// Users
		r.Route("/users", func(r chi.Router) {
			r.Get("/", userHandler.List)
			r.Post("/", userHandler.Create)
			r.Get("/{id}", userHandler.Get)
			r.Put("/{id}", userHandler.Update)
			r.Delete("/{id}", userHandler.Delete)
			r.Patch("/{id}/role", userHandler.ChangeRole)
			r.Get("/{id}/permissions", userHandler.GetPermissions)
			r.Post("/{id}/permissions", userHandler.GrantPermission)
			r.Delete("/{id}/permissions", userHandler.RevokePermission)
			r.Post("/{id}/access-giver", userHandler.GrantAccessGiver)
			r.Delete("/{id}/access-giver", userHandler.RevokeAccessGiver)
		})

		// Roles
		r.Route("/roles", func(r chi.Router) {
			r.Get("/", roleHandler.List)
			r.Get("/{id}", roleHandler.Get)
		})

		// Audit logs
		r.Get("/audit", auditHandler.List)
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

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("server stopped")
}
