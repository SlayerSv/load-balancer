package logger

import (
	"log/slog"
	"os"
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

func (s *Slog) Info(msg string, keyVals ...any) {
	s.logger.Info(msg, keyVals...)
}

func (s *Slog) Warn(msg string, keyVals ...any) {
	s.logger.Warn(msg, keyVals...)
}

func (s *Slog) Error(msg string, keyVals ...any) {
	s.logger.Error(msg, keyVals...)
}

func (s *Slog) Debug(msg string, keyVals ...any) {
	s.logger.Debug(msg, keyVals...)
}

// NewSlog creates a Logger using slog.
func NewSlog(file *os.File, opts *slog.HandlerOptions) Logger {
	return &Slog{
		slog.New(slog.NewJSONHandler(file, nil)),
	}
}
