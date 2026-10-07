package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boring-design/elastic-fruit-runner/internal/buildinfo"
)

// These values are set by goreleaser through its default ldflags
// (-X main.version, -X main.commit, -X main.date).
// A plain go build keeps version as dev and reads commit and date
// from the Go build info when the checkout provides them.
var (
	version = "dev"
	commit  string
	date    string
)

const unknownBuildDate = "unknown"

// versionString returns the one line version text shared by
// the version subcommand and the --version flag.
func versionString() string {
	build := buildinfo.Current()
	c := commit
	if c == "" {
		c = buildinfo.VCSRevision(build)
	}
	d := date
	if d == "" {
		d = buildinfo.Setting(build, "vcs.time")
	}
	if d == "" {
		d = unknownBuildDate
	}
	return fmt.Sprintf("elastic-fruit-runner %s (%s, %s)", version, c, d)
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
