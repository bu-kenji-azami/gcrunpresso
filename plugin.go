package ecspresso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/fujiwara/cfn-lookup/cfn"
	"github.com/fujiwara/ssm-lookup/ssm"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/google/go-jsonnet"
	"github.com/kayac/ecspresso/v2/external"
	"github.com/kayac/ecspresso/v2/secretsmanager"
	"github.com/samber/lo"
)

var defaultPluginNames = []string{"ssm", "secretsmanager"}

type ConfigPlugin struct {
	Name       string         `yaml:"name" json:"name,omitempty"`
	Config     map[string]any `yaml:"config" json:"config,omitempty"`
	FuncPrefix string         `yaml:"func_prefix,omitempty" json:"func_prefix,omitempty"`
}

func (p ConfigPlugin) Setup(ctx context.Context, c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(ctx, p, c)
	case "cloudformation":
		return setupPluginCFn(ctx, p, c)
	case "ssm":
		return setupPluginSSM(ctx, p, c)
	case "secretsmanager":
		return setupPluginSecretsManager(ctx, p, c)
	case "external":
		return setupPluginExternal(ctx, p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func (p ConfigPlugin) AppendFuncMap(c *Config, funcMap template.FuncMap) error {
	modified := make(template.FuncMap, len(funcMap))
	for funcName, f := range funcMap {
		name := p.FuncPrefix + funcName
		for _, appendedFuncs := range c.templateFuncs {
			if _, exists := appendedFuncs[name]; exists {
				if lo.Contains(defaultPluginNames, p.Name) {
					LogDebug("template function %s already exists by default plugins. skip", name)
					continue
				}
				return fmt.Errorf("template function %s already exists. set func_prefix to %s plugin", name, p.Name)
			}
		}
		modified[name] = f
	}
	c.templateFuncs = append(c.templateFuncs, modified)
	return nil
}

func (p ConfigPlugin) AppendJsonnetNativeFuncs(c *Config, funcs []*jsonnet.NativeFunction) error {
	for _, f := range funcs {
		f.Name = p.FuncPrefix + f.Name
		for _, appendedFuncs := range c.jsonnetNativeFuncs {
			if appendedFuncs.Name == f.Name {
				if lo.Contains(defaultPluginNames, p.Name) {
					LogDebug("jsonnet native function %s already exists by default plugins. skip", f.Name)
					continue
				}
				return fmt.Errorf("jsonnet native function %s already exists. set func_prefix to %s plugin", f.Name, p.Name)
			}
		}
		c.jsonnetNativeFuncs = append(c.jsonnetNativeFuncs, f)
	}
	return nil
}

func setupPluginTFState(ctx context.Context, p ConfigPlugin, c *Config) error {
	var loc string
	if p.Config["path"] != nil {
		path, ok := p.Config["path"].(string)
		if !ok {
			return errors.New("tfstate plugin requires path for tfstate file as a string")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.dir, path)
		}
		loc = path
	} else if p.Config["url"] != nil {
		u, ok := p.Config["url"].(string)
		if !ok {
			return errors.New("tfstate plugin requires url for tfstate URL as a string")
		}
		loc = u
	} else {
		return errors.New("tfstate plugin requires path or url for tfstate location")
	}

	var optional bool
	if v, exists := p.Config["optional"]; exists {
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("tfstate plugin: optional must be a bool, got %T", v)
		}
		optional = b
	}
	state, err := tfstate.ReadURL(ctx, loc)
	if err != nil {
		if !optional {
			return err
		}
		LogWarn("tfstate plugin: failed to read tfstate, continuing with empty state because optional=true",
			"location", loc, "error", err.Error())
		state = tfstate.Empty()
	}
	c.pluginInstances = append(c.pluginInstances, pluginInstance{
		name:       "tfstate",
		funcPrefix: p.FuncPrefix,
		value:      state,
	})

	if err := p.AppendFuncMap(c, state.FuncMap(ctx)); err != nil {
		return err
	}
	return p.AppendJsonnetNativeFuncs(c, state.JsonnetNativeFuncs(ctx))
}

func setupPluginCFn(ctx context.Context, p ConfigPlugin, c *Config) error {
	cache := sync.Map{}
	lookup := cfn.New(c.awsv2Config, &cache)
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}

func setupPluginSSM(ctx context.Context, p ConfigPlugin, c *Config) error {
	cache := sync.Map{}
	lookup := ssm.New(c.awsv2Config, &cache)
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}

func setupPluginSecretsManager(ctx context.Context, p ConfigPlugin, c *Config) error {
	lookup := secretsmanager.NewApp(c.awsv2Config)
	if err := p.AppendFuncMap(c, lookup.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, lookup.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}

func setupPluginExternal(ctx context.Context, p ConfigPlugin, c *Config) error {
	extCfg := &external.Config{}
	b, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin config: %w", err)
	}
	if err := json.Unmarshal(b, extCfg); err != nil {
		return fmt.Errorf("failed to unmarshal external plugin config: %w", err)
	}
	ext, err := external.NewPlugin(ctx, extCfg)
	if err != nil {
		return err
	}
	if err := p.AppendFuncMap(c, ext.FuncMap(ctx)); err != nil {
		return err
	}
	if err := p.AppendJsonnetNativeFuncs(c, ext.JsonnetNativeFuncs(ctx)); err != nil {
		return err
	}
	return nil
}
