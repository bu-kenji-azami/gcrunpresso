package gcrunpresso

import (
	"context"
	"log/slog"
	"time"
)

var (
	NewLogger    = newLogger
	LogLevel     = logLevel
	SetLogFormat = setLogFormat
	Map2str      = map2str
	ParseLabels  = parseLabels
)

func (d *App) SetLogger(logger *slog.Logger) {
	d.logger = logger
}

func SetLogger(logger *slog.Logger) {
	commonLogger = logger
}

func SleepContext(ctx context.Context, d time.Duration) {
	sleepContext(ctx, d)
}
