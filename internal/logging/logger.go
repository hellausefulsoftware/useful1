package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	defaultLogger *slog.Logger
	loggerMu      sync.Mutex
	initialized   bool
)

// LogLevel represents logging levels
type LogLevel string

const (
	// LogLevelDebug is for detailed debug information
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is for general operational information
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is for warning conditions that should be addressed
	LogLevelWarn LogLevel = "warn"
	// LogLevelError is for error conditions that prevent normal operation
	LogLevelError LogLevel = "error"
)

// Config holds logging configuration
type Config struct {
	Level      LogLevel
	Output     io.Writer
	JSONFormat bool
}

// DefaultConfig returns the default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      LogLevelInfo,
		Output:     os.Stdout,
		JSONFormat: false,
	}
}

// Initialize sets up the logger with the given configuration
func Initialize(cfg *Config) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if cfg == nil {
		cfg = DefaultConfig()
	}

	var level slog.Level
	switch cfg.Level {
	case LogLevelDebug:
		level = slog.LevelDebug
	case LogLevelInfo:
		level = slog.LevelInfo
	case LogLevelWarn:
		level = slog.LevelWarn
	case LogLevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.JSONFormat {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	initialized = true
}

// GetLogger returns the default logger
func GetLogger() *slog.Logger {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		Initialize(nil)
	}

	return defaultLogger
}

// Debug logs a message at debug level
func Debug(msg string, args ...any) {
	GetLogger().Debug(msg, args...)
}

// Info logs a message at info level
func Info(msg string, args ...any) {
	GetLogger().Info(msg, args...)
}

// Warn logs a message at warn level
func Warn(msg string, args ...any) {
	GetLogger().Warn(msg, args...)
}

// Error logs a message at error level
func Error(msg string, args ...any) {
	GetLogger().Error(msg, args...)
}

// WithContext returns a logger with context
func WithContext(ctx context.Context) *slog.Logger {
	return GetLogger().With("context", ctx)
}

// WithField adds a field to the logger
func WithField(key string, value any) *slog.Logger {
	return GetLogger().With(key, value)
}

// WithFields adds multiple fields to the logger
func WithFields(fields map[string]any) *slog.Logger {
	logger := GetLogger()
	for k, v := range fields {
		logger = logger.With(k, v)
	}
	return logger
}
