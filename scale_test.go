package gcrunpresso_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/kayac/gcrunpresso/v2"
)

type mockServicesAPI struct {
	svc           *runpb.Service
	lastUpdateReq *runpb.UpdateServiceRequest
}

func (m *mockServicesAPI) GetService(ctx context.Context, req *runpb.GetServiceRequest, opts ...gax.CallOption) (*runpb.Service, error) {
	if m.svc != nil {
		return m.svc, nil
	}
	return &runpb.Service{
		Name: req.Name,
		Conditions: []*runpb.Condition{
			{Type: "Ready", State: runpb.Condition_CONDITION_SUCCEEDED},
		},
	}, nil
}

func (m *mockServicesAPI) CreateService(ctx context.Context, req *runpb.CreateServiceRequest, opts ...gax.CallOption) (*run.CreateServiceOperation, error) {
	return nil, nil
}

func (m *mockServicesAPI) UpdateService(ctx context.Context, req *runpb.UpdateServiceRequest, opts ...gax.CallOption) (*run.UpdateServiceOperation, error) {
	m.lastUpdateReq = req
	if m.svc != nil {
		m.svc.Template = req.Service.Template
		if req.Service.Traffic != nil {
			m.svc.Traffic = req.Service.Traffic
		}
	}
	return nil, nil
}

func (m *mockServicesAPI) DeleteService(ctx context.Context, req *runpb.DeleteServiceRequest, opts ...gax.CallOption) (*run.DeleteServiceOperation, error) {
	return nil, nil
}

func TestScaleRequiresMinOrMax(t *testing.T) {
	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Service:  "s",
	})
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}

	err = app.Scale(t.Context(), gcrunpresso.ScaleOption{})
	if err == nil {
		t.Fatal("expected error when neither --min nor --max is provided, got nil")
	}
}

func TestScalePinnedTrafficWarningAndShiftToLatest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	mockSvc := &runpb.Service{
		Name: "projects/p/locations/l/services/s",
		Traffic: []*runpb.TrafficTarget{
			{
				Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
				Revision: "rev-pinned-1",
				Percent:  100,
			},
		},
		Conditions: []*runpb.Condition{
			{Type: "Ready", State: runpb.Condition_CONDITION_SUCCEEDED},
		},
	}

	mockClient := &mockServicesAPI{svc: mockSvc}

	app, err := gcrunpresso.New(t.Context(), &gcrunpresso.Option{
		Project:  "p",
		Location: "l",
		Service:  "s",
	}, gcrunpresso.WithServicesClient(mockClient))
	if err != nil {
		t.Fatalf("failed to create App: %v", err)
	}
	app.SetLogger(logger)

	minVal := int32(2)
	err = app.Scale(t.Context(), gcrunpresso.ScaleOption{
		Min: &minVal,
	})
	if err != nil {
		t.Fatalf("unexpected error during Scale: %v", err)
	}

	logOut := buf.String()
	if !strings.Contains(logOut, "pinned to specific revision") {
		t.Errorf("expected warning log about pinned traffic, got: %s", logOut)
	}

	if mockClient.lastUpdateReq == nil || mockClient.lastUpdateReq.Service == nil {
		t.Fatal("expected UpdateServiceRequest to be recorded")
	}
	updatedTraffic := mockClient.lastUpdateReq.Service.Traffic
	if len(updatedTraffic) != 1 || updatedTraffic[0].Type != runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST {
		t.Errorf("expected traffic shifted to LATEST, got %v", updatedTraffic)
	}
}
