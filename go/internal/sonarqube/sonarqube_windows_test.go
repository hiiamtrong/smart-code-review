//go:build windows

package sonarqube

import (
	"path/filepath"
	"strings"
	"testing"
)

// Regression: filepath.Dir emits backslashes; nested-dir dedupe used forward
// slashes in HasPrefix, so overlapping sonar.sources were passed to the scanner.
func TestDedupedDirs_backslashRootsCollapseAncestors(t *testing.T) {
	files := []string{
		filepath.Join("internal", "server", "handler", "client", "client.go"),
		filepath.Join("internal", "server", "handler", "hook.go"),
		filepath.Join("internal", "server", "model", "user.go"),
	}
	got := dedupedDirs(files)
	want := strings.Join([]string{
		filepath.ToSlash(filepath.Join("internal", "server", "handler")),
		filepath.ToSlash(filepath.Join("internal", "server", "model")),
	}, ",")
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
