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
	ClientOptions             []option.ClientOption
}

type AppOption func(*App)

func New(ctx context.Context, opt *Option, appOpts ...AppOption) (*App, error) {
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

	for _, ao := range appOpts {
		ao(app)
	}

	clientOpts := append([]option.ClientOption{}, opt.ClientOptions...)
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

	logTailClient, err := logging.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create logging tail client: %w", err)
	}
	app.logTailClient = logTailClient

	logAdminClient, err := logadmin.NewClient(ctx, conf.Project, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create logging admin client: %w", err)
	}
	app.logAdminClient = logAdminClient

	secretClient, err := secretmanager.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}
	app.secretClient = secretClient

	arClient, err := artifactregistry.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create artifact registry client: %w", err)
	}
	app.arClient = arClient

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

func (d *App) ServicesClient() *run.ServicesClient {
	return d.servicesClient
}

func (d *App) JobsClient() *run.JobsClient {
	return d.jobsClient
}

func (d *App) RevisionsClient() *run.RevisionsClient {
	return d.revisionsClient
}

func (d *App) ExecutionsClient() *run.ExecutionsClient {
	return d.executionsClient
}

func (d *App) TasksClient() *run.TasksClient {
	return d.tasksClient
}

func (d *App) LogTailClient() *logging.Client {
	return d.logTailClient
}

func (d *App) LogAdminClient() *logadmin.Client {
	return d.logAdminClient
}

func (d *App) SecretManagerClient() *secretmanager.Client {
	return d.secretClient
}

func (d *App) ArtifactRegistryClient() *artifactregistry.Client {
	return d.arClient
}

func (d *App) ResourceLocationPath() string {
	return fmt.Sprintf("projects/%s/locations/%s", d.config.Project, d.config.Location)
}

func (d *App) ResourceServicePath() string {
	return fmt.Sprintf("projects/%s/locations/%s/services/%s", d.config.Project, d.config.Location, d.config.Service)
}

func (d *App) ResourceJobPath() string {
	return fmt.Sprintf("projects/%s/locations/%s/jobs/%s", d.config.Project, d.config.Location, d.config.Job)
}

func (d *App) ResourceRevisionPath(revision string) string {
	return fmt.Sprintf("projects/%s/locations/%s/services/%s/revisions/%s", d.config.Project, d.config.Location, d.config.Service, revision)
}

func (d *App) ResourceExecutionPath(execution string) string {
	return fmt.Sprintf("projects/%s/locations/%s/jobs/%s/executions/%s", d.config.Project, d.config.Location, d.config.Job, execution)
}

func (d *App) Close() error {
	var errs []error
	if d.servicesClient != nil {
		if err := d.servicesClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.jobsClient != nil {
		if err := d.jobsClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.revisionsClient != nil {
		if err := d.revisionsClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.executionsClient != nil {
		if err := d.executionsClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.tasksClient != nil {
		if err := d.tasksClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.logTailClient != nil {
		if err := d.logTailClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.logAdminClient != nil {
		if err := d.logAdminClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.secretClient != nil {
		if err := d.secretClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if d.arClient != nil {
		if err := d.arClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}
	return nil
}
