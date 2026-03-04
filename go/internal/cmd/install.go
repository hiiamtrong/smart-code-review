package cmd

import (
	"fmt"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/git"
	"github.com/hiiamtrong/smart-code-review/internal/installer"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install pre-commit hook in the current git repository",
	RunE:  runInstall,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove pre-commit hook from the current git repository",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	if _, err := config.Load(); err != nil {
		return fmt.Errorf("config not found — run 'ai-review setup' first")
	}

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	hooksDir, err := installer.GetHooksDir(repoRoot)
	if err != nil {
		return fmt.Errorf("get hooks dir: %w", err)
	}

	if installer.IsHookInstalled(hooksDir) {
		display.LogSuccess("Hook already installed")
		return nil
	}

	if err := installer.WritePreCommitHook(hooksDir); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	display.LogSuccess(fmt.Sprintf("Hook installed: %s/pre-commit", hooksDir))
	fmt.Println()
	fmt.Println("The hook will run on every 'git commit'.")
	fmt.Println("To remove: ai-review uninstall")
	return nil
}

func runUninstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	hooksDir, err := installer.GetHooksDir(repoRoot)
	if err != nil {
		return fmt.Errorf("get hooks dir: %w", err)
	}

	removed, err := installer.RemovePreCommitHook(hooksDir)
	if err != nil {
		return fmt.Errorf("remove hook: %w", err)
	}

	if removed {
		display.LogSuccess("Hook removed")
	} else {
		display.LogWarn("Hook not found or not managed by ai-review")
	}
	return nil
}
