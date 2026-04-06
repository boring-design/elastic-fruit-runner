package controller_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/actions/scaleset"
	"github.com/google/go-github/v79/github"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/controller"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/management/migrations"
)

func TestControllerFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires GitHub API and Docker")
	}

	configURL := os.Getenv("EFR_TEST_CONFIG_URL")
	if configURL == "" {
		t.Skip("EFR_TEST_CONFIG_URL not set, skipping integration test")
	}

	client := createTestClient(t, configURL)
	image := envOr("EFR_TEST_RUNNER_IMAGE", "ghcr.io/quipper/actions-runner:2.332.0")
	b := backend.NewDockerBackend(image, "")
	jobStore := setupTestJobStore(t)

	scaleSetName := "efr-test-" + randSuffix()
	rsCfg := &config.RunnerSetConfig{
		Name:       scaleSetName,
		Backend:    "docker",
		Image:      image,
		Labels:     []string{"self-hosted", "Linux", "X64"},
		MaxRunners: 2,
	}
	runnerGroup := envOr("EFR_TEST_RUNNER_GROUP", "Default")

	ctrl := controller.New(rsCfg, runnerGroup, 15*time.Minute, client, b, configURL, jobStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ctrl.Run(ctx)
	}()

	t.Log("waiting for controller to connect...")
	waitForConnected(t, ctrl, 60*time.Second)
	t.Log("controller connected, triggering workflow dispatch")

	workflowToken := mustEnv(t, "EFR_TEST_WORKFLOW_TOKEN")
	workflowOrg := mustEnv(t, "EFR_TEST_WORKFLOW_ORG")
	workflowRepo := mustEnv(t, "EFR_TEST_WORKFLOW_REPO")
	workflowFile := envOr("EFR_TEST_WORKFLOW_FILE", "test-job.yaml")

	ghClient := github.NewClient(nil).WithAuthToken(workflowToken)

	runID := triggerWorkflow(t, ghClient, workflowOrg, workflowRepo, workflowFile, scaleSetName)
	t.Logf("workflow dispatched, run ID: %d", runID)

	result := waitForWorkflowCompletion(t, ghClient, workflowOrg, workflowRepo, runID, 10*time.Minute)
	t.Logf("workflow completed: status=%s, conclusion=%s", result.GetStatus(), result.GetConclusion())

	if result.GetStatus() != "completed" {
		t.Errorf("workflow status = %q, want %q", result.GetStatus(), "completed")
	}
	if result.GetConclusion() != "success" {
		t.Errorf("workflow conclusion = %q, want %q", result.GetConclusion(), "success")
	}

	// Wait for async cleanup to finish
	time.Sleep(10 * time.Second)

	jobs := jobStore.Snapshot()
	if len(jobs) == 0 {
		t.Error("expected at least one recorded job, got none")
	}
	for _, j := range jobs {
		t.Logf("  job: id=%s runner=%s result=%s", j.ID, j.RunnerName, j.Result)
	}

	runners := ctrl.GetRunners()
	if len(runners) > 0 {
		t.Logf("warning: %d runners still active after job completion", len(runners))
	}

	t.Log("shutting down controller...")
	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("controller exited with unexpected error: %v", err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("timeout waiting for controller shutdown")
	}
	t.Log("controller shut down successfully")
}

func createTestClient(t *testing.T, configURL string) *scaleset.Client {
	t.Helper()

	appClientID := os.Getenv("EFR_TEST_APP_CLIENT_ID")
	appInstallID := os.Getenv("EFR_TEST_APP_INSTALLATION_ID")
	appKeyPath := os.Getenv("EFR_TEST_APP_PRIVATE_KEY_PATH")

	if appClientID != "" && appInstallID != "" && appKeyPath != "" {
		installID, err := strconv.ParseInt(appInstallID, 10, 64)
		if err != nil {
			t.Fatalf("invalid EFR_TEST_APP_INSTALLATION_ID: %v", err)
		}
		keyBytes, err := os.ReadFile(appKeyPath)
		if err != nil {
			t.Fatalf("read private key %s: %v", appKeyPath, err)
		}
		client, err := scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: configURL,
			GitHubAppAuth: scaleset.GitHubAppAuth{
				ClientID:       appClientID,
				InstallationID: installID,
				PrivateKey:     string(keyBytes),
			},
		})
		if err != nil {
			t.Fatalf("create scaleset client with GitHub App: %v", err)
		}
		return client
	}

	pat := os.Getenv("EFR_TEST_PAT")
	if pat == "" {
		t.Fatal("either EFR_TEST_PAT or EFR_TEST_APP_* environment variables must be set")
	}

	client, err := scaleset.NewClientWithPersonalAccessToken(
		scaleset.NewClientWithPersonalAccessTokenConfig{
			GitHubConfigURL:     configURL,
			PersonalAccessToken: pat,
		},
	)
	if err != nil {
		t.Fatalf("create scaleset client with PAT: %v", err)
	}
	return client
}

func setupTestJobStore(t *testing.T) *management.JobStore {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return management.NewJobStore(db)
}

func waitForConnected(t *testing.T, ctrl *controller.ScaleSetController, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for controller to connect")
		case <-ticker.C:
			if ctrl.IsConnected() {
				return
			}
		}
	}
}

func triggerWorkflow(t *testing.T, client *github.Client, org, repo, file, scaleSetName string) int64 {
	t.Helper()

	ctx := context.Background()
	dispatchTime := time.Now().UTC()

	resp, err := client.Actions.CreateWorkflowDispatchEventByFileName(
		ctx, org, repo, file,
		github.CreateWorkflowDispatchEventRequest{
			Ref: "main",
			Inputs: map[string]any{
				"scaleset_name": scaleSetName,
			},
		},
	)
	if err != nil {
		t.Fatalf("trigger workflow dispatch: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("workflow dispatch returned status %d, want 204", resp.StatusCode)
	}

	// Wait for the run to appear
	time.Sleep(10 * time.Second)

	runs, _, err := client.Actions.ListWorkflowRunsByFileName(
		ctx, org, repo, file,
		&github.ListWorkflowRunsOptions{
			Event:   "workflow_dispatch",
			Created: ">=" + dispatchTime.Format(time.RFC3339),
			ListOptions: github.ListOptions{
				PerPage: 10,
			},
		},
	)
	if err != nil {
		t.Fatalf("list workflow runs: %v", err)
	}
	if len(runs.WorkflowRuns) == 0 {
		t.Fatal("no workflow runs found after dispatch")
	}

	var latest *github.WorkflowRun
	var latestTime time.Time
	for _, run := range runs.WorkflowRuns {
		created := run.GetCreatedAt().Time
		if created.After(latestTime) {
			latestTime = created
			latest = run
		}
	}
	if latest == nil {
		t.Fatal("could not find the triggered workflow run")
	}

	return latest.GetID()
}

func waitForWorkflowCompletion(t *testing.T, client *github.Client, org, repo string, runID int64, timeout time.Duration) *github.WorkflowRun {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for workflow run %d to complete", runID)
			return nil
		case <-ticker.C:
			run, _, err := client.Actions.GetWorkflowRunByID(ctx, org, repo, runID)
			if err != nil {
				t.Logf("polling workflow run %d: %v", runID, err)
				continue
			}
			t.Logf("workflow run %d: status=%s", runID, run.GetStatus())
			if run.GetStatus() == "completed" {
				return run
			}
		}
	}
}

func mustEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("environment variable %s not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func randSuffix() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])[:5]
}
