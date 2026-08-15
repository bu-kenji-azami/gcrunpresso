package gcrunpresso

import (
	"context"
)

type InitOption struct {
	Dir string `help:"directory to write configuration files" default:"."`
}

func (d *App) Init(ctx context.Context, opt InitOption) error {
	d.LogInfo("init command called")
	return nil
}
