package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var (
	flagSearchTop  bool
	flagSearchTime bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search pages and databases by title",
	Long: `Search pages and databases in the local Notion cache by title.

Performs case-insensitive exact substring matching. Multi-word queries match
as a literal string.

By default, searches all pages and databases. Use --top to restrict to
top-level items only.

Exits with code 1 when no results match.

Examples:
  nogo search meeting
  nogo search --top design
  nogo search --time roadmap`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().BoolVar(&flagSearchTop, "top", false, "search top-level items only")
	searchCmd.Flags().BoolVar(&flagSearchTime, "time", false, "show last edited time for each result")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.ToLower(args[0])

	reader, err := notiondesktop.Open("")
	if err != nil {
		return err
	}
	defer reader.Close()

	rows, err := reader.ListPages(!flagSearchTop)
	if err != nil {
		return err
	}

	var matched []notiondesktop.PageEntry
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Title), query) {
			matched = append(matched, r)
		}
	}

	if len(matched) == 0 {
		fmt.Fprintf(os.Stderr, "No results for %q.\n", args[0])
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if flagSearchTime {
		fmt.Fprintln(w, "TYPE\tID\tLAST_EDITED\tTITLE")
		for _, r := range matched {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Type, r.ID, r.LastEdited, r.Title)
		}
	} else {
		fmt.Fprintln(w, "TYPE\tID\tTITLE")
		for _, r := range matched {
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.Type, r.ID, r.Title)
		}
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "\n%d result(s) for %q\n", len(matched), args[0])
	return nil
}