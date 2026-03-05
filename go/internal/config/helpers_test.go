package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

// setTestHome overrides environment variables so that ConfigDir() resolves to
// dir/.config/ai-review on every platform.
//
//   - Unix:    ConfigDir() uses $HOME/.config/ai-review
//   - Windows: ConfigDir() uses %APPDATA%\ai-review
//
// By setting APPDATA to dir/.config on Windows we get the same layout.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join(dir, ".config"))
	}
}
