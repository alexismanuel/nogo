package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var (
	flagListAll  bool
	flagListTime bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List pages and databases from the local Notion cache",
	Long: `List pages and databases available in the Notion desktop SQLite cache.

By default, only top-level items (pages and databases at the root of a
workspace) are shown. Use --all to include every page in the cache.

Examples:
  nogo list          # show top-level pages and databases
  nogo list --all    # show all pages in the cache`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&flagListAll, "all", false, "list all pages, not just top-level ones")
	listCmd.Flags().BoolVar(&flagListTime, "time", false, "show last edited time for each item")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	reader, err := notiondesktop.Open("")
	if err != nil {
		return err
	}
	defer reader.Close()

	rows, err := reader.ListPages(flagListAll)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "No pages found in local cache.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if flagListTime {
		fmt.Fprintln(w, "TYPE\tID\tLAST_EDITED\tTITLE")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Type, r.ID, r.LastEdited, r.Title)
		}
	} else {
		fmt.Fprintln(w, "TYPE\tID\tTITLE")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.Type, r.ID, r.Title)
		}
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "\n%d items\n", len(rows))
	return nil
}