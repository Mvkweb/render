package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// Logger is a wrapper around slog.Logger.
type Logger struct {
	*slog.Logger
}

// New creates a new Logger.
func New() *Logger {
	return &Logger{
		slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		})),
	}
}
