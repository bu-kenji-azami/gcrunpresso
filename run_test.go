package gcrunpresso_test

import (
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestRunOption(t *testing.T) {
	opt := gcrunpresso.RunOption{
		Tasks: 2,
		Wait:  true,
	}
	if opt.Tasks != 2 {
		t.Errorf("expected tasks 2, got %d", opt.Tasks)
	}
	if !opt.Wait {
		t.Error("expected wait true, got false")
	}
}
