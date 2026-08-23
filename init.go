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

// writeDefinitionFile writes a generated definition or config file with owner-only
// permissions. os.WriteFile applies its mode only when it creates the file, so an
// overwrite under --force would otherwise keep a pre-existing world-readable mode --
// and these files can contain plaintext values copied from the remote resource.
func writeDefinitionFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", path, err)
	}
	return nil
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
	if err := writeDefinitionFile(svcFilePath, svcYAML); err != nil {
		return err
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
	if err := writeDefinitionFile(cfgFilePath, cfgYAML); err != nil {
		return err
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
	if err := writeDefinitionFile(jobFilePath, jobYAML); err != nil {
		return err
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
	if err := writeDefinitionFile(cfgFilePath, cfgYAML); err != nil {
		return err
	}
	d.LogInfo("wrote gcrunpresso config", "file", cfgFilePath)

	return nil
}

// sensitiveKeywords are matched as substrings against the env var NAME. Entries must not
// be prefixes of each other ("PASS" would subsume "PASSWORD" while also matching
// BYPASS_CACHE; "KEY" already covers "API_KEY"), and PASSWD is spelled out because it is
// not a substring of PASSWORD.
var sensitiveKeywords = []string{
	"SECRET", "PASSWORD", "PASSWD", "KEY", "TOKEN", "CREDENTIAL", "AUTH",
	"DATABASE_URL", "DB_URI", "DSN", "CONNECTION_STRING", "PRIVATE", "SIGNING",
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
