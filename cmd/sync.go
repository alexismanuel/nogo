package cmd

import (
	"fmt"

	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Show when the local Notion cache was last synced",
	Long: `Show freshness information about the local Notion desktop cache.
Reports the last auto-sync, last full refetch, and DB file modification
time so you can decide whether cached data is recent enough.

Examples:
  nogo sync`,
	Args: cobra.NoArgs,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	sync, err := notiondesktop.LastSynced()
	if err != nil {
		return err
	}

	fmt.Printf("DB modified:    %s\n", sync.DBModified)
	if sync.LastAutoSync != "" {
		fmt.Printf("Last auto-sync: %s\n", sync.LastAutoSync)
	}
	if sync.LastRefetch != "" {
		fmt.Printf("Last refetch:   %s\n", sync.LastRefetch)
	}
	return nil
}