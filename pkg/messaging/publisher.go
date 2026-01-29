package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// Publisher handles publishing events to RabbitMQ
type Publisher struct {
	channel  *amqp.Channel
	exchange string
	source   string
	logger   *logger.Logger
}

// NewPublisher creates a new publisher for the given exchange
func NewPublisher(rmq *RabbitMQ, exchange, source string, log *logger.Logger) (*Publisher, error) {
	// Declare the exchange
	if err := rmq.DeclareExchange(exchange); err != nil {
		return nil, fmt.Errorf("failed to declare exchange %s: %w", exchange, err)
	}

	return &Publisher{
		channel:  rmq.Channel(),
		exchange: exchange,
		source:   source,
		logger:   log,
	}, nil
}

// Publish publishes an event to the exchange
func (p *Publisher) Publish(ctx context.Context, eventType string, data interface{}) error {
	correlationID := getCorrelationID(ctx)

	event, err := NewEvent(eventType, p.source, correlationID, data)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.channel.PublishWithContext(ctx,
		p.exchange, // exchange
		eventType,  // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: correlationID,
			Body:          body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug().
		Str("event_type", eventType).
		Str("event_id", event.ID).
		Str("correlation_id", correlationID).
		Msg("event published")

	return nil
}

// PublishWithRoutingKey publishes an event with a custom routing key
func (p *Publisher) PublishWithRoutingKey(ctx context.Context, routingKey string, event *Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.channel.PublishWithContext(ctx,
		p.exchange, // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: event.CorrelationID,
			Body:          body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Debug().
		Str("routing_key", routingKey).
		Str("event_id", event.ID).
		Msg("event published")

	return nil
}

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// getCorrelationID retrieves the correlation ID from context
func getCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}
