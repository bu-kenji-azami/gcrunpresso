package gcrunpresso_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/kayac/gcrunpresso/v2"
)

func TestWarnPlaintextSecretsInEnv(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "test-project",
		Location: "asia-northeast1",
		Service:  "test-svc",
	})
	if err != nil {
		t.Fatalf("failed to initialize App: %v", err)
	}
	app.SetLogger(logger)

	envs := []*runpb.EnvVar{
		{
			Name: "NORMAL_CONFIG",
			Values: &runpb.EnvVar_Value{
				Value: "production",
			},
		},
		{
			Name: "DATABASE_PASSWORD",
			Values: &runpb.EnvVar_Value{
				Value: "supersecret",
			},
		},
		{
			Name: "API_KEY",
			Values: &runpb.EnvVar_Value{
				Value: "xyz123",
			},
		},
		{
			Name: "DATABASE_URL",
			Values: &runpb.EnvVar_Value{
				Value: "postgres://user:pass@localhost:5432/db",
			},
		},
		{
			Name: "SECRET_KEY_PROPER",
			Values: &runpb.EnvVar_ValueSource{
				ValueSource: &runpb.EnvVarSource{
					SecretKeyRef: &runpb.SecretKeySelector{
						Secret:  "projects/test-project/secrets/my-secret",
						Version: "latest",
					},
				},
			},
		},
	}

	app.WarnPlaintextSecretsInEnv("app-container", envs)

	logOut := buf.String()
	if !strings.Contains(logOut, "DATABASE_PASSWORD") {
		t.Errorf("expected warning for DATABASE_PASSWORD, got: %s", logOut)
	}
	if !strings.Contains(logOut, "API_KEY") {
		t.Errorf("expected warning for API_KEY, got: %s", logOut)
	}
	if !strings.Contains(logOut, "DATABASE_URL") {
		t.Errorf("expected warning for DATABASE_URL, got: %s", logOut)
	}
	if strings.Contains(logOut, "SECRET_KEY_PROPER") {
		t.Errorf("expected NO warning for secretKeyRef SECRET_KEY_PROPER, got: %s", logOut)
	}
	if strings.Contains(logOut, "NORMAL_CONFIG") {
		t.Errorf("expected NO warning for NORMAL_CONFIG, got: %s", logOut)
	}
}
