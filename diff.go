package gcrunpresso

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
)

type DiffOption struct {
	ExitCode bool `help:"exit with code 1 if differences are found" default:"false"`
}

func (d *App) Diff(ctx context.Context, opt DiffOption) error {
	d.LogInfo("comparing local definition with remote resource", "service", d.config.Service, "job", d.config.Job)

	var diffText string
	var err error

	if d.config.Service != "" {
		diffText, err = d.diffService(ctx)
	} else if d.config.Job != "" {
		diffText, err = d.diffJob(ctx)
	} else {
		return fmt.Errorf("either service or job must be specified in config")
	}

	if err != nil {
		return err
	}

	if diffText == "" {
		d.LogInfo("no differences found between local and remote")
		return nil
	}

	// Print colorized diff
	printColorizedDiff(diffText)

	if opt.ExitCode {
		return fmt.Errorf("differences found")
	}
	return nil
}

func (d *App) diffService(ctx context.Context) (string, error) {
	localSvc, err := d.LoadServiceDefinition("")
	if err != nil {
		return "", err
	}

	remoteSvc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	if err != nil {
		if isNotFoundError(err) {
			localYAML, err := MarshalProtoYAML(localSvc)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Service %s does not exist on remote. New service definition:\n%s", d.config.Service, string(localYAML)), nil
		}
		return "", fmt.Errorf("failed to get remote service %s: %w", d.ResourceServicePath(), err)
	}

	return DiffServices(localSvc, remoteSvc)
}

func (d *App) diffJob(ctx context.Context) (string, error) {
	localJob, err := d.LoadJobDefinition("")
	if err != nil {
		return "", err
	}

	remoteJob, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
		Name: d.ResourceJobPath(),
	})
	if err != nil {
		if isNotFoundError(err) {
			localYAML, err := MarshalProtoYAML(localJob)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Job %s does not exist on remote. New job definition:\n%s", d.config.Job, string(localYAML)), nil
		}
		return "", fmt.Errorf("failed to get remote job %s: %w", d.ResourceJobPath(), err)
	}

	return DiffJobs(localJob, remoteJob)
}

// DiffServices computes the text diff between local and remote Service specifications.
func DiffServices(local, remote *runpb.Service) (string, error) {
	cleanedRemote := CleanRemoteService(remote, local)
	cleanedLocal := CleanLocalService(local)

	remoteYAML, err := MarshalProtoYAML(cleanedRemote)
	if err != nil {
		return "", err
	}
	localYAML, err := MarshalProtoYAML(cleanedLocal)
	if err != nil {
		return "", err
	}

	return cmp.Diff(string(remoteYAML), string(localYAML)), nil
}

// DiffJobs computes the text diff between local and remote Job specifications.
func DiffJobs(local, remote *runpb.Job) (string, error) {
	cleanedRemote := CleanRemoteJob(remote, local)
	cleanedLocal := CleanLocalJob(local)

	remoteYAML, err := MarshalProtoYAML(cleanedRemote)
	if err != nil {
		return "", err
	}
	localYAML, err := MarshalProtoYAML(cleanedLocal)
	if err != nil {
		return "", err
	}

	return cmp.Diff(string(remoteYAML), string(localYAML)), nil
}

// CleanRemoteService strips server-generated read-only fields from remote Service.
func CleanRemoteService(remote, local *runpb.Service) *runpb.Service {
	clone := proto.Clone(remote).(*runpb.Service)
	clone.Name = ""
	clone.Uid = ""
	clone.Generation = 0
	clone.CreateTime = nil
	clone.UpdateTime = nil
	clone.DeleteTime = nil
	clone.ExpireTime = nil
	clone.Etag = ""
	clone.Reconciling = false
	clone.Conditions = nil
	clone.TerminalCondition = nil
	clone.LatestReadyRevision = ""
	clone.LatestCreatedRevision = ""
	clone.Uri = ""
	clone.Urls = nil
	clone.ObservedGeneration = 0
	clone.Creator = ""
	clone.LastModifier = ""
	clone.SatisfiesPzs = false

	if clone.Template != nil {
		if local != nil && local.Template != nil && local.Template.Revision == "" {
			clone.Template.Revision = ""
		}
	}
	return clone
}

// CleanLocalService strips transient or comparison-irrelevant fields from local Service.
func CleanLocalService(local *runpb.Service) *runpb.Service {
	clone := proto.Clone(local).(*runpb.Service)
	clone.Name = ""
	clone.Uid = ""
	clone.Conditions = nil
	clone.TerminalCondition = nil
	return clone
}

// CleanRemoteJob strips server-generated read-only fields from remote Job.
func CleanRemoteJob(remote, local *runpb.Job) *runpb.Job {
	clone := proto.Clone(remote).(*runpb.Job)
	clone.Name = ""
	clone.Uid = ""
	clone.Generation = 0
	clone.CreateTime = nil
	clone.UpdateTime = nil
	clone.DeleteTime = nil
	clone.ExpireTime = nil
	clone.Etag = ""
	clone.Reconciling = false
	clone.Conditions = nil
	clone.TerminalCondition = nil
	clone.LatestCreatedExecution = nil
	clone.ObservedGeneration = 0
	clone.Creator = ""
	clone.LastModifier = ""
	clone.SatisfiesPzs = false
	return clone
}

// CleanLocalJob strips transient fields from local Job.
func CleanLocalJob(local *runpb.Job) *runpb.Job {
	clone := proto.Clone(local).(*runpb.Job)
	clone.Name = ""
	clone.Uid = ""
	clone.Conditions = nil
	clone.TerminalCondition = nil
	return clone
}

func printColorizedDiff(diff string) {
	lines := strings.Split(diff, "\n")
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	for _, line := range lines {
		if strings.HasPrefix(line, "-") {
			red.Println(line)
		} else if strings.HasPrefix(line, "+") {
			green.Println(line)
		} else if strings.HasPrefix(line, "@") {
			cyan.Println(line)
		} else {
			fmt.Println(line)
		}
	}
}
