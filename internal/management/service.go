package management

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/githubclient"
	sqlcdb "github.com/boring-design/elastic-fruit-runner/internal/storage/sqlc"
)

// RunnerSetView is the assembled view of a runner set for external consumers.
type RunnerSetView struct {
	Info      controller.RunnerSetInfo
	Scope     string
	Connected bool
	Runners   []controller.RunnerSnapshot
}

// Service manages all ScaleSetControllers and provides aggregated read access.
type Service struct {
	cfg             *config.Config
	controllers     []*controller.ScaleSetController
	jobs            *JobStore
	db              *sql.DB
	queries         *sqlcdb.Queries
	databasePath    string
	hostSampleCount int

	wg sync.WaitGroup
}

// New creates a ScaleSetControllerManagementService from the given config
// and an already opened database. It creates GitHub clients, backends, and
// controllers but does not start them.
func New(cfg *config.Config, db *sql.DB) (*Service, error) {
	databasePath, err := cfg.DatabasePath()
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	svc := &Service{
		cfg:          cfg,
		db:           db,
		queries:      sqlcdb.New(db),
		jobs:         NewJobStore(db),
		databasePath: databasePath,
	}

	for i := range cfg.Orgs {
		org := &cfg.Orgs[i]
		client, err := githubclient.New(org.ConfigURL(), &org.Auth)
		if err != nil {
			return nil, fmt.Errorf("create client for org %s: %w", org.Org, err)
		}
		scope := "org: " + org.Org
		for j := range org.RunnerSets {
			rs := &org.RunnerSets[j]
			b, err := createBackend(rs)
			if err != nil {
				return nil, fmt.Errorf("create backend for runner set %s: %w", rs.Name, err)
			}
			ctrl := controller.New(rs, org.RunnerGroup, cfg.IdleTimeout, client, b, scope, svc.jobs)
			svc.controllers = append(svc.controllers, ctrl)
		}
	}

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		client, err := githubclient.New(repo.ConfigURL(), &repo.Auth)
		if err != nil {
			return nil, fmt.Errorf("create client for repo %s: %w", repo.Repo, err)
		}
		scope := "repo: " + repo.Repo
		for j := range repo.RunnerSets {
			rs := &repo.RunnerSets[j]
			b, err := createBackend(rs)
			if err != nil {
				return nil, fmt.Errorf("create backend for runner set %s: %w", rs.Name, err)
			}
			ctrl := controller.New(rs, "Default", cfg.IdleTimeout, client, b, scope, svc.jobs)
			svc.controllers = append(svc.controllers, ctrl)
		}
	}

	return svc, nil
}

// Start launches all controllers in background goroutines with automatic retry.
func (svc *Service) Start(ctx context.Context) {
	for _, ctrl := range svc.controllers {
		svc.wg.Add(1)
		go svc.runController(ctx, ctrl)
	}
}

// Wait blocks until all controllers have stopped.
func (svc *Service) Wait() {
	svc.wg.Wait()
}

// ListRunnerSets returns an assembled view of all runner sets.
func (svc *Service) ListRunnerSets() []RunnerSetView {
	views := make([]RunnerSetView, 0, len(svc.controllers))
	for _, ctrl := range svc.controllers {
		views = append(views, RunnerSetView{
			Info:      ctrl.GetRunnerSetInfo(),
			Scope:     ctrl.GetScope(),
			Connected: ctrl.IsConnected(),
			Runners:   ctrl.GetRunners(),
		})
	}
	return views
}

// ListJobRecords returns job history, most-recent-first.
func (svc *Service) ListJobRecords() []JobRecord {
	return svc.jobs.Snapshot()
}

func (svc *Service) FindJobRecords(filter JobFilter) JobPage {
	return svc.jobs.List(filter)
}

func (svc *Service) GetJobRecord(jobID string) (*JobRecord, error) {
	return svc.jobs.Get(jobID)
}

func (svc *Service) GetJobLogs(jobID string, after int64, pageSize int) (logs []JobLog, nextSequence int64) {
	return svc.jobs.Logs(jobID, after, pageSize)
}

func (svc *Service) GetJobSamples(jobID string) []ResourceSample {
	return svc.jobs.Samples(jobID)
}

func (svc *Service) runController(ctx context.Context, ctrl *controller.ScaleSetController) {
	defer svc.wg.Done()
	info := ctrl.GetRunnerSetInfo()
	for {
		err := ctrl.Run(ctx)
		if ctx.Err() != nil {
			slog.Info("controller stopped", "runnerSet", info.Name, "err", err)
			return
		}
		slog.Error("controller exited with error, restarting", "runnerSet", info.Name, "err", err)
		time.Sleep(5 * time.Second)
	}
}

func createBackend(rs *config.RunnerSetConfig) (backend.Backend, error) {
	switch rs.Backend {
	case "tart":
		return backend.NewTartBackend(rs.Image), nil
	case "docker":
		return backend.NewDockerBackend(rs.Image, rs.Platform), nil
	default:
		return nil, fmt.Errorf("unknown backend %q for runner set %q", rs.Backend, rs.Name)
	}
}
