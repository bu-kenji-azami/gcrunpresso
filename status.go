package gcrunpresso

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"cloud.google.com/go/logging/logadmin"
	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
	"google.golang.org/api/iterator"
)

type StatusOption struct {
	Events bool `help:"show recent logs and events" default:"false"`
	JSON   bool `help:"output status in JSON format" default:"false"`
}

type ServiceStatusResult struct {
	Name                  string              `json:"name"`
	URI                   string              `json:"uri"`
	LatestReadyRevision   string              `json:"latest_ready_revision"`
	LatestCreatedRevision string              `json:"latest_created_revision"`
	Scaling               *ScalingResult      `json:"scaling,omitempty"`
	Traffic               []TrafficItemResult `json:"traffic"`
	Conditions            []ConditionResult   `json:"conditions"`
	RecentEvents          []EventItemResult   `json:"recent_events,omitempty"`
}

type JobStatusResult struct {
	Name                   string            `json:"name"`
	TaskCount              int32             `json:"task_count"`
	Parallelism            int32             `json:"parallelism"`
	MaxRetries             int32             `json:"max_retries"`
	Timeout                string            `json:"timeout,omitempty"`
	LatestCreatedExecution string            `json:"latest_created_execution,omitempty"`
	Conditions             []ConditionResult `json:"conditions"`
	RecentEvents           []EventItemResult `json:"recent_events,omitempty"`
}

type EventItemResult struct {
	Timestamp string `json:"timestamp"`
	Severity  string `json:"severity,omitempty"`
	Message   string `json:"message"`
}

type ScalingResult struct {
	MinInstanceCount int32 `json:"min_instance_count"`
	MaxInstanceCount int32 `json:"max_instance_count"`
}

type TrafficItemResult struct {
	Type     string `json:"type"`
	Revision string `json:"revision"`
	Tag      string `json:"tag,omitempty"`
	Percent  int32  `json:"percent"`
}

type ConditionResult struct {
	Type    string `json:"type"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
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

	result := ServiceStatusResult{
		Name:                  svc.Name,
		URI:                   svc.Uri,
		LatestReadyRevision:   svc.LatestReadyRevision,
		LatestCreatedRevision: svc.LatestCreatedRevision,
	}

	if svc.Template != nil && svc.Template.Scaling != nil {
		result.Scaling = &ScalingResult{
			MinInstanceCount: svc.Template.Scaling.MinInstanceCount,
			MaxInstanceCount: svc.Template.Scaling.MaxInstanceCount,
		}
	}

	for _, t := range svc.Traffic {
		rev := t.Revision
		if rev == "" && t.Type == runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST {
			rev = "(latest)"
		}
		result.Traffic = append(result.Traffic, TrafficItemResult{
			Type:     t.Type.String(),
			Revision: rev,
			Tag:      t.Tag,
			Percent:  t.Percent,
		})
	}

	for _, c := range svc.Conditions {
		result.Conditions = append(result.Conditions, ConditionResult{
			Type:    c.Type,
			State:   formatConditionState(c.State),
			Message: c.Message,
		})
	}

	if opt.Events {
		filter := fmt.Sprintf(`resource.type="cloud_run_revision" AND resource.labels.service_name="%s" AND resource.labels.location="%s"`, d.config.Service, d.config.Location)
		result.RecentEvents = d.fetchRecentEvents(ctx, filter, recentEventsLimit)
	}

	if opt.JSON {
		return printJSON(result)
	}

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	bold.Printf("Service: %s\n", svc.Name)
	fmt.Printf("URL:     %s\n", svc.Uri)
	fmt.Printf("Latest Ready Revision:   %s\n", svc.LatestReadyRevision)
	fmt.Printf("Latest Created Revision: %s\n\n", svc.LatestCreatedRevision)

	if result.Scaling != nil {
		fmt.Printf("Scaling: min=%d, max=%d\n\n", result.Scaling.MinInstanceCount, result.Scaling.MaxInstanceCount)
	}

	// Traffic Table
	bold.Println("Traffic Allocations:")
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tREVISION\tTAG\tPERCENT")
	for _, t := range result.Traffic {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d%%\n", t.Type, t.Revision, t.Tag, t.Percent)
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

	if len(result.RecentEvents) > 0 {
		fmt.Println()
		bold.Println("Recent Events:")
		for _, ev := range result.RecentEvents {
			fmt.Printf("  %s [%s] %s\n", ev.Timestamp, ev.Severity, ev.Message)
		}
	}

	return nil
}

func (d *App) statusJob(ctx context.Context, opt StatusOption) error {
	job, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
		Name: d.ResourceJobPath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get job status %s: %w", d.ResourceJobPath(), err)
	}

	result := JobStatusResult{
		Name: job.Name,
	}

	if job.Template != nil {
		result.TaskCount = job.Template.TaskCount
		result.Parallelism = job.Template.Parallelism
		if job.Template.Template != nil {
			result.MaxRetries = job.Template.Template.GetMaxRetries()
			if job.Template.Template.Timeout != nil {
				result.Timeout = job.Template.Template.Timeout.AsDuration().String()
			}
		}
	}
	if job.LatestCreatedExecution != nil {
		result.LatestCreatedExecution = job.LatestCreatedExecution.Name
	}

	for _, c := range job.Conditions {
		result.Conditions = append(result.Conditions, ConditionResult{
			Type:    c.Type,
			State:   formatConditionState(c.State),
			Message: c.Message,
		})
	}

	if opt.Events {
		filter := fmt.Sprintf(`resource.type="cloud_run_job" AND resource.labels.job_name="%s" AND resource.labels.location="%s"`, d.config.Job, d.config.Location)
		result.RecentEvents = d.fetchRecentEvents(ctx, filter, recentEventsLimit)
	}

	if opt.JSON {
		return printJSON(result)
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

	if len(result.RecentEvents) > 0 {
		fmt.Println()
		bold.Println("Recent Events:")
		for _, ev := range result.RecentEvents {
			fmt.Printf("  %s [%s] %s\n", ev.Timestamp, ev.Severity, ev.Message)
		}
	}

	return nil
}

// recentEventsLimit is how many of the most recent log entries `status --events` reports.
const recentEventsLimit = 10

// fetchRecentEvents returns up to limit of the most recent log entries matching filter.
// Entries are requested newest-first; logadmin lists oldest-to-newest by default, which
// would otherwise surface the oldest entries in the log rather than the recent ones.
func (d *App) fetchRecentEvents(ctx context.Context, filter string, limit int) []EventItemResult {
	if d.logAdminClient == nil {
		return nil
	}
	it := d.logAdminClient.Entries(ctx, logadmin.Filter(filter), logadmin.NewestFirst())
	var events []EventItemResult
	for len(events) < limit {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			d.LogWarn("failed to fetch recent events", "error", err)
			break
		}
		msg := fmt.Sprintf("%v", entry.Payload)
		events = append(events, EventItemResult{
			Timestamp: entry.Timestamp.Format("15:04:05"),
			Severity:  entry.Severity.String(),
			Message:   msg,
		})
	}
	return events
}

func formatConditionState(state runpb.Condition_State) string {
	switch state {
	case runpb.Condition_CONDITION_SUCCEEDED:
		return "READY"
	case runpb.Condition_CONDITION_FAILED:
		return "FAILED"
	case runpb.Condition_CONDITION_PENDING:
		return "PENDING"
	default:
		return "UNKNOWN"
	}
}
