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
gcrunpresso run --args "--migrate" --env "DRY_RUN=false"
```

### Rolling Back a Service
```bash
# Immediate 100% traffic switch to preceding healthy revision
gcrunpresso rollback

# Revert service template specification
gcrunpresso rollback --revert-service
```

### Inspection & Observability
```bash
# Check service status, active URLs, traffic splits, and conditions
gcrunpresso status

# List revisions
gcrunpresso revisions

# List job executions
gcrunpresso executions

# Verify images and secrets
gcrunpresso verify

# Render evaluated YAML
gcrunpresso render
```
