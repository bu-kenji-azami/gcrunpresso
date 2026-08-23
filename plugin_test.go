package gcrunpresso_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kayac/gcrunpresso/v2"
)

func TestSecretManagerPluginPureStringRef(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "gcr.io/my-project/app:v1"
      env:
        - name: "DB_PASS"
          valueSource:
            secretKeyRef:
              secret: {{ secretmanager_ref "my-db-pass" }}
              version: "latest"
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: test-project
location: asia-northeast1
service: test-service
service_definition: service.yaml
plugins:
  - name: secretmanager
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}

	svc, err := app.LoadServiceDefinition("")
	if err != nil {
		t.Fatalf("failed to load service definition: %v", err)
	}

	if svc.Template == nil || len(svc.Template.Containers) == 0 {
		t.Fatal("expected template containers")
	}

	env := svc.Template.Containers[0].Env[0]
	if env.GetValueSource() == nil || env.GetValueSource().GetSecretKeyRef() == nil {
		t.Fatalf("expected secretKeyRef value source, got %v", env)
	}
	expectedSecret := "projects/test-project/secrets/my-db-pass"
	if env.GetValueSource().GetSecretKeyRef().Secret != expectedSecret {
		t.Errorf("expected secret ref %q, got %q", expectedSecret, env.GetValueSource().GetSecretKeyRef().Secret)
	}
}

func TestSecretManagerPluginDeprecatedSecretHardError(t *testing.T) {
	tmpDir := t.TempDir()

	serviceYAML := `
template:
  containers:
    - image: "gcr.io/my-project/app:v1"
      env:
        - name: "DB_PASS"
          value: {{ secret "my-db-pass" }}
`
	servicePath := filepath.Join(tmpDir, "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceYAML), 0644); err != nil {
		t.Fatal(err)
	}

	configYAML := `
project: test-project
location: asia-northeast1
service: test-service
service_definition: service.yaml
plugins:
  - name: secretmanager
`
	configPath := filepath.Join(tmpDir, "gcrunpresso.yml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatal(err)
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		ConfigFilePath: configPath,
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}

	_, err = app.LoadServiceDefinition("")
	if err == nil {
		t.Fatal("expected hard error when using deprecated secret function, got nil")
	}
	if !strings.Contains(err.Error(), "secret template function has been removed") {
		t.Errorf("expected rotation/removal error message, got: %v", err)
	}
}
