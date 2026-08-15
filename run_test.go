package gcrunpresso_test

import (
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestBuildJobOverridesEmpty(t *testing.T) {
	overrides, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides != nil {
		t.Errorf("expected nil overrides when no flags provided, got %v", overrides)
	}
}

func TestBuildJobOverridesTasks(t *testing.T) {
	overrides, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{
		Tasks: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides == nil {
		t.Fatal("expected overrides, got nil")
	}
	if overrides.TaskCount != 5 {
		t.Errorf("expected TaskCount 5, got %d", overrides.TaskCount)
	}
}

func TestBuildJobOverridesArgsAndEnv(t *testing.T) {
	overrides, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{
		Args: []string{"--migrate", "--verbose"},
		Env:  []string{"ENVIRONMENT=production", "DEBUG=true"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overrides == nil || len(overrides.ContainerOverrides) == 0 {
		t.Fatal("expected container overrides, got nil or empty")
	}

	co := overrides.ContainerOverrides[0]
	if len(co.Args) != 2 || co.Args[0] != "--migrate" || co.Args[1] != "--verbose" {
		t.Errorf("unexpected args: %v", co.Args)
	}
	if len(co.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(co.Env))
	}
	if co.Env[0].Name != "ENVIRONMENT" || co.Env[0].GetValue() != "production" {
		t.Errorf("unexpected env var 0: %v", co.Env[0])
	}
	if co.Env[1].Name != "DEBUG" || co.Env[1].GetValue() != "true" {
		t.Errorf("unexpected env var 1: %v", co.Env[1])
	}
}

func TestBuildJobOverridesInvalidEnv(t *testing.T) {
	_, err := gcrunpresso.BuildJobOverrides(gcrunpresso.RunOption{
		Env: []string{"INVALID_ENV_WITHOUT_EQUALS"},
	})
	if err == nil {
		t.Fatal("expected error for invalid env syntax, got nil")
	}
}
