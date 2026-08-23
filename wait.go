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

func (d *App) WaitForServiceReady(ctx context.Context, servicePath string) (*runpb.Service, error) {
	d.LogInfo("waiting for service condition Ready == True", "service", servicePath, "timeout", d.Timeout())
	return waitResourceReady(ctx, d.Timeout(), servicePath, func(ctx context.Context) (*runpb.Service, *runpb.Condition, []*runpb.Condition, error) {
		svc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
			Name: servicePath,
		})
		if err != nil {
			d.LogWarn("failed to get service status, retrying", "error", err)
			return nil, nil, nil, err
		}
		return svc, svc.TerminalCondition, svc.Conditions, nil
	})
}

// WaitForJobReady polls the Cloud Run Job until Ready condition is SUCCEEDED.
func (d *App) WaitForJobReady(ctx context.Context, jobPath string) (*runpb.Job, error) {
	d.LogInfo("waiting for job condition Ready == True", "job", jobPath, "timeout", d.Timeout())
	return waitResourceReady(ctx, d.Timeout(), jobPath, func(ctx context.Context) (*runpb.Job, *runpb.Condition, []*runpb.Condition, error) {
		job, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
			Name: jobPath,
		})
		if err != nil {
			d.LogWarn("failed to get job status, retrying", "error", err)
			return nil, nil, nil, err
		}
		return job, job.TerminalCondition, job.Conditions, nil
	})
}

func waitResourceReady[T any](
	ctx context.Context,
	timeout time.Duration,
	resourceName string,
	fetch func(context.Context) (T, *runpb.Condition, []*runpb.Condition, error),
) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var zero T
	for {
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("timeout or context canceled waiting for %s: %w", resourceName, ctx.Err())
		case <-ticker.C:
			res, tc, conds, err := fetch(ctx)
			if err != nil {
				continue
			}

			if tc != nil {
				switch tc.State {
				case runpb.Condition_CONDITION_SUCCEEDED:
					return res, nil
				case runpb.Condition_CONDITION_FAILED:
					return zero, fmt.Errorf("resource %s failed: %s: %s", resourceName, tc.Type, tc.Message)
				}
			}

			for _, cond := range conds {
				if cond != nil && cond.Type == "Ready" {
					if cond.State == runpb.Condition_CONDITION_SUCCEEDED {
						return res, nil
					}
					if cond.State == runpb.Condition_CONDITION_FAILED {
						return zero, fmt.Errorf("resource %s condition failed: %s: %s", resourceName, cond.Type, cond.Message)
					}
				}
			}
		}
	}
}
