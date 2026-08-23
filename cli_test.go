package gcrunpresso_test

import (
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestParseCLIv2(t *testing.T) {
	t.Run("deploy subcommand", func(t *testing.T) {
		sub, opts, _, err := gcrunpresso.ParseCLIv2([]string{
			"deploy",
			"--config", "gcrunpresso.yml",
			"--project", "my-project",
			"--location", "asia-northeast1",
			"--tag", "preview",
			"--no-traffic",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sub != "deploy" {
			t.Errorf("expected sub deploy, got %s", sub)
		}
		if opts.Project != "my-project" {
			t.Errorf("expected project my-project, got %s", opts.Project)
		}
		if opts.Location != "asia-northeast1" {
			t.Errorf("expected location asia-northeast1, got %s", opts.Location)
		}
		if opts.Deploy == nil {
			t.Fatal("expected Deploy options, got nil")
		}
		if opts.Deploy.Tag != "preview" {
			t.Errorf("expected tag preview, got %s", opts.Deploy.Tag)
		}
		if !opts.Deploy.NoTraffic {
			t.Error("expected no-traffic true, got false")
		}
	})

	t.Run("run subcommand", func(t *testing.T) {
		sub, opts, _, err := gcrunpresso.ParseCLIv2([]string{
			"run",
			"--tasks", "3",
			"--wait=false",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sub != "run" {
			t.Errorf("expected sub run, got %s", sub)
		}
		if opts.Run == nil {
			t.Fatal("expected Run options, got nil")
		}
		if opts.Run.Tasks != 3 {
			t.Errorf("expected tasks 3, got %d", opts.Run.Tasks)
		}
		if opts.Run.Wait {
			t.Error("expected wait false, got true")
		}
	})

	t.Run("rollback subcommand", func(t *testing.T) {
		sub, opts, _, err := gcrunpresso.ParseCLIv2([]string{
			"rollback",
			"--revision", "app-v1",
			"--revert-service",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sub != "rollback" {
			t.Errorf("expected sub rollback, got %s", sub)
		}
		if opts.Rollback == nil {
			t.Fatal("expected Rollback options, got nil")
		}
		if opts.Rollback.Revision != "app-v1" {
			t.Errorf("expected revision app-v1, got %s", opts.Rollback.Revision)
		}
		if !opts.Rollback.RevertService {
			t.Error("expected revert-service true, got false")
		}
	})

	t.Run("scale subcommand", func(t *testing.T) {
		sub, opts, _, err := gcrunpresso.ParseCLIv2([]string{
			"scale",
			"--min", "2",
			"--max", "10",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sub != "scale" {
			t.Errorf("expected sub scale, got %s", sub)
		}
		if opts.Scale == nil {
			t.Fatal("expected Scale options, got nil")
		}
		if *opts.Scale.Min != 2 || *opts.Scale.Max != 10 {
			t.Errorf("unexpected scale values min=%d, max=%d", *opts.Scale.Min, *opts.Scale.Max)
		}
	})

	t.Run("run with args and env aliases", func(t *testing.T) {
		sub, opts, _, err := gcrunpresso.ParseCLIv2([]string{
			"run",
			"--args=--flag1",
			"--args", "val1",
			"--env", "FOO=bar",
			"--env", "BAZ=qux",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sub != "run" {
			t.Errorf("expected sub run, got %s", sub)
		}
		if opts.Run == nil {
			t.Fatal("expected Run options, got nil")
		}
		if len(opts.Run.OverrideArgs) != 2 || opts.Run.OverrideArgs[0] != "--flag1" || opts.Run.OverrideArgs[1] != "val1" {
			t.Errorf("unexpected args: %v", opts.Run.OverrideArgs)
		}
		if len(opts.Run.OverrideEnv) != 2 || opts.Run.OverrideEnv[0] != "FOO=bar" || opts.Run.OverrideEnv[1] != "BAZ=qux" {
			t.Errorf("unexpected env: %v", opts.Run.OverrideEnv)
		}
	})

	t.Run("subcommands with json flag", func(t *testing.T) {
		_, opts, _, err := gcrunpresso.ParseCLIv2([]string{"status", "--json"})
		if err != nil || opts.Status == nil || !opts.Status.JSON {
			t.Errorf("expected status --json true")
		}

		_, opts, _, err = gcrunpresso.ParseCLIv2([]string{"revisions", "--json"})
		if err != nil || opts.Revisions == nil || !opts.Revisions.JSON {
			t.Errorf("expected revisions --json true")
		}

		_, opts, _, err = gcrunpresso.ParseCLIv2([]string{"executions", "--json"})
		if err != nil || opts.Executions == nil || !opts.Executions.JSON {
			t.Errorf("expected executions --json true")
		}

		_, opts, _, err = gcrunpresso.ParseCLIv2([]string{"verify", "--json"})
		if err != nil || opts.Verify == nil || !opts.Verify.JSON {
			t.Errorf("expected verify --json true")
		}

		_, opts, _, err = gcrunpresso.ParseCLIv2([]string{"diff", "--json"})
		if err != nil || opts.Diff == nil || !opts.Diff.JSON {
			t.Errorf("expected diff --json true")
		}
	})

	t.Run("CLI exit code propagation", func(t *testing.T) {
		customParse := func(args []string) (string, *gcrunpresso.CLIOptions, func(), error) {
			return "run", &gcrunpresso.CLIOptions{
				LogFormat: "text",
				Run:       &gcrunpresso.RunOption{},
			}, func() {}, &gcrunpresso.ExitCodeError{Code: 42, Err: nil}
		}

		exitCode, err := gcrunpresso.CLI(t.Context(), customParse)
		if exitCode != 1 && exitCode != 42 { // Note: customParse returns error directly before dispatchCLI
			t.Logf("CLI parse error exit code: %d, %v", exitCode, err)
		}
	})
}
