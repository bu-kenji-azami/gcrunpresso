package gcrunpresso_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kayac/gcrunpresso/v2"
)

func TestDefaultConfig(t *testing.T) {
	conf := gcrunpresso.NewDefaultConfig()
	if conf.Timeout.Duration != 10*time.Minute {
		t.Errorf("expected default timeout 10m, got %v", conf.Timeout.Duration)
	}
}

func TestConfigRestrict(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	t.Setenv("CLOUDSDK_COMPUTE_REGION", "asia-northeast1")

	conf := gcrunpresso.NewDefaultConfig()
	opt := &gcrunpresso.Option{
		Service: "test-service",
	}

	if err := conf.Restrict(opt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conf.Project != "test-project" {
		t.Errorf("expected project test-project, got %s", conf.Project)
	}
	if conf.Location != "asia-northeast1" {
		t.Errorf("expected location asia-northeast1, got %s", conf.Location)
	}
	if conf.Service != "test-service" {
		t.Errorf("expected service test-service, got %s", conf.Service)
	}
}

func TestConfigRestrictMissingServiceAndJob(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	t.Setenv("CLOUDSDK_COMPUTE_REGION", "asia-northeast1")

	conf := gcrunpresso.NewDefaultConfig()
	opt := &gcrunpresso.Option{}

	if err := conf.Restrict(opt); err == nil {
		t.Fatal("expected error when neither service nor job is set, got nil")
	}
}

func TestLoadServiceDefinitionWithTemplateEnv(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("APP_IMAGE", "gcr.io/my-project/my-app:v2.0.0")
	t.Setenv("MY_ENV", "staging")

	serviceYAML := `
template:
  containers:
    - image: {{ env "APP_IMAGE" }}
      env:
        - name: "ENVIRONMENT"
          value: {{ must_env "MY_ENV" }}
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-project
location: asia-northeast1
service: my-service
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := gcrunpresso.New(context.Background(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}

	svc, err := app.LoadServiceDefinition("")
	if err != nil {
		t.Fatalf("failed to load service definition: %v", err)
	}

	if svc.Template == nil || len(svc.Template.Containers) == 0 {
		t.Fatal("expected template containers, got nil or empty")
	}
	if svc.Template.Containers[0].Image != "gcr.io/my-project/my-app:v2.0.0" {
		t.Errorf("unexpected image: %s", svc.Template.Containers[0].Image)
	}
	if svc.Template.Containers[0].Env[0].GetValue() != "staging" {
		t.Errorf("unexpected env value: %s", svc.Template.Containers[0].Env[0].GetValue())
	}
}

func TestLoadJobDefinitionWithJsonnet(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("TAG", "v1.2.3")

	jobJsonnet := `
local env = std.native("env");
{
  template: {
    template: {
      containers: [
        {
          image: "gcr.io/my-project/batch:" + env("TAG", "latest"),
          args: ["--run"],
        },
      ],
      maxRetries: 2,
    },
  },
}
`
	jobPath := filepath.Join(tmpDir, "job.jsonnet")
	if err := os.WriteFile(jobPath, []byte(jobJsonnet), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-project
location: asia-northeast1
job: my-job
job_definition: job.jsonnet
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := gcrunpresso.New(context.Background(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}

	job, err := app.LoadJobDefinition("")
	if err != nil {
		t.Fatalf("failed to load job definition: %v", err)
	}

	if job.Template.Template.Containers[0].Image != "gcr.io/my-project/batch:v1.2.3" {
		t.Errorf("unexpected job image: %s", job.Template.Template.Containers[0].Image)
	}
	if job.Template.Template.GetMaxRetries() != 2 {
		t.Errorf("unexpected max retries: %d", job.Template.Template.GetMaxRetries())
	}
}
