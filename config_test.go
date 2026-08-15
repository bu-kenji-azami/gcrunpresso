package gcrunpresso_test

import (
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
