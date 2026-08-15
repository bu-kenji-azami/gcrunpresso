package gcrunpresso_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/api/option"
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

	app, err := gcrunpresso.New(context.Background(), &gcrunpresso.Option{
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
