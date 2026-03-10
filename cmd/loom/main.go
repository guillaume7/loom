// Command loom is the CLI entry point for the Loom autonomous workflow driver.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionText() string {
	return fmt.Sprintf("loom %s\ncommit: %s\nbuilt: %s\n", version, commit, date)
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var showVersion bool

	root := &cobra.Command{
		Use:           "loom",
		Short:         "Loom — autonomous GitHub workflow driver",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if showVersion {
				_, err := fmt.Fprint(cmd.OutOrStdout(), versionText())
				return err
			}
			return cmd.Help()
		},
	}
	root.Flags().BoolVar(&showVersion, "version", false, "Print the Loom version, commit, and build date")
	root.AddCommand(
		newVersionCmd(),
		newMCPCmd(),
		newStartCmd(),
		newStatusCmd(),
		newPauseCmd(),
		newResumeCmd(),
		newResetCmd(),
		newLogCmd(),
	)
	return root
}
