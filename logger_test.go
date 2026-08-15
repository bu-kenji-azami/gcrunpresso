package gcrunpresso_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/gcrunpresso/v2"
)

var logLevels = []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

func TestCommonLogger(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		for _, level := range logLevels {
			b := new(bytes.Buffer)
			gcrunpresso.SetLogFormat(format)
			logger := gcrunpresso.NewLogger(b)
			gcrunpresso.LogLevel.Set(level)
			gcrunpresso.SetLogger(logger)

			gcrunpresso.LogDebug("test %s", level)
			gcrunpresso.LogInfo("test message", "level", level.String())
			gcrunpresso.LogWarn("test message", "level", level.String())
			gcrunpresso.LogError("test %s", level)
			t.Log(b.String())
		}
	}
}

func TestLogger(t *testing.T) {
	app := &gcrunpresso.App{}
	for _, format := range []string{"text", "json"} {
		for _, level := range logLevels {
			b := new(bytes.Buffer)
			gcrunpresso.SetLogFormat(format)
			logger := gcrunpresso.NewLogger(b)
			gcrunpresso.LogLevel.Set(level)
			app.SetLogger(logger)

			app.LogDebug("test %s", "test")
			app.LogInfo("test message", "key", "value")
			app.LogWarn("test message", "key", "value")
			app.LogError("test %s", "test")
			t.Log(b.String())
		}
	}
}

func TestStructuredLogOutput(t *testing.T) {
	t.Run("json format has structured fields", func(t *testing.T) {
		b := new(bytes.Buffer)
		gcrunpresso.SetLogFormat("json")
		logger := gcrunpresso.NewLogger(b)
		gcrunpresso.LogLevel.Set(slog.LevelInfo)

		app := &gcrunpresso.App{}
		app.SetLogger(logger.With("project", "test-project", "service", "test-svc"))

		app.LogInfo("deployment created", "operation_id", "op-ABC123", "url", "https://example.com")

		var m map[string]any
		if err := json.Unmarshal(b.Bytes(), &m); err != nil {
			t.Fatalf("failed to parse JSON log: %s\noutput: %s", err, b.String())
		}
		if diff := cmp.Diff("deployment created", m["msg"]); diff != "" {
			t.Errorf("msg mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("op-ABC123", m["operation_id"]); diff != "" {
			t.Errorf("operation_id mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("https://example.com", m["url"]); diff != "" {
			t.Errorf("url mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("test-project", m["project"]); diff != "" {
			t.Errorf("project mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("test-svc", m["service"]); diff != "" {
			t.Errorf("service mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("text format has key:value attrs", func(t *testing.T) {
		b := new(bytes.Buffer)
		gcrunpresso.SetLogFormat("text")
		logger := gcrunpresso.NewLogger(b)
		gcrunpresso.LogLevel.Set(slog.LevelInfo)

		app := &gcrunpresso.App{}
		app.SetLogger(logger)

		app.LogInfo("starting deploy", "dry_run", true)

		output := b.String()
		if !strings.Contains(output, "[dry_run:true]") {
			t.Errorf("expected [dry_run:true] in text output: %s", output)
		}
		if !strings.Contains(output, "starting deploy") {
			t.Errorf("expected message in text output: %s", output)
		}
	})

	t.Run("plain message without attrs", func(t *testing.T) {
		b := new(bytes.Buffer)
		gcrunpresso.SetLogFormat("json")
		logger := gcrunpresso.NewLogger(b)
		gcrunpresso.LogLevel.Set(slog.LevelInfo)

		app := &gcrunpresso.App{}
		app.SetLogger(logger)

		app.LogInfo("DRY RUN OK")

		var m map[string]any
		if err := json.Unmarshal(b.Bytes(), &m); err != nil {
			t.Fatalf("failed to parse JSON log: %s\noutput: %s", err, b.String())
		}
		if diff := cmp.Diff("DRY RUN OK", m["msg"]); diff != "" {
			t.Errorf("msg mismatch (-want +got):\n%s", diff)
		}
	})
}
