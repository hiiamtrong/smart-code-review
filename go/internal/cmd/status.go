package cmd

import (
	"fmt"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/git"
	"github.com/hiiamtrong/smart-code-review/internal/installer"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show installation status",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	display.Bold.Println("AI Review Status")
	display.PrintSeparator()
	fmt.Println()

	// Config check
	cfg, err := config.LoadMerged()
	if err != nil {
		display.LogError("Config not found — run: ai-review setup")
		return nil
	}

	if cfg.AIGatewayURL != "" && cfg.AIGatewayAPIKey != "" {
		display.LogSuccess("Credentials configured")
	} else {
		display.LogWarn("Credentials incomplete — run: ai-review setup")
	}

	fmt.Printf("  AI Review:   %s\n", enabledStr(cfg.EnableAIReview))
	fmt.Printf("  SonarQube:   %s\n", enabledStr(cfg.EnableSonarQube))
	fmt.Printf("  Global config: %s\n", config.FilePath())

	// Show project config info if available.
	if projDir, err := config.ProjectConfigDir(); err == nil && projDir != "" {
		fmt.Printf("  Project config: %s/config\n", projDir)
	}
	fmt.Println()

	// Hook check (only in a git repo)
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		display.LogWarn("Not inside a git repository — hook status unavailable")
		return nil
	}

	hooksDir, err := installer.GetHooksDir(repoRoot)
	if err != nil {
		display.LogWarn("Could not determine hooks directory")
		return nil
	}

	if installer.IsHookInstalled(hooksDir) {
		display.LogSuccess(fmt.Sprintf("Hook installed: %s/pre-commit", hooksDir))
	} else {
		display.LogWarn("Hook not installed — run: ai-review install")
	}

	fmt.Println()
	return nil
}

func enabledStr(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
