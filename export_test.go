package gcrunpresso

import (
	"context"
	"log/slog"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
)

var (
	NewLogger               = newLogger
	LogLevel                = logLevel
	SetLogFormat            = setLogFormat
	Map2str                 = map2str
	ParseLabels             = parseLabels
	ValidateJobSafetyGuards = validateJobSafetyGuards
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

func (d *App) FindPrecedingHealthyRevision(ctx context.Context, currentSvc *runpb.Service) (string, error) {
	return d.findPrecedingHealthyRevision(ctx, currentSvc)
}

func (d *App) WarnPlaintextSecretsInEnv(containerName string, envVars []*runpb.EnvVar) {
	d.warnPlaintextSecretsInEnv(containerName, envVars)
}
