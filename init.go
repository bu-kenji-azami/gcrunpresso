package gcrunpresso

import (
	"context"
)

type InitOption struct {
	Service string `help:"service name to initialize from" default:""`
	Job     string `help:"job name to initialize from" default:""`
	Dir     string `help:"directory to write configuration files" default:"."`
}

func (d *App) Init(ctx context.Context, opt InitOption) error {
	d.LogInfo("init command called")
	return nil
}
