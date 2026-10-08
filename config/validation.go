package config

import (
	"bytes"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type ValidationIssue struct {
	Path    string
	Message string
}

func (i ValidationIssue) String() string {
	if i.Path == "" {
		return i.Message
	}
	return i.Path + ": " + i.Message
}

type ValidationResult struct {
	Config     *Config
	Errors     []ValidationIssue
	Warnings   []ValidationIssue
	Normalized string
}

type rawConfig struct {
	Orgs        []OrgConfig   `yaml:"orgs"`
	Repos       []RepoConfig  `yaml:"repos"`
	Cloud       *CloudConfig  `yaml:"cloud,omitempty"`
	IdleTimeout durationValue `yaml:"idle_timeout"`
	LogLevel    string        `yaml:"log_level"`
	APIAddr     string        `yaml:"api_addr"`
	CORS        CORSConfig    `yaml:"cors"`
	DBPath      string        `yaml:"db_path"`
	LogPath     string        `yaml:"log_path"`
}

type durationValue time.Duration

func (d *durationValue) UnmarshalYAML(node *yaml.Node) error {
	value, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q", node.Value)
	}
	*d = durationValue(value)
	return nil
}

func (d durationValue) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func ValidateYAML(data []byte) ValidationResult {
	var raw rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return ValidationResult{
			Errors: []ValidationIssue{{Path: "$", Message: err.Error()}},
		}
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return ValidationResult{
			Errors: []ValidationIssue{{Path: "$", Message: err.Error()}},
		}
	} else if err == nil {
		return ValidationResult{
			Errors: []ValidationIssue{{Path: "$", Message: "only one YAML document is allowed"}},
		}
	}
	cfg := &Config{
		Orgs:        raw.Orgs,
		Repos:       raw.Repos,
		Cloud:       raw.Cloud,
		IdleTimeout: time.Duration(raw.IdleTimeout),
		LogLevel:    raw.LogLevel,
		APIAddr:     raw.APIAddr,
		CORS:        raw.CORS,
		DBPath:      raw.DBPath,
		LogPath:     raw.LogPath,
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 15 * time.Minute
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	result := ValidateConfig(cfg)
	result.Config = cfg
	if len(result.Errors) == 0 {
		normalized, err := yaml.Marshal(rawConfig{
			Orgs:        cfg.Orgs,
			Repos:       cfg.Repos,
			Cloud:       cfg.Cloud,
			IdleTimeout: durationValue(cfg.IdleTimeout),
			LogLevel:    cfg.LogLevel,
			APIAddr:     cfg.APIAddr,
			CORS:        cfg.CORS,
			DBPath:      cfg.DBPath,
			LogPath:     cfg.LogPath,
		})
		if err == nil {
			result.Normalized = string(normalized)
		}
	}
	return result
}

//nolint:gocyclo // Validation keeps every field path in one clear pass.
func ValidateConfig(cfg *Config) ValidationResult {
	var result ValidationResult
	addError := func(path, message string) {
		result.Errors = append(result.Errors, ValidationIssue{Path: path, Message: message})
	}
	hasGitHubTargets := len(cfg.Orgs) > 0 || len(cfg.Repos) > 0
	if cfg.Cloud != nil && hasGitHubTargets {
		addError("$", cloudAndGitHubTargetsMessage)
	}
	if cfg.Cloud == nil && !hasGitHubTargets {
		addError("$", "at least one org or repo is required")
	}
	if cfg.Cloud != nil {
		for _, issue := range cloudIssues(cfg.Cloud) {
			addError(issue.Path, issue.Message)
		}
	}
	if cfg.IdleTimeout <= 0 || cfg.IdleTimeout > 24*time.Hour {
		addError("idle_timeout", "must be greater than 0 and no more than 24h")
	}
	if _, err := cfg.ParsedLogLevel(); err != nil {
		addError("log_level", "must be debug, info, warn, or error")
	}
	if cfg.APIAddr != "" {
		if _, _, err := net.SplitHostPort(cfg.APIAddr); err != nil {
			addError("api_addr", "must be a host and port such as :8080")
		}
	}
	if cfg.CORS.AllowCredentials && (cfg.CORS.AllowOrigin == "" || cfg.CORS.AllowOrigin == "*") {
		addError("cors.allow_origin", "must be a specific origin when credentials are allowed")
	}
	if cfg.CORS.AllowOrigin != "" && cfg.CORS.AllowOrigin != "*" {
		parsed, err := url.Parse(cfg.CORS.AllowOrigin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			addError("cors.allow_origin", "must be a valid origin URL")
		}
	}
	if cfg.CORS.MaxAge < 0 || cfg.CORS.MaxAge > 86400 {
		addError("cors.max_age", "must be between 0 and 86400")
	}
	validateCORS(cfg.CORS, addError)
	validateWritablePath(cfg.DBPath, "db_path", addError)
	validateWritablePath(cfg.LogPath, "log_path", addError)

	names := make(map[string]string)
	for index := range cfg.Orgs {
		org := &cfg.Orgs[index]
		path := "orgs[" + strconv.Itoa(index) + "]"
		if !validGitHubOwner(org.Org) {
			addError(path+".org", "must be a valid GitHub organization name")
		}
		validateAuthAll(&org.Auth, path+".auth", addError)
		if org.RunnerGroup == "" {
			org.RunnerGroup = "Default"
		}
		if len(org.RunnerSets) == 0 {
			addError(path+".runner_sets", "must contain at least one runner set")
		}
		for runnerIndex := range org.RunnerSets {
			validateRunnerSetAll(&org.RunnerSets[runnerIndex], path+".runner_sets["+strconv.Itoa(runnerIndex)+"]", names, addError)
		}
	}
	for index := range cfg.Repos {
		repo := &cfg.Repos[index]
		path := "repos[" + strconv.Itoa(index) + "]"
		parts := strings.Split(repo.Repo, "/")
		if len(parts) != 2 || !validGitHubOwner(parts[0]) || !validGitHubRepository(parts[1]) {
			addError(path+".repo", "must use owner/repo format")
		}
		validateAuthAll(&repo.Auth, path+".auth", addError)
		if len(repo.RunnerSets) == 0 {
			addError(path+".runner_sets", "must contain at least one runner set")
		}
		for runnerIndex := range repo.RunnerSets {
			validateRunnerSetAll(&repo.RunnerSets[runnerIndex], path+".runner_sets["+strconv.Itoa(runnerIndex)+"]", names, addError)
		}
	}
	if cfg.Mode() == RunModeStandalone {
		result.Warnings = append(result.Warnings, ValidationIssue{
			Path:    "$",
			Message: "GitHub connectivity is not checked during config validation",
		})
	}
	return result
}

// cloudAndGitHubTargetsMessage is the error for a config that sets both ways to get jobs.
const cloudAndGitHubTargetsMessage = "cloud and orgs/repos are mutually exclusive, keep only cloud or only orgs/repos"

// Validate returns the first problem with the cloud block, or nil.
func (c *CloudConfig) Validate() error {
	if issues := cloudIssues(c); len(issues) > 0 {
		return errors.New(issues[0].String())
	}
	return nil
}

// cloudIssues checks the cloud block. Every message carries the value it rejects.
func cloudIssues(cloud *CloudConfig) []ValidationIssue {
	var issues []ValidationIssue
	addIssue := func(path, message string) {
		issues = append(issues, ValidationIssue{Path: path, Message: message})
	}
	if cloud.ServerURL == "" {
		addIssue("cloud.server_url", "is required in cloud mode")
	} else if parsed, err := url.Parse(cloud.ServerURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		addIssue("cloud.server_url", fmt.Sprintf("%q is not a valid URL", cloud.ServerURL))
	} else if parsed.Scheme != "https" && (parsed.Scheme != "http" || !isLocalHost(parsed.Hostname())) {
		addIssue("cloud.server_url", fmt.Sprintf("%q must use https, http is only allowed for localhost and 127.0.0.1", cloud.ServerURL))
	}
	if cloud.MaxRunners < 1 {
		addIssue("cloud.max_runners", fmt.Sprintf("must be at least 1, got %d", cloud.MaxRunners))
	}
	return issues
}

func isLocalHost(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1"
}

var (
	githubOwnerPattern      = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
)

func validGitHubOwner(value string) bool {
	return githubOwnerPattern.MatchString(value) && !strings.Contains(value, "--")
}

func validGitHubRepository(value string) bool {
	return githubRepositoryPattern.MatchString(value)
}

func validateCORS(cors CORSConfig, addError func(string, string)) {
	if cors.AllowMethods != "" {
		methods := strings.Split(cors.AllowMethods, ",")
		for _, method := range methods {
			switch strings.TrimSpace(method) {
			case "GET", "POST", "OPTIONS":
			default:
				addError("cors.allow_methods", "may contain only GET, POST, and OPTIONS")
				return
			}
		}
	}
	if strings.Contains(cors.AllowHeaders, "\n") || strings.Contains(cors.ExposeHeaders, "\n") {
		addError("cors", "header lists must stay on one line")
	}
}

func validateAuthAll(auth *AuthConfig, path string, addError func(string, string)) {
	hasToken := auth.PATToken != nil
	hasApp := auth.GitHubApp != nil
	if hasToken == hasApp {
		addError(path, "configure exactly one of pat_token or github_app")
		return
	}
	if hasToken {
		if strings.TrimSpace(*auth.PATToken) == "" {
			addError(path+".pat_token", "must not be empty")
		}
		return
	}
	app := auth.GitHubApp
	if app.ClientID == "" {
		addError(path+".github_app.client_id", "is required")
	}
	if app.InstallationID <= 0 {
		addError(path+".github_app.installation_id", "must be greater than 0")
	}
	if app.PrivateKeyPath == "" {
		addError(path+".github_app.private_key_path", "is required")
		return
	}
	data, err := os.ReadFile(app.PrivateKeyPath)
	if err != nil {
		addError(path+".github_app.private_key_path", "cannot be read: "+err.Error())
		return
	}
	block, _ := pem.Decode(data)
	if block == nil || !strings.Contains(block.Type, "PRIVATE KEY") {
		addError(path+".github_app.private_key_path", "must contain a valid PEM private key")
	}
}

func validateRunnerSetAll(rs *RunnerSetConfig, path string, names map[string]string, addError func(string, string)) {
	if strings.TrimSpace(rs.Name) == "" {
		addError(path+".name", "is required")
	} else if previous, exists := names[rs.Name]; exists {
		addError(path+".name", "must be unique and is already used at "+previous)
	} else {
		names[rs.Name] = path + ".name"
	}
	if rs.Backend != "docker" && rs.Backend != "tart" {
		addError(path+".backend", "must be docker or tart")
	}
	if strings.TrimSpace(rs.Image) == "" {
		addError(path+".image", "is required")
	}
	if rs.MaxRunners < 1 || rs.MaxRunners > 1000 {
		addError(path+".max_runners", "must be between 1 and 1000")
	}
	validateRunnerSetBackendOptions(rs, path, addError)
}

// validateRunnerSetBackendOptions checks the fields that only some backends use.
func validateRunnerSetBackendOptions(rs *RunnerSetConfig, path string, addError func(string, string)) {
	if rs.Backend == "tart" && rs.Platform != "" {
		addError(path+".platform", "must be empty for the tart backend")
	}
	if rs.Backend == "docker" && rs.Platform != "" && !strings.HasPrefix(rs.Platform, "linux/") {
		addError(path+".platform", "must use linux/architecture format")
	}
	if rs.Backend == "tart" && rs.Runtime != "" {
		addError(path+".runtime", "must be empty for the tart backend")
	}
	if !isSingleWord(rs.Runtime) {
		addError(path+".runtime", "must be a single word such as runsc")
	}
}

func validateWritablePath(path, yamlPath string, addError func(string, string)) {
	if path == "" || path == ":memory:" {
		return
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			addError(yamlPath, "must be a file path")
			return
		}
		file, openErr := os.OpenFile(path, os.O_WRONLY, 0)
		if openErr != nil {
			addError(yamlPath, "file is not writable: "+openErr.Error())
			return
		}
		_ = file.Close()
	}
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			addError(yamlPath, "parent path is not a directory")
			return
		}
		file, err := os.CreateTemp(dir, ".efr-write-check")
		if err != nil {
			addError(yamlPath, "parent directory is not writable: "+err.Error())
			return
		}
		name := file.Name()
		_ = file.Close()
		_ = os.Remove(name)
		return
	}
	parent := filepath.Dir(dir)
	if _, err := os.Stat(parent); err != nil {
		addError(yamlPath, "parent directory does not exist")
	}
}
