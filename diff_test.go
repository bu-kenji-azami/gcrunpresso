package gcrunpresso_test

import (
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestDiffOption(t *testing.T) {
	opt := gcrunpresso.DiffOption{
		Unified: true,
	}
	if !opt.Unified {
		t.Error("expected unified true, got false")
	}
}
