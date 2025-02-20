package logger

import (
	"context"
	"log/slog"
	"os"
)

var defaultLogger *slog.Logger

func init() {
	// Initialize with JSON handler for production, text handler for development
	var handler slog.Handler
	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}

	defaultLogger = slog.New(handler)
}

// Info logs an info message with optional key-value pairs
func Info(ctx context.Context, msg string, args ...any) {
	defaultLogger.InfoContext(ctx, msg, args...)
}

// Error logs an error message with optional key-value pairs
func Error(ctx context.Context, msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err)
	}
	defaultLogger.ErrorContext(ctx, msg, args...)
}

// Debug logs a debug message with optional key-value pairs
func Debug(ctx context.Context, msg string, args ...any) {
	defaultLogger.DebugContext(ctx, msg, args...)
}
