package gcrunpresso_test

import (
	"strings"
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDiffServicesIdentical(t *testing.T) {
	local := &runpb.Service{
		Template: &runpb.RevisionTemplate{
			Containers: []*runpb.Container{
				{
					Image: "gcr.io/my-proj/app:v1",
				},
			},
		},
		Traffic: []*runpb.TrafficTarget{
			{
				Type:    runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST,
				Percent: 100,
			},
		},
	}

	remote := &runpb.Service{
		Name:                "projects/my-proj/locations/asia-northeast1/services/my-svc",
		Uid:                 "abc-123-uuid",
		Generation:          5,
		CreateTime:          timestamppb.Now(),
		UpdateTime:          timestamppb.Now(),
		Etag:                "etag-1234",
		LatestReadyRevision: "my-svc-00005-xyz",
		Uri:                 "https://my-svc-xyz.run.app",
		Conditions: []*runpb.Condition{
			{
				Type:  "Ready",
				State: runpb.Condition_CONDITION_SUCCEEDED,
			},
		},
		Template: &runpb.RevisionTemplate{
			Revision: "my-svc-00005-xyz",
			Containers: []*runpb.Container{
				{
					Image: "gcr.io/my-proj/app:v1",
				},
			},
		},
		Traffic: []*runpb.TrafficTarget{
			{
				Type:    runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST,
				Percent: 100,
			},
		},
	}

	diff, err := gcrunpresso.DiffServices(local, remote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff when server-generated fields are stripped, got:\n%s", diff)
	}
}

func TestDiffServicesImageDifference(t *testing.T) {
	local := &runpb.Service{
		Template: &runpb.RevisionTemplate{
			Containers: []*runpb.Container{
				{
					Image: "gcr.io/my-proj/app:v2",
				},
			},
		},
	}

	remote := &runpb.Service{
		Name:       "projects/my-proj/locations/asia-northeast1/services/my-svc",
		Uid:        "uuid-123",
		CreateTime: timestamppb.Now(),
		Template: &runpb.RevisionTemplate{
			Containers: []*runpb.Container{
				{
					Image: "gcr.io/my-proj/app:v1",
				},
			},
		},
	}

	diff, err := gcrunpresso.DiffServices(local, remote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff, got empty")
	}
	if !strings.Contains(diff, "app:v") || !strings.Contains(diff, "-") || !strings.Contains(diff, "+") {
		t.Errorf("expected diff to show image difference, got:\n%s", diff)
	}
}

func TestDiffJobsDifference(t *testing.T) {
	local := &runpb.Job{
		Template: &runpb.ExecutionTemplate{
			TaskCount: 5,
		},
	}

	remote := &runpb.Job{
		Name: "projects/my-proj/locations/asia-northeast1/jobs/my-job",
		Template: &runpb.ExecutionTemplate{
			TaskCount: 1,
		},
	}

	diff, err := gcrunpresso.DiffJobs(local, remote)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff == "" {
		t.Fatal("expected diff, got empty")
	}
	if !strings.Contains(diff, "5") || !strings.Contains(diff, "1") {
		t.Errorf("expected diff to show taskCount difference, got:\n%s", diff)
	}
}
