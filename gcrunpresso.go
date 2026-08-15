package gcrunpresso

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	logging "cloud.google.com/go/logging/apiv2"
	"cloud.google.com/go/logging/logadmin"
	run "cloud.google.com/go/run/apiv2"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

type App struct {
	config  *Config
	loader  *configLoader
	logger  *slog.Logger
	timeout time.Duration

	servicesClient   *run.ServicesClient
	jobsClient       *run.JobsClient
	revisionsClient  *run.RevisionsClient
	executionsClient *run.ExecutionsClient
	tasksClient      *run.TasksClient
	logTailClient    *logging.Client
	logAdminClient   *logadmin.Client
	secretClient     *secretmanager.Client
	arClient         *artifactregistry.Client
}

type Option struct {
	ConfigFilePath            string
	Project                   string
	Location                  string
	Service                   string
	Job                       string
	ImpersonateServiceAccount string
	Timeout                   time.Duration
}

func New(ctx context.Context, opt *Option) (*App, error) {
	conf := NewDefaultConfig()
	if opt.ConfigFilePath != "" {
		loader := newConfigLoader()
		if err := loader.Load(ctx, opt.ConfigFilePath, conf); err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}
	if err := conf.Restrict(opt); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	app := &App{
		config:  conf,
		loader:  newConfigLoader(),
		logger:  commonLogger.With("project", conf.Project, "location", conf.Location),
		timeout: conf.Timeout.Duration,
	}

	var clientOpts []option.ClientOption
	if sa := conf.ImpersonateServiceAccount; sa != "" {
		ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: sa,
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create impersonated token source for %s: %w", sa, err)
		}
		clientOpts = append(clientOpts, option.WithTokenSource(ts))
	}

	// Initialize GCP Clients
	servicesClient, err := run.NewServicesClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create run services client: %w", err)
	}
	app.servicesClient = servicesClient

	jobsClient, err := run.NewJobsClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create run jobs client: %w", err)
	}
	app.jobsClient = jobsClient

	revisionsClient, err := run.NewRevisionsClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create run revisions client: %w", err)
	}
	app.revisionsClient = revisionsClient

	executionsClient, err := run.NewExecutionsClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create run executions client: %w", err)
	}
	app.executionsClient = executionsClient

	tasksClient, err := run.NewTasksClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create run tasks client: %w", err)
	}
	app.tasksClient = tasksClient

	return app, nil
}

func (d *App) Config() *Config {
	return d.config
}

func (d *App) Timeout() time.Duration {
	if d.timeout > 0 {
		return d.timeout
	}
	return defaultTimeout
}
