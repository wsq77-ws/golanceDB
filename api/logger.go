package api

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

// Logger wraps slog.Logger with context enrichment for GlanceDB operations.
type Logger struct {
	*slog.Logger
}

// LogLevel wraps slog.Level for configuration.
type LogLevel = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// NewLogger creates a default JSON logger writing to stderr.
// Callers may override with SetLogger or Options.Logger.
func NewLogger() *Logger {
	return &Logger{
		Logger: slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

// NewTextLogger creates a text-based logger suitable for development.
func NewTextLogger(w io.Writer, level slog.Level) *Logger {
	return &Logger{
		Logger: slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: level,
		})),
	}
}

// WithOp returns a logger that includes the operation name.
func (l *Logger) WithOp(op string) *Logger {
	return &Logger{Logger: l.With("op", op)}
}

// WithTable returns a logger that includes the table name.
func (l *Logger) WithTable(table string) *Logger {
	return &Logger{Logger: l.With("table", table)}
}

// LogOperation logs the start, duration, and result of an operation.
func (l *Logger) LogOperation(ctx context.Context, op string, fn func(ctx context.Context) error) error {
	start := time.Now()
	l.DebugContext(ctx, "operation started", "op", op)
	err := fn(ctx)
	elapsed := time.Since(start)
	if err != nil {
		l.WarnContext(ctx, "operation failed",
			"op", op,
			"duration", elapsed.String(),
			"error", err.Error(),
		)
		// If it's an api.Error, also log the user-facing message.
		if apiErr, ok := err.(*Error); ok {
			l.WarnContext(ctx, "user-facing message", "message", apiErr.UserMessage())
		}
	} else {
		l.InfoContext(ctx, "operation succeeded",
			"op", op,
			"duration", elapsed.String(),
		)
	}
	return err
}

// defaultLogger is the package-level logger.
var defaultLogger = NewLogger()

// SetLogger replaces the package-level default logger.
func SetLogger(l *Logger) {
	if l != nil {
		defaultLogger = l
	}
}

// L returns the package-level default logger.
func L() *Logger {
	return defaultLogger
}
