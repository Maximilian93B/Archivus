package logger

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

// New creates a new structured logger
func New() *Logger {
	// Use JSON format for structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return &Logger{Logger: logger}
}

// NewWithLevel creates a logger with specific log level
func NewWithLevel(level slog.Level) *Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	return &Logger{Logger: logger}
}

// NewForTesting creates a logger for testing (discards output)
func NewForTesting() *Logger {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &Logger{Logger: logger}
} 