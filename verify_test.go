package gcrunpresso_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVerifyOptionExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app:v1.0.0"
      env:
        - name: "API_KEY"
          valueSource:
            secretKeyRef:
              secret: "my-api-key"
              version: "latest"
  volumes:
    - name: "vault"
      secret:
        secret: "my-db-password"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-proj
location: asia-northeast1
service: my-svc
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
		ClientOptions:  []option.ClientOption{option.WithoutAuthentication()},
	})
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	defer app.Close()

	svc, err := app.LoadServiceDefinition("")
	if err != nil {
		t.Fatalf("failed to load service: %v", err)
	}

	if len(svc.Template.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(svc.Template.Containers))
	}
	if svc.Template.Containers[0].Image != "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app:v1.0.0" {
		t.Errorf("unexpected image: %s", svc.Template.Containers[0].Image)
	}
	if svc.Template.Containers[0].Env[0].GetValueSource().GetSecretKeyRef().GetSecret() != "my-api-key" {
		t.Errorf("unexpected secret ref: %v", svc.Template.Containers[0].Env[0].GetValueSource().GetSecretKeyRef())
	}
}

type mockSecretManagerAPI struct {
	permDenied bool
	notFound   bool
}

func (m *mockSecretManagerAPI) GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest, opts ...gax.CallOption) (*secretmanagerpb.Secret, error) {
	if m.permDenied {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	if m.notFound {
		return nil, status.Error(codes.NotFound, "secret not found")
	}
	return &secretmanagerpb.Secret{Name: req.Name}, nil
}

type mockArtifactRegistryAPI struct {
	permDenied bool
	// notFound makes GetRepository 404. imageNotFound makes only GetDockerImage 404,
	// so the image-not-found branch can be reached with the repository present.
	notFound             bool
	imageNotFound        bool
	calledGetDockerImage bool
}

func (m *mockArtifactRegistryAPI) GetRepository(ctx context.Context, req *artifactregistrypb.GetRepositoryRequest, opts ...gax.CallOption) (*artifactregistrypb.Repository, error) {
	if m.permDenied {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	if m.notFound {
		return nil, status.Error(codes.NotFound, "repository not found")
	}
	return &artifactregistrypb.Repository{Name: req.Name}, nil
}

func (m *mockArtifactRegistryAPI) GetDockerImage(ctx context.Context, req *artifactregistrypb.GetDockerImageRequest, opts ...gax.CallOption) (*artifactregistrypb.DockerImage, error) {
	m.calledGetDockerImage = true
	if m.imageNotFound {
		return nil, status.Error(codes.NotFound, "image not found")
	}
	if m.permDenied {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	if m.notFound {
		return nil, status.Error(codes.NotFound, "image not found")
	}
	return &artifactregistrypb.DockerImage{Name: req.Name}, nil
}

func TestVerifyPermissionDeniedTreatedAsSkip(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app:v1.0.0"
      env:
        - name: "API_KEY"
          valueSource:
            secretKeyRef:
              secret: "my-api-key"
              version: "latest"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-proj
location: asia-northeast1
service: my-svc
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	},
		gcrunpresso.WithSecretManagerClient(&mockSecretManagerAPI{permDenied: true}),
		gcrunpresso.WithArtifactRegistryClient(&mockArtifactRegistryAPI{permDenied: true}),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	defer app.Close()

	// Should succeed (return nil) because permission errors are treated as [SKIP]
	err = app.Verify(t.Context(), gcrunpresso.VerifyOption{
		Image:   true,
		Secrets: true,
	})
	if err != nil {
		t.Fatalf("expected Verify to succeed (skipping permission denied items), but got error: %v", err)
	}
}

func TestVerifyImageDigestCallsGetDockerImage(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-proj
location: asia-northeast1
service: my-svc
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mockAR := &mockArtifactRegistryAPI{}
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	},
		gcrunpresso.WithArtifactRegistryClient(mockAR),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	defer app.Close()

	err = app.Verify(t.Context(), gcrunpresso.VerifyOption{
		Image: true,
	})
	if err != nil {
		t.Fatalf("expected Verify to succeed, got error: %v", err)
	}

	if !mockAR.calledGetDockerImage {
		t.Error("expected GetDockerImage to be called for image digest reference")
	}
}

func TestVerifyTagReferenceTreatedAsSkip(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app:v1.0.0"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-proj
location: asia-northeast1
service: my-svc
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mockAR := &mockArtifactRegistryAPI{}
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	},
		gcrunpresso.WithArtifactRegistryClient(mockAR),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	defer app.Close()

	// Tag without digest is reported as [SKIP] and does not fail verification.
	// Assert the emitted status, not just a nil error -- Verify also returns nil for OK,
	// so a regression from ErrSkipVerify to nil would otherwise go unnoticed.
	var buf bytes.Buffer
	restore := gcrunpresso.SetJSONWriter(&buf)
	err = app.Verify(t.Context(), gcrunpresso.VerifyOption{
		Image: true,
		JSON:  true,
	})
	restore()
	if err != nil {
		t.Fatalf("expected Verify to succeed (tag treated as skip), but got error: %v", err)
	}

	var items []struct {
		Target string `json:"target"`
		Status string `json:"status"`
	}
	if uerr := json.Unmarshal(buf.Bytes(), &items); uerr != nil {
		t.Fatalf("failed to unmarshal verify --json output: %v (raw: %s)", uerr, buf.String())
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 verify result, got %d (raw: %s)", len(items), buf.String())
	}
	if items[0].Status != "SKIP" {
		t.Errorf("expected tag reference to be SKIP, got %q", items[0].Status)
	}
	if mockAR.calledGetDockerImage {
		t.Error("expected GetDockerImage NOT to be called for a tag reference")
	}
}

func TestVerifyImageNotFoundReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-proj
location: asia-northeast1
service: my-svc
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Repository exists; only the specific digest is missing, so verifyImage must
	// reach GetDockerImage and treat NotFound as a failure rather than falling back
	// to the repository-existence check.
	mockAR := &mockArtifactRegistryAPI{imageNotFound: true}
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	},
		gcrunpresso.WithArtifactRegistryClient(mockAR),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	defer app.Close()

	err = app.Verify(t.Context(), gcrunpresso.VerifyOption{
		Image: true,
	})
	if !mockAR.calledGetDockerImage {
		t.Error("expected GetDockerImage to be called so the image-not-found branch is exercised")
	}
	if err == nil {
		t.Fatal("expected Verify to fail for non-existent image, got nil")
	}
}

func TestVerifySecretNotFoundReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-proj/my-repo/app:v1"
      env:
        - name: "API_KEY"
          valueSource:
            secretKeyRef:
              secret: "missing-secret"
              version: "latest"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: my-proj
location: asia-northeast1
service: my-svc
service_definition: service.yaml
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	mockSM := &mockSecretManagerAPI{notFound: true}
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	},
		gcrunpresso.WithSecretManagerClient(mockSM),
	)
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	defer app.Close()

	err = app.Verify(t.Context(), gcrunpresso.VerifyOption{
		Image:   false,
		Secrets: true,
	})
	if err == nil {
		t.Fatal("expected Verify to fail for non-existent secret, got nil")
	}
}
