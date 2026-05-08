package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var (
	flagListTop  bool
	flagListTime bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List pages and databases from the local Notion cache",
	Long: `List all pages and databases available in the Notion desktop SQLite cache.

By default, every page and database in the cache is shown. Use --top to
restrict output to top-level items only (pages and databases at the root of
a workspace).

Examples:
  nogo list          # show all pages and databases
  nogo list --top    # show top-level items only`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolVar(&flagListTop, "top", false, "show top-level items only")
	listCmd.Flags().BoolVar(&flagListTime, "time", false, "show last edited time for each item")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	reader, err := notiondesktop.Open("")
	if err != nil {
		return err
	}
	defer reader.Close()

	rows, err := reader.ListPages(!flagListTop)
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