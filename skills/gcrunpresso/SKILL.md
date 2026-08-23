---
name: gcrunpresso
description: Comprehensive operational guide and CLI reference for gcrunpresso (Google Cloud Run deployment and management tool).
---

# gcrunpresso Operational Skill

`gcrunpresso` is a CLI tool for managing Google Cloud Run Services and Jobs as code.

## Key Concepts & Conventions

1. **Manifest Schema**:
   - `service.yaml` and `job.yaml` use the **Google Cloud Run Admin API v2** schema (`runpb.Service`, `runpb.Job`).
   - Do NOT use Knative `serving.knative.dev/v1` format.
2. **Configuration File**:
   - `gcrunpresso.yml` defines `project`, `location`, `service` (or `job`), and `service_definition` (or `job_definition`).
3. **Template & Jsonnet Evaluation**:
   - Template functions: `{{ env "KEY" "default" }}`, `{{ must_env "KEY" }}`, `{{ secretmanager_ref "name" }}`, `{{ tfstate "path" }}`, `{{ tfstate_output "name" }}`.
   - Jsonnet native functions: `std.native("env")`, `std.native("must_env")`, `std.native("secretmanager_ref")`, `std.native("tfstate")`.

## Common CLI Patterns

### Deploying a Service
```bash
# Preview diff first
gcrunpresso diff

# Deploy with 100% traffic shift
gcrunpresso deploy

# Deploy canary with tag (no base traffic shift)
gcrunpresso deploy --no-traffic --tag candidate
```

### Running a Job
```bash
# Trigger job and follow logs in real-time
gcrunpresso run

# Override container arguments and environment
gcrunpresso run --override-args "--migrate" --override-env "DRY_RUN=false"
```

`run` propagates the container's exit status: `0` when every task succeeded, `2`-`255` for the
highest non-zero container exit code across tasks, and `1` for either an internal gcrunpresso
failure or a task that exited `1`. Branch on the exit code rather than parsing log text.
`--parallelism` was removed; set `template.parallelism` in the Job definition instead.

### Rolling Back a Service
```bash
# Immediate 100% traffic switch to preceding healthy revision
gcrunpresso rollback

# Revert service template specification
gcrunpresso rollback --revert-service
```

### Inspection & Observability
```bash
# Check service status, active URLs, traffic splits, and conditions (--json supported)
gcrunpresso status
gcrunpresso status --json
gcrunpresso status --events

# List revisions (--json supported)
gcrunpresso revisions
gcrunpresso revisions --json

# List job executions (--json supported)
gcrunpresso executions
gcrunpresso executions --json

# Verify images and secrets (--json supported)
gcrunpresso verify
gcrunpresso verify --json

# Render evaluated YAML (--json supported)
gcrunpresso render
gcrunpresso render --json
```
