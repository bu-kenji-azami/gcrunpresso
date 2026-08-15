package gcrunpresso

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
)

type StatusOption struct {
	Events bool `help:"show recent logs and events" default:"false"`
}

func (d *App) Status(ctx context.Context, opt StatusOption) error {
	d.LogInfo("fetching status", "service", d.config.Service, "job", d.config.Job)

	if d.config.Service != "" {
		return d.statusService(ctx, opt)
	}
	if d.config.Job != "" {
		return d.statusJob(ctx, opt)
	}
	return fmt.Errorf("either service or job must be specified in config")
}

func (d *App) statusService(ctx context.Context, opt StatusOption) error {
	svc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get service status %s: %w", d.ResourceServicePath(), err)
	}

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	bold.Printf("Service: %s\n", svc.Name)
	fmt.Printf("URL:     %s\n", svc.Uri)
	fmt.Printf("Latest Ready Revision:   %s\n", svc.LatestReadyRevision)
	fmt.Printf("Latest Created Revision: %s\n\n", svc.LatestCreatedRevision)

	if svc.Template != nil && svc.Template.Scaling != nil {
		fmt.Printf("Scaling: min=%d, max=%d\n\n", svc.Template.Scaling.MinInstanceCount, svc.Template.Scaling.MaxInstanceCount)
	}

	// Traffic Table
	bold.Println("Traffic Allocations:")
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tREVISION\tTAG\tPERCENT")
	for _, t := range svc.Traffic {
		rev := t.Revision
		if rev == "" && t.Type == runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST {
			rev = "(latest)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d%%\n", t.Type, rev, t.Tag, t.Percent)
	}
	w.Flush()
	fmt.Println()

	// Conditions Table
	bold.Println("Conditions:")
	w = tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tTYPE\tMESSAGE")
	for _, c := range svc.Conditions {
		statusStr := "[ ]"
		if c.State == runpb.Condition_CONDITION_SUCCEEDED {
			statusStr = green.Sprint("[OK]")
		} else if c.State == runpb.Condition_CONDITION_FAILED {
			statusStr = red.Sprint("[FAIL]")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", statusStr, c.Type, c.Message)
	}
	w.Flush()

	return nil
}

func (d *App) statusJob(ctx context.Context, opt StatusOption) error {
	job, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
		Name: d.ResourceJobPath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get job status %s: %w", d.ResourceJobPath(), err)
	}

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	bold.Printf("Job: %s\n", job.Name)
	if job.Template != nil {
		fmt.Printf("Task Count:  %d\n", job.Template.TaskCount)
		fmt.Printf("Parallelism: %d\n", job.Template.Parallelism)
		if job.Template.Template != nil {
			fmt.Printf("Max Retries: %d\n", job.Template.Template.GetMaxRetries())
			if job.Template.Template.Timeout != nil {
				fmt.Printf("Timeout:     %s\n", job.Template.Template.Timeout.AsDuration())
			}
		}
	}
	if job.LatestCreatedExecution != nil {
		fmt.Printf("Latest Created Execution: %s\n", job.LatestCreatedExecution.Name)
	}
	fmt.Println()

	// Conditions Table
	bold.Println("Conditions:")
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tTYPE\tMESSAGE")
	for _, c := range job.Conditions {
		statusStr := "[ ]"
		if c.State == runpb.Condition_CONDITION_SUCCEEDED {
			statusStr = green.Sprint("[OK]")
		} else if c.State == runpb.Condition_CONDITION_FAILED {
			statusStr = red.Sprint("[FAIL]")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", statusStr, c.Type, c.Message)
	}
	w.Flush()

	return nil
}
