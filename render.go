package gcrunpresso

import (
	"context"
)

type RenderOption struct {
	Format string `help:"output format (yaml or json)" default:"yaml" enum:"yaml,json"`
}

func (d *App) Render(ctx context.Context, opt RenderOption) error {
	d.LogInfo("render command called")
	return nil
}
