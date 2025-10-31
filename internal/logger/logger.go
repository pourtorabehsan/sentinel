// Package logger provides structured logging capabilities using Go's standard log/slog.
// It wraps slog to provide convenient helpers for adding context like cluster, node, and operation IDs.
package logger

import (
	"io"
	"log/slog"
	"os"
)

// Level represents the severity of a log message.
type Level = slog.Level

// Common log levels for convenience.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// New creates a new structured logger with JSON output format.
// The logger writes to stdout and uses the specified log level.
func New(level Level) *slog.Logger {
	return NewWithWriter(os.Stdout, level)
}

// NewWithWriter creates a new structured logger that writes to the specified writer.
// This is useful for testing or custom output destinations.
func NewWithWriter(w io.Writer, level Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(w, opts)
	return slog.New(handler)
}

// WithCluster returns a logger with the cluster ID added to all log messages.
func WithCluster(logger *slog.Logger, clusterID string) *slog.Logger {
	return logger.With("cluster", clusterID)
}

// WithNode returns a logger with the node identifier added to all log messages.
// The node parameter can be a host:port combination or any unique identifier.
func WithNode(logger *slog.Logger, node string) *slog.Logger {
	return logger.With("node", node)
}

// WithOperation returns a logger with the operation ID added to all log messages.
// This is useful for tracking operations across multiple log entries.
func WithOperation(logger *slog.Logger, operationID string) *slog.Logger {
	return logger.With("operation_id", operationID)
}

// WithGroup returns a logger with the group ID added to all log messages.
func WithGroup(logger *slog.Logger, groupID string) *slog.Logger {
	return logger.With("group", groupID)
}

// WithContext returns a logger with multiple context fields added at once.
// This is more efficient than chaining multiple With calls.
func WithContext(logger *slog.Logger, groupID, clusterID, node string) *slog.Logger {
	return logger.With(
		"group", groupID,
		"cluster", clusterID,
		"node", node,
	)
}

// ParseLevel converts a string level name to a slog.Level.
// Valid levels are: debug, info, warn, error.
// Returns LevelInfo for invalid or empty level strings.
func ParseLevel(level string) Level {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
