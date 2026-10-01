package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Load reads configuration from a YAML config file.
// A non empty configPath selects that file, an empty one uses the default search paths.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("idle_timeout", "15m")
	v.SetDefault("log_level", "info")

	// Config file search
	if configPath != "" {
		v.SetConfigFile(configPath)
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

	if err := v.BindEnv("log_level", "LOG_LEVEL"); err != nil {
		return nil, fmt.Errorf("bind env LOG_LEVEL: %w", err)
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

	cfg.FilePath = v.ConfigFileUsed()
	if cfg.FilePath != "" {
		absolutePath, err := filepath.Abs(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("resolve config path %s: %w", cfg.FilePath, err)
		}
		cfg.FilePath = absolutePath
		data, err := os.ReadFile(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("read loaded config %s: %w", cfg.FilePath, err)
		}
		validation := ValidateYAML(data)
		if len(validation.Errors) > 0 {
			return nil, fmt.Errorf("validate config %s: %s", cfg.FilePath, validation.Errors[0].String())
		}
		loadedLogLevel := cfg.LogLevel
		cfg = validation.Config
		cfg.LogLevel = loadedLogLevel
		cfg.FilePath = absolutePath
		sum := sha256.Sum256(data)
		cfg.LoadedHash = hex.EncodeToString(sum[:])
		cfg.LoadedYAML = data
	}

	return cfg, nil
}

// FindConfigPath returns the absolute form of the requested config path,
// or the first existing default path when configPath is empty.
func FindConfigPath(configPath string) string {
	if configPath != "" {
		path, _ := filepath.Abs(configPath)
		return path
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths := []string{
			filepath.Join(home, ".elastic-fruit-runner", "config.yaml"),
			"/opt/homebrew/var/elastic-fruit-runner/config.yaml",
			"/usr/local/var/elastic-fruit-runner/config.yaml",
			"/etc/elastic-fruit-runner/config.yaml",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		return paths[0]
	}
	return "/etc/elastic-fruit-runner/config.yaml"
}
