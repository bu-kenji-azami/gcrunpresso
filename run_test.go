package gcrunpresso_test

import (
	"context"
	"errors"
	"testing"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/kayac/gcrunpresso/v2"
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

type mockRunJobsAPI struct{}

func (m *mockRunJobsAPI) GetJob(ctx context.Context, req *runpb.GetJobRequest, opts ...gax.CallOption) (*runpb.Job, error) {
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
	return nil, nil
}

type mockRunExecutionsAPI struct {
	exec *runpb.Execution
}

func (m *mockRunExecutionsAPI) GetExecution(ctx context.Context, req *runpb.GetExecutionRequest, opts ...gax.CallOption) (*runpb.Execution, error) {
	return m.exec, nil
}
func (m *mockRunExecutionsAPI) ListExecutions(ctx context.Context, req *runpb.ListExecutionsRequest) ([]*runpb.Execution, error) {
	return []*runpb.Execution{m.exec}, nil
}

type mockRunTasksAPI struct {
	tasks []*runpb.Task
}

func (m *mockRunTasksAPI) ListTasks(ctx context.Context, req *runpb.ListTasksRequest) ([]*runpb.Task, error) {
	return m.tasks, nil
}

func TestAppRunExitCodePropagation(t *testing.T) {
	mockJobs := &mockRunJobsAPI{}
	mockExecs := &mockRunExecutionsAPI{
		exec: &runpb.Execution{
			Name:        "projects/p/locations/l/jobs/my-job/executions/exec-1",
			FailedCount: 1,
			TaskCount:   1,
			Conditions: []*runpb.Condition{
				{Type: "Completed", State: runpb.Condition_CONDITION_FAILED, Message: "task failed"},
			},
		},
	}
	mockTasks := &mockRunTasksAPI{
		tasks: []*runpb.Task{
			{
				Name: "projects/p/locations/l/jobs/my-job/executions/exec-1/tasks/0",
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
