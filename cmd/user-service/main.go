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

// Frontend URL for invitation links
const frontendURL = "http://localhost:3000"

func main() {
	// Load configuration
	cfg, err := config.Load("user-service")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("user-service", cfg.Server.Environment)
	log.Info().Msg("starting User Service")

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
	publisher, err := events.NewUserEventPublisher(rmq, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create event publisher")
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	inviteRepo := repository.NewInvitationRepository(db)

	// Initialize services
	userService := service.NewUserService(userRepo, roleRepo, auditRepo, publisher, log)
	inviteService := service.NewInvitationService(inviteRepo, userRepo, roleRepo, auditRepo, publisher, log, frontendURL)

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService, log)
	roleHandler := handler.NewRoleHandler(roleRepo, log)
	auditHandler := handler.NewAuditHandler(auditRepo, log)
	inviteHandler := handler.NewInvitationHandler(inviteService, log)

	// Create router
	r := chi.NewRouter()

	// Global middleware (no tenant required)
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))

	// Health check (no tenant required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"status":   "healthy",
			"service":  "user-service",
			"database": db.Health(r.Context()),
			"rabbitmq": rmq.Health(),
		})
	})

	// Internal endpoints (no tenant required - used during login to find tenant)
	r.Route("/api/v1/internal", func(r chi.Router) {
		r.Post("/validate-credentials", userHandler.ValidateCredentials)
		r.Get("/users/{id}", userHandler.GetUserInternal)
	})

	// Public endpoints (no tenant required)
	r.Route("/api/v1/public", func(r chi.Router) {
		// Get invitation info by token (for acceptance page)
		r.Get("/invitations/token/{token}", inviteHandler.GetByToken)
		// Accept invitation (creates user)
		r.Post("/invitations/token/{token}/accept", inviteHandler.Accept)
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

		// Invitations (authenticated endpoints)
		r.Route("/invitations", func(r chi.Router) {
			r.Get("/", inviteHandler.List)
			r.Post("/", inviteHandler.Create)
			r.Get("/{id}", inviteHandler.Get)
			r.Post("/{id}/revoke", inviteHandler.Revoke)
			r.Post("/{id}/resend", inviteHandler.Resend)
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
