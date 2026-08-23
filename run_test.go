package gcrunpresso_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
func (m *mockRunJobsAPI) RunJob(ctx context.Context, req *runpb.RunJobRequest, opts ...gax.CallOption) (*run.RunJobOperation, error) {
	if m.expectedJobPath != "" && req.Name != m.expectedJobPath {
		return nil, fmt.Errorf("unexpected job path: %s", req.Name)
	}
	return &run.RunJobOperation{}, nil
}

type mockRunExecutionsAPI struct {
	exec               *runpb.Execution
	getExecutionErr    error
	expectedExecPrefix string
}

func (m *mockRunExecutionsAPI) GetExecution(ctx context.Context, req *runpb.GetExecutionRequest, opts ...gax.CallOption) (*runpb.Execution, error) {
	if m.getExecutionErr != nil {
		return nil, m.getExecutionErr
	}
	if m.expectedExecPrefix != "" && !strings.HasPrefix(req.Name, m.expectedExecPrefix) {
		return nil, fmt.Errorf("unexpected execution path: %s (expected prefix %s)", req.Name, m.expectedExecPrefix)
	}
	return m.exec, nil
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
		expectedExecPrefix: jobPath + "/executions",
		exec: &runpb.Execution{
			Name:        jobPath + "/executions/exec-1",
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
	t.Run("nil operation returns error", func(t *testing.T) {
		_, err := gcrunpresso.ResolveExecutionPath(t.Context(), nil, "projects/p/locations/l/jobs/my-job")
		if err == nil {
			t.Error("expected error for nil operation, got nil")
		}
	})

	t.Run("operation fallback path", func(t *testing.T) {
		op := &run.RunJobOperation{}
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		path, err := gcrunpresso.ResolveExecutionPath(ctx, op, "projects/p/locations/l/jobs/my-job")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(path, "projects/p/locations/l/jobs/my-job/executions") {
			t.Errorf("expected execution path to start with job executions, got %s", path)
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
}
