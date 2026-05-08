package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nogo",
	Short: "nogo — Notion local cache reader",
	Long: `Zero-config Mac Notion cache reader to Markdown.

Quick start:
  nogo get <url>     # fetch a page or database from local cache`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}