package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Load reads configuration from (in order of ascending priority):
//  1. Built-in defaults
//  2. Dotenv file (.env in current working directory)
//  3. Config file (YAML)
//  4. Environment variables
//  5. CLI flags (--config, --url, --token)
func Load() (*Config, error) {
	return loadWithArgs(os.Args[1:])
}

func loadWithArgs(args []string) (*Config, error) {
	// Load .env from current working directory at the very beginning
	// (does NOT overwrite existing env vars)
	_ = godotenv.Load()

	flags := pflag.NewFlagSet("elastic-fruit-runner", pflag.ContinueOnError)
	configPath := flags.String("config", "", "Path to config file (default: ~/.elastic-fruit-runner/config.yaml)")
	flags.String("url", "", "GitHub config URL (overrides config file)")
	flags.String("token", "", "GitHub PAT (overrides config file)")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	v := viper.New()

	// Defaults
	v.SetDefault("runner_group", "Default")
	v.SetDefault("idle_timeout", "15m")

	// Config file search
	if *configPath != "" {
		v.SetConfigFile(*configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".elastic-fruit-runner"))
		}
		v.AddConfigPath("/opt/homebrew/var/elastic-fruit-runner")
		v.AddConfigPath("/usr/local/var/elastic-fruit-runner")
		v.AddConfigPath("/etc/elastic-fruit-runner")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			slog.Info("no config file found, using default config")
		} else {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// Bind env vars explicitly (names don't follow a simple prefix pattern)
	_ = v.BindEnv("github.url", "GITHUB_CONFIG_URL")
	_ = v.BindEnv("github.token", "GITHUB_TOKEN")
	_ = v.BindEnv("github.app.client_id", "GITHUB_APP_CLIENT_ID")
	_ = v.BindEnv("github.app.installation_id", "GITHUB_APP_INSTALLATION_ID")
	_ = v.BindEnv("github.app.private_key_path", "GITHUB_APP_PRIVATE_KEY_PATH")
	_ = v.BindEnv("runner_group", "GITHUB_RUNNER_GROUP")

	// Flag overrides (highest priority) — only if explicitly set
	if f := flags.Lookup("url"); f != nil && f.Changed {
		v.Set("github.url", f.Value.String())
	}
	if f := flags.Lookup("token"); f != nil && f.Changed {
		v.Set("github.token", f.Value.String())
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg, viper.DecoderConfigOption(func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "yaml"
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		)
	})); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}
