# gcrunpresso

`gcrunpresso` is a deployment and management tool for **Google Cloud Run** (Services & Jobs).

(pronounced the same as "espresso" ☕)

`gcrunpresso` allows you to manage Cloud Run Services and Jobs as code in YAML, JSON, or Jsonnet files, enabling version control and infrastructure-as-code practices with a smooth developer experience inspired by `ecspresso`. You can generate configuration files from existing Cloud Run services or jobs using `gcrunpresso init`.

Definition files support Go template syntax and Jsonnet native functions to embed environment variables, Terraform state values (GCS & local), Secret Manager secrets, and external command outputs. Before deploying, you can preview changes with `diff` and validate referenced container images and secrets with `verify`. If something goes wrong, `rollback` helps you safely revert traffic or specifications to a previous revision.

---

## Table of Contents

- [Features](#features)
- [Install](#install)
- [Quick Start](#quick-start)
- [Configuration File (`gcrunpresso.yml`)](#configuration-file-gcrunpressoyml)
- [Service Definition (`service.yaml`)](#service-definition-serviceyaml)
- [Job Definition (`job.yaml`)](#job-definition-jobyaml)
- [Subcommands](#subcommands)
  - [deploy](#deploy)
  - [run](#run)
  - [rollback](#rollback)
  - [scale](#scale)
  - [status](#status)
  - [revisions](#revisions)
  - [executions](#executions)
  - [diff](#diff)
  - [init](#init)
  - [verify](#verify)
  - [render](#render)
  - [wait](#wait)
  - [delete](#delete)
- [Template Syntax & Jsonnet Support](#template-syntax--jsonnet-support)
- [Plugins](#plugins)
  - [tfstate (GCS & Local)](#tfstate-plugin)
  - [secretmanager](#secretmanager-plugin)
  - [external](#external-plugin)
- [Authentication & Impersonation](#authentication--impersonation)
- [CI/CD Integration](#cicd-integration)
- [LICENSE](#license)

---

## Features

- **Protobuf-First Manifest Model**: Native compatibility with Google Cloud Run Admin API v2 schema (`cloud.google.com/go/run/apiv2/runpb`). Unknown fields and misspelled keys are rejected strictly upon loading.
- **Unified Service & Job Management**: Full support for long-running web services and batch/scheduled jobs.
- **Flexible Traffic Splitting**: Gradual rollouts, tag-based canary deployments (`--tag`), zero-traffic preview deployments (`--no-traffic`), and arbitrary traffic split specifications (`--traffic "latest=20,v1=80"`).
- **Safety Guards**: Detects and prevents silent resets of critical server-managed fields (`ingress`, `template.vpc_access`, `template.service_account`).
- **Real-Time Dual-Tier Log Streaming**: Zero-delay gRPC log streaming via Cloud Logging v2 `TailLogEntries` with automatic fallback to `logadmin` polling.
- **Intelligent Semantic Diff**: Compares local manifests with remote Cloud Run state while automatically filtering server-generated read-only fields.
- **Pre-flight Verification**: Validates container image existence (Artifact Registry) and Secret Manager secrets before deployment.
- **Rich Plugin Ecosystem**: Seamless integration with Terraform State (`gs://` and local), GCP Secret Manager, and external CLI tools.

---

## Install

### Homebrew (macOS and Linux)

```bash
brew install kayac/tap/gcrunpresso
```

### Go Install

```bash
go install github.com/kayac/gcrunpresso/v2/cmd/gcrunpresso@latest
```

### GitHub Releases

Download pre-built binary archives for your OS and architecture from [Releases](https://github.com/kayac/gcrunpresso/releases).

### GitHub Actions

```yaml
- uses: kayac/gcrunpresso@v2
  with:
    version: latest
```

---

## Quick Start

### 1. Initialize from existing Cloud Run resource

```bash
# Export from existing Cloud Run Service
gcrunpresso init \
  --project my-gcp-project \
  --location asia-northeast1 \
  --service web-api \
  --dir .
```

This creates:
- `gcrunpresso.yml`: Core tool configuration
- `service.yaml`: Cloud Run Service definition (API v2 schema)

### 2. Preview Diff

```bash
gcrunpresso diff
```

### 3. Deploy

```bash
gcrunpresso deploy
```

---

## Configuration File (`gcrunpresso.yml`)

```yaml
project: my-gcp-project
location: asia-northeast1
service: web-api
service_definition: service.yaml

# Optional: Service Account Impersonation
# impersonate_service_account: deployer@my-gcp-project.iam.gserviceaccount.com

# Optional: Default timeout for LRO and readiness polling
timeout: 10m

# Optional: Plugins
plugins:
  - name: tfstate
    config:
      url: gs://my-terraform-state-bucket/terraform.tfstate
  - name: secretmanager
```

For Cloud Run Jobs, replace `service` and `service_definition` with:

```yaml
project: my-gcp-project
location: asia-northeast1
job: batch-migrate
job_definition: job.yaml
```

---

## Service Definition (`service.yaml`)

`gcrunpresso` uses the canonical Google Cloud Run Admin API v2 schema:

```yaml
template:
  containers:
    - image: asia-northeast1-docker.pkg.dev/my-gcp-project/app-repo/web-api:{{ env "TAG" "latest" }}
      env:
        - name: ENVIRONMENT
          value: production
        - name: DATABASE_PASSWORD
          valueSource:
            secretKeyRef:
              secret: db-password
              version: latest
      resources:
        limits:
          cpu: "1000m"
          memory: "512Mi"
  scaling:
    minInstanceCount: 1
    maxInstanceCount: 10
  serviceAccount: sa-web-api@my-gcp-project.iam.gserviceaccount.com

traffic:
  - type: TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST
    percent: 100
```

---

## Job Definition (`job.yaml`)

```yaml
template:
  taskCount: 1
  parallelism: 1
  template:
    containers:
      - image: asia-northeast1-docker.pkg.dev/my-gcp-project/app-repo/batch:{{ env "TAG" "latest" }}
        args:
          - "--migrate"
    maxRetries: 3
    timeout: "600s"
    serviceAccount: sa-batch@my-gcp-project.iam.gserviceaccount.com
```

---

## Subcommands

### `deploy`
Deploy Cloud Run Service or Job from local definitions.

```bash
# Standard deployment (100% traffic to new revision)
gcrunpresso deploy

# Deploy with revision tag (canary / preview URL)
gcrunpresso deploy --tag preview

# Deploy without shifting main traffic (pinned to existing revision)
gcrunpresso deploy --no-traffic --tag candidate

# Custom traffic split
gcrunpresso deploy --traffic "latest=20,web-api-00001=80"

# Dry run preview
gcrunpresso deploy --dry-run
```

### `run`
Execute Cloud Run Job with optional runtime parameter overrides and real-time log streaming.

```bash
# Execute job and follow logs in real-time
gcrunpresso run

# Override container arguments, environment variables, and task count
gcrunpresso run --args "--seed" --env "FORCE=true" --tasks 3

# Trigger job asynchronously without waiting
gcrunpresso run --no-wait --no-follow
```

### `rollback`
Rollback Service traffic or specification to a healthy revision.

```bash
# Route 100% traffic to preceding healthy revision
gcrunpresso rollback

# Route traffic to a specific revision
gcrunpresso rollback --revision web-api-00002-abc

# Revert service template to match target revision specification and deploy as new revision
gcrunpresso rollback --revision web-api-00002-abc --revert-service
```

### `scale`
Scale Service min and max instances without rewriting the local manifest.

```bash
gcrunpresso scale --min 2 --max 20
```

### `status`
Display detailed service or job health, active URLs, traffic distribution, and conditions.

```bash
gcrunpresso status
```

### `revisions`
List revisions for a Cloud Run Service with traffic percentage, tags, images, and health status.

```bash
gcrunpresso revisions
```

### `executions`
List execution history for a Cloud Run Job.

```bash
gcrunpresso executions
```

### `diff`
Display a unified colorized diff between local definition files and the remote Cloud Run resource.

```bash
gcrunpresso diff

# Exit with code 1 if differences exist (ideal for CI drift detection)
gcrunpresso diff --exit-code
```

### `verify`
Validate referenced container images (Artifact Registry) and Secret Manager secrets.

```bash
gcrunpresso verify
```

### `render`
Render configuration files with all template functions and Jsonnet expressions evaluated.

```bash
gcrunpresso render
gcrunpresso render --json
```

### `delete`
Delete Cloud Run Service or Job with confirmation prompt.

```bash
gcrunpresso delete
gcrunpresso delete --force
```

---

## Template Syntax & Jsonnet Support

### Go Template Functions

- `{{ env "KEY" "default_value" }}`: Read environment variable with fallback.
- `{{ must_env "KEY" }}`: Require environment variable (aborts if unset).
- `{{ secretmanager_ref "secret-name" }}`: Returns Secret Manager resource path (`projects/PROJECT/secrets/secret-name`) for use with `valueSource.secretKeyRef`.
- `{{ tfstate "module.vpc.network_name" }}`: Lookup value from Terraform state.
- `{{ tfstate_output "service_url" }}`: Lookup output from Terraform state.

### Jsonnet Native Functions

You can use `.jsonnet` files for `gcrunpresso.jsonnet`, `service.jsonnet`, or `job.jsonnet`:

```jsonnet
local env = std.native("env");
local secretmanager_ref = std.native("secretmanager_ref");

{
  template: {
    containers: [
      {
        image: "asia-northeast1-docker.pkg.dev/my-proj/repo/app:" + env("TAG", "v1.0.0"),
        env: [
          {
            name: "APP_SECRET",
            valueSource: {
              secretKeyRef: {
                secret: secretmanager_ref("my-app-secret"),
                version: "latest",
              },
            },
          },
        ],
      },
    ],
  },
}
```

---

## Plugins

### `tfstate` Plugin
Read outputs from Terraform state files stored locally or in Google Cloud Storage (`gs://`).

```yaml
plugins:
  - name: tfstate
    config:
      url: gs://my-bucket/terraform.tfstate
      # optional: true
```

### `secretmanager` Plugin
Resolves Google Cloud Secret Manager resource references (`projects/PROJECT/secrets/NAME`) without accessing secret payloads. The Cloud Run runtime Service Account requires the `roles/secretmanager.secretAccessor` IAM role.

> [!NOTE]
> For security, `gcrunpresso` never resolves secret payloads in plaintext. If secrets were previously deployed in plaintext using the legacy `secret` function, rotate them immediately in Secret Manager and update your definitions to use `valueSource.secretKeyRef`.

```yaml
plugins:
  - name: secretmanager
```

### `external` Plugin
Execute custom commands and inject their output.

```yaml
plugins:
  - name: external
    config:
      name: my_cmd
      command: ["jq", "-n"]
      num_args: 1
```

---

## Authentication & Impersonation

`gcrunpresso` natively supports **Google Cloud Application Default Credentials (ADC)**.

To use Service Account Impersonation for least-privilege CI/CD runners:

```bash
gcrunpresso deploy --impersonate-service-account deployer@my-gcp-project.iam.gserviceaccount.com
```

Or set in `gcrunpresso.yml`:
```yaml
impersonate_service_account: deployer@my-gcp-project.iam.gserviceaccount.com
```

---

## CI/CD Integration

### GitHub Actions Workflow Example

```yaml
name: Deploy to Cloud Run

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write

    steps:
      - uses: actions/checkout@v4

      - uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: projects/123456/locations/global/workloadIdentityPools/github/providers/github-provider
          service_account: deployer@my-project.iam.gserviceaccount.com

      - uses: kayac/gcrunpresso@v2
        with:
          version: latest

      - name: Verify Pre-flight
        run: gcrunpresso verify

      - name: Preview Diff
        run: gcrunpresso diff

      - name: Deploy Service
        env:
          TAG: ${{ github.sha }}
        run: gcrunpresso deploy
```

---

## LICENSE

MIT License - see [LICENSE](LICENSE) for details.
