package cmd

import (
	"fmt"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/git"
	"github.com/hiiamtrong/smart-code-review/internal/installer"
	"github.com/spf13/cobra"
)

const (
	msgAlreadyRegistered = "Hook already registered in .pre-commit-config.yaml"
	msgNotAGitRepo       = "not a git repository: %w"
	msgToRemove          = "To remove: ai-review uninstall"
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
	if _, err := config.LoadMerged(); err != nil {
		return fmt.Errorf("config not found — run 'ai-review setup' first")
	}

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf(msgNotAGitRepo, err)
	}

	hooksDir, err := installer.GetHooksDir(repoRoot)
	if err != nil {
		return fmt.Errorf("get hooks dir: %w", err)
	}

	// If pre-commit.com framework is detected, inject into .pre-commit-config.yaml
	// instead of writing directly to the hook file.
	if installer.DetectPreCommitFramework(repoRoot) {
		return installViaPreCommitFramework(repoRoot)
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
	fmt.Println(msgToRemove)
	return nil
}

func installViaPreCommitFramework(repoRoot string) error {
	if installer.IsPreCommitConfigInstalled(repoRoot) {
		display.LogSuccess(msgAlreadyRegistered)
		return nil
	}

	display.LogInfo("pre-commit.com framework detected — injecting into .pre-commit-config.yaml")

	injected, err := installer.InjectPreCommitConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("inject pre-commit config: %w", err)
	}
	if !injected {
		display.LogSuccess(msgAlreadyRegistered)
		return nil
	}

	display.LogSuccess("Hook added to .pre-commit-config.yaml as local hook")
	fmt.Println()
	fmt.Println("Run 'pre-commit install' if you haven't already.")
	fmt.Println(msgToRemove)
	return nil
}

func runUninstall(cmd *cobra.Command, args []string) error {
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return fmt.Errorf(msgNotAGitRepo, err)
	}

	hooksDir, err := installer.GetHooksDir(repoRoot)
	if err != nil {
		return fmt.Errorf("get hooks dir: %w", err)
	}

	var removed bool

	// Try removing from .pre-commit-config.yaml first.
	if installer.DetectPreCommitFramework(repoRoot) {
		pcRemoved, err := installer.RemovePreCommitConfig(repoRoot)
		if err != nil {
			return fmt.Errorf("remove from pre-commit config: %w", err)
		}
		if pcRemoved {
			removed = true
			display.LogSuccess("Hook removed from .pre-commit-config.yaml")
		}
	}

	// Also try removing from hook file (in case both methods were used).
	hookRemoved, err := installer.RemovePreCommitHook(hooksDir)
	if err != nil {
		return fmt.Errorf("remove hook: %w", err)
	}
	if hookRemoved {
		removed = true
		display.LogSuccess("Hook removed from " + hooksDir)
	}

	if !removed {
		display.LogWarn("Hook not found or not managed by ai-review")
	}
	return nil
}
