package gcrunpresso

import (
	"context"
)

type VerifyOption struct {
	DryRun bool `help:"dry run" default:"false"`
}

func (d *App) Verify(ctx context.Context, opt VerifyOption) error {
	d.LogInfo("verify command called")
	return nil
}
