package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newFakeRepo creates a minimal temp directory that looks like a git repo root.
func newFakeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ─── GetHooksDir ─────────────────────────────────────────────────────────────

func TestGetHooksDir_Default(t *testing.T) {
	repo := newFakeRepo(t)
	got, err := GetHooksDir(repo)
	if err != nil {
		t.Fatalf("GetHooksDir: %v", err)
	}
	want := filepath.Join(repo, ".git", "hooks")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGetHooksDir_Husky(t *testing.T) {
	repo := newFakeRepo(t)
	huskyDir := filepath.Join(repo, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := GetHooksDir(repo)
	if err != nil {
		t.Fatalf("GetHooksDir: %v", err)
	}
	if got != huskyDir {
		t.Errorf("got %q, want husky dir %q", got, huskyDir)
	}
}

// ─── WritePreCommitHook ───────────────────────────────────────────────────────

func TestWritePreCommitHook_CreatesFile(t *testing.T) {
	hooksDir := t.TempDir()
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf("WritePreCommitHook: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if !strings.Contains(string(data), hookMarker) {
		t.Error("hook file missing AI-REVIEW-HOOK marker")
	}
	if !strings.Contains(string(data), "exec ai-review run-hook") {
		t.Error("hook file missing exec ai-review run-hook line")
	}
}

func TestWritePreCommitHook_FileIsExecutable(t *testing.T) {
	hooksDir := t.TempDir()
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf("WritePreCommitHook: %v", err)
	}

	info, err := os.Stat(filepath.Join(hooksDir, "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		t.Errorf("hook file not executable: mode %o", info.Mode())
	}
}

func TestWritePreCommitHook_OverwritesOwnHook(t *testing.T) {
	hooksDir := t.TempDir()

	// First install.
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// Second install (overwrite our own hook) should succeed.
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf("second write (overwrite own hook): %v", err)
	}
}

func TestWritePreCommitHook_RefusesToOverwriteForeignHook(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, "pre-commit")

	// Write a hook without our marker.
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho 'other hook'\n"), 0755); err != nil {
		t.Fatal(err)
	}

	err := WritePreCommitHook(hooksDir)
	if err == nil {
		t.Fatal("expected error when overwriting foreign hook, got nil")
	}
	if !strings.Contains(err.Error(), "not created by ai-review") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestWritePreCommitHook_CreatesHooksDirIfMissing(t *testing.T) {
	base := t.TempDir()
	hooksDir := filepath.Join(base, "deep", "hooks")
	// hooksDir does not exist yet.

	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf("WritePreCommitHook: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err != nil {
		t.Errorf("hook not created: %v", err)
	}
}

// ─── RemovePreCommitHook ──────────────────────────────────────────────────────

func TestRemovePreCommitHook_RemovesOwnHook(t *testing.T) {
	hooksDir := t.TempDir()
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatal(err)
	}

	removed, err := RemovePreCommitHook(hooksDir)
	if err != nil {
		t.Fatalf("RemovePreCommitHook: %v", err)
	}
	if !removed {
		t.Error("expected removed=true")
	}
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); !os.IsNotExist(err) {
		t.Error("hook file should have been deleted")
	}
}

func TestRemovePreCommitHook_SkipsForeignHook(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho foreign\n"), 0755); err != nil {
		t.Fatal(err)
	}

	removed, err := RemovePreCommitHook(hooksDir)
	if err != nil {
		t.Fatalf("RemovePreCommitHook: %v", err)
	}
	if removed {
		t.Error("expected removed=false for foreign hook")
	}
	// Foreign hook must not be deleted.
	if _, err := os.Stat(hookPath); err != nil {
		t.Error("foreign hook should still exist")
	}
}

func TestRemovePreCommitHook_NoHookFile(t *testing.T) {
	hooksDir := t.TempDir()
	removed, err := RemovePreCommitHook(hooksDir)
	if err != nil {
		t.Fatalf("RemovePreCommitHook: %v", err)
	}
	if removed {
		t.Error("expected removed=false when hook does not exist")
	}
}

// ─── IsHookInstalled ─────────────────────────────────────────────────────────

func TestIsHookInstalled_True(t *testing.T) {
	hooksDir := t.TempDir()
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatal(err)
	}
	if !IsHookInstalled(hooksDir) {
		t.Error("expected IsHookInstalled=true after WritePreCommitHook")
	}
}

func TestIsHookInstalled_FalseWhenMissing(t *testing.T) {
	hooksDir := t.TempDir()
	if IsHookInstalled(hooksDir) {
		t.Error("expected IsHookInstalled=false when no hook file")
	}
}

func TestIsHookInstalled_FalseForForeignHook(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho other\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if IsHookInstalled(hooksDir) {
		t.Error("expected IsHookInstalled=false for foreign hook")
	}
}
