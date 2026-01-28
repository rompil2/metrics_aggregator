// path: internal/logger
package logger

import (
	"context"
	"os"

	"github.com/rs/zerolog"
)

var globalLogger zerolog.Logger

type contextKey string

const loggerKey contextKey = "logger"

func init() {
	// Настройка логгера по умолчанию
	globalLogger = zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger().Level(zerolog.InfoLevel)
}

// SetGlobalLogger sets the global logger instance used by the package.
// It should be called during application initialization to configure logging behavior.
func SetGlobalLogger(logger zerolog.Logger) {
	globalLogger = logger
}

// Get returns the current global logger instance.
// This is typically used in middleware or handlers that do not have access to a request-scoped logger.
func Get() zerolog.Logger {
	return globalLogger
}

// WithContext creates a new logger derived from the global logger with the provided fields attached.
// It is useful for adding structured context (e.g., request ID, user ID) at the beginning of a request lifecycle.
func WithContext(fields map[string]interface{}) zerolog.Logger {
	return globalLogger.With().Fields(fields).Logger()
}

// WithLogger associates a logger instance with the given context.
// The logger can later be retrieved using FromContext.
func WithLogger(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// FromContext retrieves a logger from the provided context.
// If no logger is found in the context, it falls back to the global logger.
func FromContext(ctx context.Context) zerolog.Logger {
	if log, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		return log
	}
	return Get()
}
