package gcrunpresso

import (
	"context"
)

type RollbackOption struct {
	Revision      string `help:"revision name to rollback to" default:""`
	RevertService bool   `help:"revert service template to target revision specification" default:"false"`
	DryRun        bool   `help:"dry run" default:"false"`
}

func (d *App) Rollback(ctx context.Context, opt RollbackOption) error {
	d.LogInfo("rollback command called", withDryRun(opt.DryRun)...)
	return nil
}
