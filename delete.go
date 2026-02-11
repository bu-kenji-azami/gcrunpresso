package ecspresso

import (
	"context"
	"fmt"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type DeleteOption struct {
	DryRun    bool `help:"dry-run" default:"false"`
	Force     bool `help:"delete without confirmation" default:"false"`
	Terminate bool `help:"delete with terminate tasks" default:"false"`
}

func (opt DeleteOption) DryRunString() string {
	if opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Delete(ctx context.Context, opt DeleteOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.LogInfo("Deleting service", withDryRun(opt.DryRun)...)
	sv, err := d.DescribeServiceStatus(ctx, 3)
	if err != nil {
		return err
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN OK")
		return nil
	}

	if !opt.Force {
		service := prompter.Prompt(`Enter the service name to DELETE`, "")
		if service != *sv.ServiceName {
			d.LogInfo("Aborted")
			return fmt.Errorf("confirmation failed")
		}
	}
	if sv.isExpressMode() {
		return d.deleteExpressGatewayService(ctx, sv, opt)
	} else {
		return d.deleteService(ctx, sv, opt)
	}
}

func (d *App) deleteService(ctx context.Context, sv *Service, opt DeleteOption) error {
	in := &ecs.DeleteServiceInput{
		Cluster: &d.config.Cluster,
		Service: sv.ServiceName,
		Force:   &opt.Terminate, // == aws ecs delete-service --force
	}
	if _, err := d.ecs.DeleteService(ctx, in); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}
	d.LogInfo("Service is deleted")
	return nil
}
