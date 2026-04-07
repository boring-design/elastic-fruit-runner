//go:build integration

package integration_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/cucumber/godog"
	"github.com/google/go-github/v79/github"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/boring-design/elastic-fruit-runner/config"
	controlplanev1 "github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1"
	"github.com/boring-design/elastic-fruit-runner/gen/controlplane/v1/controlplanev1connect"
	"github.com/boring-design/elastic-fruit-runner/internal/api"
	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
	"github.com/boring-design/elastic-fruit-runner/internal/management"
	"github.com/boring-design/elastic-fruit-runner/internal/management/migrations"
	"github.com/boring-design/elastic-fruit-runner/internal/tart"
	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

// scenarioState holds shared state across steps within a single scenario.
type scenarioState struct {
	// config steps
	tempDir    string
	configFile string
	cfg        *config.Config
	envVars    map[string]string
	oldEnvVars map[string]string

	// binpath steps
	oldWellKnownDirs []string
	binpathTmpDir    string
	lookupResult     string
	lookupResult2    string

	// jobstore steps
	jobStore *management.JobStore
	db       *sql.DB

	// vitals steps
	vitalsResult vitals.Vitals

	// management service + API steps
	mgmtService    *management.Service
	mgmtCancel     context.CancelFunc
	vitalsSvc      *vitals.Service
	apiServer      *http.Server
	apiClient      controlplanev1connect.ControlPlaneServiceClient
	scaleSetName   string
	workflowRunID  int64
	workflowResult *github.WorkflowRun
	runnerSetsResp *controlplanev1.ListRunnerSetsResponse
	jobRecordsResp *controlplanev1.ListJobRecordsResponse

	// tart steps
	tartMgr    *tart.Manager
	tartVMName string
	tartVMIP   string
	tartPrefix string
}

func initializeScenario(sc *godog.ScenarioContext) {
	state := &scenarioState{
		envVars:    make(map[string]string),
		oldEnvVars: make(map[string]string),
	}

	sc.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		// restore env vars
		for key, old := range state.oldEnvVars {
			if old == "\x00" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, old)
			}
		}
		// restore binpath state
		if state.oldWellKnownDirs != nil {
			binpath.ResetForTesting(state.oldWellKnownDirs)
		}
		// close db
		if state.db != nil {
			state.db.Close()
		}
		return ctx, nil
	})

	// ---- Config steps ----
	sc.Step(`^a clean temporary directory$`, func() {
		state.tempDir, _ = os.MkdirTemp("", "efr-bdd-*")
		state.setEnv("HOME", state.tempDir)
	})

	sc.Step(`^a config file with content:$`, func(content *godog.DocString) error {
		if state.tempDir == "" {
			state.tempDir, _ = os.MkdirTemp("", "efr-bdd-*")
		}
		state.configFile = filepath.Join(state.tempDir, "config.yaml")
		return os.WriteFile(state.configFile, []byte(content.Content), 0o644)
	})

	sc.Step(`^the environment variable "([^"]*)" is set to "([^"]*)"$`, func(key, value string) {
		state.setEnv(key, value)
	})

	sc.Step(`^I load the configuration without arguments$`, func() error {
		var err error
		state.cfg, err = config.LoadWithArgs(nil)
		return err
	})

	sc.Step(`^I load the configuration with that file$`, func() error {
		var err error
		state.cfg, err = config.LoadWithArgs([]string{"--config", state.configFile})
		return err
	})

	sc.Step(`^the idle timeout should be "([^"]*)"$`, func(expected string) error {
		d, err := time.ParseDuration(expected)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", expected, err)
		}
		if state.cfg.IdleTimeout != d {
			return fmt.Errorf("idle timeout = %v, want %v", state.cfg.IdleTimeout, d)
		}
		return nil
	})

	sc.Step(`^the log level should be "([^"]*)"$`, func(expected string) error {
		if state.cfg.LogLevel != expected {
			return fmt.Errorf("log level = %q, want %q", state.cfg.LogLevel, expected)
		}
		return nil
	})

	sc.Step(`^the parsed log level should be slog\.LevelDebug$`, func() error {
		level, err := state.cfg.ParsedLogLevel()
		if err != nil {
			return fmt.Errorf("ParsedLogLevel() error: %w", err)
		}
		if level != slog.LevelDebug {
			return fmt.Errorf("ParsedLogLevel() = %v, want %v", level, slog.LevelDebug)
		}
		return nil
	})

	sc.Step(`^there should be (\d+) org configured$`, func(n int) error {
		if len(state.cfg.Orgs) != n {
			return fmt.Errorf("org count = %d, want %d", len(state.cfg.Orgs), n)
		}
		return nil
	})

	sc.Step(`^org (\d+) should have name "([^"]*)"$`, func(i int, name string) error {
		if state.cfg.Orgs[i].Org != name {
			return fmt.Errorf("org[%d].Org = %q, want %q", i, state.cfg.Orgs[i].Org, name)
		}
		return nil
	})

	sc.Step(`^org (\d+) should have runner group "([^"]*)"$`, func(i int, group string) error {
		if state.cfg.Orgs[i].RunnerGroup != group {
			return fmt.Errorf("org[%d].RunnerGroup = %q, want %q", i, state.cfg.Orgs[i].RunnerGroup, group)
		}
		return nil
	})

	sc.Step(`^org (\d+) should use GitHub App auth with client ID "([^"]*)"$`, func(i int, clientID string) error {
		app := state.cfg.Orgs[i].Auth.GitHubApp
		if app == nil {
			return fmt.Errorf("org[%d].Auth.GitHubApp is nil", i)
		}
		if app.ClientID != clientID {
			return fmt.Errorf("org[%d].Auth.GitHubApp.ClientID = %q, want %q", i, app.ClientID, clientID)
		}
		return nil
	})

	sc.Step(`^org (\d+) should have (\d+) runner set$`, func(i, n int) error {
		if len(state.cfg.Orgs[i].RunnerSets) != n {
			return fmt.Errorf("org[%d] runner set count = %d, want %d", i, len(state.cfg.Orgs[i].RunnerSets), n)
		}
		return nil
	})

	sc.Step(`^org (\d+) runner set (\d+) should have name "([^"]*)"$`, func(i, j int, name string) error {
		if state.cfg.Orgs[i].RunnerSets[j].Name != name {
			return fmt.Errorf("org[%d].RunnerSets[%d].Name = %q, want %q", i, j, state.cfg.Orgs[i].RunnerSets[j].Name, name)
		}
		return nil
	})

	sc.Step(`^org (\d+) runner set (\d+) should have max runners (\d+)$`, func(i, j, max int) error {
		if state.cfg.Orgs[i].RunnerSets[j].MaxRunners != max {
			return fmt.Errorf("org[%d].RunnerSets[%d].MaxRunners = %d, want %d", i, j, state.cfg.Orgs[i].RunnerSets[j].MaxRunners, max)
		}
		return nil
	})

	sc.Step(`^org (\d+) runner set (\d+) should have platform "([^"]*)"$`, func(i, j int, platform string) error {
		if state.cfg.Orgs[i].RunnerSets[j].Platform != platform {
			return fmt.Errorf("org[%d].RunnerSets[%d].Platform = %q, want %q", i, j, state.cfg.Orgs[i].RunnerSets[j].Platform, platform)
		}
		return nil
	})

	sc.Step(`^there should be (\d+) repo configured$`, func(n int) error {
		if len(state.cfg.Repos) != n {
			return fmt.Errorf("repo count = %d, want %d", len(state.cfg.Repos), n)
		}
		return nil
	})

	sc.Step(`^repo (\d+) should have name "([^"]*)"$`, func(i int, name string) error {
		if state.cfg.Repos[i].Repo != name {
			return fmt.Errorf("repo[%d].Repo = %q, want %q", i, state.cfg.Repos[i].Repo, name)
		}
		return nil
	})

	sc.Step(`^repo (\d+) should use PAT auth with token "([^"]*)"$`, func(i int, token string) error {
		if state.cfg.Repos[i].Auth.PATToken == nil {
			return fmt.Errorf("repo[%d].Auth.PATToken is nil", i)
		}
		if *state.cfg.Repos[i].Auth.PATToken != token {
			return fmt.Errorf("repo[%d].Auth.PATToken = %q, want %q", i, *state.cfg.Repos[i].Auth.PATToken, token)
		}
		return nil
	})

	sc.Step(`^repo (\d+) should have (\d+) runner set$`, func(i, n int) error {
		if len(state.cfg.Repos[i].RunnerSets) != n {
			return fmt.Errorf("repo[%d] runner set count = %d, want %d", i, len(state.cfg.Repos[i].RunnerSets), n)
		}
		return nil
	})

	// ---- Binpath steps ----
	sc.Step(`^the well-known dirs are cleared$`, func() {
		state.oldWellKnownDirs = binpath.ResetForTesting(nil)
	})

	sc.Step(`^a temporary directory with a fake binary "([^"]*)"$`, func(name string) error {
		state.binpathTmpDir, _ = os.MkdirTemp("", "efr-bdd-binpath-*")
		fakeBin := filepath.Join(state.binpathTmpDir, name)
		return os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755)
	})

	sc.Step(`^the well-known dirs are set to that directory$`, func() {
		state.oldWellKnownDirs = binpath.ResetForTesting([]string{state.binpathTmpDir})
	})

	sc.Step(`^I look up the binary "([^"]*)"$`, func(name string) {
		state.lookupResult = binpath.Lookup(name)
	})

	sc.Step(`^I look up the binary "([^"]*)" twice$`, func(name string) {
		state.lookupResult = binpath.Lookup(name)
		state.lookupResult2 = binpath.Lookup(name)
	})

	sc.Step(`^the result should be an absolute path$`, func() error {
		if !filepath.IsAbs(state.lookupResult) {
			return fmt.Errorf("expected absolute path, got %q", state.lookupResult)
		}
		return nil
	})

	sc.Step(`^the result should be the path to "([^"]*)" in that directory$`, func(name string) error {
		expected := filepath.Join(state.binpathTmpDir, name)
		if state.lookupResult != expected {
			return fmt.Errorf("expected %q, got %q", expected, state.lookupResult)
		}
		return nil
	})

	sc.Step(`^the result should be "([^"]*)"$`, func(expected string) error {
		if state.lookupResult != expected {
			return fmt.Errorf("expected %q, got %q", expected, state.lookupResult)
		}
		return nil
	})

	sc.Step(`^both results should be identical$`, func() error {
		if state.lookupResult != state.lookupResult2 {
			return fmt.Errorf("cache inconsistency: %q != %q", state.lookupResult, state.lookupResult2)
		}
		return nil
	})

	// ---- Jobstore steps ----
	sc.Step(`^a fresh in-memory job store$`, func() error {
		var err error
		state.db, err = sql.Open("sqlite", ":memory:")
		if err != nil {
			return fmt.Errorf("open in-memory sqlite: %w", err)
		}
		state.db.SetMaxOpenConns(1)

		goose.SetBaseFS(migrations.FS)
		if err := goose.SetDialect("sqlite3"); err != nil {
			return fmt.Errorf("set goose dialect: %w", err)
		}
		if err := goose.Up(state.db, "."); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
		state.jobStore = management.NewJobStore(state.db)
		return nil
	})

	sc.Step(`^I record job "([^"]*)" started on runner "([^"]*)" in set "([^"]*)"$`, func(jobID, runner, set string) {
		state.jobStore.RecordJobStarted(set, jobID, runner)
	})

	sc.Step(`^job "([^"]*)" was started on runner "([^"]*)" in set "([^"]*)"$`, func(jobID, runner, set string) {
		state.jobStore.RecordJobStarted(set, jobID, runner)
	})

	sc.Step(`^I record job "([^"]*)" completed with result "([^"]*)"$`, func(jobID, result string) {
		state.jobStore.RecordJobCompleted(jobID, result)
	})

	sc.Step(`^the snapshot should contain (\d+) jobs?$`, func(n int) error {
		jobs := state.jobStore.Snapshot()
		if len(jobs) != n {
			return fmt.Errorf("snapshot contains %d jobs, want %d", len(jobs), n)
		}
		return nil
	})

	sc.Step(`^job "([^"]*)" should have runner name "([^"]*)"$`, func(jobID, expected string) error {
		for _, j := range state.jobStore.Snapshot() {
			if j.ID == jobID {
				if j.RunnerName != expected {
					return fmt.Errorf("job %q runner name = %q, want %q", jobID, j.RunnerName, expected)
				}
				return nil
			}
		}
		return fmt.Errorf("job %q not found", jobID)
	})

	sc.Step(`^job "([^"]*)" should have runner set name "([^"]*)"$`, func(jobID, expected string) error {
		for _, j := range state.jobStore.Snapshot() {
			if j.ID == jobID {
				if j.RunnerSetName != expected {
					return fmt.Errorf("job %q runner set name = %q, want %q", jobID, j.RunnerSetName, expected)
				}
				return nil
			}
		}
		return fmt.Errorf("job %q not found", jobID)
	})

	sc.Step(`^job "([^"]*)" should have result "([^"]*)"$`, func(jobID, expected string) error {
		for _, j := range state.jobStore.Snapshot() {
			if j.ID == jobID {
				if j.Result != expected {
					return fmt.Errorf("job %q result = %q, want %q", jobID, j.Result, expected)
				}
				return nil
			}
		}
		return fmt.Errorf("job %q not found", jobID)
	})

	sc.Step(`^job "([^"]*)" should not have a completion time$`, func(jobID string) error {
		for _, j := range state.jobStore.Snapshot() {
			if j.ID == jobID {
				if j.CompletedAt != nil {
					return fmt.Errorf("job %q has completion time %v, expected nil", jobID, j.CompletedAt)
				}
				return nil
			}
		}
		return fmt.Errorf("job %q not found", jobID)
	})

	sc.Step(`^job "([^"]*)" should have a completion time$`, func(jobID string) error {
		for _, j := range state.jobStore.Snapshot() {
			if j.ID == jobID {
				if j.CompletedAt == nil {
					return fmt.Errorf("job %q has no completion time", jobID)
				}
				return nil
			}
		}
		return fmt.Errorf("job %q not found", jobID)
	})

	sc.Step(`^the following jobs were started:$`, func(table *godog.Table) {
		for _, row := range table.Rows[1:] {
			state.jobStore.RecordJobStarted(row.Cells[2].Value, row.Cells[0].Value, row.Cells[1].Value)
		}
	})

	sc.Step(`^the snapshot should have jobs in order: (.+)$`, func(orderStr string) error {
		ids := strings.Split(orderStr, ", ")
		jobs := state.jobStore.Snapshot()
		if len(jobs) != len(ids) {
			return fmt.Errorf("snapshot has %d jobs, want %d", len(jobs), len(ids))
		}
		for i, id := range ids {
			if jobs[i].ID != id {
				return fmt.Errorf("job[%d] = %q, want %q", i, jobs[i].ID, id)
			}
		}
		return nil
	})

	sc.Step(`^(\d+) jobs were started in set "([^"]*)"$`, func(n int, set string) {
		for i := range n {
			state.jobStore.RecordJobStarted(set, fmt.Sprintf("job-%d", i), fmt.Sprintf("runner-%d", i))
		}
	})

	sc.Step(`^(\d+) jobs are started and completed concurrently in set "([^"]*)"$`, func(n int, set string) {
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				id := fmt.Sprintf("job-%d", idx)
				state.jobStore.RecordJobStarted(set, id, fmt.Sprintf("runner-%d", idx))
				state.jobStore.RecordJobCompleted(id, "Succeeded")
			}(i)
		}
		wg.Wait()
	})

	// ---- Vitals steps ----
	sc.Step(`^I collect system vitals twice with a short delay$`, func() {
		vitals.Collect()
		time.Sleep(100 * time.Millisecond)
		state.vitalsResult = vitals.Collect()
	})

	sc.Step(`^CPU usage should be between 0 and 100 percent$`, func() error {
		v := state.vitalsResult.CPUUsagePercent
		if v < 0 || v > 100 {
			return fmt.Errorf("CPU usage = %.2f%%, out of [0, 100] range", v)
		}
		return nil
	})

	sc.Step(`^memory usage should be between 0 and 100 percent \(exclusive of zero\)$`, func() error {
		v := state.vitalsResult.MemoryUsagePercent
		if v <= 0 || v > 100 {
			return fmt.Errorf("memory usage = %.2f%%, out of (0, 100] range", v)
		}
		return nil
	})

	sc.Step(`^disk usage should be between 0 and 100 percent \(exclusive of zero\)$`, func() error {
		v := state.vitalsResult.DiskUsagePercent
		if v <= 0 || v > 100 {
			return fmt.Errorf("disk usage = %.2f%%, out of (0, 100] range", v)
		}
		return nil
	})

	// ---- Management service + API steps ----
	sc.Step(`^a management service config with PAT auth and docker backend$`, func() error {
		pat := os.Getenv("EFR_TEST_PAT")
		if pat == "" {
			return godog.ErrPending
		}
		return state.buildMgmtConfig(config.AuthConfig{PATToken: &pat})
	})

	sc.Step(`^a management service config with GitHub App auth and docker backend$`, func(ctx context.Context) (context.Context, error) {
		appClientID := os.Getenv("EFR_TEST_APP_CLIENT_ID")
		appInstallID := os.Getenv("EFR_TEST_APP_INSTALLATION_ID")
		appKeyPath := os.Getenv("EFR_TEST_APP_PRIVATE_KEY_PATH")
		if appClientID == "" || appInstallID == "" || appKeyPath == "" {
			return ctx, godog.ErrPending
		}
		installID, err := strconv.ParseInt(appInstallID, 10, 64)
		if err != nil {
			return ctx, fmt.Errorf("invalid EFR_TEST_APP_INSTALLATION_ID: %w", err)
		}
		auth := config.AuthConfig{
			GitHubApp: &config.GitHubAppConfig{
				ClientID:       appClientID,
				InstallationID: installID,
				PrivateKeyPath: appKeyPath,
			},
		}
		return ctx, state.buildMgmtConfig(auth)
	})

	sc.Step(`^a management service is created from the config$`, func() error {
		var err error
		state.mgmtService, err = management.New(state.cfg)
		if err != nil {
			return fmt.Errorf("management.New: %w", err)
		}
		return nil
	})

	sc.Step(`^a vitals service is created$`, func() {
		state.vitalsSvc = vitals.New(time.Now())
	})

	sc.Step(`^an API server is started$`, func() error {
		srv := api.NewServer(state.mgmtService, state.vitalsSvc, state.cfg.IdleTimeout, state.cfg.CORS)
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		state.apiServer = &http.Server{Handler: srv.Handler()}
		go state.apiServer.Serve(listener)

		baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
		state.apiClient = controlplanev1connect.NewControlPlaneServiceClient(http.DefaultClient, baseURL)
		return nil
	})

	sc.Step(`^the management service is started$`, func() {
		ctx, cancel := context.WithCancel(context.Background())
		state.mgmtCancel = cancel
		state.mgmtService.Start(ctx)
	})

	sc.Step(`^a controller connects within (\d+) seconds$`, func(seconds int) error {
		deadline := time.After(time.Duration(seconds) * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-deadline:
				return fmt.Errorf("timeout waiting for controller to connect after %ds", seconds)
			case <-ticker.C:
				for _, v := range state.mgmtService.ListRunnerSets() {
					if v.Connected {
						return nil
					}
				}
			}
		}
	})

	sc.Step(`^I query the runner sets API$`, func() error {
		resp, err := state.apiClient.ListRunnerSets(context.Background(), connect.NewRequest(&controlplanev1.ListRunnerSetsRequest{}))
		if err != nil {
			return fmt.Errorf("ListRunnerSets: %w", err)
		}
		state.runnerSetsResp = resp.Msg
		return nil
	})

	sc.Step(`^the runner sets response should contain (\d+) set$`, func(n int) error {
		if len(state.runnerSetsResp.RunnerSets) != n {
			return fmt.Errorf("runner sets count = %d, want %d", len(state.runnerSetsResp.RunnerSets), n)
		}
		return nil
	})

	sc.Step(`^the first runner set should have the configured name$`, func() error {
		if len(state.runnerSetsResp.RunnerSets) == 0 {
			return fmt.Errorf("no runner sets in response")
		}
		got := state.runnerSetsResp.RunnerSets[0].Name
		if got != state.scaleSetName {
			return fmt.Errorf("runner set name = %q, want %q", got, state.scaleSetName)
		}
		return nil
	})

	sc.Step(`^a workflow is dispatched$`, func() error {
		workflowToken := os.Getenv("EFR_TEST_WORKFLOW_TOKEN")
		workflowOrg := os.Getenv("EFR_TEST_WORKFLOW_ORG")
		workflowRepo := os.Getenv("EFR_TEST_WORKFLOW_REPO")
		workflowFile := envOrDefault("EFR_TEST_WORKFLOW_FILE", "test-job.yaml")

		if workflowToken == "" || workflowOrg == "" || workflowRepo == "" {
			return fmt.Errorf("EFR_TEST_WORKFLOW_TOKEN, EFR_TEST_WORKFLOW_ORG, EFR_TEST_WORKFLOW_REPO must be set")
		}

		ghClient := github.NewClient(nil).WithAuthToken(workflowToken)
		runID, err := dispatchAndFindWorkflow(ghClient, workflowOrg, workflowRepo, workflowFile, state.scaleSetName)
		if err != nil {
			return err
		}
		state.workflowRunID = runID
		return nil
	})

	sc.Step(`^the workflow completes successfully within (\d+) minutes$`, func(minutes int) error {
		workflowToken := os.Getenv("EFR_TEST_WORKFLOW_TOKEN")
		workflowOrg := os.Getenv("EFR_TEST_WORKFLOW_ORG")
		workflowRepo := os.Getenv("EFR_TEST_WORKFLOW_REPO")
		ghClient := github.NewClient(nil).WithAuthToken(workflowToken)

		result, err := waitForCompletion(ghClient, workflowOrg, workflowRepo, state.workflowRunID, time.Duration(minutes)*time.Minute)
		if err != nil {
			return err
		}
		state.workflowResult = result

		if result.GetStatus() != "completed" {
			return fmt.Errorf("workflow status = %q, want %q", result.GetStatus(), "completed")
		}
		if result.GetConclusion() != "success" {
			return fmt.Errorf("workflow conclusion = %q, want %q", result.GetConclusion(), "success")
		}
		return nil
	})

	sc.Step(`^I query the job records API$`, func() error {
		time.Sleep(10 * time.Second)
		resp, err := state.apiClient.ListJobRecords(context.Background(), connect.NewRequest(&controlplanev1.ListJobRecordsRequest{}))
		if err != nil {
			return fmt.Errorf("ListJobRecords: %w", err)
		}
		state.jobRecordsResp = resp.Msg
		return nil
	})

	sc.Step(`^there should be at least (\d+) job record$`, func(n int) error {
		if len(state.jobRecordsResp.JobRecords) < n {
			return fmt.Errorf("job records count = %d, want at least %d", len(state.jobRecordsResp.JobRecords), n)
		}
		return nil
	})

	sc.Step(`^the management service is stopped$`, func() {
		if state.apiServer != nil {
			state.apiServer.Close()
		}
		if state.mgmtCancel != nil {
			state.mgmtCancel()
		}
	})

	sc.Step(`^the management service should shut down cleanly$`, func() {
		if state.mgmtService != nil {
			state.mgmtService.Wait()
			state.mgmtService.Close()
		}
	})

	// ---- Tart VM steps ----
	sc.Step(`^a tart manager$`, func(ctx context.Context) (context.Context, error) {
		if binpath.Lookup("tart") == "tart" {
			return ctx, godog.ErrPending
		}
		state.tartMgr = tart.NewManager()
		state.tartPrefix = "efr-tart-test"
		return ctx, nil
	})

	sc.Step(`^I pull the VM image$`, func() error {
		image := envOrDefault("EFR_TEST_TART_IMAGE", "ghcr.io/cirruslabs/macos-tahoe-base:latest")
		return state.tartMgr.Pull(context.Background(), image)
	})

	sc.Step(`^the VM image should exist locally$`, func() error {
		image := envOrDefault("EFR_TEST_TART_IMAGE", "ghcr.io/cirruslabs/macos-tahoe-base:latest")
		exists, err := state.tartMgr.ImageExists(context.Background(), image)
		if err != nil {
			return fmt.Errorf("check image exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("image %q not found locally after pull", image)
		}
		return nil
	})

	sc.Step(`^I clone a VM with a random name$`, func() error {
		image := envOrDefault("EFR_TEST_TART_IMAGE", "ghcr.io/cirruslabs/macos-tahoe-base:latest")
		state.tartVMName = state.tartPrefix + "-" + randomSuffix()
		return state.tartMgr.Clone(context.Background(), image, state.tartVMName)
	})

	sc.Step(`^I start the cloned VM$`, func() error {
		return state.tartMgr.Start(context.Background(), state.tartVMName)
	})

	sc.Step(`^I wait for the VM IP address$`, func() error {
		ip, err := state.tartMgr.IPAddress(context.Background(), state.tartVMName)
		if err != nil {
			return err
		}
		state.tartVMIP = ip
		return nil
	})

	sc.Step(`^the VM IP should be a valid address$`, func() error {
		if net.ParseIP(state.tartVMIP) == nil {
			return fmt.Errorf("invalid IP address: %q", state.tartVMIP)
		}
		return nil
	})

	sc.Step(`^I exec "([^"]*)" in the VM$`, func(cmd string) error {
		return state.tartMgr.Exec(context.Background(), state.tartVMName, "bash", "-c", cmd)
	})

	sc.Step(`^the exec should succeed$`, func() error {
		// The step above already returns error on failure
		return nil
	})

	sc.Step(`^I stop and delete the VM$`, func() error {
		if err := state.tartMgr.Stop(context.Background(), state.tartVMName); err != nil {
			return fmt.Errorf("stop VM: %w", err)
		}
		return state.tartMgr.Delete(context.Background(), state.tartVMName)
	})

	sc.Step(`^the VM should no longer exist$`, func() error {
		vms, err := state.tartMgr.List(context.Background())
		if err != nil {
			return err
		}
		for _, name := range vms {
			if name == state.tartVMName {
				return fmt.Errorf("VM %q still exists after delete", state.tartVMName)
			}
		}
		return nil
	})

	sc.Step(`^listing local VMs should include the cloned VM$`, func() error {
		vms, err := state.tartMgr.List(context.Background())
		if err != nil {
			return err
		}
		for _, name := range vms {
			if name == state.tartVMName {
				return nil
			}
		}
		return fmt.Errorf("VM %q not found in list", state.tartVMName)
	})

	sc.Step(`^I cleanup all VMs with the test prefix$`, func() error {
		vms, err := state.tartMgr.List(context.Background())
		if err != nil {
			return err
		}
		for _, name := range vms {
			if strings.HasPrefix(name, state.tartPrefix+"-") {
				_ = state.tartMgr.Stop(context.Background(), name)
				_ = state.tartMgr.Delete(context.Background(), name)
			}
		}
		return nil
	})

	sc.Step(`^listing local VMs should not include the cloned VM$`, func() error {
		vms, err := state.tartMgr.List(context.Background())
		if err != nil {
			return err
		}
		for _, name := range vms {
			if name == state.tartVMName {
				return fmt.Errorf("VM %q still exists after cleanup", state.tartVMName)
			}
		}
		return nil
	})
}

// buildMgmtConfig creates a management service config from env vars with the given auth.
func (s *scenarioState) buildMgmtConfig(auth config.AuthConfig) error {
	configURL := os.Getenv("EFR_TEST_CONFIG_URL")
	if configURL == "" {
		return fmt.Errorf("EFR_TEST_CONFIG_URL not set")
	}

	parts := strings.Split(strings.TrimRight(configURL, "/"), "/")
	orgName := parts[len(parts)-1]

	image := envOrDefault("EFR_TEST_RUNNER_IMAGE", "ghcr.io/quipper/actions-runner:2.332.0")
	s.scaleSetName = "efr-test-" + randomSuffix()
	s.cfg = &config.Config{
		Orgs: []config.OrgConfig{
			{
				Org:         orgName,
				Auth:        auth,
				RunnerGroup: envOrDefault("EFR_TEST_RUNNER_GROUP", "Default"),
				RunnerSets: []config.RunnerSetConfig{
					{
						Name:       s.scaleSetName,
						Backend:    "docker",
						Image:      image,
						Labels:     []string{"self-hosted", "Linux", "X64"},
						MaxRunners: 2,
					},
				},
			},
		},
		IdleTimeout: 15 * time.Minute,
		LogLevel:    "debug",
		DBPath:      filepath.Join(os.TempDir(), fmt.Sprintf("efr-test-%s.db", randomSuffix())),
	}
	return nil
}

// setEnv sets an environment variable and records the old value for restoration.
func (s *scenarioState) setEnv(key, value string) {
	if _, exists := s.oldEnvVars[key]; !exists {
		old, ok := os.LookupEnv(key)
		if !ok {
			s.oldEnvVars[key] = "\x00"
		} else {
			s.oldEnvVars[key] = old
		}
	}
	os.Setenv(key, value)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func randomSuffix() string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])[:5]
}

func dispatchAndFindWorkflow(client *github.Client, org, repo, file, scaleSetName string) (int64, error) {
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
		return 0, fmt.Errorf("trigger workflow dispatch: %w", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return 0, fmt.Errorf("workflow dispatch returned status %d, want 204", resp.StatusCode)
	}

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
		return 0, fmt.Errorf("list workflow runs: %w", err)
	}
	if len(runs.WorkflowRuns) == 0 {
		return 0, fmt.Errorf("no workflow runs found after dispatch")
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
		return 0, fmt.Errorf("could not find the triggered workflow run")
	}
	return latest.GetID(), nil
}

func waitForCompletion(client *github.Client, org, repo string, runID int64, timeout time.Duration) (*github.WorkflowRun, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for workflow run %d to complete", runID)
		case <-ticker.C:
			run, _, err := client.Actions.GetWorkflowRunByID(ctx, org, repo, runID)
			if err != nil {
				continue
			}
			if run.GetStatus() == "completed" {
				return run, nil
			}
		}
	}
}
