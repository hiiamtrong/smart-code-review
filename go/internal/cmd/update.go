package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ai-review to the latest version",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	display.LogInfo("Checking for latest release...")

	release, err := updater.FetchLatest()
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}

	if release.Tag == "v"+Version || release.Tag == Version {
		display.LogSuccess(fmt.Sprintf("Already up to date (%s)", Version))
		return nil
	}

	display.LogInfo(fmt.Sprintf("Updating %s → %s", Version, release.Tag))

	if err := updater.ReplaceCurrentBinary(release.DownloadURL); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	display.LogSuccess(fmt.Sprintf("Updated to %s — restart to use the new version", release.Tag))
	return nil
}
