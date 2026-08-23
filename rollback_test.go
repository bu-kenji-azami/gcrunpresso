package gcrunpresso_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/kayac/gcrunpresso/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockRevisionsAPI struct {
	revisions []*runpb.Revision
}

func (m *mockRevisionsAPI) GetRevision(ctx context.Context, req *runpb.GetRevisionRequest, opts ...gax.CallOption) (*runpb.Revision, error) {
	for _, r := range m.revisions {
		if r.Name == req.Name {
			return r, nil
		}
	}
	return nil, nil
}

func (m *mockRevisionsAPI) ListRevisions(ctx context.Context, req *runpb.ListRevisionsRequest) ([]*runpb.Revision, error) {
	return m.revisions, nil
}

func TestFindPrecedingHealthyRevision(t *testing.T) {
	now := time.Now()
	revs := []*runpb.Revision{
		{
			Name:       "projects/p/locations/l/services/s/revisions/rev-unhealthy",
			CreateTime: timestamppb.New(now.Add(-1 * time.Minute)),
			Conditions: []*runpb.Condition{
				{Type: "Ready", State: runpb.Condition_CONDITION_FAILED},
			},
		},
		{
			Name:       "projects/p/locations/l/services/s/revisions/rev-serving-current",
			CreateTime: timestamppb.New(now),
			Conditions: []*runpb.Condition{
				{Type: "Ready", State: runpb.Condition_CONDITION_SUCCEEDED},
			},
		},
		{
			Name:       "projects/p/locations/l/services/s/revisions/rev-healthy-target",
			CreateTime: timestamppb.New(now.Add(-5 * time.Minute)),
			Conditions: []*runpb.Condition{
				{Type: "Ready", State: runpb.Condition_CONDITION_SUCCEEDED},
			},
		},
		{
			Name:       "projects/p/locations/l/services/s/revisions/rev-old",
			CreateTime: timestamppb.New(now.Add(-10 * time.Minute)),
			Conditions: []*runpb.Condition{
				{Type: "Ready", State: runpb.Condition_CONDITION_SUCCEEDED},
			},
		},
	}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Service:  "s",
	}, gcrunpresso.WithRevisionsClient(&mockRevisionsAPI{revisions: revs}))
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	currentSvc := &runpb.Service{
		Name: "projects/p/locations/l/services/s",
		TrafficStatuses: []*runpb.TrafficTargetStatus{
			{Revision: "rev-serving-current", Percent: 100},
		},
	}

	target, err := app.FindPrecedingHealthyRevision(t.Context(), currentSvc)
	if err != nil {
		t.Fatalf("unexpected error finding preceding healthy revision: %v", err)
	}

	// Should choose rev-healthy-target (newest ready revision that is not currently serving)
	if target != "rev-healthy-target" {
		t.Errorf("expected rev-healthy-target, got %s", target)
	}
}
