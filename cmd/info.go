package cmd

import (
	"fmt"

	"github.com/alexismanuel/nogo/internal/notion"
	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <page-or-database-url>",
	Short: "Show metadata and sync freshness for a cached page or database",
	Long: `Show metadata (ID, title, last edited time) for a page or database
from the local Notion desktop cache. Also shows when the cache was last
synced, so you can decide whether the data is fresh enough.

Examples:
  nogo info https://www.notion.so/My-Page-abc123def456
  nogo info abc123def456`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func printSyncInfo() {
	sync, err := notiondesktop.LastSynced()
	if err != nil || sync == nil {
		return
	}
	fmt.Printf("\nCache sync:\n")
	if sync.LastAutoSync != "" {
		fmt.Printf("  Last auto-sync: %s\n", sync.LastAutoSync)
	}
	if sync.LastRefetch != "" {
		fmt.Printf("  Last refetch:   %s\n", sync.LastRefetch)
	}
	fmt.Printf("  DB modified:    %s\n", sync.DBModified)
}

func runInfo(cmd *cobra.Command, args []string) error {
	objectID, err := notion.ParsePageID(args[0])
	if err != nil {
		return fmt.Errorf("invalid ID or URL: %w", err)
	}

	reader, err := notiondesktop.Open("")
	if err != nil {
		return err
	}
	defer reader.Close()

	// Try database first, then page.
	db, dbErr := reader.GetDatabase(objectID)
	if dbErr == nil {
		title := reader.GetDatabaseTitle(db)
		meta, _ := reader.GetBlockMeta(objectID)
		fmt.Printf("Type:         database\n")
		fmt.Printf("ID:           %s\n", db.ID)
		fmt.Printf("Title:        %s\n", title)
		fmt.Printf("Last edited:  %s\n", meta.LastEdited)
		fmt.Printf("Created:      %s\n", meta.Created)
		printSyncInfo()
		return nil
	}

	page, err := reader.GetPage(objectID)
	if err != nil {
		return fmt.Errorf("not found in local cache (database: %s, page: %w)", dbErr, err)
	}

	title := reader.GetPageTitle(page)
	fmt.Printf("Type:         page\n")
	fmt.Printf("ID:           %s\n", page.ID)
	fmt.Printf("Title:        %s\n", title)
	fmt.Printf("Last edited:  %s\n", page.LastEditedTime)
	fmt.Printf("Created:      %s\n", page.CreatedTime)
	printSyncInfo()
	return nil
}