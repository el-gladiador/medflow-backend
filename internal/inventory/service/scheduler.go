package service

import (
	"context"
	"time"

	"github.com/medflow/medflow-backend/pkg/database"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/tenant"
)

// AlertScheduler runs alert scans periodically across all tenants.
// It queries public.tenants for active tenants and runs scans with each tenant's context.
type AlertScheduler struct {
	scanner  *AlertScanner
	db       *database.DB
	interval time.Duration
	logger   *logger.Logger
	cancel   context.CancelFunc
}

// NewAlertScheduler creates a new alert scheduler
func NewAlertScheduler(scanner *AlertScanner, db *database.DB, interval time.Duration, log *logger.Logger) *AlertScheduler {
	return &AlertScheduler{
		scanner:  scanner,
		db:       db,
		interval: interval,
		logger:   log,
	}
}

// Start starts the scheduler in a background goroutine.
// On each tick it queries all active tenants and runs ScanAll for each.
func (s *AlertScheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	go func() {
		s.logger.Info().Dur("interval", s.interval).Msg("alert scheduler started")

		// Run an initial scan immediately
		s.runScanCycle(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info().Msg("alert scheduler stopped")
				return
			case <-ticker.C:
				s.runScanCycle(ctx)
			}
		}
	}()
}

// Stop stops the scheduler goroutine
func (s *AlertScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// runScanCycle queries all active tenants and runs scans for each
func (s *AlertScheduler) runScanCycle(ctx context.Context) {
	start := time.Now()
	s.logger.Info().Msg("starting alert scan cycle")

	tenantIDs, err := s.getActiveTenantIDs(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to query active tenants")
		return
	}

	s.logger.Info().Int("tenant_count", len(tenantIDs)).Msg("scanning tenants for alerts")

	for _, tenantID := range tenantIDs {
		// Create a tenant-scoped context for this scan
		tenantCtx := tenant.WithTenantID(ctx, tenantID)

		if err := s.scanner.ScanAll(tenantCtx); err != nil {
			s.logger.Error().Err(err).Str("tenant_id", tenantID).Msg("alert scan failed for tenant")
		}
	}

	s.logger.Info().
		Dur("duration", time.Since(start)).
		Int("tenant_count", len(tenantIDs)).
		Msg("alert scan cycle completed")
}

// getActiveTenantIDs queries all active tenant IDs from public.tenants.
// This is a direct query on the public schema which has no RLS, so no
// tenant context is needed.
func (s *AlertScheduler) getActiveTenantIDs(ctx context.Context) ([]string, error) {
	var tenantIDs []string
	query := `SELECT id FROM public.tenants WHERE is_active = TRUE`
	err := s.db.DB.SelectContext(ctx, &tenantIDs, query)
	if err != nil {
		return nil, err
	}
	return tenantIDs, nil
}
