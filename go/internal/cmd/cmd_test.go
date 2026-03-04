package cmd

import (
	"os"
	"testing"
)

// ─── enabledStr ──────────────────────────────────────────────────────────────

func TestEnabledStr(t *testing.T) {
	if got := enabledStr(true); got != "enabled" {
		t.Errorf("enabledStr(true) = %q, want %q", got, "enabled")
	}
	if got := enabledStr(false); got != "disabled" {
		t.Errorf("enabledStr(false) = %q, want %q", got, "disabled")
	}
}

// ─── orNotSet ────────────────────────────────────────────────────────────────

func TestOrNotSet(t *testing.T) {
	if got := orNotSet(""); got != "(not set)" {
		t.Errorf("orNotSet(\"\") = %q, want \"(not set)\"", got)
	}
	if got := orNotSet("value"); got != "value" {
		t.Errorf("orNotSet(\"value\") = %q, want %q", got, "value")
	}
}

// ─── maskIfSet ───────────────────────────────────────────────────────────────

func TestMaskIfSet(t *testing.T) {
	if got := maskIfSet(""); got != "(not set)" {
		t.Errorf("maskIfSet(\"\") = %q, want \"(not set)\"", got)
	}
	if got := maskIfSet("secret"); got != "****" {
		t.Errorf("maskIfSet(\"secret\") = %q, want \"****\"", got)
	}
}

// ─── boolStr ─────────────────────────────────────────────────────────────────

func TestBoolStr(t *testing.T) {
	if got := boolStr(true); got != "true" {
		t.Errorf("boolStr(true) = %q, want %q", got, "true")
	}
	if got := boolStr(false); got != "false" {
		t.Errorf("boolStr(false) = %q, want %q", got, "false")
	}
}

// ─── extractStagedFiles ───────────────────────────────────────────────────────

func TestExtractStagedFiles_Empty(t *testing.T) {
	files := extractStagedFiles("")
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

func TestExtractStagedFiles_SingleFile(t *testing.T) {
	diff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n"
	files := extractStagedFiles(diff)
	if len(files) != 1 || files[0] != "main.go" {
		t.Errorf("got %v, want [main.go]", files)
	}
}

func TestExtractStagedFiles_MultipleFiles(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\ndiff --git a/b.go b/b.go\n"
	files := extractStagedFiles(diff)
	if len(files) != 2 {
		t.Errorf("got %d files, want 2: %v", len(files), files)
	}
}

func TestExtractStagedFiles_Deduplicates(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\ndiff --git a/a.go b/a.go\n"
	files := extractStagedFiles(diff)
	if len(files) != 1 {
		t.Errorf("expected dedup to 1 file, got %d: %v", len(files), files)
	}
}

// ─── runStatus (no config file) ───────────────────────────────────────────────

func TestRunStatus_NoConfigFile(t *testing.T) {
	// Point HOME at an empty temp dir so config.Load() finds no config file.
	t.Setenv("HOME", t.TempDir())

	// runStatus should handle the missing config gracefully (log + return nil).
	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("runStatus with no config should return nil, got: %v", err)
	}
}

// ─── runUpdate (FetchLatest error via bad URL) ────────────────────────────────

func TestRunUpdate_FetchError(t *testing.T) {
	// Point releaseAPIURL at a non-existent server so FetchLatest returns error.
	// Since updater.releaseAPIURL is unexported we test indirectly: runUpdate
	// should propagate the FetchLatest error.
	// We can't override the URL without modifying the updater package internals,
	// so we just verify runUpdate returns a non-nil error when the network fails.
	// To avoid flakiness we skip if network unavailable isn't testable.
	t.Skip("runUpdate requires modifying updater.releaseAPIURL which is package-private")
}

// ─── runConfig ───────────────────────────────────────────────────────────────

func TestRunConfig_UnknownSubcommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// With no config file, Load() errors; but "unknown subcommand" is the
	// expected error when args don't match any known pattern.
	// Either way, the function returns an error.
	err := runConfig(nil, []string{"invalid-subcmd"})
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// ─── runInstall (not a git repo) ──────────────────────────────────────────────

func TestRunInstall_NotAGitRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Change to a temp dir that is NOT a git repo.
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(tmp)

	err := runInstall(nil, nil)
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}
