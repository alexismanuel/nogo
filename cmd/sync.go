package cmd

import (
	"fmt"

	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Show when the local Notion cache was last synced",
	Long: `Show three timestamps from Notion's local cache:

  DB modified    — when notion.db was last written to (most reliable signal)
  Last auto-sync — when Notion last ran an incremental sync
  Last refetch   — when Notion last did a full data re-download

Compare Last auto-sync against a page's Last edited (nogo info) to judge
staleness. If auto-sync is older, the cache is stale — run nogo refresh.

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