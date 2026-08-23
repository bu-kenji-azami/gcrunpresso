# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gcrunpresso is a deployment management tool for Google Cloud Run (Services and Jobs) written in Go. It manages Cloud Run resources through declarative configuration files (YAML, JSON, or Jsonnet).

## Git Workflow

- Default branch: `v2`
- Do not commit directly to `v2`. Always create a feature branch first.
- When changing `action.yml`, test via the GitHub Actions composite action.

## Build Commands

```bash
# Build the binary
make

# Install to GOPATH/bin
make install

# Run all tests
make test

# Run a single test
go test -v -run TestFunctionName ./...

# Format code (run before committing)
go fmt ./...

# Build release packages
make packages
```

## Architecture

### Core Components

- **App** (`gcrunpresso.go`): Main application struct that holds GCP GAPIC clients (Cloud Run Services/Jobs/Tasks/Revisions/Executions, Secret Manager, Artifact Registry, Cloud Logging) and orchestrates operations
- **Config** (`config.go`): Configuration loading and validation with 3-tier precedence (`CLI > YAML > ENV`). Supports YAML, JSON, and Jsonnet formats with template functions
- **CLI** (`cli.go`, `cliv2.go`): Command-line interface using Kong. Each subcommand (deploy, run, rollback, scale, status, revisions, executions, diff, verify, init, render, wait, delete) has its own option struct and handler

### Command Structure

Commands are defined as option structs in their respective files:
- `deploy.go` - Deploy/update Cloud Run Services and Jobs with safety guards and traffic management
- `run.go` - Execute Cloud Run Jobs with real-time log streaming and container overrides
- `rollback.go` - Rollback Cloud Run Service traffic or revert template to preceding healthy revision
- `scale.go` - Adjust min/max instance scaling
- `status.go` - Display service/job readiness, traffic split, and recent Cloud Logging events
- `revisions.go` - List service revisions with traffic allocation and health status
- `executions.go` - List job executions with task counts and completion status
- `diff.go` - Show semantic diff between local definition and remote Cloud Run resource
- `verify.go` - Validate container image availability (Artifact Registry) and secrets (Secret Manager)
- `init.go` - Generate configuration and YAML definitions from existing Cloud Run resources

### Template System

Definition files support Go template syntax via `github.com/kayac/go-config`:
- `{{ env "VAR" "default" }}` - Environment variable with default
- `{{ must_env "VAR" }}` - Required environment variable
- `{{ secretmanager_ref "secret-name" }}` - Returns Secret Manager resource string (`projects/{project}/secrets/{name}`) for `valueSource.secretKeyRef`

### Plugin System (`plugin.go`)

Plugins extend template functions and Jsonnet native functions:
- `secretmanager` - Secret Manager pure string reference generation (`secretmanager_ref`)
- `tfstate` - Terraform state lookups
- `external` - Execute external commands

### Google Cloud SDK Usage

Uses Google Cloud Go GAPIC SDK (`cloud.google.com/go/run/apiv2`, `cloud.google.com/go/secretmanager/apiv1`, `cloud.google.com/go/artifactregistry/apiv1`, `cloud.google.com/go/logging/apiv2`). Key clients are initialized in `New()` with option injection (`AppOption`) for testability.

## Testing

Tests use the standard Go testing framework. Test files are colocated with implementation files (`*_test.go`).

```bash
# Run specific test file
go test -v ./... -run TestConfig

# Run with race detection
go test -race ./...
```

- Use `github.com/google/go-cmp/cmp` for value comparisons in tests
  ```go
  if diff := cmp.Diff(want, got); diff != "" {
      t.Errorf("mismatch (-want +got):\n%s", diff)
  }
  ```
- Use `t.Context()` instead of `context.Background()` in tests

## Code Style

- Use `go fmt ./...` before committing
- Error messages should be lowercase and not end with punctuation
- Use `fmt.Errorf("failed to X: %w", err)` for error wrapping
- Logging uses `slog` via helper methods (LogInfo, LogWarn, LogDebug, LogError)
