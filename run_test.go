package gcrunpresso_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuildJobOverridesEmpty(t *testing.T) {
	overrides, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides != nil {
		t.Errorf("expected nil overrides when no flags provided, got %v", overrides)
	}
}

func TestBuildJobOverridesTasks(t *testing.T) {
	overrides, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{
		Tasks: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides == nil {
		t.Fatal("expected overrides, got nil")
	}
	if overrides.TaskCount != 5 {
		t.Errorf("expected TaskCount 5, got %d", overrides.TaskCount)
	}
}

func TestBuildJobOverridesArgsAndEnv(t *testing.T) {
	overrides, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{
		OverrideArgs: []string{"--migrate", "--verbose"},
		OverrideEnv:  []string{"ENVIRONMENT=production", "DEBUG=true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides == nil || len(overrides.ContainerOverrides) == 0 {
		t.Fatal("expected container overrides, got nil or empty")
	}

	co := overrides.ContainerOverrides[0]
	if len(co.Args) != 2 || co.Args[0] != "--migrate" || co.Args[1] != "--verbose" {
		t.Errorf("unexpected args: %v", co.Args)
	}
	if len(co.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(co.Env))
	}
	if co.Env[0].Name != "ENVIRONMENT" || co.Env[0].GetValue() != "production" {
		t.Errorf("unexpected env var 0: %v", co.Env[0])
	}
	if co.Env[1].Name != "DEBUG" || co.Env[1].GetValue() != "true" {
		t.Errorf("unexpected env var 1: %v", co.Env[1])
	}
}

func TestBuildJobOverridesInvalidEnv(t *testing.T) {
	_, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{
		OverrideEnv: []string{"INVALID_ENV_WITHOUT_EQUALS"},
	})
	if err == nil {
		t.Fatal("expected error for invalid env syntax, got nil")
	}
}

func TestExtractMaxExitCode(t *testing.T) {
	tasks := []*runpb.Task{
		{
			LastAttemptResult: &runpb.TaskAttemptResult{
				ExitCode: 0,
			},
		},
		{
			LastAttemptResult: &runpb.TaskAttemptResult{
				ExitCode: 42,
			},
		},
		{
			LastAttemptResult: &runpb.TaskAttemptResult{
				ExitCode: 7,
			},
		},
	}

	err := gcrunpresso.ExtractMaxExitCode(tasks, nil)
	if err == nil {
		t.Fatal("expected non-nil error when task exit code is 42")
	}
	var exitCoder interface{ ExitCode() int }
	if !errors.As(err, &exitCoder) {
		t.Fatalf("expected ExitCodeError, got %T", err)
	}
	if exitCoder.ExitCode() != 42 {
		t.Errorf("expected exit code 42, got %d", exitCoder.ExitCode())
	}
}

type mockRunJobsAPI struct {
	expectedJobPath string
	runOp           gcrunpresso.JobRunOperation
}

func (m *mockRunJobsAPI) GetJob(ctx context.Context, req *runpb.GetJobRequest, opts ...gax.CallOption) (*runpb.Job, error) {
	if m.expectedJobPath != "" && req.Name != m.expectedJobPath {
		return nil, fmt.Errorf("unexpected job path: %s", req.Name)
	}
	return &runpb.Job{Name: req.Name}, nil
}
func (m *mockRunJobsAPI) CreateJob(ctx context.Context, req *runpb.CreateJobRequest, opts ...gax.CallOption) (*run.CreateJobOperation, error) {
	return nil, nil
}
func (m *mockRunJobsAPI) UpdateJob(ctx context.Context, req *runpb.UpdateJobRequest, opts ...gax.CallOption) (*run.UpdateJobOperation, error) {
	return nil, nil
}
func (m *mockRunJobsAPI) DeleteJob(ctx context.Context, req *runpb.DeleteJobRequest, opts ...gax.CallOption) (*run.DeleteJobOperation, error) {
	return nil, nil
}
func (m *mockRunJobsAPI) RunJob(ctx context.Context, req *runpb.RunJobRequest, opts ...gax.CallOption) (gcrunpresso.JobRunOperation, error) {
	if m.expectedJobPath != "" && req.Name != m.expectedJobPath {
		return nil, fmt.Errorf("unexpected job path: %s", req.Name)
	}
	if m.runOp != nil {
		return m.runOp, nil
	}
	return &fakeJobRunOperation{
		mdResults: []*runpb.Execution{{Name: req.Name + "/executions/run-1"}},
	}, nil
}

// fakeJobRunOperation is a working JobRunOperation fake: Metadata returns queued
// results in order (the last repeats), so tests script how quickly an execution
// name becomes visible instead of relying on a zero-value operation that panics.
type fakeJobRunOperation struct {
	name      string
	mdResults []*runpb.Execution
	pollErr   error

	mdCalls   int
	pollCalls int
}

func (f *fakeJobRunOperation) Name() string { return f.name }

func (f *fakeJobRunOperation) Metadata() (*runpb.Execution, error) {
	f.mdCalls++
	if len(f.mdResults) == 0 {
		return nil, fmt.Errorf("metadata unavailable")
	}
	if f.mdCalls > len(f.mdResults) {
		return f.mdResults[len(f.mdResults)-1], nil
	}
	return f.mdResults[f.mdCalls-1], nil
}

func (f *fakeJobRunOperation) Poll(ctx context.Context) (*runpb.Execution, error) {
	f.pollCalls++
	return nil, f.pollErr
}

type mockRunExecutionsAPI struct {
	exec             *runpb.Execution
	getExecutionErr  error
	expectedExecPath string // exact match; rejects fabricated ID-less paths

	mu           sync.Mutex
	getExecCalls int
}

func (m *mockRunExecutionsAPI) GetExecution(ctx context.Context, req *runpb.GetExecutionRequest, opts ...gax.CallOption) (*runpb.Execution, error) {
	m.mu.Lock()
	m.getExecCalls++
	m.mu.Unlock()
	if m.getExecutionErr != nil {
		return nil, m.getExecutionErr
	}
	if m.expectedExecPath != "" && req.Name != m.expectedExecPath {
		return nil, fmt.Errorf("unexpected execution path: %s (expected %s)", req.Name, m.expectedExecPath)
	}
	return m.exec, nil
}

func (m *mockRunExecutionsAPI) getExecCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getExecCalls
}
func (m *mockRunExecutionsAPI) ListExecutions(ctx context.Context, req *runpb.ListExecutionsRequest) ([]*runpb.Execution, error) {
	return []*runpb.Execution{m.exec}, nil
}

type mockRunTasksAPI struct {
	tasks                []*runpb.Task
	expectedParentPrefix string
}

func (m *mockRunTasksAPI) ListTasks(ctx context.Context, req *runpb.ListTasksRequest) ([]*runpb.Task, error) {
	if m.expectedParentPrefix != "" && !strings.HasPrefix(req.Parent, m.expectedParentPrefix) {
		return nil, fmt.Errorf("unexpected tasks parent: %s", req.Parent)
	}
	return m.tasks, nil
}

func TestAppRunExitCodePropagation(t *testing.T) {
	jobPath := "projects/p/locations/l/jobs/my-job"
	mockJobs := &mockRunJobsAPI{expectedJobPath: jobPath}
	mockExecs := &mockRunExecutionsAPI{
		expectedExecPath: jobPath + "/executions/run-1",
		exec: &runpb.Execution{
			Name:        jobPath + "/executions/run-1",
			FailedCount: 1,
			TaskCount:   1,
			Conditions: []*runpb.Condition{
				{Type: "Completed", State: runpb.Condition_CONDITION_FAILED, Message: "task failed"},
			},
		},
	}
	mockTasks := &mockRunTasksAPI{
		expectedParentPrefix: jobPath + "/executions",
		tasks: []*runpb.Task{
			{
				Name: jobPath + "/executions/exec-1/tasks/0",
				LastAttemptResult: &runpb.TaskAttemptResult{
					ExitCode: 42,
				},
			},
		},
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Job:      "my-job",
	},
		gcrunpresso.WithJobsClient(mockJobs),
		gcrunpresso.WithExecutionsClient(mockExecs),
		gcrunpresso.WithTasksClient(mockTasks),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	err = app.Run(t.Context(), gcrunpresso.RunOption{
		Wait:   true,
		Follow: false,
	})
	if err == nil {
		t.Fatal("expected non-nil error from app.Run, got nil")
	}

	var exitCoder interface{ ExitCode() int }
	if !errors.As(err, &exitCoder) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitCoder.ExitCode() != 42 {
		t.Errorf("expected exit code 42, got %d", exitCoder.ExitCode())
	}
}

func TestResolveExecutionPath(t *testing.T) {
	jobPath := "projects/p/locations/l/jobs/my-job"

	t.Run("nil operation returns error", func(t *testing.T) {
		_, err := gcrunpresso.ResolveExecutionPath(t.Context(), nil, jobPath)
		if err == nil {
			t.Error("expected error for nil operation, got nil")
		}
	})

	t.Run("immediate metadata success", func(t *testing.T) {
		op := &fakeJobRunOperation{mdResults: []*runpb.Execution{{Name: jobPath + "/executions/e-now"}}}
		path, err := gcrunpresso.ResolveExecutionPath(t.Context(), op, jobPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != jobPath+"/executions/e-now" {
			t.Errorf("expected immediate execution path, got %s", path)
		}
		if op.pollCalls != 0 {
			t.Errorf("expected no polling when metadata resolves immediately, got %d", op.pollCalls)
		}
	})

	t.Run("poll succeeds after metadata lags", func(t *testing.T) {
		op := &fakeJobRunOperation{mdResults: []*runpb.Execution{
			{Name: jobPath},                          // not populated yet
			{Name: jobPath},                          // still missing at first tick
			{Name: jobPath + "/executions/e-polled"}, // visible after Poll
		}}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		path, err := gcrunpresso.ResolveExecutionPath(ctx, op, jobPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != jobPath+"/executions/e-polled" {
			t.Errorf("expected polled execution path, got %s", path)
		}
		if op.pollCalls != 1 {
			t.Errorf("expected exactly 1 poll before success, got %d", op.pollCalls)
		}
	})

	t.Run("operation name used when metadata never populates", func(t *testing.T) {
		opName := jobPath + "/executions/e-name/operations/op-1"
		op := &fakeJobRunOperation{name: opName} // Metadata errors forever
		ctx, cancel := context.WithTimeout(t.Context(), 1200*time.Millisecond)
		defer cancel()
		path, err := gcrunpresso.ResolveExecutionPath(ctx, op, jobPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != opName {
			t.Errorf("expected execution-bearing operation name %s, got %s", opName, path)
		}
	})

	t.Run("timeout without execution-bearing name is an error", func(t *testing.T) {
		op := &fakeJobRunOperation{name: "projects/p/locations/l/operations/op-x"}
		ctx, cancel := context.WithTimeout(t.Context(), 1200*time.Millisecond)
		defer cancel()
		_, err := gcrunpresso.ResolveExecutionPath(ctx, op, jobPath)
		if err == nil {
			t.Fatal("expected error when the execution name never resolves")
		}
		if !strings.Contains(err.Error(), "execution name not available") {
			t.Errorf("expected resolution failure error, got %v", err)
		}
	})
}

func TestMonitorExecutionPermissionDeniedFastFail(t *testing.T) {
	jobPath := "projects/p/locations/l/jobs/my-job"
	mockJobs := &mockRunJobsAPI{expectedJobPath: jobPath}
	mockExecs := &mockRunExecutionsAPI{
		getExecutionErr: status.Error(codes.PermissionDenied, "caller does not have permission"),
	}
	mockTasks := &mockRunTasksAPI{
		expectedParentPrefix: jobPath + "/executions",
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Job:      "my-job",
	},
		gcrunpresso.WithJobsClient(mockJobs),
		gcrunpresso.WithExecutionsClient(mockExecs),
		gcrunpresso.WithTasksClient(mockTasks),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	start := time.Now()
	err = app.Run(t.Context(), gcrunpresso.RunOption{
		Wait:   true,
		Follow: false,
	})
	if err == nil {
		t.Fatal("expected error for permission denied, got nil")
	}
	if !strings.Contains(err.Error(), "permission") && !strings.Contains(err.Error(), "fatal") {
		t.Errorf("expected fatal permission error message, got %v", err)
	}
	if calls := mockExecs.getExecCallCount(); calls != 1 {
		t.Errorf("permission denied must fail on the first poll without retries, got %d GetExecution calls", calls)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Errorf("permission fast-fail took %s; a retry-loop regression would hang for the full timeout", elapsed)
	}
}

func TestMonitorExecutionPersistentErrorFailsFast(t *testing.T) {
	prev := *gcrunpresso.ExecutionPollInterval
	*gcrunpresso.ExecutionPollInterval = 5 * time.Millisecond
	defer func() { *gcrunpresso.ExecutionPollInterval = prev }()

	jobPath := "projects/p/locations/l/jobs/my-job"
	mockExecs := &mockRunExecutionsAPI{
		getExecutionErr: status.Error(codes.Unavailable, "backend unavailable"),
	}
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Job:      "my-job",
	},
		gcrunpresso.WithJobsClient(&mockRunJobsAPI{expectedJobPath: jobPath}),
		gcrunpresso.WithExecutionsClient(mockExecs),
		gcrunpresso.WithTasksClient(&mockRunTasksAPI{}),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	err = app.Run(t.Context(), gcrunpresso.RunOption{Wait: true})
	if err == nil {
		t.Fatal("expected persistent poll failure to surface an error")
	}
	if strings.Contains(err.Error(), "timeout or canceled") {
		t.Errorf("persistent errors must surface their cause, not a timeout: %v", err)
	}
	if !strings.Contains(err.Error(), "consecutive times") {
		t.Errorf("expected consecutive-failure error, got %v", err)
	}
	if calls := mockExecs.getExecCallCount(); calls > 11 {
		t.Errorf("expected fail-fast within 10 polls, got %d calls", calls)
	}
}

func TestMonitorExecutionNotFoundBoundStillApplies(t *testing.T) {
	prev := *gcrunpresso.ExecutionPollInterval
	*gcrunpresso.ExecutionPollInterval = 5 * time.Millisecond
	defer func() { *gcrunpresso.ExecutionPollInterval = prev }()

	jobPath := "projects/p/locations/l/jobs/my-job"
	mockExecs := &mockRunExecutionsAPI{
		getExecutionErr: status.Error(codes.NotFound, "execution not found"),
	}
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Job:      "my-job",
	},
		gcrunpresso.WithJobsClient(&mockRunJobsAPI{expectedJobPath: jobPath}),
		gcrunpresso.WithExecutionsClient(mockExecs),
		gcrunpresso.WithTasksClient(&mockRunTasksAPI{}),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	err = app.Run(t.Context(), gcrunpresso.RunOption{Wait: true})
	if err == nil {
		t.Fatal("expected NotFound bound to trigger an error")
	}
	if !strings.Contains(err.Error(), "may be wrong") {
		t.Errorf("expected wrong-path diagnosis, got %v", err)
	}
	if calls := mockExecs.getExecCallCount(); calls > 6 {
		t.Errorf("expected NotFound bound at 5 polls, got %d calls", calls)
	}
}
