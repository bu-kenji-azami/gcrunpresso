package gcrunpresso

import (
	"context"
)

type ScaleOption struct {
	Min       *int32 `help:"min instances"`
	Max       *int32 `help:"max instances"`
	NoTraffic bool   `help:"do not route traffic to new revision" default:"false"`
	DryRun    bool   `help:"dry run" default:"false"`
}

func (d *App) Scale(ctx context.Context, opt ScaleOption) error {
	d.LogInfo("scale command called", withDryRun(opt.DryRun)...)
	return nil
}
