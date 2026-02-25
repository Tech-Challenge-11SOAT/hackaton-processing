package logging

import (
	"log/slog"
	"os"
)

// New creates a JSON structured logger with the provided level.
func New(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler)
}
