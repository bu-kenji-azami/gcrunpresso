package gcrunpresso

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

const (
	DefaultConfigFilePath = "gcrunpresso.yml"
	ymlExt                = ".yml"
	yamlExt               = ".yaml"
	jsonExt               = ".json"
)

type CLIOptions struct {
	Envfile                   []string          `help:"environment files" env:"GCRUNPRESSO_ENVFILE"`
	Debug                     bool              `help:"enable debug log" env:"GCRUNPRESSO_DEBUG"`
	ExtStr                    map[string]string `help:"external string values for Jsonnet" env:"GCRUNPRESSO_EXT_STR"`
	ExtCode                   map[string]string `help:"external code values for Jsonnet" env:"GCRUNPRESSO_EXT_CODE"`
	ConfigFilePath            string            `name:"config" help:"config file" default:"gcrunpresso.yml" env:"GCRUNPRESSO_CONFIG"`
	Project                   string            `help:"GCP Project ID" env:"GOOGLE_CLOUD_PROJECT"`
	Location                  string            `help:"GCP Location/Region" env:"CLOUDSDK_COMPUTE_REGION"`
	Service                   string            `help:"Cloud Run Service name" env:"GCRUNPRESSO_SERVICE"`
	Job                       string            `help:"Cloud Run Job name" env:"GCRUNPRESSO_JOB"`
	ImpersonateServiceAccount string            `help:"Service Account email to impersonate" env:"GCRUNPRESSO_IMPERSONATE_SERVICE_ACCOUNT"`
	Timeout                   *time.Duration    `help:"timeout duration" env:"GCRUNPRESSO_TIMEOUT"`
	Color                     bool              `help:"enable colorized output" env:"GCRUNPRESSO_COLOR" default:"true" negatable:""`
	LogFormat                 string            `help:"log format" env:"GCRUNPRESSO_LOG_FORMAT" default:"text" enum:"text,json"`

	Deploy     *DeployOption     `cmd:"" help:"deploy service or job"`
	Run        *RunOption        `cmd:"" help:"run job execution"`
	Rollback   *RollbackOption   `cmd:"" help:"rollback service traffic or template"`
	Scale      *ScaleOption      `cmd:"" help:"scale service instances"`
	Status     *StatusOption     `cmd:"" help:"show status of service or job"`
	Revisions  *RevisionsOption  `cmd:"" help:"list service revisions"`
	Executions *ExecutionsOption `cmd:"" help:"list job executions"`
	Diff       *DiffOption       `cmd:"" help:"show diff between local definition and remote resource"`
	Init       *InitOption       `cmd:"" help:"create configuration files from existing Cloud Run resource"`
	Verify     *VerifyOption     `cmd:"" help:"verify referenced resources"`
	Render     *RenderOption     `cmd:"" help:"render configuration files to STDOUT"`
	Wait       *WaitOption       `cmd:"" help:"wait until service or job is ready"`
	Delete     *DeleteOption     `cmd:"" help:"delete service or job"`
	Version    struct{}          `cmd:"" help:"show version"`
}

func (opt *CLIOptions) resolveConfigFilePath() (path string) {
	path = DefaultConfigFilePath
	defer func() {
		opt.ConfigFilePath = path
	}()
	if opt.ConfigFilePath != "" && opt.ConfigFilePath != DefaultConfigFilePath {
		path = opt.ConfigFilePath
		return
	}
	for _, ext := range []string{ymlExt, yamlExt, jsonExt, jsonnetExt} {
		if _, err := os.Stat("gcrunpresso" + ext); err == nil {
			path = "gcrunpresso" + ext
			return
		}
	}
	return
}

func dispatchCLI(ctx context.Context, sub string, usage func(), opts *CLIOptions) error {
	switch sub {
	case "version", "":
		return showVersion()
	default:
		return dispatchApp(ctx, sub, usage, opts)
	}
}

func showVersion() error {
	fmt.Println("gcrunpresso " + Version)
	return nil
}

func dispatchApp(ctx context.Context, sub string, usage func(), opts *CLIOptions) error {
	appOpt := &Option{
		ConfigFilePath:            opts.resolveConfigFilePath(),
		Project:                   opts.Project,
		Location:                  opts.Location,
		Service:                   opts.Service,
		Job:                       opts.Job,
		ImpersonateServiceAccount: opts.ImpersonateServiceAccount,
	}
	if opts.Timeout != nil {
		appOpt.Timeout = *opts.Timeout
	}

	app, err := New(ctx, appOpt)
	if err != nil {
		return err
	}
	defer app.Close()
	app.LogDebug("dispatching subcommand: %s", sub)

	switch sub {
	case "deploy":
		return app.Deploy(ctx, *opts.Deploy)
	case "run":
		return app.Run(ctx, *opts.Run)
	case "rollback":
		return app.Rollback(ctx, *opts.Rollback)
	case "scale":
		return app.Scale(ctx, *opts.Scale)
	case "status":
		return app.Status(ctx, *opts.Status)
	case "revisions":
		return app.Revisions(ctx, *opts.Revisions)
	case "executions":
		return app.Executions(ctx, *opts.Executions)
	case "diff":
		return app.Diff(ctx, *opts.Diff)
	case "init":
		return app.Init(ctx, *opts.Init)
	case "verify":
		return app.Verify(ctx, *opts.Verify)
	case "render":
		return app.Render(ctx, *opts.Render)
	case "wait":
		return app.Wait(ctx, *opts.Wait)
	case "delete":
		return app.Delete(ctx, *opts.Delete)
	default:
		usage()
		return nil
	}
}

type CLIParseFunc func([]string) (string, *CLIOptions, func(), error)

func CLI(ctx context.Context, parse CLIParseFunc) (int, error) {
	sub, opts, usage, err := parse(os.Args[1:])
	if err != nil {
		return 1, err
	}
	setLogFormat(opts.LogFormat)
	if opts.Debug {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}

	if err := dispatchCLI(ctx, sub, usage, opts); err != nil {
		return 1, err
	}
	return 0, nil
}
