package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexismanuel/nogo/internal/markdown"
	"github.com/alexismanuel/nogo/internal/notion"
	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var (
	flagOutput      string
	flagFrontmatter bool
	flagStdout      bool
)

var getCmd = &cobra.Command{
	Use:   "get <page-or-database-url>",
	Short: "Fetch a Notion page or database from local cache",
	Long: `Fetch a Notion page or database by URL or ID from the local Notion
desktop SQLite cache and convert it to Markdown.

Requires the Notion desktop app to have synced the content locally.

Examples:
  nogo get https://www.notion.so/My-Page-abc123def456
  nogo get abc123def456
  nogo get <url> --output notes.md
  nogo get <url> --stdout
  nogo get <url> --frontmatter`,
	Args: cobra.ExactArgs(1),
	RunE: runGet,
}

func init() {
	getCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (default: <page-title>.md in current dir)")
	getCmd.Flags().BoolVar(&flagFrontmatter, "frontmatter", false, "Prepend YAML frontmatter with page metadata")
	getCmd.Flags().BoolVar(&flagStdout, "stdout", false, "Print markdown to stdout instead of saving to a file")
	rootCmd.AddCommand(getCmd)
}

func runGet(cmd *cobra.Command, args []string) error {
	objectID, err := notion.ParsePageID(args[0])
	if err != nil {
		return fmt.Errorf("invalid ID or URL: %w", err)
	}

	return runGetLocal(objectID)
}

func runGetLocal(objectID string) error {
	reader, err := notiondesktop.Open("")
	if err != nil {
		return err
	}
	defer reader.Close()

	fmt.Fprintf(os.Stderr, "Reading from local Notion cache...\n")

	// Try database first, then fall back to page.
	var content string
	var title string

	db, dbErr := reader.GetDatabase(objectID)
	if dbErr == nil {
		title = reader.GetDatabaseTitle(db)
		fmt.Fprintf(os.Stderr, "Database: %s\n", title)

		rows, err := reader.QueryDatabase(objectID)
		if err != nil {
			return fmt.Errorf("querying database: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Rows: %d\n", len(rows))

		var sb strings.Builder
		if flagFrontmatter {
			sb.WriteString(markdown.Frontmatter(title, db.ID, db.URL, ""))
		}
		sb.WriteString("# " + title + "\n\n")
		sb.WriteString(markdown.DatabaseTable(db, rows))
		content = sb.String()
	} else {
		page, err := reader.GetPage(objectID)
		if err != nil {
			return fmt.Errorf("not a page or database in local cache (database: %s, page: %w)", dbErr, err)
		}

		title = reader.GetPageTitle(page)
		fmt.Fprintf(os.Stderr, "Page: %s\n", title)

		fmt.Fprintf(os.Stderr, "Fetching content...\n")
		blocks, err := reader.GetBlocks(objectID)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Blocks: %d\n", len(blocks))

		var sb strings.Builder
		if flagFrontmatter {
			sb.WriteString(markdown.Frontmatter(title, page.ID, page.URL, page.LastEditedTime))
		}
		sb.WriteString("# " + title + "\n\n")
		sb.WriteString(markdown.Convert(blocks, 0))
		content = sb.String()
	}

	return writeOutput(title, content)
}

func writeOutput(title, content string) error {
	if flagStdout {
		fmt.Print(content)
		return nil
	}

	outPath := flagOutput
	if outPath == "" {
		outPath = safeFilename(title) + ".md"
	}

	if dir := filepath.Dir(outPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	abs, _ := filepath.Abs(outPath)
	fmt.Fprintf(os.Stderr, "Saved to: %s\n", abs)
	return nil
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)

func safeFilename(title string) string {
	s := strings.TrimSpace(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlnum.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		return "untitled"
	}
	return strings.ToLower(s)
}