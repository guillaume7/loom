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
	root.AddCommand(newStartCmd(), newMCPCmd())
	return root
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Loom workflow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "starting loom")
			return nil
		},
	}
}

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the Loom MCP stdio server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "starting mcp server")
			return nil
		},
	}
}
