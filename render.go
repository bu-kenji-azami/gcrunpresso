package gcrunpresso

import (
	"context"
	"fmt"
)

type RenderOption struct {
	JSON bool `help:"render as JSON instead of YAML" default:"false"`
}

func (d *App) Render(ctx context.Context, opt RenderOption) error {
	if d.config.Service != "" {
		svc, err := d.LoadServiceDefinition("")
		if err != nil {
			return err
		}
		if opt.JSON {
			b, err := MarshalService(svc)
			if err != nil {
				return err
			}
			fmt.Print(string(b))
			return nil
		}
		b, err := MarshalProtoYAML(svc)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
		return nil
	}

	if d.config.Job != "" {
		job, err := d.LoadJobDefinition("")
		if err != nil {
			return err
		}
		if opt.JSON {
			b, err := MarshalJob(job)
			if err != nil {
				return err
			}
			fmt.Print(string(b))
			return nil
		}
		b, err := MarshalProtoYAML(job)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
		return nil
	}

	return fmt.Errorf("either service or job must be specified in config to render")
}
