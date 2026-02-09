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
	"github.com/go-chi/cors"
	"github.com/medflow/medflow-backend/internal/gateway"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/httputil"
	"github.com/medflow/medflow-backend/pkg/i18n"
	"github.com/medflow/medflow-backend/pkg/logger"
)

func main() {
	// Load configuration with validation (fails fast in production if required config is missing)
	cfg, err := config.LoadWithValidation("api-gateway")
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("api-gateway", cfg.Server.Environment)
	log.Info().Msg("starting API Gateway")

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RealIP)
	r.Use(httputil.RequestID)
	r.Use(httputil.Logger(log))
	r.Use(httputil.Recoverer(log))
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS - supports subdomain-based multi-tenancy
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			// Allow localhost variations (development)
			// Matches: http://localhost:3000, http://demo-clinic.localhost:3000, etc.
			if origin == "http://localhost:3000" || origin == "http://localhost:5173" {
				return true
			}
			// Allow *.localhost:3000 subdomains for development
			if len(origin) > 21 && origin[len(origin)-15:] == ".localhost:3000" {
				return true
			}
			// Allow *.medflow.de for production
			if len(origin) > 11 && origin[len(origin)-11:] == ".medflow.de" {
				return true
			}
			// Allow medflow.de itself
			if origin == "https://medflow.de" || origin == "http://medflow.de" {
				return true
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "Accept-Language"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// i18n middleware - extract locale from Accept-Language header
	r.Use(i18n.Middleware)

	// Create proxy handler
	proxy := gateway.NewProxy(cfg, log)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.JSON(w, http.StatusOK, map[string]string{
			"status":  "healthy",
			"service": "api-gateway",
		})
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", proxy.ForwardToAuth)
			r.Post("/refresh", proxy.ForwardToAuth)

			// Protected auth routes
			r.Group(func(r chi.Router) {
				r.Use(proxy.AuthMiddleware)
				r.Post("/logout", proxy.ForwardToAuth)
				r.Get("/me", proxy.ForwardToAuth)
			})
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(proxy.AuthMiddleware)

			// User routes
			r.Route("/users", func(r chi.Router) {
				r.Get("/", proxy.ForwardToUsers)
				r.Post("/", proxy.ForwardToUsers)
				r.Get("/{id}", proxy.ForwardToUsers)
				r.Put("/{id}", proxy.ForwardToUsers)
				r.Delete("/{id}", proxy.ForwardToUsers)
				r.Patch("/{id}/role", proxy.ForwardToUsers)
				r.Get("/{id}/permissions", proxy.ForwardToUsers)
				r.Post("/{id}/permissions", proxy.ForwardToUsers)
				r.Delete("/{id}/permissions", proxy.ForwardToUsers)
				r.Post("/{id}/access-giver", proxy.ForwardToUsers)
				r.Delete("/{id}/access-giver", proxy.ForwardToUsers)
			})

			// Roles routes
			r.Route("/roles", func(r chi.Router) {
				r.Get("/", proxy.ForwardToUsers)
				r.Get("/{id}", proxy.ForwardToUsers)
			})

			// Audit routes
			r.Get("/audit", proxy.ForwardToUsers)

			// Staff routes
			r.Route("/staff", func(r chi.Router) {
				// Employee routes
				r.Route("/employees", func(r chi.Router) {
					r.Get("/", proxy.ForwardToStaff)
					r.Post("/", proxy.ForwardToStaff)
					r.Get("/{id}", proxy.ForwardToStaff)
					r.Put("/{id}", proxy.ForwardToStaff)
					r.Delete("/{id}", proxy.ForwardToStaff)
					r.Put("/{id}/personal", proxy.ForwardToStaff)
					r.Put("/{id}/contact", proxy.ForwardToStaff)
					r.Put("/{id}/financials", proxy.ForwardToStaff)
					r.Get("/{id}/files", proxy.ForwardToStaff)
					r.Post("/{id}/files", proxy.ForwardToStaff)
					r.Delete("/{id}/files/{fileId}", proxy.ForwardToStaff)
				})

				// Shift Template routes
				r.Route("/templates", func(r chi.Router) {
					r.Get("/", proxy.ForwardToStaff)
					r.Post("/", proxy.ForwardToStaff)
					r.Get("/{id}", proxy.ForwardToStaff)
					r.Put("/{id}", proxy.ForwardToStaff)
					r.Delete("/{id}", proxy.ForwardToStaff)
				})

				// Shift Assignment routes
				r.Route("/shifts", func(r chi.Router) {
					r.Get("/", proxy.ForwardToStaff)
					r.Post("/", proxy.ForwardToStaff)
					r.Post("/bulk", proxy.ForwardToStaff)
					r.Get("/{id}", proxy.ForwardToStaff)
					r.Put("/{id}", proxy.ForwardToStaff)
					r.Delete("/{id}", proxy.ForwardToStaff)
				})

				// Absence routes
				r.Route("/absences", func(r chi.Router) {
					r.Get("/", proxy.ForwardToStaff)
					r.Post("/", proxy.ForwardToStaff)
					r.Get("/{id}", proxy.ForwardToStaff)
					r.Put("/{id}", proxy.ForwardToStaff)
					r.Delete("/{id}", proxy.ForwardToStaff)
					r.Put("/{id}/approve", proxy.ForwardToStaff)
					r.Put("/{id}/reject", proxy.ForwardToStaff)
				})

				// Vacation info routes
				r.Get("/vacation-info", proxy.ForwardToStaff)

				// Employee-specific scheduling routes
				r.Route("/{employeeId}", func(r chi.Router) {
					r.Get("/shifts", proxy.ForwardToStaff)
					r.Get("/absences", proxy.ForwardToStaff)
					r.Get("/vacation-info", proxy.ForwardToStaff)
					r.Put("/vacation-info", proxy.ForwardToStaff)
				})

				// Validation routes
				r.Post("/validate/iban", proxy.ForwardToStaff)
				r.Post("/validate/tax-id", proxy.ForwardToStaff)
				r.Post("/validate/sv-number", proxy.ForwardToStaff)
			})

			// Time Tracking routes
			r.Route("/time-tracking", func(r chi.Router) {
				// Current user's status (for PersonalClockBar / StempelButton)
				r.Get("/my-status", proxy.ForwardToStaff)

				// Status and entries
				r.Get("/statuses", proxy.ForwardToStaff)
				r.Get("/entries", proxy.ForwardToStaff)
				r.Patch("/entries/{id}", proxy.ForwardToStaff)
				r.Patch("/entries/{id}/breaks", proxy.ForwardToStaff)
				r.Delete("/entries/{id}", proxy.ForwardToStaff)

				// Corrections
				r.Post("/corrections", proxy.ForwardToStaff)

				// Employee-specific time tracking
				r.Route("/employees/{id}", func(r chi.Router) {
					r.Post("/clock-in", proxy.ForwardToStaff)
					r.Post("/clock-out", proxy.ForwardToStaff)
					r.Post("/break/start", proxy.ForwardToStaff)
					r.Post("/break/end", proxy.ForwardToStaff)
					r.Post("/manual-clock-in", proxy.ForwardToStaff)
					r.Post("/manual-clock-out", proxy.ForwardToStaff)
					r.Get("/history", proxy.ForwardToStaff)
					r.Get("/corrections", proxy.ForwardToStaff)
				})
			})

			// Inventory routes
			r.Route("/inventory", func(r chi.Router) {
				// Location routes
				r.Route("/locations", func(r chi.Router) {
					r.Get("/tree", proxy.ForwardToInventory)
					r.Route("/rooms", func(r chi.Router) {
						r.Get("/", proxy.ForwardToInventory)
						r.Post("/", proxy.ForwardToInventory)
						r.Get("/{id}", proxy.ForwardToInventory)
						r.Put("/{id}", proxy.ForwardToInventory)
						r.Delete("/{id}", proxy.ForwardToInventory)
					})
					r.Route("/cabinets", func(r chi.Router) {
						r.Get("/", proxy.ForwardToInventory)
						r.Post("/", proxy.ForwardToInventory)
						r.Get("/{id}", proxy.ForwardToInventory)
						r.Put("/{id}", proxy.ForwardToInventory)
						r.Delete("/{id}", proxy.ForwardToInventory)
					})
					r.Route("/shelves", func(r chi.Router) {
						r.Get("/", proxy.ForwardToInventory)
						r.Post("/", proxy.ForwardToInventory)
						r.Get("/{id}", proxy.ForwardToInventory)
						r.Put("/{id}", proxy.ForwardToInventory)
						r.Delete("/{id}", proxy.ForwardToInventory)
					})
				})

				// Item routes
				r.Route("/items", func(r chi.Router) {
					r.Get("/", proxy.ForwardToInventory)
					r.Post("/", proxy.ForwardToInventory)
					r.Get("/{id}", proxy.ForwardToInventory)
					r.Put("/{id}", proxy.ForwardToInventory)
					r.Delete("/{id}", proxy.ForwardToInventory)
					r.Get("/{id}/batches", proxy.ForwardToInventory)
					r.Post("/{id}/batches", proxy.ForwardToInventory)
				})

				// Batch routes
				r.Route("/batches", func(r chi.Router) {
					r.Get("/{id}", proxy.ForwardToInventory)
					r.Put("/{id}", proxy.ForwardToInventory)
					r.Delete("/{id}", proxy.ForwardToInventory)
					r.Post("/{id}/adjust", proxy.ForwardToInventory)
				})

				// Alerts
				r.Get("/alerts", proxy.ForwardToInventory)
				r.Put("/alerts/{id}/acknowledge", proxy.ForwardToInventory)

				// Dashboard
				r.Get("/dashboard/stats", proxy.ForwardToInventory)
			})
		})
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
