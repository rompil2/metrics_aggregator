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
		Logger()
}

// SetGlobalLogger устанавливает глобальный логгер
func SetGlobalLogger(logger zerolog.Logger) {
	globalLogger = logger
}

// Get возвращает глобальный логгер
func Get() zerolog.Logger {
	return globalLogger
}

// WithContext добавляет контекст к логгеру
func WithContext(fields map[string]interface{}) zerolog.Logger {
	return globalLogger.With().Fields(fields).Logger()
}

func WithLogger(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

func FromContext(ctx context.Context) zerolog.Logger {
	if log, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		return log
	}
	return Get()
}
