package gcrunpresso

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/goccy/go-yaml"
)

type InitOption struct {
	Dir   string `help:"directory to write configuration files" default:"."`
	Force bool   `help:"overwrite existing files" default:"false"`
}

func (d *App) Init(ctx context.Context, opt InitOption) error {
	d.LogInfo("initializing configuration from remote resource", "service", d.config.Service, "job", d.config.Job, "dir", opt.Dir)

	if err := os.MkdirAll(opt.Dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", opt.Dir, err)
	}

	if d.config.Service != "" {
		return d.initService(ctx, opt)
	}
	if d.config.Job != "" {
		return d.initJob(ctx, opt)
	}
	return fmt.Errorf("either service or job must be specified to init")
}

func (d *App) initService(ctx context.Context, opt InitOption) error {
	remoteSvc, err := d.servicesClient.GetService(ctx, &runpb.GetServiceRequest{
		Name: d.ResourceServicePath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", d.ResourceServicePath(), err)
	}

	cleanedSvc := CleanRemoteService(remoteSvc, nil)
	if remoteSvc.Template != nil {
		for _, c := range remoteSvc.Template.Containers {
			d.warnPlaintextSecretsInEnv(c.Name, c.Env)
		}
	}
	svcYAML, err := MarshalProtoYAML(cleanedSvc)
	if err != nil {
		return fmt.Errorf("failed to format service YAML: %w", err)
	}

	svcFilePath := filepath.Join(opt.Dir, "service.yaml")
	if !opt.Force {
		if _, err := os.Stat(svcFilePath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", svcFilePath)
		}
	}
	if err := os.WriteFile(svcFilePath, svcYAML, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", svcFilePath, err)
	}
	d.LogInfo("wrote service definition", "file", svcFilePath)

	cfg := Config{
		Project:               d.config.Project,
		Location:              d.config.Location,
		Service:               d.config.Service,
		ServiceDefinitionPath: "service.yaml",
	}
	cfgYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to format config YAML: %w", err)
	}

	cfgFilePath := filepath.Join(opt.Dir, "gcrunpresso.yml")
	if !opt.Force {
		if _, err := os.Stat(cfgFilePath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", cfgFilePath)
		}
	}
	if err := os.WriteFile(cfgFilePath, cfgYAML, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cfgFilePath, err)
	}
	d.LogInfo("wrote gcrunpresso config", "file", cfgFilePath)

	return nil
}

func (d *App) initJob(ctx context.Context, opt InitOption) error {
	remoteJob, err := d.jobsClient.GetJob(ctx, &runpb.GetJobRequest{
		Name: d.ResourceJobPath(),
	})
	if err != nil {
		return fmt.Errorf("failed to get job %s: %w", d.ResourceJobPath(), err)
	}

	cleanedJob := CleanRemoteJob(remoteJob, nil)
	if remoteJob.Template != nil && remoteJob.Template.Template != nil {
		for _, c := range remoteJob.Template.Template.Containers {
			d.warnPlaintextSecretsInEnv(c.Name, c.Env)
		}
	}
	jobYAML, err := MarshalProtoYAML(cleanedJob)
	if err != nil {
		return fmt.Errorf("failed to format job YAML: %w", err)
	}

	jobFilePath := filepath.Join(opt.Dir, "job.yaml")
	if !opt.Force {
		if _, err := os.Stat(jobFilePath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", jobFilePath)
		}
	}
	if err := os.WriteFile(jobFilePath, jobYAML, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", jobFilePath, err)
	}
	d.LogInfo("wrote job definition", "file", jobFilePath)

	cfg := Config{
		Project:           d.config.Project,
		Location:          d.config.Location,
		Job:               d.config.Job,
		JobDefinitionPath: "job.yaml",
	}
	cfgYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to format config YAML: %w", err)
	}

	cfgFilePath := filepath.Join(opt.Dir, "gcrunpresso.yml")
	if !opt.Force {
		if _, err := os.Stat(cfgFilePath); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", cfgFilePath)
		}
	}
	if err := os.WriteFile(cfgFilePath, cfgYAML, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cfgFilePath, err)
	}
	d.LogInfo("wrote gcrunpresso config", "file", cfgFilePath)

	return nil
}

var sensitiveKeywords = []string{
	"SECRET", "PASSWORD", "KEY", "TOKEN", "CREDENTIAL", "API_KEY", "AUTH",
}

func (d *App) warnPlaintextSecretsInEnv(containerName string, envVars []*runpb.EnvVar) {
	for _, env := range envVars {
		if env == nil {
			continue
		}
		if vs := env.GetValueSource(); vs != nil && vs.GetSecretKeyRef() != nil {
			continue // Already using Secret Manager reference
		}
		upper := strings.ToUpper(env.Name)
		for _, kw := range sensitiveKeywords {
			if strings.Contains(upper, kw) {
				d.LogWarn("plaintext credential detected in environment variable; consider using Secret Manager with valueSource.secretKeyRef", "container", containerName, "env", env.Name)
				break
			}
		}
	}
}
