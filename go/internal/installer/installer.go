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

// appendSnippet is the line appended to existing hooks that were not created by ai-review.
const appendSnippet = "\n" + hookMarker + "\nai-review run-hook \"$@\"\n"

// WritePreCommitHook writes or appends the ai-review hook into hooksDir.
// If a hook already exists from another tool (e.g. Husky), it appends rather than overwriting.
func WritePreCommitHook(hooksDir string) error {
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")

	existing, err := os.ReadFile(hookPath)
	if err == nil {
		content := string(existing)
		// Already installed — nothing to do
		if strings.Contains(content, hookMarker) {
			return nil
		}
		// Append our snippet to the existing hook
		f, err := os.OpenFile(hookPath, os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			return fmt.Errorf("open hook for append: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(appendSnippet); err != nil {
			return fmt.Errorf("append to hook: %w", err)
		}
		return nil
	}

	// No existing hook — write our full template
	if err := os.WriteFile(hookPath, []byte(hookTemplate), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}
	return nil
}

// RemovePreCommitHook removes ai-review from the hook.
// If the hook was created entirely by ai-review, the file is deleted.
// If ai-review was appended to an existing hook, only the appended lines are removed.
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
	content := string(data)
	if !strings.Contains(content, hookMarker) {
		return false, nil
	}

	// If the file matches our full template, delete it entirely
	if strings.TrimSpace(content) == strings.TrimSpace(hookTemplate) {
		if err := os.Remove(hookPath); err != nil {
			return false, fmt.Errorf("remove hook: %w", err)
		}
		return true, nil
	}

	// Otherwise, strip only our appended lines
	cleaned := strings.Replace(content, appendSnippet, "", 1)
	if err := os.WriteFile(hookPath, []byte(cleaned), 0755); err != nil {
		return false, fmt.Errorf("update hook: %w", err)
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
