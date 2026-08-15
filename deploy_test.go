package gcrunpresso_test

import (
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestDeployOption(t *testing.T) {
	opt := gcrunpresso.DeployOption{
		Tag:       "v1",
		NoTraffic: true,
		DryRun:    true,
	}
	if opt.Tag != "v1" {
		t.Errorf("expected tag v1, got %s", opt.Tag)
	}
	if !opt.NoTraffic {
		t.Error("expected no traffic true, got false")
	}
}
