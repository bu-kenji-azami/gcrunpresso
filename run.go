package gcrunpresso

import (
	"context"
	"time"
)

type RunOption struct {
	OverrideArgs  []string          `help:"override container args" default:""`
	OverrideEnv   map[string]string `help:"override container environment variables" default:""`
	Tasks         int               `help:"number of tasks to run" default:"1"`
	Wait          bool              `help:"wait for execution completion" default:"true"`
	DrainDuration time.Duration     `help:"drain duration for logs after job completion" default:"30s"`
}

func (d *App) Run(ctx context.Context, opt RunOption) error {
	d.LogInfo("run command called")
	return nil
}
