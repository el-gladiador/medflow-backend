package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/medflow/medflow-backend/pkg/config"
	"github.com/medflow/medflow-backend/pkg/logger"
)

// RabbitMQ manages the connection to RabbitMQ
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.RabbitMQConfig
	logger  *logger.Logger
	mu      sync.RWMutex
	closed  bool
}

// New creates a new RabbitMQ connection
func New(cfg *config.RabbitMQConfig, log *logger.Logger) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		config: cfg,
		logger: log,
	}

	if err := rmq.connect(); err != nil {
		return nil, err
	}

	return rmq, nil
}

func (r *RabbitMQ) connect() error {
	var err error

	r.conn, err = amqp.Dial(r.config.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	r.channel, err = r.conn.Channel()
	if err != nil {
		r.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	if err := r.channel.Qos(r.config.PrefetchCount, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	r.logger.Info().Msg("connected to RabbitMQ")
	return nil
}

// Channel returns the current channel
func (r *RabbitMQ) Channel() *amqp.Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channel
}

// Connection returns the current connection
func (r *RabbitMQ) Connection() *amqp.Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conn
}

// Close closes the RabbitMQ connection
func (r *RabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.closed = true

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			r.logger.Warn().Err(err).Msg("failed to close channel")
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	r.logger.Info().Msg("RabbitMQ connection closed")
	return nil
}

// Health returns the health status of RabbitMQ
func (r *RabbitMQ) Health() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := map[string]string{
		"status": "up",
	}

	if r.conn == nil || r.conn.IsClosed() {
		status["status"] = "down"
		status["error"] = "connection closed"
	}

	return status
}

// DeclareExchange declares a topic exchange
func (r *RabbitMQ) DeclareExchange(name string) error {
	return r.channel.ExchangeDeclare(
		name,    // name
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
}

// DeclareQueue declares a durable queue
func (r *RabbitMQ) DeclareQueue(name string) (amqp.Queue, error) {
	return r.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange": "dlx.events",
		},
	)
}

// DeclareDeadLetterQueue declares the dead letter exchange and queue
func (r *RabbitMQ) DeclareDeadLetterQueue(serviceName string) error {
	// Declare DLX exchange
	if err := r.channel.ExchangeDeclare(
		"dlx.events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare DLX exchange: %w", err)
	}

	// Declare DLQ queue
	queueName := fmt.Sprintf("dlq.%s", serviceName)
	_, err := r.channel.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ queue: %w", err)
	}

	// Bind DLQ to DLX
	if err := r.channel.QueueBind(
		queueName,
		"#", // Catch all routing keys
		"dlx.events",
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	return nil
}

// BindQueue binds a queue to an exchange with a routing key pattern
func (r *RabbitMQ) BindQueue(queueName, exchange, routingKey string) error {
	return r.channel.QueueBind(
		queueName,
		routingKey,
		exchange,
		false,
		nil,
	)
}

// Reconnect attempts to reconnect to RabbitMQ
func (r *RabbitMQ) Reconnect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("connection is permanently closed")
	}

	for i := 0; i < r.config.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.logger.Info().Int("attempt", i+1).Msg("attempting to reconnect to RabbitMQ")

		if err := r.connect(); err != nil {
			r.logger.Warn().Err(err).Msg("reconnection attempt failed")
			time.Sleep(r.config.ReconnectDelay)
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to reconnect after %d attempts", r.config.MaxRetries)
}
