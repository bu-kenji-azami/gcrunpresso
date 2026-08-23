package gcrunpresso_test

import (
	"context"
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
}

func (m *mockSecretManagerAPI) GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest, opts ...gax.CallOption) (*secretmanagerpb.Secret, error) {
	if m.permDenied {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	return &secretmanagerpb.Secret{Name: req.Name}, nil
}

type mockArtifactRegistryAPI struct {
	permDenied bool
}

func (m *mockArtifactRegistryAPI) GetRepository(ctx context.Context, req *artifactregistrypb.GetRepositoryRequest, opts ...gax.CallOption) (*artifactregistrypb.Repository, error) {
	if m.permDenied {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}
	return &artifactregistrypb.Repository{Name: req.Name}, nil
}

func (m *mockArtifactRegistryAPI) GetDockerImage(ctx context.Context, req *artifactregistrypb.GetDockerImageRequest, opts ...gax.CallOption) (*artifactregistrypb.DockerImage, error) {
	if m.permDenied {
		return nil, status.Error(codes.PermissionDenied, "permission denied")
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
