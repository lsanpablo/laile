package log

import (
	"context"
	"log/slog"
	"os"
)

var (
	Logger        *slog.Logger
	defaultLogger *slog.Logger
)

func InitLogger() {
	// Initialize with JSON handler for production, text handler for development.
	var handler slog.Handler
	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	Logger = defaultLogger
}

func Error(ctx context.Context, msg string, args ...any) {
	defaultLogger.ErrorContext(ctx, msg, args...)
}

func Debug(ctx context.Context, msg string, args ...any) {
	defaultLogger.DebugContext(ctx, msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	defaultLogger.InfoContext(ctx, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	defaultLogger.WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.ErrorContext(ctx, msg, args...)
}
