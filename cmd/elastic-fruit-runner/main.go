package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	if err := newRootCommand().Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// newRootCommand builds the CLI.
// The --config flag is bound through viper so ELASTIC_FRUIT_RUNNER_CONFIG works as a fallback.
func newRootCommand() *cobra.Command {
	v := viper.New()

	root := &cobra.Command{
		Use:           "elastic-fruit-runner",
		Short:         "Run the Elastic Fruit Runner daemon",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := v.BindPFlag("config", cmd.Root().PersistentFlags().Lookup("config")); err != nil {
				return fmt.Errorf("bind --config flag: %w", err)
			}
			v.SetEnvPrefix("ELASTIC_FRUIT_RUNNER")
			v.AutomaticEnv()
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDaemon(v.GetString("config"))
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().String("config", "", "Path to config file (default: ~/.elastic-fruit-runner/config.yaml)")

	root.AddCommand(&cobra.Command{
		Use:   "reset-password",
		Short: "Clear the Console admin password",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return resetAdminPassword(v.GetString("config"))
		},
	})

	return root
}
