// Command loom is the CLI entry point for the Loom autonomous workflow driver.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "loom",
		Short:         "Loom — autonomous GitHub workflow driver",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(
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
