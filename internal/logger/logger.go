package logger

import (
	"log/slog"
	"os"
	"strings"
)

type Logger interface {
	// Debug logs a message at the Debug level with optional key-value pairs.
	Debug(msg string, keyVals ...any)

	// Info logs a message at the Info level with optional key-value pairs for structured logging.
	Info(msg string, keyVals ...any)

	// Warn logs a message at the Warn level with optional key-value pairs.
	Warn(msg string, keyVals ...any)

	// Error logs a message at the Error level with optional key-value pairs.
	Error(msg string, keyVals ...any)
}

type Slog struct {
	logger *slog.Logger
}

// Info logs a message at the Info level with optional key-value pairs for structured logging.
func (s *Slog) Info(msg string, keyVals ...any) {
	s.logger.Info(msg, keyVals...)
}

// Warn logs a message at the Warn level with optional key-value pairs.
func (s *Slog) Warn(msg string, keyVals ...any) {
	s.logger.Warn(msg, keyVals...)
}

// Error logs a message at the Error level with optional key-value pairs.
func (s *Slog) Error(msg string, keyVals ...any) {
	s.logger.Error(msg, keyVals...)
}

// Debug logs a message at the Debug level with optional key-value pairs.
func (s *Slog) Debug(msg string, keyVals ...any) {
	s.logger.Debug(msg, keyVals...)
}

// NewSlog creates a Logger using slog.
func NewSlog(logFilePath string, opts *slog.HandlerOptions) Logger {
	var file *os.File
	if logFilePath != "" {
		file, _ = os.OpenFile(logFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	}
	if file == nil {
		file = os.Stdout
	}
	return &Slog{
		slog.New(slog.NewJSONHandler(file, opts)),
	}
}

// GetSlogLevel returns slog Level for Slog Handler options according to provided level string:
// debug, warn, error (case insensitive). Any other value will return Info level.
func GetSlogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
