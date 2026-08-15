package gcrunpresso_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/gcrunpresso/v2"
)

type labelsTestSuite struct {
	src    string
	labels map[string]string
	ok     bool
}

var labelsTestSuites = []labelsTestSuite{
	{
		src:    "",
		labels: map[string]string{},
		ok:     true,
	},
	{
		src:    "env=production",
		labels: map[string]string{"env": "production"},
		ok:     true,
	},
	{
		src:    "env=production,tier=backend",
		labels: map[string]string{"env": "production", "tier": "backend"},
		ok:     true,
	},
	{
		src: "invalid-label",
		ok:  false,
	},
}

func TestParseLabels(t *testing.T) {
	for _, ts := range labelsTestSuites {
		labels, err := gcrunpresso.ParseLabels(ts.src)
		if ts.ok {
			if err != nil {
				t.Errorf("unexpected error for %q: %v", ts.src, err)
				continue
			}
			if diff := cmp.Diff(ts.labels, labels); diff != "" {
				t.Errorf("labels mismatch for %q (-want +got):\n%s", ts.src, diff)
			}
		} else {
			if err == nil {
				t.Errorf("expected error for %q, got nil", ts.src)
			}
		}
	}
}

func TestMap2str(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want string
	}{
		{map[string]string{"b": "2", "a": "1"}, "a=1,b=2"},
		{map[string]string{"foo": "bar", "baz": "qux", "quux": "corge"}, "baz=qux,foo=bar,quux=corge"},
		{map[string]string{}, ""},
	}

	for _, c := range cases {
		got := gcrunpresso.Map2str(c.in)
		if got != c.want {
			t.Errorf("map2str(%v) == %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSleepContext(t *testing.T) {
	t.Run("normal sleep", func(t *testing.T) {
		ctx := t.Context()
		start := time.Now()
		duration := 100 * time.Millisecond

		gcrunpresso.SleepContext(ctx, duration)

		elapsed := time.Since(start)
		if elapsed < duration {
			t.Errorf("Sleep duration was too short: %v, expected at least %v", elapsed, duration)
		}
	})

	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		start := time.Now()
		duration := 1 * time.Second

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		gcrunpresso.SleepContext(ctx, duration)

		elapsed := time.Since(start)
		if elapsed >= duration {
			t.Errorf("Sleep did not respect context cancellation, duration: %v", elapsed)
		}
		if elapsed < 50*time.Millisecond {
			t.Errorf("Sleep returned too quickly, expected at least some delay: %v", elapsed)
		}
	})
}
