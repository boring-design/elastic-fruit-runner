package management

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/actions/scaleset"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // register pure-Go SQLite driver

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/management/migrations"
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
	cfg         *config.Config
	controllers []*controller.ScaleSetController
	jobs        *JobStore
	db          *sql.DB

	wg sync.WaitGroup
}

// New creates a ScaleSetControllerManagementService from the given config.
// It initializes the SQLite database, runs migrations, creates GitHub clients,
// backends, and controllers but does not start them.
func New(cfg *config.Config) (*Service, error) {
	db, err := openJobsDB(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open jobs database: %w", err)
	}

	svc := &Service{
		cfg:  cfg,
		db:   db,
		jobs: NewJobStore(db),
	}

	for i := range cfg.Orgs {
		org := &cfg.Orgs[i]
		client, err := createClient(org.ConfigURL(), &org.Auth)
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
		client, err := createClient(repo.ConfigURL(), &repo.Auth)
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

func createClient(configURL string, auth *config.AuthConfig) (*scaleset.Client, error) {
	switch auth.Mode() {
	case config.AuthModeGitHubApp:
		pemBytes, readErr := os.ReadFile(auth.GitHubApp.PrivateKeyPath)
		if readErr != nil {
			return nil, fmt.Errorf("read GitHub App private key %s: %w", auth.GitHubApp.PrivateKeyPath, readErr)
		}
		slog.Info("authenticating with GitHub App",
			"configURL", configURL,
			"clientID", auth.GitHubApp.ClientID,
			"installationID", auth.GitHubApp.InstallationID,
		)
		return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: configURL,
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       auth.GitHubApp.ClientID,
				InstallationID: auth.GitHubApp.InstallationID,
				PrivateKey:     string(pemBytes),
			},
		})
	case config.AuthModePAT:
		slog.Info("authenticating with PAT", "configURL", configURL)
		return scaleset.NewClientWithPersonalAccessToken(
			scaleset.NewClientWithPersonalAccessTokenConfig{
				GitHubConfigURL:     configURL,
				PersonalAccessToken: *auth.PATToken,
			},
		)
	default:
		return nil, fmt.Errorf("unknown auth mode %q", auth.Mode())
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

// Close shuts down the SQLite database connection.
func (svc *Service) Close() error {
	if svc.db != nil {
		return svc.db.Close()
	}
	return nil
}

// openJobsDB opens (or creates) the SQLite database and runs migrations.
func openJobsDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("determine home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, ".elastic-fruit-runner", "jobs.db")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory %s: %w", filepath.Dir(dbPath), err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	// SQLite does not benefit from multiple connections; a single connection
	// avoids "database is locked" errors and works correctly with :memory:.
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("jobs database ready", "path", dbPath)
	return db, nil
}
