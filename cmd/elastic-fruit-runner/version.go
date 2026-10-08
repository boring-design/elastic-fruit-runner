package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-runner/internal/buildinfo"
)

// versionString returns the one line version text shared by
// the version subcommand and the --version flag.
func versionString() string {
	return fmt.Sprintf("elastic-fruit-runner %s (%s, %s)", buildinfo.Version(), buildinfo.Commit(), buildinfo.Date())
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), versionString())
			return err
		},
	}
}
