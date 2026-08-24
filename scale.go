package gcrunpresso

import (
	"context"
	"fmt"

	"cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type ScaleOption struct {
	Min       *int32 `help:"min instances"`
	Max       *int32 `help:"max instances"`
	NoTraffic bool   `help:"do not route traffic to new revision" default:"false"`
	DryRun    bool   `help:"dry run" default:"false"`
}

func (d *App) Scale(ctx context.Context, opt ScaleOption) error {
	d.LogInfo("starting scale", withDryRun(opt.DryRun, "service", d.config.Service)...)

	if d.config.Service == "" {
		return fmt.Errorf("scale is only applicable to Cloud Run Services, but service is not configured")
	}
	if opt.Min == nil && opt.Max == nil {
		return fmt.Errorf("at least one of --min or --max must be specified for scale")
	}

	remoteSvc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", d.ResourceServicePath(), err)
	}

	template := remoteSvc.Template
	if template == nil {
		template = &runpb.RevisionTemplate{}
	}
	template.Revision = "" // Clear template.revision to avoid collision
	if template.Scaling == nil {
		template.Scaling = &runpb.RevisionScaling{}
	}

	var updateMaskPaths []string
	if opt.Min != nil {
		template.Scaling.MinInstanceCount = *opt.Min
		updateMaskPaths = append(updateMaskPaths, "template.scaling.min_instance_count")
	}
	if opt.Max != nil {
		template.Scaling.MaxInstanceCount = *opt.Max
		updateMaskPaths = append(updateMaskPaths, "template.scaling.max_instance_count")
	}

	svcUpdate := &runpb.Service{
		Name:     d.ResourceServicePath(),
		Template: template,
	}

	// Traffic handling: scale only adjusts instance scaling and must never shift
	// traffic. Updating the template creates a new revision and a LATEST target
	// would follow it -- so LATEST allocations are pinned to the current
	// latest-ready revision before the table is sent back.
	isPinned := false
	hasLatest := false
	for _, t := range remoteSvc.Traffic {
		if t == nil {
			continue
		}
		switch t.Type {
		case runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION:
			if t.Revision != "" {
				isPinned = true
			}
		case runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST:
			hasLatest = true
		}
	}

	if opt.NoTraffic || isPinned {
		if len(remoteSvc.Traffic) > 0 {
			svcUpdate.Traffic = pinLatestTargets(remoteSvc.Traffic, remoteSvc.LatestReadyRevision)
			updateMaskPaths = append(updateMaskPaths, "traffic")
		}
		if hasLatest {
			if remoteSvc.LatestReadyRevision == "" {
				d.LogWarn("latest-ready revision unknown; LATEST target sent verbatim and will follow the revision created by scale")
			} else {
				d.LogInfo("LATEST traffic share pinned to current latest-ready revision; routing is unchanged by scale", "pinned_to", remoteSvc.LatestReadyRevision)
			}
		}
		if isPinned && !hasLatest {
			d.LogInfo("traffic is pinned to specific revision(s); preserved as-is while the revision created by scale receives no traffic")
		}
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN: planned service scaling configuration", "service", svcUpdate.Name)
		jsonBytes, err := MarshalService(svcUpdate)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	d.LogInfo("updating service scaling", "min", opt.Min, "max", opt.Max)
	op, err := d.servicesClient.UpdateService(ctx, &runpb.UpdateServiceRequest{
		Service:    svcUpdate,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: updateMaskPaths},
	})
	if err != nil {
		return fmt.Errorf("failed to scale service %s: %w", d.config.Service, err)
	}
	if op != nil {
		d.LogInfo("scale operation started, waiting for completion", "op", op.Name())
	}

	readySvc, err := d.WaitForServiceReady(ctx, d.ResourceServicePath())
	if err != nil {
		return err
	}
	d.LogInfo("service scaled successfully", "service", readySvc.Name, "latest_ready_revision", readySvc.LatestReadyRevision)
	return nil
}

// pinLatestTargets rewrites LATEST allocations to explicit REVISION targets bound
// to latestReady, keeping percentages and tags, so a template update triggered by
// scale cannot pull those shares onto the newly created revision. When latestReady
// is unknown the table is returned unchanged.
func pinLatestTargets(targets []*runpb.TrafficTarget, latestReady string) []*runpb.TrafficTarget {
	if latestReady == "" {
		return targets
	}
	pinned := make([]*runpb.TrafficTarget, 0, len(targets))
	for _, t := range targets {
		if t != nil && t.Type == runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST {
			t = &runpb.TrafficTarget{
				Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
				Revision: latestReady,
				Percent:  t.Percent,
				Tag:      t.Tag,
			}
		}
		pinned = append(pinned, t)
	}
	return pinned
}
