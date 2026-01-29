package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger
type Logger struct {
	zerolog.Logger
}

// New creates a new logger instance
func New(serviceName string, environment string) *Logger {
	var output io.Writer = os.Stdout

	if environment == "development" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()

	return &Logger{Logger: logger}
}

// WithRequestID returns a logger with the request ID attached
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("request_id", requestID).Logger(),
	}
}

// WithUserID returns a logger with the user ID attached
func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("user_id", userID).Logger(),
	}
}

// WithCorrelationID returns a logger with the correlation ID attached
func (l *Logger) WithCorrelationID(correlationID string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("correlation_id", correlationID).Logger(),
	}
}

// WithComponent returns a logger with the component name attached
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("component", component).Logger(),
	}
}

// WithError returns a logger with the error attached
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.Logger.With().Err(err).Logger(),
	}
}
