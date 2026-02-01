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
	"github.com/medflow/medflow-backend/internal/auth/consumers"
	"github.com/medflow/medflow-backend/internal/auth/handler"
	"github.com/medflow/medflow-backend/internal/auth/jwt"
	"github.com/medflow/medflow-backend/internal/auth/repository"
	"github.com/medflow/medflow-backend/internal/auth/service"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

func main() {
	// Load configuration with validation (fails fast in production if required config is missing)
	cfg, err := config.LoadWithValidation("auth-service")
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("auth-service", cfg.Server.Environment)
	log.Info().Msg("starting Auth Service")

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

	// Initialize repositories
	jwtManager := jwt.NewManager(&cfg.JWT)
	sessionRepo := repository.NewSessionRepository(db)
	lookupRepo := repository.NewUserTenantLookupRepository(db)

	// Initialize service
	authService := service.NewAuthService(sessionRepo, lookupRepo, jwtManager, cfg, log)
	authHandler := handler.NewAuthHandler(authService, log)

	// Initialize and start user event consumer for lookup table sync
	userConsumer, err := consumers.NewUserEventConsumer(rmq, lookupRepo, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create user event consumer")
	}

	// Create cancellable context for consumer
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	if err := userConsumer.Start(consumerCtx); err != nil {
		log.Fatal().Err(err).Msg("failed to start user event consumer")
	}
	log.Info().Msg("user event consumer started for lookup table sync")

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"status":   "healthy",
			"service":  "auth-service",
			"database": db.Health(r.Context()),
			"rabbitmq": rmq.Health(),
		})
	})

	// Auth routes
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)
		r.Post("/refresh", authHandler.Refresh)
		r.Get("/me", authHandler.Me)
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
