package gcrunpresso

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/run/apiv2/runpb"
)

type DeleteOption struct {
	Force bool `help:"force delete without confirmation prompt" default:"false"`
}

func (d *App) Delete(ctx context.Context, opt DeleteOption) error {
	if d.config.Service != "" {
		return d.deleteService(ctx, opt)
	}
	if d.config.Job != "" {
		return d.deleteJob(ctx, opt)
	}
	return fmt.Errorf("either service or job must be specified in config to delete")
}

func (d *App) deleteService(ctx context.Context, opt DeleteOption) error {
	serviceName := d.ResourceServicePath()
	if !opt.Force {
		if !confirmPrompt(fmt.Sprintf("Are you sure you want to delete Cloud Run Service %s? (y/N): ", d.config.Service)) {
			d.LogInfo("deletion canceled by user")
			return nil
		}
	}

	d.LogInfo("deleting service", "service", serviceName)
	op, err := d.servicesClient.DeleteService(ctx, &runpb.DeleteServiceRequest{
		Name: serviceName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete service %s: %w", serviceName, err)
	}
	d.LogInfo("delete service operation started, waiting for completion", "op", op.Name())
	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for service deletion %s: %w", serviceName, err)
	}
	d.LogInfo("service deleted successfully", "service", serviceName)
	return nil
}

func (d *App) deleteJob(ctx context.Context, opt DeleteOption) error {
	jobName := d.ResourceJobPath()
	if !opt.Force {
		if !confirmPrompt(fmt.Sprintf("Are you sure you want to delete Cloud Run Job %s? (y/N): ", d.config.Job)) {
			d.LogInfo("deletion canceled by user")
			return nil
		}
	}

	d.LogInfo("deleting job", "job", jobName)
	op, err := d.jobsClient.DeleteJob(ctx, &runpb.DeleteJobRequest{
		Name: jobName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete job %s: %w", jobName, err)
	}
	d.LogInfo("delete job operation started, waiting for completion", "op", op.Name())
	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for job deletion %s: %w", jobName, err)
	}
	d.LogInfo("job deleted successfully", "job", jobName)
	return nil
}

func confirmPrompt(msg string) bool {
	fmt.Print(msg)
	reader := bufio.NewReader(os.Stdin)
	ans, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}
