// Package installer handles writing, detecting, and removing the ai-review
// pre-commit hook in a git repository.
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const hookMarker = "# AI-REVIEW-HOOK"

const hookTemplate = `#!/usr/bin/env sh
# AI-REVIEW-HOOK
# Managed by ai-review. Run 'ai-review uninstall' to remove.
exec ai-review run-hook "$@"
`

// GetHooksDir returns the directory where git hooks should be written.
// Priority: .husky/ > core.hooksPath git config > .git/hooks/
func GetHooksDir(repoRoot string) (string, error) {
	// 1. Husky v5+
	huskyDir := filepath.Join(repoRoot, ".husky")
	if info, err := os.Stat(huskyDir); err == nil && info.IsDir() {
		return huskyDir, nil
	}

	// 2. Custom core.hooksPath
	out, err := exec.Command("git", "-C", repoRoot, "config", "core.hooksPath").Output()
	if err == nil {
		customPath := strings.TrimSpace(string(out))
		if customPath != "" {
			if !filepath.IsAbs(customPath) {
				customPath = filepath.Join(repoRoot, customPath)
			}
			return customPath, nil
		}
	}

	// 3. Default
	return filepath.Join(repoRoot, ".git", "hooks"), nil
}

// WritePreCommitHook writes the minimal hook script into hooksDir.
// Returns an error if an existing hook without our marker would be overwritten.
func WritePreCommitHook(hooksDir string) error {
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")

	// If a hook exists that isn't ours, refuse to overwrite
	if existing, err := os.ReadFile(hookPath); err == nil {
		if !strings.Contains(string(existing), hookMarker) {
			return fmt.Errorf(
				"pre-commit hook already exists at %s and was not created by ai-review.\n"+
					"Add the following line to it manually:\n  exec ai-review run-hook \"$@\"",
				hookPath,
			)
		}
	}

	if err := os.WriteFile(hookPath, []byte(hookTemplate), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}
	return nil
}

// RemovePreCommitHook removes the hook file if it contains our marker.
// Returns (true, nil) if removed, (false, nil) if not our hook.
func RemovePreCommitHook(hooksDir string) (bool, error) {
	hookPath := filepath.Join(hooksDir, "pre-commit")
	data, err := os.ReadFile(hookPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read hook: %w", err)
	}
	if !strings.Contains(string(data), hookMarker) {
		return false, nil
	}
	if err := os.Remove(hookPath); err != nil {
		return false, fmt.Errorf("remove hook: %w", err)
	}
	return true, nil
}

// IsHookInstalled reports whether the ai-review hook is installed in hooksDir.
func IsHookInstalled(hooksDir string) bool {
	hookPath := filepath.Join(hooksDir, "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), hookMarker)
}
