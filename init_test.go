package gcrunpresso_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/kayac/gcrunpresso/v2"
)

func TestWarnPlaintextSecretsInEnv(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "test-project",
		Location: "asia-northeast1",
		Service:  "test-svc",
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}
	app.SetLogger(logger)

	envs := []*runpb.EnvVar{
		{
			Name: "NORMAL_CONFIG",
			Values: &runpb.EnvVar_Value{
				Value: "production",
			},
		},
		{
			Name: "DATABASE_PASSWORD",
			Values: &runpb.EnvVar_Value{
				Value: "supersecret",
			},
		},
		{
			Name: "API_KEY",
			Values: &runpb.EnvVar_Value{
				Value: "xyz123",
			},
		},
		{
			Name: "DATABASE_URL",
			Values: &runpb.EnvVar_Value{
				Value: "postgres://user:pass@localhost:5432/db",
			},
		},
		{
			Name: "SECRET_KEY_PROPER",
			Values: &runpb.EnvVar_ValueSource{
				ValueSource: &runpb.EnvVarSource{
					SecretKeyRef: &runpb.SecretKeySelector{
						Secret:  "projects/test-project/secrets/my-secret",
						Version: "latest",
					},
				},
			},
		},
	}

	app.WarnPlaintextSecretsInEnv("app-container", envs)

	logOut := buf.String()
	if !strings.Contains(logOut, "DATABASE_PASSWORD") {
		t.Errorf("expected warning for DATABASE_PASSWORD, got: %s", logOut)
	}
	if !strings.Contains(logOut, "API_KEY") {
		t.Errorf("expected warning for API_KEY, got: %s", logOut)
	}
	if !strings.Contains(logOut, "DATABASE_URL") {
		t.Errorf("expected warning for DATABASE_URL, got: %s", logOut)
	}
	if strings.Contains(logOut, "SECRET_KEY_PROPER") {
		t.Errorf("expected NO warning for secretKeyRef SECRET_KEY_PROPER, got: %s", logOut)
	}
	if strings.Contains(logOut, "NORMAL_CONFIG") {
		t.Errorf("expected NO warning for NORMAL_CONFIG, got: %s", logOut)
	}
}

type mockInitServicesAPI struct{}

func (m *mockInitServicesAPI) GetService(ctx context.Context, req *runpb.GetServiceRequest, opts ...gax.CallOption) (*runpb.Service, error) {
	return &runpb.Service{
		Name: req.Name,
		Template: &runpb.RevisionTemplate{
			Containers: []*runpb.Container{
				{Image: "gcr.io/test-project/app:v1"},
			},
		},
	}, nil
}
func (m *mockInitServicesAPI) CreateService(ctx context.Context, req *runpb.CreateServiceRequest, opts ...gax.CallOption) (*run.CreateServiceOperation, error) {
	return nil, nil
}
func (m *mockInitServicesAPI) UpdateService(ctx context.Context, req *runpb.UpdateServiceRequest, opts ...gax.CallOption) (*run.UpdateServiceOperation, error) {
	return nil, nil
}
func (m *mockInitServicesAPI) DeleteService(ctx context.Context, req *runpb.DeleteServiceRequest, opts ...gax.CallOption) (*run.DeleteServiceOperation, error) {
	return nil, nil
}

type mockInitJobsAPI struct{}

func (m *mockInitJobsAPI) GetJob(ctx context.Context, req *runpb.GetJobRequest, opts ...gax.CallOption) (*runpb.Job, error) {
	return &runpb.Job{
		Name: req.Name,
		Template: &runpb.ExecutionTemplate{
			Template: &runpb.TaskTemplate{
				Containers: []*runpb.Container{
					{Image: "gcr.io/test-project/job:v1"},
				},
			},
		},
	}, nil
}
func (m *mockInitJobsAPI) CreateJob(ctx context.Context, req *runpb.CreateJobRequest, opts ...gax.CallOption) (*run.CreateJobOperation, error) {
	return nil, nil
}
func (m *mockInitJobsAPI) UpdateJob(ctx context.Context, req *runpb.UpdateJobRequest, opts ...gax.CallOption) (*run.UpdateJobOperation, error) {
	return nil, nil
}
func (m *mockInitJobsAPI) DeleteJob(ctx context.Context, req *runpb.DeleteJobRequest, opts ...gax.CallOption) (*run.DeleteJobOperation, error) {
	return nil, nil
}
func (m *mockInitJobsAPI) RunJob(ctx context.Context, req *runpb.RunJobRequest, opts ...gax.CallOption) (gcrunpresso.JobRunOperation, error) {
	return nil, nil
}

func TestInitServiceFileMode0600(t *testing.T) {
	tmpDir := t.TempDir()

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "test-project",
		Location: "asia-northeast1",
		Service:  "test-svc",
	},
		gcrunpresso.WithServicesClient(&mockInitServicesAPI{}),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	err = app.Init(t.Context(), gcrunpresso.InitOption{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("unexpected error during init: %v", err)
	}

	svcPath := filepath.Join(tmpDir, "service.yaml")
	fi, err := os.Stat(svcPath)
	if err != nil {
		t.Fatalf("failed to stat service.yaml: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("expected service.yaml mode 0600, got %o", perm)
	}

	cfgPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	fi, err = os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("failed to stat gcrunpresso.yml: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("expected gcrunpresso.yml mode 0600, got %o", perm)
	}
}

// os.WriteFile applies its mode only when it CREATES the file, so overwriting an
// existing world-readable definition under --force would silently keep 0644.
func TestInitServiceForceOverwriteTightensFileMode(t *testing.T) {
	tmpDir := t.TempDir()

	svcPath := filepath.Join(tmpDir, "service.yaml")
	cfgPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	for _, p := range []string{svcPath, cfgPath} {
		if err := os.WriteFile(p, []byte("stale: true\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0644); err != nil {
			t.Fatal(err)
		}
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "test-project",
		Location: "asia-northeast1",
		Service:  "test-svc",
	},
		gcrunpresso.WithServicesClient(&mockInitServicesAPI{}),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	if err := app.Init(t.Context(), gcrunpresso.InitOption{Dir: tmpDir, Force: true}); err != nil {
		t.Fatalf("unexpected error during init --force: %v", err)
	}

	for _, p := range []string{svcPath, cfgPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", p, err)
		}
		if perm := fi.Mode().Perm(); perm != 0600 {
			t.Errorf("expected %s mode 0600 after --force overwrite, got %o", filepath.Base(p), perm)
		}
	}
}

func TestInitJobFileMode0600(t *testing.T) {
	tmpDir := t.TempDir()

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "test-project",
		Location: "asia-northeast1",
		Job:      "test-job",
	},
		gcrunpresso.WithJobsClient(&mockInitJobsAPI{}),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	err = app.Init(t.Context(), gcrunpresso.InitOption{
		Dir: tmpDir,
	})
	if err != nil {
		t.Fatalf("unexpected error during init: %v", err)
	}

	jobPath := filepath.Join(tmpDir, "job.yaml")
	fi, err := os.Stat(jobPath)
	if err != nil {
		t.Fatalf("failed to stat job.yaml: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("expected job.yaml mode 0600, got %o", perm)
	}

	cfgPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	fi, err = os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("failed to stat gcrunpresso.yml: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Errorf("expected gcrunpresso.yml mode 0600, got %o", perm)
	}
}
