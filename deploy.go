package gcrunpresso

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type DeployOption struct {
	DryRun    bool   `help:"dry run" default:"false"`
	Tag       string `help:"tag for revision" default:""`
	NoTraffic bool   `help:"deploy without routing base traffic" default:"false"`
	Traffic   string `help:"traffic split percentage specification" default:""`
}

func (d *App) Deploy(ctx context.Context, opt DeployOption) error {
	d.LogInfo("starting deploy", withDryRun(opt.DryRun, "service", d.config.Service)...)

	if d.config.Service != "" {
		return d.deployService(ctx, opt)
	}
	if d.config.Job != "" {
		return d.deployJob(ctx, opt)
	}
	return fmt.Errorf("either service or job must be specified in config")
}

func (d *App) deployService(ctx context.Context, opt DeployOption) error {
	svc, err := d.LoadServiceDefinition("")
	if err != nil {
		return err
	}
	svc.Name = d.ResourceServicePath()

	// Check remote service
	remoteSvc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	isCreate := false
	if err != nil {
		if isNotFoundError(err) {
			isCreate = true
		} else {
			return fmt.Errorf("failed to get service %s: %w", d.ResourceServicePath(), err)
		}
	}

	// Validate safety guards on update (R5)
	if !isCreate && remoteSvc != nil {
		if err := validateServiceSafetyGuards(remoteSvc, svc); err != nil {
			return err
		}
	}

	// Generate deterministic revision name
	revName := fmt.Sprintf("%s-%s", d.config.Service, time.Now().Format("20060102-150405"))
	if svc.Template == nil {
		svc.Template = &runpb.RevisionTemplate{}
	}
	if svc.Template.Revision == "" {
		svc.Template.Revision = revName
	} else {
		revName = svc.Template.Revision
	}

	// Configure traffic allocation (R5, R8)
	trafficTargets, err := BuildTrafficTargets(opt, remoteSvc, revName)
	if err != nil {
		return err
	}
	if len(trafficTargets) > 0 {
		svc.Traffic = trafficTargets
	}

	// Warn if default 100% traffic allocation overwrites non-default traffic table (#8)
	if !opt.NoTraffic && opt.Traffic == "" && remoteSvc != nil && len(remoteSvc.Traffic) > 0 {
		hasNonDefaultTraffic := false
		for _, t := range remoteSvc.Traffic {
			if t != nil && (t.Type == runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION || t.Percent < 100 || t.Tag != "") {
				hasNonDefaultTraffic = true
				break
			}
		}
		if hasNonDefaultTraffic {
			d.LogWarn("re-asserting 100% traffic to latest revision, overwriting existing non-default traffic table", "service", d.config.Service)
		}
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN: planned service configuration", "service", svc.Name)
		jsonBytes, err := MarshalService(svc)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if isCreate {
		d.LogInfo("creating new service", "service", d.config.Service, "parent", d.ResourceLocationPath())
		op, err := d.servicesClient.CreateService(ctx, &runpb.CreateServiceRequest{
			Parent:    d.ResourceLocationPath(),
			Service:   svc,
			ServiceId: d.config.Service,
		})
		if err != nil {
			return fmt.Errorf("failed to create service %s: %w", d.config.Service, err)
		}
		if op != nil {
			d.LogInfo("service create operation started, waiting for completion", "op", op.Name())
		}
	} else {
		d.LogInfo("updating service", "service", d.config.Service)
		mask := &fieldmaskpb.FieldMask{
			Paths: []string{"template", "traffic", "labels", "annotations", "ingress"},
		}
		op, err := d.servicesClient.UpdateService(ctx, &runpb.UpdateServiceRequest{
			Service:    svc,
			UpdateMask: mask,
		})
		if err != nil {
			return fmt.Errorf("failed to update service %s: %w", d.config.Service, err)
		}
		if op != nil {
			d.LogInfo("service update operation started, waiting for completion", "op", op.Name())
		}
	}

	// Wait for Service Ready condition (R5, KTD6)
	readySvc, err := d.WaitForServiceReady(ctx, d.ResourceServicePath())
	if err != nil {
		d.LogError("service deployment failed to become ready: %v", err)
		// On failure/timeout, restore previous traffic allocation (#7, #20, #28)
		if !isCreate && remoteSvc != nil && len(remoteSvc.Traffic) > 0 {
			d.LogWarn("deployment failed; abandoning revision and restoring prior traffic allocation", "abandoned_revision", revName)
			recovCtx, cancelRecov := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancelRecov()
			_, updateErr := d.servicesClient.UpdateService(recovCtx, &runpb.UpdateServiceRequest{
				Service: &runpb.Service{
					Name:    d.ResourceServicePath(),
					Traffic: remoteSvc.Traffic,
				},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"traffic"}},
			})
			if updateErr != nil {
				d.LogError("failed to restore prior traffic allocation during recovery: %v", updateErr)
			} else {
				d.LogInfo("successfully restored prior traffic allocation", "service", d.config.Service)
			}
		}
		return err
	}

	d.LogInfo("service deployed successfully", "service", readySvc.Name, "uri", readySvc.Uri, "latest_ready_revision", readySvc.LatestReadyRevision)
	return nil
}

func (d *App) deployJob(ctx context.Context, opt DeployOption) error {
	job, err := d.LoadJobDefinition("")
	if err != nil {
		return err
	}
	job.Name = d.ResourceJobPath()

	remoteJob, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
		Name: d.ResourceJobPath(),
	})
	isCreate := false
	if err != nil {
		if isNotFoundError(err) {
			isCreate = true
		} else {
			return fmt.Errorf("failed to get job %s: %w", d.ResourceJobPath(), err)
		}
	}
	// Validate safety guards on update (R6, P0 #3)
	if !isCreate && remoteJob != nil {
		if err := validateJobSafetyGuards(remoteJob, job); err != nil {
			return err
		}
	}

	if opt.DryRun {
		d.LogInfo("DRY RUN: planned job configuration", "job", job.Name)
		jsonBytes, err := MarshalJob(job)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if isCreate {
		d.LogInfo("creating new job", "job", d.config.Job, "parent", d.ResourceLocationPath())
		op, err := d.jobsClient.CreateJob(ctx, &runpb.CreateJobRequest{
			Parent: d.ResourceLocationPath(),
			Job:    job,
			JobId:  d.config.Job,
		})
		if err != nil {
			return fmt.Errorf("failed to create job %s: %w", d.config.Job, err)
		}
		if op != nil {
			d.LogInfo("job create operation started, waiting for completion", "op", op.Name())
		}
	} else {
		d.LogInfo("updating job", "job", d.config.Job)
		op, err := d.jobsClient.UpdateJob(ctx, &runpb.UpdateJobRequest{
			Job: job,
		})
		if err != nil {
			return fmt.Errorf("failed to update job %s: %w", d.config.Job, err)
		}
		if op != nil {
			d.LogInfo("job update operation started, waiting for completion", "op", op.Name())
		}
	}

	readyJob, err := d.WaitForJobReady(ctx, d.ResourceJobPath())
	if err != nil {
		return err
	}
	d.LogInfo("job deployed successfully", "job", readyJob.Name)
	return nil
}

func validateJobSafetyGuards(remote, local *runpb.Job) error {
	if remote == nil || local == nil {
		return nil
	}

	// 1. BinaryAuthorization
	if remote.BinaryAuthorization != nil && local.BinaryAuthorization == nil {
		return fmt.Errorf("safety guard violation: remote Job has binary_authorization configured, but rendered manifest omits 'binary_authorization'. Please declare binary_authorization in job.yaml to prevent security policy reset")
	}

	// 2. Labels
	if len(remote.Labels) > 0 && len(local.Labels) == 0 {
		return fmt.Errorf("safety guard violation: remote Job has labels configured, but rendered manifest omits 'labels'. Please declare labels in job.yaml to prevent metadata reset")
	}

	// 3. Annotations
	if len(remote.Annotations) > 0 && len(local.Annotations) == 0 {
		return fmt.Errorf("safety guard violation: remote Job has annotations configured, but rendered manifest omits 'annotations'. Please declare annotations in job.yaml to prevent metadata reset")
	}

	if remote.Template != nil {
		locExec := local.Template
		if locExec == nil {
			locExec = &runpb.ExecutionTemplate{}
		}

		// 4. TaskCount
		if remote.Template.TaskCount > 0 && locExec.TaskCount == 0 {
			return fmt.Errorf("safety guard violation: remote Job has task_count=%d configured, but rendered manifest omits 'template.task_count'", remote.Template.TaskCount)
		}

		// 5. Parallelism
		if remote.Template.Parallelism > 0 && locExec.Parallelism == 0 {
			return fmt.Errorf("safety guard violation: remote Job has parallelism=%d configured, but rendered manifest omits 'template.parallelism'", remote.Template.Parallelism)
		}

		if remote.Template.Template != nil {
			remTask := remote.Template.Template
			locTask := locExec.Template
			if locTask == nil {
				locTask = &runpb.TaskTemplate{}
			}

			// 6. ServiceAccount
			if remTask.ServiceAccount != "" && locTask.ServiceAccount == "" {
				return fmt.Errorf("safety guard violation: remote Job has service_account=%s configured, but rendered manifest omits 'template.template.service_account'. Please declare service_account in job.yaml to prevent fallback to default compute SA", remTask.ServiceAccount)
			}

			// 7. VpcAccess
			if remTask.VpcAccess != nil && locTask.VpcAccess == nil {
				return fmt.Errorf("safety guard violation: remote Job has VPC access configured, but rendered manifest omits 'template.template.vpc_access'. Please declare vpc_access in job.yaml to prevent egress reset")
			}

			// 8. EncryptionKey
			if remTask.EncryptionKey != "" && locTask.EncryptionKey == "" {
				return fmt.Errorf("safety guard violation: remote Job has encryption_key (CMEK) configured, but rendered manifest omits 'template.template.encryption_key'. Please declare encryption_key in job.yaml to prevent downgrade to Google-managed key")
			}

			// 9. Retries (oneof TaskTemplate_MaxRetries)
			if remTask.Retries != nil && locTask.Retries == nil {
				return fmt.Errorf("safety guard violation: remote Job has max_retries configured, but rendered manifest omits 'template.template.max_retries'")
			}

			// 10. Timeout
			if remTask.Timeout != nil && locTask.Timeout == nil {
				return fmt.Errorf("safety guard violation: remote Job has timeout configured, but rendered manifest omits 'template.template.timeout'")
			}

			// 11. ExecutionEnvironment
			if remTask.ExecutionEnvironment != runpb.ExecutionEnvironment_EXECUTION_ENVIRONMENT_UNSPECIFIED &&
				locTask.ExecutionEnvironment == runpb.ExecutionEnvironment_EXECUTION_ENVIRONMENT_UNSPECIFIED {
				return fmt.Errorf("safety guard violation: remote Job has execution_environment=%s configured, but rendered manifest omits 'template.template.execution_environment'", remTask.ExecutionEnvironment)
			}
		}
	}

	return nil
}

func validateServiceSafetyGuards(remote, local *runpb.Service) error {
	if remote.Ingress != runpb.IngressTraffic_INGRESS_TRAFFIC_UNSPECIFIED &&
		local.Ingress == runpb.IngressTraffic_INGRESS_TRAFFIC_UNSPECIFIED {
		return fmt.Errorf("safety guard violation: remote Service has ingress=%s configured, but rendered manifest omits 'ingress'. Please declare 'ingress' in service.yaml to prevent silent exposure", remote.Ingress)
	}

	if remote.Template != nil && local.Template != nil {
		if remote.Template.GetVpcAccess() != nil && local.Template.GetVpcAccess() == nil {
			return fmt.Errorf("safety guard violation: remote Service has VPC access configured, but rendered manifest omits 'template.vpc_access'. Please declare VPC access in service.yaml to prevent egress reset")
		}
		if remote.Template.GetServiceAccount() != "" && local.Template.GetServiceAccount() == "" {
			return fmt.Errorf("safety guard violation: remote Service has custom service_account=%s, but rendered manifest omits 'template.service_account'. Please declare service_account in service.yaml to prevent fallback to default compute SA", remote.Template.GetServiceAccount())
		}
	}
	return nil
}

// BuildTrafficTargets computes the runpb.TrafficTarget slice based on DeployOption and remote Service state.
func BuildTrafficTargets(opt DeployOption, remoteSvc *runpb.Service, revName string) ([]*runpb.TrafficTarget, error) {
	if opt.NoTraffic {
		if remoteSvc != nil && remoteSvc.LatestReadyRevision != "" {
			targets := []*runpb.TrafficTarget{
				{
					Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
					Revision: remoteSvc.LatestReadyRevision,
					Percent:  100,
				},
			}
			if opt.Tag != "" {
				targets = append(targets, &runpb.TrafficTarget{
					Type:     runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION,
					Revision: revName,
					Tag:      opt.Tag,
					Percent:  0,
				})
			}
			return targets, nil
		}
		// If remote doesn't have a ready revision, default to 100% on new revision
	}

	if opt.Traffic != "" {
		// Parse <revision-or-tag>=<percent> specs
		parts := strings.Split(opt.Traffic, ",")
		var targets []*runpb.TrafficTarget
		var totalPercent int32

		for _, p := range parts {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid traffic spec %q: must be <target>=<percent>", p)
			}
			targetName := strings.TrimSpace(kv[0])
			pct, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid traffic percentage in %q: %w", p, err)
			}

			target := &runpb.TrafficTarget{
				Percent: int32(pct),
			}
			totalPercent += int32(pct)

			if targetName == "latest" {
				target.Type = runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST
				if opt.Tag != "" {
					target.Tag = opt.Tag
				}
			} else {
				target.Type = runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION
				target.Revision = targetName
			}
			targets = append(targets, target)
		}

		if totalPercent != 100 {
			return nil, fmt.Errorf("traffic percentages must sum to 100, got %d", totalPercent)
		}
		return targets, nil
	}

	// Default: 100% to latest revision
	target := &runpb.TrafficTarget{
		Type:    runpb.TrafficTargetAllocationType_TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST,
		Percent: 100,
	}
	if opt.Tag != "" {
		target.Tag = opt.Tag
	}
	return []*runpb.TrafficTarget{target}, nil
}
