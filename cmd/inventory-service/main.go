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
	publisher, err := events.NewInventoryEventPublisher(rmq, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create event publisher")
	}

	// Initialize repositories
	locationRepo := repository.NewLocationRepository(db)
	itemRepo := repository.NewItemRepository(db)
	batchRepo := repository.NewBatchRepository(db)
	alertRepo := repository.NewAlertRepository(db)
	userCacheRepo := repository.NewUserCacheRepository(db)

	// Initialize service
	inventoryService := service.NewInventoryService(locationRepo, itemRepo, batchRepo, alertRepo, publisher, log)

	// Initialize handlers
	locationHandler := handler.NewLocationHandler(locationRepo, log)
	itemHandler := handler.NewItemHandler(inventoryService, log)
	batchHandler := handler.NewBatchHandler(inventoryService, log)
	alertHandler := handler.NewAlertHandler(alertRepo, log)
	dashboardHandler := handler.NewDashboardHandler(inventoryService, log)

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

	// Middleware
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))
	r.Use(httputil.TenantMiddleware) // Extract tenant context from headers

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]interface{}{
			"status":   "healthy",
			"service":  "inventory-service",
			"database": db.Health(r.Context()),
			"rabbitmq": rmq.Health(),
		})
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
			})
			r.Route("/shelves", func(r chi.Router) {
				r.Get("/", locationHandler.ListShelves)
				r.Post("/", locationHandler.CreateShelf)
				r.Get("/{id}", locationHandler.GetShelf)
				r.Put("/{id}", locationHandler.UpdateShelf)
				r.Delete("/{id}", locationHandler.DeleteShelf)
			})
		})

		// Item routes
		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemHandler.List)
			r.Post("/", itemHandler.Create)
			r.Get("/{id}", itemHandler.Get)
			r.Put("/{id}", itemHandler.Update)
			r.Delete("/{id}", itemHandler.Delete)
			r.Get("/{id}/batches", batchHandler.ListByItem)
			r.Post("/{id}/batches", batchHandler.Create)
		})

		// Batch routes
		r.Route("/batches", func(r chi.Router) {
			r.Get("/{id}", batchHandler.Get)
			r.Put("/{id}", batchHandler.Update)
			r.Delete("/{id}", batchHandler.Delete)
			r.Post("/{id}/adjust", batchHandler.AdjustStock)
		})

		// Alert routes
		r.Get("/alerts", alertHandler.List)
		r.Put("/alerts/{id}/acknowledge", alertHandler.Acknowledge)

		// Dashboard
		r.Get("/dashboard/stats", dashboardHandler.GetStats)
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
