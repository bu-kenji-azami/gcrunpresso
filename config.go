package gcrunpresso

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/goccy/go-yaml"
	"github.com/kayac/go-config"
)

const (
	defaultTimeout = 10 * time.Minute
	jsonnetExt     = ".jsonnet"
)

var definitionFileExtensions = []string{".yaml", ".yml", ".json", ".jsonnet"}

type Config struct {
	Project                   string          `yaml:"project" json:"project"`
	Location                  string          `yaml:"location" json:"location"`
	Service                   string          `yaml:"service,omitempty" json:"service,omitempty"`
	Job                       string          `yaml:"job,omitempty" json:"job,omitempty"`
	ServiceDefinitionPath     string          `yaml:"service_definition,omitempty" json:"service_definition,omitempty"`
	JobDefinitionPath         string          `yaml:"job_definition,omitempty" json:"job_definition,omitempty"`
	ImpersonateServiceAccount string          `yaml:"impersonate_service_account,omitempty" json:"impersonate_service_account,omitempty"`
	Timeout                   Duration        `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Plugins                   []*PluginConfig `yaml:"plugins,omitempty" json:"plugins,omitempty"`

	dir string
}

type PluginConfig struct {
	Name   string         `yaml:"name" json:"name"`
	Type   string         `yaml:"type" json:"type"`
	Config map[string]any `yaml:"config" json:"config"`
}

func NewDefaultConfig() *Config {
	return &Config{
		Timeout: Duration{Duration: defaultTimeout},
	}
}

type configLoader struct {
	*config.Loader
	VM      *jsonnet.VM
	FuncMap template.FuncMap
}

func newConfigLoader() *configLoader {
	cl := &configLoader{
		Loader:  config.New(),
		VM:      jsonnet.MakeVM(),
		FuncMap: make(template.FuncMap),
	}
	cl.Funcs(template.FuncMap{
		"env": func(keys ...string) string {
			if len(keys) == 0 {
				return ""
			}
			val := os.Getenv(keys[0])
			if val != "" {
				return val
			}
			if len(keys) > 1 {
				return keys[1]
			}
			return ""
		},
		"must_env": func(key string) (string, error) {
			val := os.Getenv(key)
			if val == "" {
				return "", fmt.Errorf("environment variable %s is not set", key)
			}
			return val, nil
		},
	})
	cl.VM.NativeFunction(&jsonnet.NativeFunction{
		Name:   "env",
		Params: []ast.Identifier{"key", "default"},
		Func: func(args []any) (any, error) {
			key, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("first argument must be string")
			}
			val := os.Getenv(key)
			if val != "" {
				return val, nil
			}
			if len(args) > 1 {
				if def, ok := args[1].(string); ok {
					return def, nil
				}
			}
			return "", nil
		},
	})
	cl.VM.NativeFunction(&jsonnet.NativeFunction{
		Name:   "must_env",
		Params: []ast.Identifier{"key"},
		Func: func(args []any) (any, error) {
			key, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("must_env: key must be a string")
			}
			val, ok := os.LookupEnv(key)
			if !ok || val == "" {
				return nil, fmt.Errorf("must_env: %s is not set", key)
			}
			return val, nil
		},
	})
	return cl
}

func (l *configLoader) Load(ctx context.Context, path string, c *Config) error {
	c.dir = filepath.Dir(path)
	var content []byte
	var err error

	if filepath.Ext(path) == jsonnetExt {
		jsonStr, err := l.VM.EvaluateFile(path)
		if err != nil {
			return fmt.Errorf("failed to evaluate jsonnet %s: %w", path, err)
		}
		content, err = l.ReadWithEnvBytes([]byte(jsonStr))
		if err != nil {
			return fmt.Errorf("failed to process env in %s: %w", path, err)
		}
	} else {
		content, err = l.ReadWithEnv(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
	}

	if err := yaml.Unmarshal(content, c); err != nil {
		return fmt.Errorf("failed to parse config %s: %w", path, err)
	}

	for _, p := range c.Plugins {
		cp := ConfigPlugin{
			Name:   p.Name,
			Config: p.Config,
		}
		if err := cp.Setup(ctx, c, l); err != nil {
			return fmt.Errorf("failed to setup plugin %s: %w", p.Name, err)
		}
	}

	return nil
}

func (c *Config) Restrict(opt *Option) error {
	if opt.Project != "" {
		c.Project = opt.Project
	}
	if c.Project == "" {
		c.Project = os.Getenv("GOOGLE_CLOUD_PROJECT")
		if c.Project == "" {
			c.Project = os.Getenv("CLOUDSDK_CORE_PROJECT")
		}
	}

	if opt.Location != "" {
		c.Location = opt.Location
	}
	if c.Location == "" {
		c.Location = os.Getenv("CLOUDSDK_COMPUTE_REGION")
	}

	if opt.Service != "" {
		c.Service = opt.Service
	}
	if opt.Job != "" {
		c.Job = opt.Job
	}
	if opt.ImpersonateServiceAccount != "" {
		c.ImpersonateServiceAccount = opt.ImpersonateServiceAccount
	}
	if opt.Timeout > 0 {
		c.Timeout = Duration{Duration: opt.Timeout}
	}

	if c.Project == "" {
		return fmt.Errorf("project is required (set in config, --project, or GOOGLE_CLOUD_PROJECT)")
	}
	if c.Location == "" {
		return fmt.Errorf("location is required (set in config, --location, or CLOUDSDK_COMPUTE_REGION)")
	}
	if c.Service == "" && c.Job == "" {
		return fmt.Errorf("either service or job is required")
	}

	// Auto-resolve definition paths if not set
	if c.Service != "" && c.ServiceDefinitionPath == "" {
		for _, ext := range definitionFileExtensions {
			p := filepath.Join(c.dir, "service"+ext)
			if _, err := os.Stat(p); err == nil {
				c.ServiceDefinitionPath = p
				break
			}
		}
	}
	if c.Job != "" && c.JobDefinitionPath == "" {
		for _, ext := range definitionFileExtensions {
			p := filepath.Join(c.dir, "job"+ext)
			if _, err := os.Stat(p); err == nil {
				c.JobDefinitionPath = p
				break
			}
		}
	}

	return nil
}

// LoadServiceDefinition loads and parses a Cloud Run Service definition file.
func (d *App) LoadServiceDefinition(path string) (*runpb.Service, error) {
	if path == "" {
		path = d.config.ServiceDefinitionPath
	}
	if path == "" {
		return nil, fmt.Errorf("service definition path is not set")
	}
	if !filepath.IsAbs(path) && d.config.dir != "" {
		path = filepath.Join(d.config.dir, path)
	}

	b, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read service definition file %s: %w", path, err)
	}

	var svc runpb.Service
	if err := UnmarshalService(b, &svc, false); err != nil {
		return nil, fmt.Errorf("failed to parse service definition %s: %w", path, err)
	}

	return &svc, nil
}

// LoadJobDefinition loads and parses a Cloud Run Job definition file.
func (d *App) LoadJobDefinition(path string) (*runpb.Job, error) {
	if path == "" {
		path = d.config.JobDefinitionPath
	}
	if path == "" {
		return nil, fmt.Errorf("job definition path is not set")
	}
	if !filepath.IsAbs(path) && d.config.dir != "" {
		path = filepath.Join(d.config.dir, path)
	}

	b, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read job definition file %s: %w", path, err)
	}

	var job runpb.Job
	if err := UnmarshalJob(b, &job, false); err != nil {
		return nil, fmt.Errorf("failed to parse job definition %s: %w", path, err)
	}

	return &job, nil
}

func (d *App) readDefinitionFile(path string) ([]byte, error) {
	if filepath.Ext(path) == jsonnetExt {
		jsonStr, err := d.loader.VM.EvaluateFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet %s: %w", path, err)
		}
		return d.loader.ReadWithEnvBytes([]byte(jsonStr))
	}
	return d.loader.ReadWithEnv(path)
}
