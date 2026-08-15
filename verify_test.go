package gcrunpresso_test

import (
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestVerifyOption(t *testing.T) {
	opt := gcrunpresso.VerifyOption{
		DryRun: true,
	}
	if !opt.DryRun {
		t.Error("expected dry-run true, got false")
	}
}
