package gcrunpresso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/kayac/gcrunpresso/v2/external"
	"google.golang.org/api/option"
)

type ConfigPlugin struct {
	Name       string         `yaml:"name" json:"name,omitempty"`
	Config     map[string]any `yaml:"config" json:"config,omitempty"`
	FuncPrefix string         `yaml:"func_prefix,omitempty" json:"func_prefix,omitempty"`
}

func (p ConfigPlugin) Setup(ctx context.Context, c *Config, l *configLoader) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(ctx, p, c, l)
	case "external":
		return setupPluginExternal(ctx, p, c, l)
	case "secretmanager":
		return setupPluginSecretManager(ctx, p, c, l)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func setupPluginTFState(ctx context.Context, p ConfigPlugin, c *Config, l *configLoader) error {
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

	prefix := p.FuncPrefix
	lookupFunc := func(path string) (any, error) {
		obj, err := state.Lookup(path)
		if err != nil {
			return nil, err
		}
		return obj, nil
	}
	outputFunc := func(name string) (any, error) {
		obj, err := state.Lookup("output." + name)
		if err != nil {
			return nil, err
		}
		return obj, nil
	}

	fnName := "tfstate"
	fnOutputName := "tfstate_output"
	if prefix != "" {
		fnName = prefix
		fnOutputName = prefix + "_output"
	}

	if l.FuncMap != nil {
		l.FuncMap[fnName] = lookupFunc
		l.FuncMap[fnOutputName] = outputFunc
		l.Loader.Funcs(l.FuncMap)
	}

	if l.VM != nil {
		l.VM.NativeFunction(&jsonnet.NativeFunction{
			Name:   fnName,
			Params: []ast.Identifier{"path"},
			Func: func(args []any) (any, error) {
				path, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("tfstate: path must be string")
				}
				return lookupFunc(path)
			},
		})
		l.VM.NativeFunction(&jsonnet.NativeFunction{
			Name:   fnOutputName,
			Params: []ast.Identifier{"name"},
			Func: func(args []any) (any, error) {
				name, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("tfstate_output: name must be string")
				}
				return outputFunc(name)
			},
		})
	}

	return nil
}

func setupPluginSecretManager(ctx context.Context, p ConfigPlugin, c *Config, l *configLoader) error {
	client, err := secretmanager.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %w", err)
	}

	prefix := p.FuncPrefix
	if prefix == "" {
		prefix = "secret"
	}

	fetchSecret := func(name string) (string, error) {
		secretPath := name
		if !strings.HasPrefix(secretPath, "projects/") {
			secretPath = fmt.Sprintf("projects/%s/secrets/%s/versions/latest", c.Project, name)
		} else if !strings.Contains(secretPath, "/versions/") {
			secretPath = fmt.Sprintf("%s/versions/latest", secretPath)
		}

		resp, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: secretPath,
		})
		if err != nil {
			return "", fmt.Errorf("failed to access secret %s: %w", secretPath, err)
		}
		return string(resp.Payload.Data), nil
	}

	if l.FuncMap != nil {
		l.FuncMap[prefix] = fetchSecret
		l.Loader.Funcs(l.FuncMap)
	}

	if l.VM != nil {
		l.VM.NativeFunction(&jsonnet.NativeFunction{
			Name:   prefix,
			Params: []ast.Identifier{"name"},
			Func: func(args []any) (any, error) {
				name, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("secret: name must be string")
				}
				return fetchSecret(name)
			},
		})
	}

	return nil
}

func setupPluginExternal(ctx context.Context, p ConfigPlugin, c *Config, l *configLoader) error {
	extCfg := &external.Config{}
	b, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin config: %w", err)
	}
	if err := json.Unmarshal(b, extCfg); err != nil {
		return fmt.Errorf("failed to unmarshal external plugin config: %w", err)
	}
	extPlugin, err := external.NewPlugin(ctx, extCfg)
	if err != nil {
		return err
	}

	prefix := p.FuncPrefix
	if prefix == "" {
		prefix = extCfg.Name
	}
	if prefix == "" {
		prefix = "external"
	}

	execFunc := func(args ...string) (any, error) {
		return extPlugin.Exec(ctx, args)
	}

	if l.FuncMap != nil {
		l.FuncMap[prefix] = execFunc
		l.Loader.Funcs(l.FuncMap)
	}

	if l.VM != nil {
		l.VM.NativeFunction(&jsonnet.NativeFunction{
			Name:   prefix,
			Params: []ast.Identifier{"args"},
			Func: func(args []any) (any, error) {
				var strArgs []string
				if len(args) > 0 {
					if list, ok := args[0].([]any); ok {
						for _, item := range list {
							strArgs = append(strArgs, fmt.Sprintf("%v", item))
						}
					} else {
						strArgs = append(strArgs, fmt.Sprintf("%v", args[0]))
					}
				}
				return extPlugin.Exec(ctx, strArgs)
			},
		})
	}

	return nil
}
