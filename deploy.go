package gcrunpresso

import (
	"context"
)

type DeployOption struct {
	DryRun    bool   `help:"dry run" default:"false"`
	Tag       string `help:"tag for revision" default:""`
	NoTraffic bool   `help:"deploy without routing base traffic" default:"false"`
	Traffic   string `help:"traffic split percentage specification" default:""`
	Job       bool   `help:"deploy as Cloud Run Job instead of Service" default:"false"`
}

func (d *App) Deploy(ctx context.Context, opt DeployOption) error {
	d.LogInfo("deploy command called", withDryRun(opt.DryRun)...)
	return nil
}
