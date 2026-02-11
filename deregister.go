package ecspresso

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/samber/lo"
)

type DeregisterOption struct {
	DryRun   bool   `help:"dry run" default:"false"`
	Keeps    *int   `help:"number of task definitions to keep except in-use"`
	Revision string `help:"revision number or 'latest'" default:""`
	Force    bool   `help:"force deregister without confirmation" default:"false"`
	Delete   bool   `help:"delete task definition on deregistered" default:"false"`
}

func (opt DeregisterOption) DryRunString() string {
	if opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Deregister(ctx context.Context, opt DeregisterOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()
	d.LogInfo("Starting deregister task definition", withDryRun(opt.DryRun)...)

	inUse, err := d.inUseRevisions(ctx)
	if err != nil {
		return err
	}

	if opt.Revision != "" {
		return d.deregiserRevision(ctx, opt, inUse)
	} else if opt.Keeps != nil && *opt.Keeps > 0 {
		return d.deregisterKeeps(ctx, opt, inUse)
	}
	return fmt.Errorf("--revision or --keeps required")
}

func (d *App) deregiserRevision(ctx context.Context, opt DeregisterOption, inUse map[string]string) error {
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	var rv int32
	switch opt.Revision {
	case "latest":
		res, err := d.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: td.Family,
			Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
		})
		if err != nil {
			return fmt.Errorf("failed to describe task definition %s: %w", *td.Family, err)
		}
		rv = res.TaskDefinition.Revision
	default:
		if v, err := strconv.ParseInt(opt.Revision, 10, 64); err != nil {
			return fmt.Errorf("invalid revision number: %w", err)
		} else {
			rv = int32(v)
		}
	}

	name := fmt.Sprintf("%s:%d", aws.ToString(td.Family), rv)

	if s := inUse[name]; s != "" {
		return fmt.Errorf("%s is in use by %s", name, s)
	}

	if opt.DryRun {
		d.LogInfo("task definition will be deregistered", "task_definition", name)
		d.LogInfo("DRY RUN OK")
		return nil
	}
	confirmed := opt.Force || prompter.YesNo(fmt.Sprintf("Deregister %s ?", name), false)
	if !confirmed {
		d.LogInfo("Aborted")
		return fmt.Errorf("confirmation failed")
	}

	d.LogInfo("deregistering", "task_definition", name)
	if _, err := d.ecs.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(name),
	}); err != nil {
		return fmt.Errorf("failed to deregister task definition: %w", err)
	}
	d.LogInfo("task definition deregistered", "task_definition", name)
	if opt.Delete {
		d.LogInfo("deleting task definition", "task_definition", name)
		if _, err := d.ecs.DeleteTaskDefinitions(ctx, &ecs.DeleteTaskDefinitionsInput{
			TaskDefinitions: []string{name},
		}); err != nil {
			return fmt.Errorf("failed to delete task definition: %w", err)
		}
		d.LogInfo("task definition deleted", "task_definition", name)
	}
	return nil
}

func (d *App) deregisterKeeps(ctx context.Context, opt DeregisterOption, inUse map[string]string) error {
	keeps := aws.ToInt(opt.Keeps)
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	names := []string{}
	var nextToken *string
	for {
		res, err := d.ecs.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: td.Family,
			NextToken:    nextToken,
		})
		if err != nil {
			return fmt.Errorf("failed to list task definitions: %w", err)
		}
		for _, a := range res.TaskDefinitionArns {
			name, err := taskDefinitionToName(a)
			if err != nil {
				continue
			}
			if s := inUse[name]; s != "" {
				d.LogInfo("task definition in use, skipping", "task_definition", name, "used_by", s)
			} else {
				d.LogDebug("%s is marked to deregister", name)
				names = append(names, name)
			}
		}
		if nextToken = res.NextToken; nextToken == nil {
			break
		}
	}

	deregs := []string{}
	idx := len(names) - keeps
	if idx <= 0 {
		d.LogInfo("No need to deregister task definitions")
		return nil
	}
	for i, name := range names {
		if i < idx {
			d.LogInfo("task definition will be deregistered", "task_definition", name)
			deregs = append(deregs, name)
		}
	}
	if opt.DryRun {
		d.LogInfo("DRY RUN OK")
		return nil
	}

	deregistered := 0
	confirmed := opt.Force || prompter.YesNo(fmt.Sprintf("Deregister %d revisons?", len(deregs)), false)
	if !confirmed {
		d.LogInfo("Aborted")
		return fmt.Errorf("confirmation failed")
	}
	for _, name := range deregs {
		d.LogInfo("deregistering", "task_definition", name)
		if _, err := d.ecs.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(name),
		}); err != nil {
			return fmt.Errorf("failed to deregister task definition: %w", err)
		}
		d.LogInfo("task definition deregistered", "task_definition", name)
		sleepContext(ctx, time.Second)
		deregistered++
	}
	d.LogInfo("task definitions deregistered", "count", deregistered)
	if opt.Delete {
		for _, names := range lo.Chunk(deregs, 10) { // 10 is max batch size
			d.LogInfo("deleting task definitions", "task_definitions", strings.Join(names, ","))
			if _, err := d.ecs.DeleteTaskDefinitions(ctx, &ecs.DeleteTaskDefinitionsInput{
				TaskDefinitions: names,
			}); err != nil {
				return fmt.Errorf("failed to delete task definition: %w", err)
			}
			d.LogInfo("task definitions deleted", "count", len(names))
		}
	}
	return nil
}

func (d *App) inUseRevisions(ctx context.Context) (map[string]string, error) {
	inUse := make(map[string]string)
	tasks, err := d.listTasks(ctx)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		name, _ := taskDefinitionToName(*task.TaskDefinitionArn)
		st := aws.ToString(task.LastStatus)
		if st == "" {
			st = aws.ToString(task.DesiredStatus)
		}
		if st != "STOPPED" {
			// ignore STOPPED tasks for in use
			inUse[name] = st + " task"
		}
		d.LogDebug("%s is in use by tasks", name)
	}

	if d.config.Service != "" {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return nil, err
		}
		for _, dp := range sv.Deployments {
			name, _ := taskDefinitionToName(*dp.TaskDefinition)
			inUse[name] = fmt.Sprintf("%s deployment", *dp.Status)
			d.LogDebug("%s is in use by deployments", name)
		}
	}
	return inUse, nil
}

func taskDefinitionToName(a string) (string, error) {
	an, err := arn.Parse(a)
	if err != nil {
		return "", err
	}
	n := strings.SplitN(an.Resource, "/", 2)
	return n[1], nil
}
