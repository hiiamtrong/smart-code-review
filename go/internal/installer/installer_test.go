package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	testInitialRepos = "repos: []\n"
	testAIReviewID          = "id: ai-review"
	testSkipWindows         = "permission test not reliable on windows"
	testGetHooksDirFmt      = "GetHooksDir: %v"
	testExpectedRemovedTrue  = "expected removed=true"
	testWriteHookFmt        = "WritePreCommitHook: %v"
	testRemoveHookFmt       = "RemovePreCommitHook: %v"
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
		t.Fatalf(testGetHooksDirFmt, err)
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
		t.Fatalf(testGetHooksDirFmt, err)
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
		// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil { //nolint:gosec
			t.Skipf("git setup failed: %v", err)
		}
	}

	got, err := GetHooksDir(repo)
	if err != nil {
		t.Fatalf(testGetHooksDirFmt, err)
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
		t.Fatalf(testWriteHookFmt, err)
	}

	hookPath := filepath.Join(hooksDir, preCommitFile)
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
		t.Fatalf(testWriteHookFmt, err)
	}

	info, err := os.Stat(filepath.Join(hooksDir, preCommitFile))
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
	hookPath := filepath.Join(hooksDir, preCommitFile)

	original := "#!/bin/sh\necho 'other hook'\n"
	if err := os.WriteFile(hookPath, []byte(original), 0755); err != nil {
		t.Fatal(err)
	}

	if err := WritePreCommitHook(hooksDir); err != nil {
		t.Fatalf(testWriteHookFmt, err)
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
	hookPath := filepath.Join(hooksDir, preCommitFile)

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
		t.Fatalf(testWriteHookFmt, err)
	}
	if _, err := os.Stat(filepath.Join(hooksDir, preCommitFile)); err != nil {
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
		t.Fatalf(testRemoveHookFmt, err)
	}
	if !removed {
		t.Error(testExpectedRemovedTrue)
	}
	if _, err := os.Stat(filepath.Join(hooksDir, preCommitFile)); !os.IsNotExist(err) {
		t.Error("hook file should have been deleted")
	}
}

func TestRemovePreCommitHook_SkipsForeignHook(t *testing.T) {
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, preCommitFile)
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho foreign\n"), 0755); err != nil {
		t.Fatal(err)
	}

	removed, err := RemovePreCommitHook(hooksDir)
	if err != nil {
		t.Fatalf(testRemoveHookFmt, err)
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
	hookPath := filepath.Join(hooksDir, preCommitFile)

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
		t.Fatalf(testRemoveHookFmt, err)
	}
	if !removed {
		t.Error(testExpectedRemovedTrue)
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
		t.Fatalf(testRemoveHookFmt, err)
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
	hookPath := filepath.Join(hooksDir, preCommitFile)
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho other\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if IsHookInstalled(hooksDir) {
		t.Error("expected IsHookInstalled=false for foreign hook")
	}
}

// ─── DetectPreCommitFramework ────────────────────────────────────────────────

func TestDetectPreCommitFramework_Found(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, preCommitConfigYAML), []byte(testInitialRepos), 0644); err != nil {
		t.Fatal(err)
	}
	if !DetectPreCommitFramework(repo) {
		t.Error("expected true when .pre-commit-config.yaml exists")
	}
}

func TestDetectPreCommitFramework_NotFound(t *testing.T) {
	repo := t.TempDir()
	if DetectPreCommitFramework(repo) {
		t.Error("expected false when .pre-commit-config.yaml does not exist")
	}
}

// ─── InjectPreCommitConfig ───────────────────────────────────────────────────

func TestInjectPreCommitConfig_Injects(t *testing.T) {
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	initial := "repos:\n  - repo: https://github.com/pre-commit/pre-commit-hooks\n    rev: v4.4.0\n    hooks:\n      - id: trailing-whitespace\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	injected, err := InjectPreCommitConfig(repo)
	if err != nil {
		t.Fatalf("InjectPreCommitConfig: %v", err)
	}
	if !injected {
		t.Error("expected injected=true")
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	if !strings.Contains(content, testAIReviewID) {
		t.Error("expected ai-review hook in config")
	}
	if !strings.Contains(content, "repo: local") {
		t.Error("expected repo: local block")
	}
	// Original content preserved
	if !strings.Contains(content, "trailing-whitespace") {
		t.Error("original hooks should be preserved")
	}
}

func TestInjectPreCommitConfig_Idempotent(t *testing.T) {
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	if err := os.WriteFile(cfgPath, []byte(testInitialRepos), 0644); err != nil {
		t.Fatal(err)
	}

	// First inject
	if _, err := InjectPreCommitConfig(repo); err != nil {
		t.Fatal(err)
	}
	// Second inject — should be no-op
	injected, err := InjectPreCommitConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if injected {
		t.Error("expected injected=false on second call")
	}

	data, _ := os.ReadFile(cfgPath)
	if strings.Count(string(data), testAIReviewID) != 1 {
		t.Error("ai-review hook should appear exactly once")
	}
}

func TestInjectPreCommitConfig_FileNotFound(t *testing.T) {
	repo := t.TempDir()
	_, err := InjectPreCommitConfig(repo)
	if err == nil {
		t.Error("expected error when .pre-commit-config.yaml does not exist")
	}
}

// ─── RemovePreCommitConfig ───────────────────────────────────────────────────

func TestRemovePreCommitConfig_Removes(t *testing.T) {
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	if err := os.WriteFile(cfgPath, []byte(testInitialRepos), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InjectPreCommitConfig(repo); err != nil {
		t.Fatal(err)
	}

	removed, err := RemovePreCommitConfig(repo)
	if err != nil {
		t.Fatalf("RemovePreCommitConfig: %v", err)
	}
	if !removed {
		t.Error(testExpectedRemovedTrue)
	}

	data, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(data), testAIReviewID) {
		t.Error("ai-review hook should be removed")
	}
}

func TestRemovePreCommitConfig_NotPresent(t *testing.T) {
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	if err := os.WriteFile(cfgPath, []byte(testInitialRepos), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := RemovePreCommitConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("expected removed=false when ai-review not in config")
	}
}

func TestRemovePreCommitConfig_NoFile(t *testing.T) {
	repo := t.TempDir()
	removed, err := RemovePreCommitConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("expected removed=false when file does not exist")
	}
}

// ─── IsPreCommitConfigInstalled ──────────────────────────────────────────────

func TestIsPreCommitConfigInstalled_True(t *testing.T) {
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	if err := os.WriteFile(cfgPath, []byte(testInitialRepos), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := InjectPreCommitConfig(repo); err != nil {
		t.Fatal(err)
	}

	if !IsPreCommitConfigInstalled(repo) {
		t.Error("expected true after injection")
	}
}

func TestIsPreCommitConfigInstalled_False(t *testing.T) {
	repo := t.TempDir()
	if IsPreCommitConfigInstalled(repo) {
		t.Error("expected false when no config file")
	}
}

// ─── Error paths (permission / filesystem errors) ────────────────────────────

func TestWritePreCommitHook_MkdirAllError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(testSkipWindows)
	}
	// Create a read-only parent so MkdirAll fails.
	base := t.TempDir()
	readonlyDir := filepath.Join(base, "readonly")
	os.MkdirAll(readonlyDir, 0555)
	defer os.Chmod(readonlyDir, 0755)

	hooksDir := filepath.Join(readonlyDir, "subdir", "hooks")
	err := WritePreCommitHook(hooksDir)
	if err == nil {
		t.Error("expected error when MkdirAll fails")
	}
}

func TestWritePreCommitHook_AppendToReadonlyHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(testSkipWindows)
	}
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, preCommitFile)
	// Create a read-only foreign hook.
	os.WriteFile(hookPath, []byte("#!/bin/sh\necho other\n"), 0444)
	defer os.Chmod(hookPath, 0755)

	err := WritePreCommitHook(hooksDir)
	if err == nil {
		t.Error("expected error when hook file is read-only")
	}
}

func TestRemovePreCommitHook_ReadError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(testSkipWindows)
	}
	hooksDir := t.TempDir()
	hookPath := filepath.Join(hooksDir, preCommitFile)
	os.WriteFile(hookPath, []byte(hookTemplate), 0000)
	defer os.Chmod(hookPath, 0755)

	_, err := RemovePreCommitHook(hooksDir)
	if err == nil {
		t.Error("expected error when hook file is unreadable")
	}
}

func TestInjectPreCommitConfig_AppendError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(testSkipWindows)
	}
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	os.WriteFile(cfgPath, []byte(testInitialRepos), 0444)
	defer os.Chmod(cfgPath, 0644)

	_, err := InjectPreCommitConfig(repo)
	if err == nil {
		t.Error("expected error when config file is read-only")
	}
}

func TestRemovePreCommitConfig_ReadError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(testSkipWindows)
	}
	repo := t.TempDir()
	cfgPath := filepath.Join(repo, preCommitConfigYAML)
	os.WriteFile(cfgPath, []byte(testInitialRepos), 0000)
	defer os.Chmod(cfgPath, 0644)

	_, err := RemovePreCommitConfig(repo)
	if err == nil {
		t.Error("expected error when config file is unreadable")
	}
}
