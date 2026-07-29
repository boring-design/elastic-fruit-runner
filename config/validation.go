package config

import (
	"bytes"
	"encoding/pem"
	"fmt"
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
	cfg := &Config{
		Orgs:        raw.Orgs,
		Repos:       raw.Repos,
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
	if len(cfg.Orgs) == 0 && len(cfg.Repos) == 0 {
		addError("$", "at least one org or repo is required")
	}
	if cfg.IdleTimeout <= 0 {
		addError("idle_timeout", "must be greater than 0")
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
	validateWritablePath(cfg.DBPath, "db_path", addError)
	validateWritablePath(cfg.LogPath, "log_path", addError)

	names := make(map[string]string)
	for index := range cfg.Orgs {
		org := &cfg.Orgs[index]
		path := "orgs[" + strconv.Itoa(index) + "]"
		if !validGitHubName(org.Org) {
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
		if len(parts) != 2 || !validGitHubName(parts[0]) || !validGitHubName(parts[1]) {
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
	result.Warnings = append(result.Warnings, ValidationIssue{
		Path:    "$",
		Message: "GitHub connectivity is not checked during config validation",
	})
	return result
}

var githubNamePattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9_.-]{0,98}[A-Za-z0-9])?$`)

func validGitHubName(value string) bool {
	return githubNamePattern.MatchString(value)
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
	if rs.Backend == "tart" && rs.Platform != "" {
		addError(path+".platform", "must be empty for the tart backend")
	}
	if rs.Backend == "docker" && rs.Platform != "" && !strings.HasPrefix(rs.Platform, "linux/") {
		addError(path+".platform", "must use linux/architecture format")
	}
}

func validateWritablePath(path, yamlPath string, addError func(string, string)) {
	if path == "" || path == ":memory:" {
		return
	}
	dir := filepath.Dir(path)
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
