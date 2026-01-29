package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// MessageHandler is a function that handles a message
type MessageHandler func(ctx context.Context, event *Event) error

// Consumer handles consuming events from RabbitMQ
type Consumer struct {
	rmq       *RabbitMQ
	queueName string
	handlers  map[string]MessageHandler
	logger    *logger.Logger
}

// NewConsumer creates a new consumer for the given queue
func NewConsumer(rmq *RabbitMQ, queueName string, log *logger.Logger) (*Consumer, error) {
	// Declare the queue
	_, err := rmq.DeclareQueue(queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}

	return &Consumer{
		rmq:       rmq,
		queueName: queueName,
		handlers:  make(map[string]MessageHandler),
		logger:    log,
	}, nil
}

// Subscribe subscribes to an exchange with a routing key pattern
func (c *Consumer) Subscribe(exchange, routingKeyPattern string) error {
	// Declare the exchange first
	if err := c.rmq.DeclareExchange(exchange); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Bind the queue to the exchange
	if err := c.rmq.BindQueue(c.queueName, exchange, routingKeyPattern); err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	c.logger.Info().
		Str("queue", c.queueName).
		Str("exchange", exchange).
		Str("routing_key", routingKeyPattern).
		Msg("subscribed to exchange")

	return nil
}

// RegisterHandler registers a handler for a specific event type
func (c *Consumer) RegisterHandler(eventType string, handler MessageHandler) {
	c.handlers[eventType] = handler
}

// Start starts consuming messages from the queue
func (c *Consumer) Start(ctx context.Context) error {
	msgs, err := c.rmq.Channel().Consume(
		c.queueName, // queue
		"",          // consumer tag (auto-generated)
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.logger.Info().Str("queue", c.queueName).Msg("consumer started")

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info().Str("queue", c.queueName).Msg("consumer stopped")
				return
			case msg, ok := <-msgs:
				if !ok {
					c.logger.Warn().Msg("message channel closed")
					return
				}
				c.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

func (c *Consumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
	var event Event
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.Error().Err(err).Msg("failed to unmarshal event")
		// Reject without requeue for malformed messages
		msg.Reject(false)
		return
	}

	// Add correlation ID to context
	ctx = WithCorrelationID(ctx, event.CorrelationID)

	handler, ok := c.handlers[event.Type]
	if !ok {
		c.logger.Debug().
			Str("event_type", event.Type).
			Msg("no handler registered for event type")
		msg.Ack(false)
		return
	}

	c.logger.Debug().
		Str("event_type", event.Type).
		Str("event_id", event.ID).
		Str("correlation_id", event.CorrelationID).
		Msg("processing event")

	if err := handler(ctx, &event); err != nil {
		c.logger.Error().
			Err(err).
			Str("event_type", event.Type).
			Str("event_id", event.ID).
			Msg("failed to process event")

		// Check retry count from headers
		retryCount := getRetryCount(msg)
		if retryCount >= 3 {
			// Send to dead letter queue
			c.logger.Warn().
				Str("event_id", event.ID).
				Int("retry_count", retryCount).
				Msg("max retries exceeded, sending to DLQ")
			msg.Reject(false)
			return
		}

		// Requeue for retry
		msg.Nack(false, true)
		return
	}

	msg.Ack(false)
}

func getRetryCount(msg amqp.Delivery) int {
	if msg.Headers == nil {
		return 0
	}

	if deaths, ok := msg.Headers["x-death"].([]interface{}); ok {
		for _, death := range deaths {
			if d, ok := death.(amqp.Table); ok {
				if count, ok := d["count"].(int64); ok {
					return int(count)
				}
			}
		}
	}

	return 0
}
