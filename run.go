package gcrunpresso

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	loggingpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"cloud.google.com/go/logging/logadmin"
	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RunOption struct {
	OverrideArgs []string `name:"override-args" aliases:"args" help:"container argument overrides" default:""`
	OverrideEnv  []string `name:"override-env" aliases:"env" help:"environment variable overrides (KEY=VALUE)" default:""`
	Tasks        int32    `help:"number of tasks to execute" default:"0"`
	Wait         bool     `help:"wait for execution to complete" default:"true" negatable:""`
	Follow       bool     `help:"stream logs in real-time" default:"true" negatable:""`
	DryRun       bool     `help:"dry run" default:"false"`
}

func (d *App) Run(ctx context.Context, opt RunOption) error {
	d.LogInfo("starting job execution", withDryRun(opt.DryRun, "job", d.config.Job)...)

	if d.config.Job == "" {
		return fmt.Errorf("run command is only applicable to Cloud Run Jobs, but job is not configured")
	}

	overrides, err := BuildJobOverrides(opt)
	if err != nil {
		return err
	}

	req := &runpb.RunJobRequest{
		Name:      d.ResourceJobPath(),
		Overrides: overrides,
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN: planned job execution request", "job", req.Name)
		if overrides != nil {
			d.LogInfo("overrides", "task_count", overrides.TaskCount, "containers", len(overrides.ContainerOverrides))
		}
		return nil
	}

	d.LogInfo("triggering job execution", "job", d.config.Job)
	op, err := d.jobsClient.RunJob(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to run job %s: %w", d.config.Job, err)
	}

	// Extract execution name from operation metadata
	execPath := ""
	if md, err := op.Metadata(); err == nil && md != nil && md.Name != "" {
		execPath = md.Name
	}
	if execPath == "" || !strings.Contains(execPath, "/executions/") {
		// Poll operation briefly to get execution name if not immediately populated
		pollCtx, cancelPoll := context.WithTimeout(ctx, 10*time.Second)
		defer cancelPoll()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
	pollLoop:
		for {
			select {
			case <-pollCtx.Done():
				break pollLoop
			case <-ticker.C:
				if md, err := op.Metadata(); err == nil && md != nil && md.Name != "" {
					execPath = md.Name
					break pollLoop
				}
				if _, err := op.Poll(pollCtx); err == nil {
					if md, err := op.Metadata(); err == nil && md != nil && md.Name != "" {
						execPath = md.Name
						break pollLoop
					}
				}
			}
		}
	}
	if execPath == "" {
		if strings.Contains(op.Name(), "/executions/") {
			execPath = op.Name()
		} else {
			execPath = fmt.Sprintf("%s/executions/%s", d.ResourceJobPath(), arnToName(op.Name()))
		}
	}
	execName := arnToName(execPath)
	d.LogInfo("job execution started", "execution", execName, "path", execPath)

	if !opt.Wait {
		d.LogInfo("job triggered asynchronously (--wait=false specified)", "execution", execName, "path", execPath)
		return nil
	}

	// Execution timeout
	timeout := d.Timeout()
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	logStreamCtx, cancelLogStream := context.WithCancel(ctx)

	if opt.Follow {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.streamExecutionLogs(logStreamCtx, execName)
		}()
	}

	// Monitor execution status
	execErr := d.monitorExecution(execCtx, execPath)

	// Stop log streaming with safe drain window derived from root ctx (N6)
	if opt.Follow {
		drainCtx, cancelDrain := context.WithTimeout(ctx, 30*time.Second)
		defer cancelDrain()

		drainDone := make(chan struct{})
		go func() {
			time.Sleep(3 * time.Second)
			cancelLogStream()
			wg.Wait()
			close(drainDone)
		}()

		select {
		case <-drainCtx.Done():
			cancelLogStream()
			wg.Wait()
		case <-drainDone:
		}
	} else {
		cancelLogStream()
	}

	// Fetch task exit codes via tasksClient
	taskCtx, cancelTask := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTask()

	tasks, listErr := d.tasksClient.ListTasks(taskCtx, &runpb.ListTasksRequest{
		Parent: execPath,
	})
	if listErr != nil {
		d.LogWarn("failed to list tasks for execution exit codes", "error", listErr)
	}

	return ExtractMaxExitCode(tasks, execErr)
}

// ExtractMaxExitCode computes the highest non-zero exit code across tasks.
func ExtractMaxExitCode(tasks []*runpb.Task, execErr error) error {
	var maxExitCode int32
	var hasTaskExitCode bool

	for _, t := range tasks {
		if t != nil && t.LastAttemptResult != nil {
			hasTaskExitCode = true
			if t.LastAttemptResult.ExitCode > maxExitCode {
				maxExitCode = t.LastAttemptResult.ExitCode
			}
		}
	}

	if execErr != nil {
		if maxExitCode > 0 {
			return &ExitCodeError{Code: int(maxExitCode), Err: execErr}
		}
		return &ExitCodeError{Code: 1, Err: execErr}
	}

	if maxExitCode > 0 {
		return &ExitCodeError{Code: int(maxExitCode), Err: fmt.Errorf("job completed with task exit code %d", maxExitCode)}
	}

	if !hasTaskExitCode && execErr != nil {
		return &ExitCodeError{Code: 1, Err: execErr}
	}

	return nil
}

// BuildJobOverrides converts RunOption into runpb.RunJobRequest_Overrides.
func BuildJobOverrides(opt RunOption) (*runpb.RunJobRequest_Overrides, error) {
	args := opt.OverrideArgs
	env := opt.OverrideEnv
	if len(args) == 0 && len(env) == 0 && opt.Tasks == 0 {
		return nil, nil
	}

	overrides := &runpb.RunJobRequest_Overrides{}
	if opt.Tasks > 0 {
		overrides.TaskCount = opt.Tasks
	}

	var containerOverride *runpb.RunJobRequest_Overrides_ContainerOverride
	if len(args) > 0 {
		if containerOverride == nil {
			containerOverride = &runpb.RunJobRequest_Overrides_ContainerOverride{}
		}
		containerOverride.Args = args
	}

	if len(env) > 0 {
		if containerOverride == nil {
			containerOverride = &runpb.RunJobRequest_Overrides_ContainerOverride{}
		}
		for _, e := range env {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid environment variable override %q: must be KEY=VALUE", e)
			}
			containerOverride.Env = append(containerOverride.Env, &runpb.EnvVar{
				Name: kv[0],
				Values: &runpb.EnvVar_Value{
					Value: kv[1],
				},
			})
		}
	}

	if containerOverride != nil {
		overrides.ContainerOverrides = []*runpb.RunJobRequest_Overrides_ContainerOverride{containerOverride}
	}

	return overrides, nil
}

func (d *App) monitorExecution(ctx context.Context, execPath string) error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	d.LogInfo("monitoring job execution status", "path", execPath)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout or canceled while waiting for execution %s: %w", execPath, ctx.Err())
		case <-ticker.C:
			exec, err := d.executionsClient.GetExecution(ctx, &runpb.GetExecutionRequest{
				Name: execPath,
			})
			if err != nil {
				d.LogWarn("failed to poll execution status, retrying", "error", err)
				continue
			}

			// Check conditions
			if !exec.Reconciling {
				for _, c := range exec.Conditions {
					if c.Type == "Completed" || c.Type == "Ready" {
						if c.State == runpb.Condition_CONDITION_SUCCEEDED {
							d.LogInfo("job execution completed successfully", "succeeded", exec.SucceededCount, "task_count", exec.TaskCount)
							return nil
						}
						if c.State == runpb.Condition_CONDITION_FAILED {
							return fmt.Errorf("job execution failed: %s: %s (succeeded: %d, failed: %d, cancelled: %d)", c.Type, c.Message, exec.SucceededCount, exec.FailedCount, exec.CancelledCount)
						}
					}
				}
				if exec.FailedCount > 0 {
					return fmt.Errorf("job execution failed with %d failed tasks out of %d", exec.FailedCount, exec.TaskCount)
				}
				if exec.SucceededCount == exec.TaskCount && exec.TaskCount > 0 {
					d.LogInfo("job execution succeeded", "task_count", exec.TaskCount)
					return nil
				}
			}
		}
	}
}

// streamExecutionLogs streams logs in real-time, attempting TailLogEntries first, then falling back to logadmin.
func (d *App) streamExecutionLogs(ctx context.Context, execName string) {
	filter := fmt.Sprintf(`resource.type="cloud_run_job" AND resource.labels.job_name="%s" AND resource.labels.location="%s" AND labels."run.googleapis.com/execution_name"="%s"`,
		d.config.Job, d.config.Location, execName)

	d.LogInfo("starting log stream", "filter", filter)

	var lastSeen time.Time
	// Primary: TailLogEntries gRPC streaming (KTD3)
	err := d.tailLogStream(ctx, filter, &lastSeen)
	if err == nil || ctx.Err() != nil {
		return
	}

	d.LogWarn("TailLogEntries stream ended or unsupported, falling back to logadmin polling", "error", err)
	// Fallback: logadmin polling
	d.pollLogAdmin(ctx, filter, lastSeen)
}

func (d *App) tailLogStream(ctx context.Context, filter string, lastSeen *time.Time) error {
	if d.logTailClient == nil {
		return fmt.Errorf("log tail client not initialized")
	}

	stream, err := d.logTailClient.TailLogEntries(ctx)
	if err != nil {
		return fmt.Errorf("failed to open tail stream: %w", err)
	}
	defer stream.CloseSend()

	req := &loggingpb.TailLogEntriesRequest{
		ResourceNames: []string{"projects/" + d.config.Project},
		Filter:        filter,
		BufferWindow:  durationpb.New(0),
	}
	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send tail request: %w", err)
	}

	dim := color.New(color.Faint)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			resp, err := stream.Recv()
			if err == io.EOF || ctx.Err() != nil {
				return nil
			}
			if err != nil {
				return err
			}
			for _, entry := range resp.Entries {
				if entry.Timestamp != nil && entry.Timestamp.AsTime().After(*lastSeen) {
					*lastSeen = entry.Timestamp.AsTime()
				}
				printLogEntry(dim, entry.Timestamp, entry.Labels["run.googleapis.com/task_index"], extractPayload(entry))
			}
		}
	}
}

func (d *App) pollLogAdmin(ctx context.Context, filter string, lastSeen time.Time) {
	if d.logAdminClient == nil {
		return
	}

	dim := color.New(color.Faint)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentFilter := filter
			if !lastSeen.IsZero() {
				currentFilter = fmt.Sprintf(`%s AND timestamp > "%s"`, filter, lastSeen.Format(time.RFC3339Nano))
			}

			it := d.logAdminClient.Entries(ctx, logadmin.Filter(currentFilter))
			for {
				entry, err := it.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					d.LogWarn("failed to poll log entries", "error", err)
					break
				}
				if entry.Timestamp.After(lastSeen) {
					lastSeen = entry.Timestamp
				}
				taskIdx := entry.Labels["run.googleapis.com/task_index"]
				msg := fmt.Sprintf("%v", entry.Payload)
				printLogEntry(dim, timestamppb.New(entry.Timestamp), taskIdx, msg)
			}
		}
	}
}

func printLogEntry(dim *color.Color, ts *timestamppb.Timestamp, taskIdx, msg string) {
	timeStr := ""
	if ts != nil {
		timeStr = ts.AsTime().Format("15:04:05.000")
	}
	if taskIdx != "" {
		fmt.Printf("%s [%s] %s\n", dim.Sprint(timeStr), taskIdx, msg)
	} else {
		fmt.Printf("%s %s\n", dim.Sprint(timeStr), msg)
	}
}

func extractPayload(entry *loggingpb.LogEntry) string {
	if entry == nil {
		return ""
	}
	switch p := entry.Payload.(type) {
	case *loggingpb.LogEntry_TextPayload:
		return p.TextPayload
	case *loggingpb.LogEntry_JsonPayload:
		return fmt.Sprintf("%v", p.JsonPayload.AsMap())
	default:
		return fmt.Sprintf("%v", entry.Payload)
	}
}
