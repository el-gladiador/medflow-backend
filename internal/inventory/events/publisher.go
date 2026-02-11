package events

import (
	"context"

	"github.com/medflow/medflow-backend/internal/inventory/repository"
	"github.com/medflow/medflow-backend/pkg/logger"
	"github.com/medflow/medflow-backend/pkg/messaging"
)

// InventoryEventPublisher publishes inventory-related events
type InventoryEventPublisher struct {
	publisher *messaging.Publisher
	logger    *logger.Logger
}

// NewInventoryEventPublisher creates a new inventory event publisher
func NewInventoryEventPublisher(rmq *messaging.RabbitMQ, log *logger.Logger) (*InventoryEventPublisher, error) {
	publisher, err := messaging.NewPublisher(rmq, messaging.ExchangeInventoryEvents, "inventory-service", log)
	if err != nil {
		return nil, err
	}

	return &InventoryEventPublisher{
		publisher: publisher,
		logger:    log,
	}, nil
}

// PublishStockAdjusted publishes a stock adjusted event
func (p *InventoryEventPublisher) PublishStockAdjusted(ctx context.Context, adj *repository.StockAdjustment) {
	if p == nil { return }
	batchID := ""
	if adj.BatchID != nil {
		batchID = *adj.BatchID
	}

	reason := ""
	if adj.Reason != nil {
		reason = *adj.Reason
	}

	data := messaging.StockAdjustedEvent{
		ItemID:      adj.ItemID,
		BatchID:     batchID,
		Adjustment:  adj.Quantity,
		NewQuantity: adj.NewQuantity,
		PerformedBy: adj.PerformedBy,
		Reason:      reason,
	}

	if err := p.publisher.Publish(ctx, messaging.EventStockAdjusted, data); err != nil {
		p.logger.Error().Err(err).Str("item_id", adj.ItemID).Msg("failed to publish stock adjusted event")
	}
}

// PublishAlertGenerated publishes an alert generated event
func (p *InventoryEventPublisher) PublishAlertGenerated(ctx context.Context, alert *repository.InventoryAlert) {
	if p == nil { return }
	batchID := ""
	if alert.BatchID != nil {
		batchID = *alert.BatchID
	}

	data := messaging.AlertGeneratedEvent{
		AlertID:   alert.ID,
		AlertType: alert.AlertType,
		Severity:  alert.Severity,
		Message:   alert.Message,
		ItemID:    alert.ItemID,
		BatchID:   batchID,
	}

	if err := p.publisher.Publish(ctx, messaging.EventAlertGenerated, data); err != nil {
		p.logger.Error().Err(err).Str("alert_id", alert.ID).Msg("failed to publish alert generated event")
	}
}
