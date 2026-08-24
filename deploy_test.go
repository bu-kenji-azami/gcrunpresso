package gcrunpresso_test

import (
	"strings"
	"testing"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/kayac/gcrunpresso/v2"
	api "google.golang.org/genproto/googleapis/api"
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
		name           string
		remote         *runpb.Job
		localOmit      *runpb.Job
		localMatch     *runpb.Job
		expectedSubstr string
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
			expectedSubstr: "binary_authorization",
		},
		{
			name: "Labels",
			remote: &runpb.Job{
				Labels: map[string]string{"env": "prod"},
			},
			localOmit:      &runpb.Job{},
			localMatch:     &runpb.Job{Labels: map[string]string{"env": "prod"}},
			expectedSubstr: "omits 'labels'",
		},
		{
			name: "Annotations",
			remote: &runpb.Job{
				Annotations: map[string]string{"managed-by": "terraform"},
			},
			localOmit:      &runpb.Job{},
			localMatch:     &runpb.Job{Annotations: map[string]string{"managed-by": "terraform"}},
			expectedSubstr: "omits 'annotations'",
		},
		{
			name: "LaunchStage",
			remote: &runpb.Job{
				LaunchStage: api.LaunchStage_BETA,
			},
			localOmit:      &runpb.Job{},
			localMatch:     &runpb.Job{LaunchStage: api.LaunchStage_BETA},
			expectedSubstr: "omits 'launch_stage'",
		},
		{
			name: "ExecutionTemplate.Labels",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Labels: map[string]string{"app": "worker"},
				},
			},
			localOmit:      &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch:     &runpb.Job{Template: &runpb.ExecutionTemplate{Labels: map[string]string{"app": "worker"}}},
			expectedSubstr: "omits 'template.labels'",
		},
		{
			name: "ExecutionTemplate.Annotations",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Annotations: map[string]string{"note": "critical"},
				},
			},
			localOmit:      &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch:     &runpb.Job{Template: &runpb.ExecutionTemplate{Annotations: map[string]string{"note": "critical"}}},
			expectedSubstr: "omits 'template.annotations'",
		},
		{
			name: "TaskCount",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{TaskCount: 10},
			},
			localOmit:      &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch:     &runpb.Job{Template: &runpb.ExecutionTemplate{TaskCount: 5}},
			expectedSubstr: "omits 'template.task_count'",
		},
		{
			name: "Parallelism",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{Parallelism: 4},
			},
			localOmit:      &runpb.Job{Template: &runpb.ExecutionTemplate{}},
			localMatch:     &runpb.Job{Template: &runpb.ExecutionTemplate{Parallelism: 2}},
			expectedSubstr: "omits 'template.parallelism'",
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
			expectedSubstr: "omits 'template.template.service_account'",
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
			expectedSubstr: "omits 'template.template.vpc_access'",
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
			expectedSubstr: "omits 'template.template.encryption_key'",
		},
		{
			name: "Retries (oneof)",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						Retries: &runpb.TaskTemplate_MaxRetries{MaxRetries: 5},
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
			expectedSubstr: "omits 'template.template.max_retries'",
		},
		{
			name: "Timeout",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{Timeout: &durationpb.Duration{Seconds: 900}},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{Timeout: &durationpb.Duration{Seconds: 300}},
				},
			},
			expectedSubstr: "omits 'template.template.timeout'",
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
			expectedSubstr: "omits 'template.template.execution_environment'",
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
			expectedSubstr: "omits 'template.template.node_selector'",
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
			expectedSubstr: "omits 'template.template.gpu_zonal_redundancy_disabled'",
		},
		{
			name: "Volumes",
			remote: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						Volumes: []*runpb.Volume{{Name: "secrets"}},
					},
				},
			},
			localOmit: &runpb.Job{Template: &runpb.ExecutionTemplate{Template: &runpb.TaskTemplate{}}},
			localMatch: &runpb.Job{
				Template: &runpb.ExecutionTemplate{
					Template: &runpb.TaskTemplate{
						Volumes: []*runpb.Volume{{Name: "secrets"}},
					},
				},
			},
			expectedSubstr: "omits 'template.template.volumes'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Omitting field must return an error containing the expected guard message
			err := gcrunpresso.ValidateJobSafetyGuards(tc.remote, tc.localOmit)
			if err == nil {
				t.Fatalf("expected safety guard violation when %s is omitted in local manifest, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.expectedSubstr) {
				t.Fatalf("expected safety guard error for %s to contain %q, got: %v", tc.name, tc.expectedSubstr, err)
			}

			// Specifying field must pass
			if err := gcrunpresso.ValidateJobSafetyGuards(tc.remote, tc.localMatch); err != nil {
				t.Fatalf("unexpected error when %s is specified in local manifest: %v", tc.name, err)
			}
		})
	}
}

// Cloud Run assumes GA when launch_stage is unset and reports the effective stage on
// read, so an ordinary job reads back as GA. Omitting launch_stage from job.yaml is the
// normal case and must not trip the safety guard.
func TestValidateJobSafetyGuardsGALaunchStageIsNotAViolation(t *testing.T) {
	remote := &runpb.Job{LaunchStage: api.LaunchStage_GA}
	local := &runpb.Job{}

	if err := gcrunpresso.ValidateJobSafetyGuards(remote, local); err != nil {
		t.Errorf("GA launch_stage omitted locally must not trip the guard, got: %v", err)
	}
}

// Cloud Run fills server-side defaults into read responses for several template
// fields -- proto docs: task_count "Defaults to 1", max_retries "Defaults to 3",
// timeout "Defaults to 600 seconds", service_account "the project's default
// service account". A manifest omitting them is the normal case and must not
// trip the safety guards; this is the same failure mode launch_stage had.
func TestValidateJobSafetyGuardsAPIDefaultsAreNotViolations(t *testing.T) {
	defaultFilledJob := func() *runpb.Job {
		return &runpb.Job{
			Name: "projects/123456789/locations/asia-northeast1/jobs/my-job",
			Template: &runpb.ExecutionTemplate{
				TaskCount: 1,
				Template: &runpb.TaskTemplate{
					ServiceAccount: "123456789-compute@developer.gserviceaccount.com",
					Retries:        &runpb.TaskTemplate_MaxRetries{MaxRetries: 3},
					Timeout:        &durationpb.Duration{Seconds: 600},
				},
			},
		}
	}

	t.Run("all documented API defaults filled by the server", func(t *testing.T) {
		if err := gcrunpresso.ValidateJobSafetyGuards(defaultFilledJob(), &runpb.Job{}); err != nil {
			t.Errorf("API-reported defaults omitted locally must not trip any guard, got: %v", err)
		}
	})

	t.Run("compute default service account alone", func(t *testing.T) {
		remote := &runpb.Job{
			Name: "projects/123456789/locations/asia-northeast1/jobs/my-job",
			Template: &runpb.ExecutionTemplate{
				Template: &runpb.TaskTemplate{ServiceAccount: "123456789-compute@developer.gserviceaccount.com"},
			},
		}
		if err := gcrunpresso.ValidateJobSafetyGuards(remote, &runpb.Job{}); err != nil {
			t.Errorf("default compute SA omitted locally must not trip the guard, got: %v", err)
		}
	})

	t.Run("custom service account is still guarded", func(t *testing.T) {
		remote := defaultFilledJob()
		remote.Template.Template.ServiceAccount = "custom@proj.iam.gserviceaccount.com"
		err := gcrunpresso.ValidateJobSafetyGuards(remote, &runpb.Job{})
		if err == nil || !strings.Contains(err.Error(), "service_account") {
			t.Errorf("expected service_account guard for non-default SA, got: %v", err)
		}
	})

	// Job.name carries "either project id or number" (proto), while the default
	// compute service account always carries the project number. Recognising the
	// default must not depend on the two agreeing, or every deploy omitting
	// service_account is blocked whenever the project is configured by id.
	t.Run("project id in the job name still recognizes the default SA", func(t *testing.T) {
		remote := defaultFilledJob()
		remote.Name = "projects/my-project-id/locations/asia-northeast1/jobs/my-job"
		if err := gcrunpresso.ValidateJobSafetyGuards(remote, &runpb.Job{}); err != nil {
			t.Errorf("default compute SA must be recognized regardless of how the project is named, got: %v", err)
		}
	})

	t.Run("non-numeric account in the developer domain is still guarded", func(t *testing.T) {
		remote := defaultFilledJob()
		remote.Template.Template.ServiceAccount = "my-app-compute@developer.gserviceaccount.com"
		err := gcrunpresso.ValidateJobSafetyGuards(remote, &runpb.Job{})
		if err == nil || !strings.Contains(err.Error(), "service_account") {
			t.Errorf("only the project-number-shaped default may bypass the guard, got: %v", err)
		}
	})
}
