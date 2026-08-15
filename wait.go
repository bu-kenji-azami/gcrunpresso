package gcrunpresso

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
)

type WaitOption struct {
}

func (d *App) Wait(ctx context.Context, opt WaitOption) error {
	d.LogInfo("waiting for resource readiness", "service", d.config.Service, "job", d.config.Job)
	if d.config.Service != "" {
		svc, err := d.WaitForServiceReady(ctx, d.ResourceServicePath())
		if err != nil {
			return err
		}
		d.LogInfo("service is ready", "service", svc.Name, "uri", svc.Uri)
		return nil
	}
	if d.config.Job != "" {
		job, err := d.WaitForJobReady(ctx, d.ResourceJobPath())
		if err != nil {
			return err
		}
		d.LogInfo("job is ready", "job", job.Name)
		return nil
	}
	return fmt.Errorf("either service or job must be specified to wait")
}

// WaitForServiceReady polls the Cloud Run Service until TerminalCondition or Ready condition is SUCCEEDED.
func (d *App) WaitForServiceReady(ctx context.Context, servicePath string) (*runpb.Service, error) {
	timeout := d.Timeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	d.LogInfo("waiting for service condition Ready == True", "service", servicePath, "timeout", timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout or context canceled waiting for service %s: %w", servicePath, ctx.Err())
		case <-ticker.C:
			svc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
				Name: servicePath,
			})
			if err != nil {
				d.LogWarn("failed to get service status, retrying", "error", err)
				continue
			}

			// Check terminal condition
			if tc := svc.TerminalCondition; tc != nil {
				switch tc.State {
				case runpb.Condition_CONDITION_SUCCEEDED:
					return svc, nil
				case runpb.Condition_CONDITION_FAILED:
					return nil, fmt.Errorf("service deployment failed: %s: %s", tc.Type, tc.Message)
				}
			}

			// Check Ready condition in conditions list
			for _, cond := range svc.Conditions {
				if cond.Type == "Ready" {
					if cond.State == runpb.Condition_CONDITION_SUCCEEDED {
						return svc, nil
					}
					if cond.State == runpb.Condition_CONDITION_FAILED {
						return nil, fmt.Errorf("service condition failed: %s: %s", cond.Type, cond.Message)
					}
				}
			}
		}
	}
}

// WaitForJobReady polls the Cloud Run Job until Ready condition is SUCCEEDED.
func (d *App) WaitForJobReady(ctx context.Context, jobPath string) (*runpb.Job, error) {
	timeout := d.Timeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	d.LogInfo("waiting for job condition Ready == True", "job", jobPath, "timeout", timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout or context canceled waiting for job %s: %w", jobPath, ctx.Err())
		case <-ticker.C:
			job, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
				Name: jobPath,
			})
			if err != nil {
				d.LogWarn("failed to get job status, retrying", "error", err)
				continue
			}

			if tc := job.TerminalCondition; tc != nil {
				switch tc.State {
				case runpb.Condition_CONDITION_SUCCEEDED:
					return job, nil
				case runpb.Condition_CONDITION_FAILED:
					return nil, fmt.Errorf("job ready condition failed: %s: %s", tc.Type, tc.Message)
				}
			}

			for _, cond := range job.Conditions {
				if cond.Type == "Ready" {
					if cond.State == runpb.Condition_CONDITION_SUCCEEDED {
						return job, nil
					}
					if cond.State == runpb.Condition_CONDITION_FAILED {
						return nil, fmt.Errorf("job condition failed: %s: %s", cond.Type, cond.Message)
					}
				}
			}
		}
	}
}
