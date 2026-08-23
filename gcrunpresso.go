package gcrunpresso

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	logging "cloud.google.com/go/logging/apiv2"
	"cloud.google.com/go/logging/logadmin"
	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type ServicesAPI interface {
	GetService(ctx context.Context, req *runpb.GetServiceRequest, opts ...gax.CallOption) (*runpb.Service, error)
	CreateService(ctx context.Context, req *runpb.CreateServiceRequest, opts ...gax.CallOption) (*run.CreateServiceOperation, error)
	UpdateService(ctx context.Context, req *runpb.UpdateServiceRequest, opts ...gax.CallOption) (*run.UpdateServiceOperation, error)
	DeleteService(ctx context.Context, req *runpb.DeleteServiceRequest, opts ...gax.CallOption) (*run.DeleteServiceOperation, error)
}

type JobsAPI interface {
	GetJob(ctx context.Context, req *runpb.GetJobRequest, opts ...gax.CallOption) (*runpb.Job, error)
	CreateJob(ctx context.Context, req *runpb.CreateJobRequest, opts ...gax.CallOption) (*run.CreateJobOperation, error)
	UpdateJob(ctx context.Context, req *runpb.UpdateJobRequest, opts ...gax.CallOption) (*run.UpdateJobOperation, error)
	DeleteJob(ctx context.Context, req *runpb.DeleteJobRequest, opts ...gax.CallOption) (*run.DeleteJobOperation, error)
	RunJob(ctx context.Context, req *runpb.RunJobRequest, opts ...gax.CallOption) (*run.RunJobOperation, error)
}

type TasksAPI interface {
	ListTasks(ctx context.Context, req *runpb.ListTasksRequest) ([]*runpb.Task, error)
}

type RevisionsAPI interface {
	GetRevision(ctx context.Context, req *runpb.GetRevisionRequest, opts ...gax.CallOption) (*runpb.Revision, error)
	ListRevisions(ctx context.Context, req *runpb.ListRevisionsRequest) ([]*runpb.Revision, error)
}

type ExecutionsAPI interface {
	GetExecution(ctx context.Context, req *runpb.GetExecutionRequest, opts ...gax.CallOption) (*runpb.Execution, error)
	ListExecutions(ctx context.Context, req *runpb.ListExecutionsRequest) ([]*runpb.Execution, error)
}

type SecretManagerAPI interface {
	GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest, opts ...gax.CallOption) (*secretmanagerpb.Secret, error)
}

type ArtifactRegistryAPI interface {
	GetRepository(ctx context.Context, req *artifactregistrypb.GetRepositoryRequest, opts ...gax.CallOption) (*artifactregistrypb.Repository, error)
	GetDockerImage(ctx context.Context, req *artifactregistrypb.GetDockerImageRequest, opts ...gax.CallOption) (*artifactregistrypb.DockerImage, error)
}

type tasksClientAdapter struct {
	client *run.TasksClient
}

func (a *tasksClientAdapter) ListTasks(ctx context.Context, req *runpb.ListTasksRequest) ([]*runpb.Task, error) {
	if a == nil || a.client == nil {
		return nil, nil
	}
	it := a.client.ListTasks(ctx, req)
	var tasks []*runpb.Task
	for {
		task, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

type revisionsClientAdapter struct {
	client *run.RevisionsClient
}

func (a *revisionsClientAdapter) GetRevision(ctx context.Context, req *runpb.GetRevisionRequest, opts ...gax.CallOption) (*runpb.Revision, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("revisions client is not initialized")
	}
	return a.client.GetRevision(ctx, req, opts...)
}

func (a *revisionsClientAdapter) ListRevisions(ctx context.Context, req *runpb.ListRevisionsRequest) ([]*runpb.Revision, error) {
	if a == nil || a.client == nil {
		return nil, nil
	}
	it := a.client.ListRevisions(ctx, req)
	var revs []*runpb.Revision
	for {
		rev, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		revs = append(revs, rev)
	}
	return revs, nil
}

type executionsClientAdapter struct {
	client *run.ExecutionsClient
}

func (a *executionsClientAdapter) GetExecution(ctx context.Context, req *runpb.GetExecutionRequest, opts ...gax.CallOption) (*runpb.Execution, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("executions client is not initialized")
	}
	return a.client.GetExecution(ctx, req, opts...)
}

func (a *executionsClientAdapter) ListExecutions(ctx context.Context, req *runpb.ListExecutionsRequest) ([]*runpb.Execution, error) {
	if a == nil || a.client == nil {
		return nil, nil
	}
	it := a.client.ListExecutions(ctx, req)
	var execs []*runpb.Execution
	for {
		exec, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	return execs, nil
}

type App struct {
	config  *Config
	loader  *configLoader
	logger  *slog.Logger
	timeout time.Duration

	servicesClient   ServicesAPI
	jobsClient       JobsAPI
	revisionsClient  RevisionsAPI
	executionsClient ExecutionsAPI
	tasksClient      TasksAPI
	logTailClient    *logging.Client
	logAdminClient   *logadmin.Client
	secretClient     SecretManagerAPI
	arClient         ArtifactRegistryAPI

	closers []io.Closer
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

func WithServicesClient(c ServicesAPI) AppOption {
	return func(a *App) { a.servicesClient = c }
}

func WithJobsClient(c JobsAPI) AppOption {
	return func(a *App) { a.jobsClient = c }
}

func WithTasksClient(c TasksAPI) AppOption {
	return func(a *App) { a.tasksClient = c }
}

func WithRevisionsClient(c RevisionsAPI) AppOption {
	return func(a *App) { a.revisionsClient = c }
}

func WithExecutionsClient(c ExecutionsAPI) AppOption {
	return func(a *App) { a.executionsClient = c }
}

func WithSecretManagerClient(c SecretManagerAPI) AppOption {
	return func(a *App) { a.secretClient = c }
}

func WithArtifactRegistryClient(c ArtifactRegistryAPI) AppOption {
	return func(a *App) { a.arClient = c }
}

func New(ctx context.Context, opt *Option, appOpts ...AppOption) (*App, error) {
	conf := NewDefaultConfig()
	loader := newConfigLoader()
	if opt.ConfigFilePath != "" {
		if err := loader.Load(ctx, opt.ConfigFilePath, conf); err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}
	if err := conf.Restrict(opt); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	app := &App{
		config:  conf,
		loader:  loader, // Retain loaded loader with plugin funcs (Finding #5)
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

	// Initialize GAPIC clients if not injected
	if app.servicesClient == nil {
		c, err := run.NewServicesClient(ctx, clientOpts...)
		if err == nil {
			app.servicesClient = c
			app.closers = append(app.closers, c)
		}
	}

	if app.jobsClient == nil {
		c, err := run.NewJobsClient(ctx, clientOpts...)
		if err == nil {
			app.jobsClient = c
			app.closers = append(app.closers, c)
		}
	}

	if app.revisionsClient == nil {
		c, err := run.NewRevisionsClient(ctx, clientOpts...)
		if err == nil {
			app.revisionsClient = &revisionsClientAdapter{client: c}
			app.closers = append(app.closers, c)
		}
	}

	if app.executionsClient == nil {
		c, err := run.NewExecutionsClient(ctx, clientOpts...)
		if err == nil {
			app.executionsClient = &executionsClientAdapter{client: c}
			app.closers = append(app.closers, c)
		}
	}

	if app.tasksClient == nil {
		c, err := run.NewTasksClient(ctx, clientOpts...)
		if err == nil {
			app.tasksClient = &tasksClientAdapter{client: c}
			app.closers = append(app.closers, c)
		}
	}

	if app.logTailClient == nil {
		c, err := logging.NewClient(ctx, clientOpts...)
		if err == nil {
			app.logTailClient = c
			app.closers = append(app.closers, c)
		}
	}

	if app.logAdminClient == nil {
		c, err := logadmin.NewClient(ctx, conf.Project, clientOpts...)
		if err == nil {
			app.logAdminClient = c
			app.closers = append(app.closers, c)
		}
	}

	if app.secretClient == nil {
		c, err := secretmanager.NewClient(ctx, clientOpts...)
		if err == nil {
			app.secretClient = c
			app.closers = append(app.closers, c)
		}
	}

	if app.arClient == nil {
		c, err := artifactregistry.NewClient(ctx, clientOpts...)
		if err == nil {
			app.arClient = c
			app.closers = append(app.closers, c)
		}
	}

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

func (d *App) ServicesClient() ServicesAPI {
	return d.servicesClient
}

func (d *App) JobsClient() JobsAPI {
	return d.jobsClient
}

func (d *App) RevisionsClient() RevisionsAPI {
	return d.revisionsClient
}

func (d *App) ExecutionsClient() ExecutionsAPI {
	return d.executionsClient
}

func (d *App) TasksClient() TasksAPI {
	return d.tasksClient
}

func (d *App) LogTailClient() *logging.Client {
	return d.logTailClient
}

func (d *App) LogAdminClient() *logadmin.Client {
	return d.logAdminClient
}

func (d *App) SecretManagerClient() SecretManagerAPI {
	return d.secretClient
}

func (d *App) ArtifactRegistryClient() ArtifactRegistryAPI {
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
	for _, c := range d.closers {
		if c != nil {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing clients: %v", errs)
	}
	return nil
}
