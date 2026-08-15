package gcrunpresso

import (
	"context"
)

type ExecutionsOption struct {
	Limit int `help:"limit number of executions to show" default:"20"`
}

func (d *App) Executions(ctx context.Context, opt ExecutionsOption) error {
	d.LogInfo("executions command called")
	return nil
}
