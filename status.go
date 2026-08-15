package gcrunpresso

import (
	"context"
)

type StatusOption struct {
	Events bool `help:"show recent events" default:"false"`
}

func (d *App) Status(ctx context.Context, opt StatusOption) error {
	d.LogInfo("status command called")
	return nil
}
