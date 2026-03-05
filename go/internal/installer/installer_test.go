package installer

import (
	"os"
	"os/exec"
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

func TestGetHooksDir_CustomCoreHooksPath(t *testing.T) {
	repo := t.TempDir()
	cmds := [][]string{
		{"git", "init", repo},
		{"git", "-C", repo, "config", "user.email", "test@test.com"},
		{"git", "-C", repo, "config", "user.name", "Tester"},
		{"git", "-C", repo, "config", "core.hooksPath", ".custom-hooks"},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			t.Skipf("git setup failed: %v", err)
		}
	}

	got, err := GetHooksDir(repo)
	if err != nil {
		t.Fatalf("GetHooksDir: %v", err)
	}
	want := filepath.Join(repo, ".custom-hooks")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
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

func TestWritePreCommitHook_AppendsToForeignHook(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, "pre-commit")

	original := "#!/bin/sh\necho 'other hook'\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}

	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf("WritePreCommitHook: %v", err)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	content := string(data)
	// Original content preserved
	if !strings.Contains(content, "echo 'other hook'") {
		t.Error("original hook content was lost")
	}
	// Our marker appended
	if !strings.Contains(content, hookMarker) {
		t.Error("hook marker not appended")
	}
	// Our command appended
	if !strings.Contains(content, "ai-review run-hook") {
		t.Error("ai-review run-hook not appended")
	}
}

func TestWritePreCommitHook_AppendsIdempotent(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, "pre-commit")

	original := "#!/bin/sh\necho 'other hook'\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}

	// Install twice
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatal(err)
	}
	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(hookPath)
	if strings.Count(string(data), hookMarker) != 1 {
		t.Error("marker should appear exactly once after double install")
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

func TestRemovePreCommitHook_StripsAppendedLines(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, "pre-commit")

	original := "#!/bin/sh\necho 'husky hook'\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	// Append our hook
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

	// File should still exist with original content
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal("hook file should still exist after removing appended lines")
	}
	if string(data) != original {
		t.Errorf("expected original content %q, got %q", original, string(data))
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
