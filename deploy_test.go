package gcrunpresso_test

import (
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/kayac/gcrunpresso/v2"
)

func TestBuildTrafficTargetsDefault(t *testing.T) {
	targets, err := gcrunpresso.BuildTrafficTargets(gcrunpresso.DeployOption{}, nil, "my-svc-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Type != runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST || targets[0].Percent != 100 {
		t.Errorf("expected 100%% latest, got %v", targets[0])
	}
}

func TestBuildTrafficTargetsWithTag(t *testing.T) {
	targets, err := gcrunpresso.BuildTrafficTargets(gcrunpresso.DeployOption{Tag: "preview"}, nil, "my-svc-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 || targets[0].Tag != "preview" || targets[0].Percent != 100 {
		t.Errorf("expected tag preview with 100%%, got %v", targets[0])
	}
}

func TestBuildTrafficTargetsNoTraffic(t *testing.T) {
	remoteSvc := &runpb.Service{
		LatestReadyRevision: "my-svc-old-ready",
	}
	targets, err := gcrunpresso.BuildTrafficTargets(gcrunpresso.DeployOption{
		NoTraffic: true,
		Tag:       "candidate",
	}, remoteSvc, "my-svc-002")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	// Target 1: 100% on old ready revision
	if targets[0].Revision != "my-svc-old-ready" || targets[0].Percent != 100 {
		t.Errorf("expected 100%% on my-svc-old-ready, got %v", targets[0])
	}
	// Target 2: Tag on candidate with 0% base traffic
	if targets[1].Revision != "my-svc-002" || targets[1].Tag != "candidate" || targets[1].Percent != 0 {
		t.Errorf("expected tagged 0%% on candidate, got %v", targets[1])
	}
}

func TestBuildTrafficTargetsCustomSplit(t *testing.T) {
	targets, err := gcrunpresso.BuildTrafficTargets(gcrunpresso.DeployOption{
		Traffic: "latest=20,my-svc-v1=80",
	}, nil, "my-svc-002")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	if targets[0].Type != runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST || targets[0].Percent != 20 {
		t.Errorf("expected 20%% latest, got %v", targets[0])
	}
	if targets[1].Revision != "my-svc-v1" || targets[1].Percent != 80 {
		t.Errorf("expected 80%% on my-svc-v1, got %v", targets[1])
	}
}

func TestBuildTrafficTargetsInvalidSum(t *testing.T) {
	_, err := gcrunpresso.BuildTrafficTargets(gcrunpresso.DeployOption{
		Traffic: "latest=30,my-svc-v1=60",
	}, nil, "my-svc-002")

	if err == nil {
		t.Fatal("expected error when percentages sum to 90 != 100, got nil")
	}
}

func TestBuildTrafficTargetsInvalidSyntax(t *testing.T) {
	_, err := gcrunpresso.BuildTrafficTargets(gcrunpresso.DeployOption{
		Traffic: "invalid-syntax",
	}, nil, "my-svc-002")

	if err == nil {
		t.Fatal("expected error for invalid traffic syntax, got nil")
	}
}
