package gcrunpresso

import (
	"context"
)

type RevisionsOption struct {
	Limit int `help:"limit number of revisions to show" default:"20"`
}

func (d *App) Revisions(ctx context.Context, opt RevisionsOption) error {
	d.LogInfo("revisions command called")
	return nil
}
