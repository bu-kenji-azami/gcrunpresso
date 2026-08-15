package gcrunpresso_test

import (
	"context"
	"testing"
	"time"

	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/api/option"
)

func TestAppResourcePaths(t *testing.T) {
	ctx := context.Background()
	app, err := gcrunpresso.New(ctx, &gcrunpresso.Option{
		Project:       "my-test-proj",
		Location:      "asia-northeast1",
		Service:       "web-api",
		Job:           "batch-migrate",
		ClientOptions: []option.ClientOption{option.WithoutAuthentication()},
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}
	defer app.Close()

	if app.ResourceLocationPath() != "projects/my-test-proj/locations/asia-northeast1" {
		t.Errorf("unexpected location path: %s", app.ResourceLocationPath())
	}
	if app.ResourceServicePath() != "projects/my-test-proj/locations/asia-northeast1/services/web-api" {
		t.Errorf("unexpected service path: %s", app.ResourceServicePath())
	}
	if app.ResourceJobPath() != "projects/my-test-proj/locations/asia-northeast1/jobs/batch-migrate" {
		t.Errorf("unexpected job path: %s", app.ResourceJobPath())
	}
	if app.ResourceRevisionPath("web-api-00001-abc") != "projects/my-test-proj/locations/asia-northeast1/services/web-api/revisions/web-api-00001-abc" {
		t.Errorf("unexpected revision path: %s", app.ResourceRevisionPath("web-api-00001-abc"))
	}
	if app.ResourceExecutionPath("batch-migrate-xyz") != "projects/my-test-proj/locations/asia-northeast1/jobs/batch-migrate/executions/batch-migrate-xyz" {
		t.Errorf("unexpected execution path: %s", app.ResourceExecutionPath("batch-migrate-xyz"))
	}
}

func TestAppTimeout(t *testing.T) {
	ctx := context.Background()
	app, err := gcrunpresso.New(ctx, &gcrunpresso.Option{
		Project:       "my-test-proj",
		Location:      "asia-northeast1",
		Service:       "web-api",
		Timeout:       15 * time.Minute,
		ClientOptions: []option.ClientOption{option.WithoutAuthentication()},
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}
	defer app.Close()

	if app.Timeout() != 15*time.Minute {
		t.Errorf("expected timeout 15m, got %v", app.Timeout())
	}
}

func TestAppClientAccessors(t *testing.T) {
	ctx := context.Background()
	app, err := gcrunpresso.New(ctx, &gcrunpresso.Option{
		Project:       "my-test-proj",
		Location:      "asia-northeast1",
		Service:       "web-api",
		ClientOptions: []option.ClientOption{option.WithoutAuthentication()},
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}
	defer app.Close()

	if app.ServicesClient() == nil {
		t.Error("expected ServicesClient, got nil")
	}
	if app.JobsClient() == nil {
		t.Error("expected JobsClient, got nil")
	}
	if app.RevisionsClient() == nil {
		t.Error("expected RevisionsClient, got nil")
	}
	if app.ExecutionsClient() == nil {
		t.Error("expected ExecutionsClient, got nil")
	}
	if app.TasksClient() == nil {
		t.Error("expected TasksClient, got nil")
	}
	if app.LogTailClient() == nil {
		t.Error("expected LogTailClient, got nil")
	}
	if app.LogAdminClient() == nil {
		t.Error("expected LogAdminClient, got nil")
	}
	if app.SecretManagerClient() == nil {
		t.Error("expected SecretManagerClient, got nil")
	}
	if app.ArtifactRegistryClient() == nil {
		t.Error("expected ArtifactRegistryClient, got nil")
	}
}
