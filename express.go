package ecspresso

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-jsonnet/formatter"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/kylelemons/godebug/diff"
)

type ExpressGatewayService struct {
	*ecs.CreateExpressGatewayServiceInput
}

func (e *ExpressGatewayService) SetTags(tags []types.Tag) {
	e.Tags = tags
}

func (e *ExpressGatewayService) GetTags() []types.Tag {
	return e.Tags
}

func (d *App) initExpressGatewayService(ctx context.Context, opt InitOption) (*ExpressGatewayService, *Service, error) {
	conf := d.config
	ex, sv, err := d.DescribeExpressGatewayService(ctx)
	if err != nil {
		return nil, nil, err
	}

	// remove fields in definition that will be set by config
	ex.ServiceName = nil
	ex.Cluster = nil

	b, err := MarshalJSONForAPI(ex)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal express gateway service to JSON: %w", err)
	}
	if opt.Jsonnet {
		out, err := formatter.Format(conf.ExpressDefinitionPath, string(b), formatter.DefaultOptions())
		if err != nil {
			return nil, nil, fmt.Errorf("unable to format express gateway service as Jsonnet: %w", err)
		}
		b = []byte(out)
	}
	d.LogInfo("save the express gateway service %s to %s", aws.ToString(ex.ServiceName), conf.ExpressDefinitionPath)
	if err := d.saveFile(conf.ExpressDefinitionPath, b, CreateFileMode, opt.ForceOverwrite); err != nil {
		return nil, nil, err
	}

	return ex, sv, nil
}

func (d *App) DescribeExpressGatewayService(ctx context.Context) (*ExpressGatewayService, *Service, error) {
	sv, err := d.DescribeService(ctx)
	if err != nil {
		return nil, nil, err
	}
	if sv.ResourceManagementType != types.ResourceManagementTypeEcs {
		return nil, nil, fmt.Errorf("service %s is not an express mode", aws.ToString(sv.ServiceName))
	}
	res, err := d.ecs.DescribeExpressGatewayService(ctx, &ecs.DescribeExpressGatewayServiceInput{
		ServiceArn: sv.ServiceArn,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe express gateway service: %w", err)
	}
	rex := res.Service
	if res.Service == nil {
		return nil, nil, ErrNotFound("express gateway service is not found")
	}
	if len(rex.ActiveConfigurations) == 0 {
		return nil, nil, fmt.Errorf("express gateway service %s has no active configuration", aws.ToString(rex.ServiceName))
	}
	ac := rex.ActiveConfigurations[0]
	in := &ecs.CreateExpressGatewayServiceInput{
		Cluster:               aws.String(d.config.Cluster),
		ServiceName:           rex.ServiceName,
		InfrastructureRoleArn: rex.InfrastructureRoleArn,
		ExecutionRoleArn:      ac.ExecutionRoleArn,
		Cpu:                   ac.Cpu,
		Memory:                ac.Memory,
		HealthCheckPath:       ac.HealthCheckPath,
		NetworkConfiguration:  ac.NetworkConfiguration,
		ScalingTarget:         ac.ScalingTarget,
		TaskRoleArn:           ac.TaskRoleArn,
		PrimaryContainer:      ac.PrimaryContainer,
		Tags:                  sv.Tags,
	}
	return &ExpressGatewayService{
		CreateExpressGatewayServiceInput: in,
	}, sv, nil
}

func (d *App) LoadExpressDefinition(path string) (*ExpressGatewayService, error) {
	if path == "" {
		return nil, fmt.Errorf("express_definition is not defined")
	}

	var ex ExpressGatewayService
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load express definition %s: %w", path, err)
	}
	if err := unmarshalJSON(src, &ex, path); err != nil {
		return nil, fmt.Errorf("failed to load express definition %s: %w", path, err)
	}

	ex.ServiceName = aws.String(d.config.Service)
	ex.Cluster = &d.config.Cluster

	if err := d.config.Ignore.Apply(&ex); err != nil {
		return nil, fmt.Errorf("failed to apply ignore: %w", err)
	}
	return &ex, nil
}

func diffExpressGatewayServices(ctx context.Context, local, remote *ExpressGatewayService, localPath string, opt *DiffOption) (bool, error) {
	var remoteArn string
	if remote != nil {
		remoteArn = aws.ToString(remote.ServiceName)
	}

	//localSvForDiff := ServiceDefinitionForDiff(local)
	//remoteSvForDiff := ServiceDefinitionForDiff(remote)

	newSvBytes, err := MarshalJSONForAPI(local)
	if err != nil {
		return false, fmt.Errorf("failed to marshal new express gateway service definition: %w", err)
	}
	remoteSvBytes, err := MarshalJSONForAPI(remote)
	if err != nil {
		return false, fmt.Errorf("failed to marshal remote express gateway service definition: %w", err)
	}

	remoteSv := toDiffString(remoteSvBytes)
	newSv := toDiffString(newSvBytes)
	if remoteSv == newSv {
		return false, nil
	}

	switch {
	case opt.External != "":
		return true, diffExternal(ctx, opt.External, "service", remoteSv, newSv, opt)
	case opt.Unified:
		edits := myers.ComputeEdits(span.URIFromPath(remoteArn), remoteSv, newSv)
		ds := fmt.Sprint(gotextdiff.ToUnified(remoteArn, localPath, remoteSv, edits))
		fmt.Fprint(opt.w, coloredDiff(ds))
		return true, nil
	default:
		ds := diff.Diff(remoteSv, newSv)
		fmt.Fprint(opt.w, coloredDiff(fmt.Sprintf("--- %s\n+++ %s\n%s", remoteArn, localPath, ds)))
		return true, nil
	}
}
