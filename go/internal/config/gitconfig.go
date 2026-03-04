package config

import (
	"os/exec"
	"strings"
)

// gitLocalConfigImpl reads a per-repo git config value using os/exec directly.
// This avoids an import cycle with internal/git. Returns "" on any error.
func gitLocalConfigImpl(key string) string {
	out, err := exec.Command("git", "config", "--local", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
