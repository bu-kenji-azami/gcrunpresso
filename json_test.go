package gcrunpresso_test

import (
	"strings"
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestUnmarshalServiceValidYAML(t *testing.T) {
	yamlContent := `
template:
  containers:
    - image: "asia-northeast1-docker.pkg.dev/my-project/repo/app:v1.0.0"
      env:
        - name: "ENV"
          value: "production"
      resources:
        limits:
          cpu: "1000m"
          memory: "512Mi"
  scaling:
    minInstanceCount: 1
    maxInstanceCount: 10
traffic:
  - type: TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST
    percent: 100
`

	var svc runpb.Service
	if err := gcrunpresso.UnmarshalService([]byte(yamlContent), &svc, false); err != nil {
		t.Fatalf("failed to unmarshal service YAML: %v", err)
	}

	if svc.Template == nil || len(svc.Template.Containers) == 0 {
		t.Fatal("expected containers, got nil or empty")
	}
	if svc.Template.Containers[0].Image != "asia-northeast1-docker.pkg.dev/my-project/repo/app:v1.0.0" {
		t.Errorf("unexpected container image: %s", svc.Template.Containers[0].Image)
	}
	if svc.Template.Scaling.MinInstanceCount != 1 || svc.Template.Scaling.MaxInstanceCount != 10 {
		t.Errorf("unexpected scaling config: min=%d max=%d", svc.Template.Scaling.MinInstanceCount, svc.Template.Scaling.MaxInstanceCount)
	}
	if len(svc.Traffic) != 1 || svc.Traffic[0].Percent != 100 {
		t.Errorf("unexpected traffic: %v", svc.Traffic)
	}
}

func TestUnmarshalJobValidYAML(t *testing.T) {
	yamlContent := `
template:
  template:
    containers:
      - image: "asia-northeast1-docker.pkg.dev/my-project/repo/batch:v1.0.0"
        args:
          - "--migrate"
    maxRetries: 3
    timeout: "300s"
`

	var job runpb.Job
	if err := gcrunpresso.UnmarshalJob([]byte(yamlContent), &job, false); err != nil {
		t.Fatalf("failed to unmarshal job YAML: %v", err)
	}

	if job.Template == nil || job.Template.Template == nil || len(job.Template.Template.Containers) == 0 {
		t.Fatal("expected job containers, got nil or empty")
	}
	if job.Template.Template.Containers[0].Image != "asia-northeast1-docker.pkg.dev/my-project/repo/batch:v1.0.0" {
		t.Errorf("unexpected container image: %s", job.Template.Template.Containers[0].Image)
	}
	if job.Template.Template.GetMaxRetries() != 3 {
		t.Errorf("unexpected max retries: %d", job.Template.Template.GetMaxRetries())
	}
}

func TestUnmarshalServiceUnknownFieldRejected(t *testing.T) {
	invalidYAML := `
template:
  contaienrs: # typo
    - image: "app:v1"
`
	var svc runpb.Service
	err := gcrunpresso.UnmarshalService([]byte(invalidYAML), &svc, false)
	if err == nil {
		t.Fatal("expected error on misspelled unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected error message to mention unknown field, got: %v", err)
	}
}

func TestUnmarshalServiceKnativeManifestRejected(t *testing.T) {
	knativeYAML := `
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-knative-service
spec:
  template:
    spec:
      containers:
        - image: "gcr.io/my-project/app:latest"
`
	var svc runpb.Service
	err := gcrunpresso.UnmarshalService([]byte(knativeYAML), &svc, false)
	if err == nil {
		t.Fatal("expected error on Knative v1 manifest, got nil")
	}
	if !strings.Contains(err.Error(), "Knative serving.knative.dev/v1 manifest detected") {
		t.Errorf("expected error message to mention Knative v1 schema detection, got: %v", err)
	}
}

func TestMarshalServiceAndJob(t *testing.T) {
	svc := &runpb.Service{
		Template: &runpb.RevisionTemplate{
			Containers: []*runpb.Container{
				{
					Image: "gcr.io/my-project/app:v1",
				},
			},
		},
	}

	b, err := gcrunpresso.MarshalService(svc)
	if err != nil {
		t.Fatalf("failed to marshal service: %v", err)
	}

	var roundTrip runpb.Service
	if err := gcrunpresso.UnmarshalService(b, &roundTrip, false); err != nil {
		t.Fatalf("failed to unmarshal marshaled service JSON: %v", err)
	}

	if diff := cmp.Diff(svc, &roundTrip, protocmp.Transform()); diff != "" {
		t.Errorf("service mismatch (-want +got):\n%s", diff)
	}
}
