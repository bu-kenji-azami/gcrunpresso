package gcrunpresso

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type RollbackOption struct {
	Revision      string `help:"revision name to rollback to" default:""`
	RevertService bool   `help:"revert service template to target revision specification" default:"false"`
	DryRun        bool   `help:"dry run" default:"false"`
}

func (d *App) Rollback(ctx context.Context, opt RollbackOption) error {
	d.LogInfo("starting rollback", withDryRun(opt.DryRun, "service", d.config.Service)...)

	if d.config.Service == "" {
		return fmt.Errorf("rollback is only applicable to Cloud Run Services, but service is not configured")
	}

	remoteSvc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", d.ResourceServicePath(), err)
	}

	// Find target revision
	targetRevName := opt.Revision
	if targetRevName == "" {
		targetRevName, err = d.findPrecedingHealthyRevision(ctx, remoteSvc)
		if err != nil {
			return err
		}
	}
	d.LogInfo("selected rollback target revision", "revision", targetRevName)

	if opt.RevertService {
		return d.rollbackRevertTemplate(ctx, remoteSvc, targetRevName, opt)
	}

	return d.rollbackTrafficOnly(ctx, remoteSvc, targetRevName, opt)
}

func (d *App) findPrecedingHealthyRevision(ctx context.Context, currentSvc *runpb.Service) (string, error) {
	revisions, err := d.revisionsClient.ListRevisions(ctx, &runpb.ListRevisionsRequest{
		Parent: d.ResourceServicePath(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list revisions: %w", err)
	}

	if len(revisions) < 2 {
		return "", fmt.Errorf("service %s has fewer than 2 revisions, cannot rollback automatically", currentSvc.Name)
	}

	// Sort revisions strictly by CreateTime descending (D15)
	sort.Slice(revisions, func(i, j int) bool {
		t1 := revisions[i].GetCreateTime().AsTime()
		t2 := revisions[j].GetCreateTime().AsTime()
		return t1.After(t2)
	})

	// Exclude all currently serving revisions (D1, J1)
	servingRevs := FindServingRevisions(currentSvc)
	servingMap := make(map[string]struct{})
	for _, r := range servingRevs {
		servingMap[r] = struct{}{}
	}

	// Find the first revision that is ready and not currently serving
	for _, rev := range revisions {
		shortName := arnToName(rev.Name)
		if _, isServing := servingMap[shortName]; isServing {
			continue
		}
		// Check if revision is healthy / ready
		isReady := false
		for _, cond := range rev.Conditions {
			if cond.Type == "Ready" && cond.State == runpb.Condition_CONDITION_SUCCEEDED {
				isReady = true
				break
			}
		}
		if isReady {
			return shortName, nil
		}
	}

	return "", fmt.Errorf("no preceding healthy revision found for service %s", currentSvc.Name)
}

func (d *App) rollbackTrafficOnly(ctx context.Context, currentSvc *runpb.Service, targetRev string, opt RollbackOption) error {
	svcUpdate := &runpb.Service{
		Name: d.ResourceServicePath(),
		Traffic: []*runpb.TrafficTarget{
			{
				Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
				Revision: targetRev,
				Percent:  100,
			},
		},
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN: planned traffic rollback", "target_revision", targetRev)
		jsonBytes, err := MarshalService(svcUpdate)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	d.LogInfo("switching 100% traffic to revision", "revision", targetRev)
	op, err := d.servicesClient.UpdateService(ctx, &runpb.UpdateServiceRequest{
		Service:    svcUpdate,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"traffic"}},
	})
	if err != nil {
		return fmt.Errorf("failed to update service traffic for rollback: %w", err)
	}
	if op != nil {
		d.LogInfo("traffic update operation started, waiting for completion", "op", op.Name())
	}

	readySvc, err := d.WaitForServiceReady(ctx, d.ResourceServicePath())
	if err != nil {
		return err
	}
	d.LogInfo("rollback complete: traffic routed 100% to revision", "revision", targetRev, "uri", readySvc.Uri)
	return nil
}

func (d *App) rollbackRevertTemplate(ctx context.Context, currentSvc *runpb.Service, targetRevName string, opt RollbackOption) error {
	targetRevPath := d.ResourceRevisionPath(targetRevName)
	targetRev, err := d.revisionsClient.GetRevision(ctx, &runpb.GetRevisionRequest{
		Name: targetRevPath,
	})
	if err != nil {
		return fmt.Errorf("failed to get target revision %s: %w", targetRevPath, err)
	}

	// Build RevisionTemplate from target Revision (R9, #9)
	template := &runpb.RevisionTemplate{
		Containers:                    targetRev.Containers,
		Volumes:                       targetRev.Volumes,
		ServiceAccount:                targetRev.ServiceAccount,
		Scaling:                       targetRev.Scaling,
		VpcAccess:                     targetRev.VpcAccess,
		Timeout:                       targetRev.Timeout,
		MaxInstanceRequestConcurrency: targetRev.MaxInstanceRequestConcurrency,
		Labels:                        targetRev.Labels,
		Annotations:                   targetRev.Annotations,
		ExecutionEnvironment:          targetRev.ExecutionEnvironment,
		EncryptionKey:                 targetRev.EncryptionKey,
		SessionAffinity:               targetRev.SessionAffinity,
		NodeSelector:                  targetRev.NodeSelector,
	}
	if currentSvc.Template != nil {
		template.HealthCheckDisabled = currentSvc.Template.HealthCheckDisabled
	}

	svcUpdate := &runpb.Service{
		Name:     d.ResourceServicePath(),
		Template: template,
		Traffic: []*runpb.TrafficTarget{
			{
				Type:    runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST,
				Percent: 100,
			},
		},
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN: planned template revert rollback", "target_source_revision", targetRevName)
		jsonBytes, err := MarshalService(svcUpdate)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	d.LogInfo("deploying reverted template from revision", "source_revision", targetRevName)
	op, err := d.servicesClient.UpdateService(ctx, &runpb.UpdateServiceRequest{
		Service:    svcUpdate,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"template", "traffic"}},
	})
	if err != nil {
		return fmt.Errorf("failed to update service template for rollback: %w", err)
	}
	if op != nil {
		d.LogInfo("template revert operation started, waiting for completion", "op", op.Name())
	}

	readySvc, err := d.WaitForServiceReady(ctx, d.ResourceServicePath())
	if err != nil {
		return err
	}
	d.LogInfo("revert rollback complete: deployed new revision", "new_revision", readySvc.LatestReadyRevision, "uri", readySvc.Uri)
	return nil
}

func arnToName(s string) string {
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}
