package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/boring-design/elastic-fruit-runner/config"
	"github.com/boring-design/elastic-fruit-runner/internal/cloudagent"
	"github.com/boring-design/elastic-fruit-runner/internal/configstate"
	"github.com/boring-design/elastic-fruit-runner/internal/storage"
)

const enrollTimeout = 60 * time.Second

func newEnrollCommand(v *viper.Viper) *cobra.Command {
	var serverURL, token string
	var maxRunners int
	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Connect this host to Elastic Fruit Cloud",
		Long: "Register this host with an Elastic Fruit Cloud server using a one time token.\n" +
			"The command writes the agent credential and the cloud block of the config file.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runEnroll(cmd.Context(), v.GetString("config"), cloudagent.EnrollInput{
				ServerURL:  serverURL,
				Token:      token,
				MaxRunners: maxRunners,
			})
		},
	}
	cmd.Flags().StringVar(&serverURL, "server", "", "Elastic Fruit Cloud server URL, for example https://cloud.example.com")
	cmd.Flags().StringVar(&token, "token", "", "One time enrollment token from the cloud console")
	cmd.Flags().IntVar(&maxRunners, "max-runners", 1, "Maximum number of runners on this host")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

// runEnroll registers the host, saves the credential, and updates the config file.
func runEnroll(parent context.Context, requestedConfigPath string, input cloudagent.EnrollInput) error {
	cloud := &config.CloudConfig{ServerURL: input.ServerURL, MaxRunners: input.MaxRunners}
	if err := cloud.Validate(); err != nil {
		return err
	}
	configPath := config.FindConfigPath(requestedConfigPath)
	existing, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read config %s: %w", configPath, err)
	}
	// Check the config change and open the database before talking to the
	// cloud so a one time token is never spent when the local save would fail.
	updatedYAML, err := config.WithCloudBlock(existing, cloud)
	if err != nil {
		return fmt.Errorf("prepare cloud block for %s: %w", configPath, err)
	}
	if validation := config.ValidateYAML(updatedYAML); len(validation.Errors) > 0 {
		return fmt.Errorf("config %s would not be valid in cloud mode: %s", configPath, validation.Errors[0].String())
	}
	credentialPath, err := cloudagent.CredentialPath()
	if err != nil {
		return err
	}
	databasePath, err := databasePathForConfig(existing)
	if err != nil {
		return err
	}
	db, err := storage.Open(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	configState := configstate.NewForConfigMode(configPath, db, time.Now())

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	enrollCtx, cancel := context.WithTimeout(ctx, enrollTimeout)
	defer cancel()
	result, err := cloudagent.Enroll(enrollCtx, input)
	if err != nil {
		return err
	}

	if err := cloudagent.SaveCredential(credentialPath, result.Credential); err != nil {
		return err
	}
	if err := saveCloudConfig(configState, configPath, updatedYAML); err != nil {
		return fmt.Errorf("credential saved to %s but config update failed: %w", credentialPath, err)
	}

	backends := make([]string, 0, len(result.Backends))
	for _, b := range result.Backends {
		backends = append(backends, b.Backend+" "+b.Version)
	}
	if len(backends) == 0 {
		backends = append(backends, "none, install docker or tart before starting the daemon")
	}
	fmt.Fprintf(os.Stdout, "Enrolled with %s as %s.\n", input.ServerURL, result.AgentName)
	fmt.Fprintf(os.Stdout, "Credential: %s\n", credentialPath)
	fmt.Fprintf(os.Stdout, "Config:     %s\n", configPath)
	fmt.Fprintf(os.Stdout, "Backends:   %s\n", strings.Join(backends, ", "))
	fmt.Fprintln(os.Stdout, "Next step: start or restart the daemon, for example:")
	fmt.Fprintln(os.Stdout, "  elastic-fruit-runner")
	fmt.Fprintln(os.Stdout, "  brew services restart elastic-fruit-runner")
	fmt.Fprintln(os.Stdout, "  sudo systemctl restart elastic-fruit-runner")
	return nil
}

// saveCloudConfig writes the config through the config state service so the
// change is recorded as a revision in the database.
func saveCloudConfig(state *configstate.Service, configPath string, updatedYAML []byte) error {
	result, err := state.Save(updatedYAML, "enroll")
	if err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("updated config %s is not valid: %s", configPath, result.Errors[0].String())
	}
	return nil
}

// databasePathForConfig honors db_path from the existing config when it parses.
func databasePathForConfig(existing []byte) (string, error) {
	if len(existing) > 0 {
		result := config.ValidateYAML(existing)
		if result.Config != nil && result.Config.DBPath != "" {
			return result.Config.DBPath, nil
		}
	}
	return config.DefaultDatabasePath()
}
