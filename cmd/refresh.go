package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/alexismanuel/nogo/internal/notiondesktop"
	"github.com/spf13/cobra"
)

var flagWait int

var refreshCmd = &cobra.Command{
	Use:   "refresh <page-id-or-url>",
	Short: "Force a cache sync for a specific page from Notion, then quit Notion",
	Long: `Open a specific Notion page to force a cache sync, then quit Notion.

A page ID or URL is required. Notion opens that page, syncs its content
to the local cache, and then quits.

Use --wait to set the maximum time to wait (default 30s).
Use --keep to leave Notion running after sync instead of quitting.

Examples:
  nogo refresh abc123def456                    # sync a specific page, then quit
  nogo refresh abc123def456 --wait=60          # allow up to 60s for sync
  nogo refresh abc123def456 --keep             # sync page, leave Notion running
  nogo refresh https://notion.so/Page-abc123def456  # works with full URLs`,
	Args: cobra.ExactArgs(1),
	RunE: runRefresh,
}

var flagKeep bool

func init() {
	refreshCmd.Flags().IntVar(&flagWait, "wait", 30, "max seconds to wait for cache update")
	refreshCmd.Flags().BoolVar(&flagKeep, "keep", false, "keep Notion running after sync instead of quitting")
	rootCmd.AddCommand(refreshCmd)
}

func runRefresh(cmd *cobra.Command, args []string) error {
	// Capture pre-refresh state.
	beforeDBMod := getDBModTime()

	objectID, err := notiondesktop.ResolvePageID(args[0])
	if err != nil {
		return fmt.Errorf("invalid ID or URL: %w", err)
	}
	notionURL := "notion://www.notion.so/" + objectID
	fmt.Fprintf(os.Stderr, "Opening page in Notion: %s\n", notionURL)
	if err := openBrowser(notionURL); err != nil {
		return fmt.Errorf("opening Notion: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Waiting up to %ds for cache update...\n", flagWait)
	updated := pollCacheUpdate(beforeDBMod, time.Duration(flagWait)*time.Second)

	if !updated {
		fmt.Fprintln(os.Stderr, "⏳ Timed out. Cache may still be updating — try 'nogo sync' to check.")
	} else {
		fmt.Fprintln(os.Stderr, "✅ Cache updated.")
	}

	// Quit Notion unless --keep.
	if !flagKeep {
		quitNotion()
		fmt.Fprintln(os.Stderr, "Notion closed.")
	}

	return nil
}

func getDBModTime() time.Time {
	path := notiondesktop.DefaultPath()
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

func pollCacheUpdate(before time.Time, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		after := getDBModTime()
		if after.After(before) {
			return true
		}
	}
	return false
}

func quitNotion() {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("osascript", "-e", `tell application "Notion" to quit`).Run()
	case "windows":
		exec.Command("taskkill", "/IM", "Notion.exe").Run()
	default:
		exec.Command("pkill", "-x", "notion").Run()
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	return exec.Command(cmd, args...).Start()
}