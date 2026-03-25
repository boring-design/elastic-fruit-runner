package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Load reads configuration from a YAML config file.
// The --config flag overrides the default search paths.
func Load() (*Config, error) {
	return loadWithArgs(os.Args[1:])
}

func loadWithArgs(args []string) (*Config, error) {
	flags := pflag.NewFlagSet("elastic-fruit-runner", pflag.ContinueOnError)
	configPath := flags.String("config", "", "Path to config file (default: ~/.elastic-fruit-runner/config.yaml)")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	v := viper.New()

	// Defaults
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
