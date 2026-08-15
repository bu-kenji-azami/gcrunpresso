package gcrunpresso

import (
	"context"
)

type DeleteOption struct {
	Force bool `help:"delete without confirmation prompt" default:"false"`
}

func (d *App) Delete(ctx context.Context, opt DeleteOption) error {
	d.LogInfo("delete command called")
	return nil
}
