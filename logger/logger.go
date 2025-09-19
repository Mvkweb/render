package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// New creates a new slog.Logger.
func New() *slog.Logger {
	return slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	}))
}
