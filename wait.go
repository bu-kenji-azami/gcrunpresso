package gcrunpresso

import (
	"context"
)

type WaitOption struct {
}

func (d *App) Wait(ctx context.Context, opt WaitOption) error {
	d.LogInfo("wait command called")
	return nil
}
