package gcrunpresso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/kayac/gcrunpresso/v2/external"
)

type ConfigPlugin struct {
	Name       string         `yaml:"name" json:"name,omitempty"`
	Config     map[string]any `yaml:"config" json:"config,omitempty"`
	FuncPrefix string         `yaml:"func_prefix,omitempty" json:"func_prefix,omitempty"`
}

func (p ConfigPlugin) Setup(ctx context.Context, c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(ctx, p, c)
	case "external":
		return setupPluginExternal(ctx, p, c)
	case "secretmanager":
		return setupPluginSecretManager(ctx, p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
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

	_ = state
	return nil
}

func setupPluginSecretManager(ctx context.Context, p ConfigPlugin, c *Config) error {
	// Implemented in U7
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
	_, err = external.NewPlugin(ctx, extCfg)
	if err != nil {
		return err
	}
	return nil
}
