package gcrunpresso_test

import (
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/protobuf/types/known/durationpb"
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

func TestValidateJobSafetyGuards(t *testing.T) {
	trueVal := true

	tests := []struct {
		name       string
		remote     *runpb.Job
		localOmit  *runpb.Job
		localMatch *runpb.Job
	}{
		{
			name: "BinaryAuthorization",
			remote: &runpb.Job{
				BinaryAuthorization: &runpb.BinaryAuthorization{
					BinauthzMethod: &runpb.BinaryAuthorization_Policy{
						Policy: "projects/p/platforms/cloudRun/policies/default",
					},
				},
			},
			localOmit: &runpb.Job{},
			localMatch: &runpb.Job{
				BinaryAuthorization: &runpb.BinaryAuthorization{
					BinauthzMethod: &runpb.BinaryAuthorization_UseDefault{UseDefault: true},
				},
			},
		},
		{
			name: "Labels",
			remote: &runpb.Job{
				Labels: map[string]string{"env": "prod"},
			},
			localOmit:  &runpb.Job{},
			localMatch: &runpb.Job{Labels: map[string]string{"env": "prod"}},
		},
		{
			name: "Annotations",
			remote: &runpb.Job{
				Annotations: map[string]string{"managed-by": "terraform"},
			},
			localOmit:  &runpb.Job{},
			localMatch: &runpb.Job{Annotations: map[string]string{"managed-by": "terraform"}},
		},
		{
			name: "LaunchStage",
			remote: &runpb.Job{
				LaunchStage: 1, // ALPHA / BETA
			},
			localOmit:  &runpb.Job{},
			localMatch: &runpb.Job{LaunchStage: 1},
		},
		{
			name: "ExecutionTemplate.Labels",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Labels: map[string]string{"app": "worker"},
				},
			},
			localOmit:  &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch: &runpb.Job{Template: &runpb.ExecutionTemplate{Labels: map[string]string{"app": "worker"}}},
		},
		{
			name: "ExecutionTemplate.Annotations",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Annotations: map[string]string{"note": "critical"},
				},
			},
			localOmit:  &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch: &runpb.Job{Template: &runpb.ExecutionTemplate{Annotations: map[string]string{"note": "critical"}}},
		},
		{
			name: "TaskCount",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{TaskCount: 10},
			},
			localOmit:  &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch: &runpb.Job{Template: &runpb.ExecutionTemplate{TaskCount: 5}},
		},
		{
			name: "Parallelism",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{Parallelism: 4},
			},
			localOmit:  &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch: &runpb.Job{Template: &runpb.ExecutionTemplate{Parallelism: 2}},
		},
		{
			name: "ServiceAccount",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{ServiceAccount: "sa@proj.iam.gserviceaccount.com"},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{ServiceAccount: "sa@proj.iam.gserviceaccount.com"},
				},
			},
		},
		{
			name: "VpcAccess",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{VpcAccess: &runpb.VpcAccess{Connector: "projects/p/connectors/c1"}},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{VpcAccess: &runpb.VpcAccess{Connector: "projects/p/connectors/c1"}},
				},
			},
		},
		{
			name: "EncryptionKey",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{EncryptionKey: "projects/p/locations/l/keyRings/r/cryptoKeys/k"},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{EncryptionKey: "projects/p/locations/l/keyRings/r/cryptoKeys/k"},
				},
			},
		},
		{
			name: "Retries (oneof)",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						Retries: &runpb.TaskTemplate_MaxRetries{MaxRetries: 3},
					},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						Retries: &runpb.TaskTemplate_MaxRetries{MaxRetries: 1},
					},
				},
			},
		},
		{
			name: "Timeout",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{Timeout: &durationpb.Duration{Seconds: 600}},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{Timeout: &durationpb.Duration{Seconds: 300}},
				},
			},
		},
		{
			name: "ExecutionEnvironment",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						ExecutionEnvironment: runpb.ExecutionEnvironment_EXECUTION_ENVIRONMENT_GEN2,
					},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						ExecutionEnvironment: runpb.ExecutionEnvironment_EXECUTION_ENVIRONMENT_GEN2,
					},
				},
			},
		},
		{
			name: "NodeSelector",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						NodeSelector: &runpb.NodeSelector{Accelerator: "nvidia-l4"},
					},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						NodeSelector: &runpb.NodeSelector{Accelerator: "nvidia-l4"},
					},
				},
			},
		},
		{
			name: "GpuZonalRedundancyDisabled",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						GpuZonalRedundancyDisabled: &trueVal,
					},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						GpuZonalRedundancyDisabled: &trueVal,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Omitting field must return an error
			if err := gcrunpresso.ValidateJobSafetyGuards(tc.remote, tc.localOmit); err == nil {
				t.Fatalf("expected safety guard violation when %s is omitted in local manifest, got nil", tc.name)
			}

			// Specifying field must pass
			if err := gcrunpresso.ValidateJobSafetyGuards(tc.remote, tc.localMatch); err != nil {
				t.Fatalf("unexpected error when %s is specified in local manifest: %v", tc.name, err)
			}
		})
	}
}
