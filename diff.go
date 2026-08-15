package gcrunpresso

import (
	"context"
	"io"
	"os"
)

type DiffOption struct {
	Unified bool `help:"unified diff" default:"true"`
	w       io.Writer
}

func (opt *DiffOption) SetWriter(w io.Writer) {
	opt.w = w
}

func (opt *DiffOption) getWriter() io.Writer {
	if opt.w != nil {
		return opt.w
	}
	return os.Stdout
}

func (d *App) Diff(ctx context.Context, opt DiffOption) error {
	d.LogInfo("diff command called")
	return nil
}
