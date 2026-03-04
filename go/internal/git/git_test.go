package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newTempRepo creates a temporary git repository with one committed file.
func newTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Tester"},
	}
	for _, c := range cmds {
		if err := exec.Command(c[0], c[1:]...).Run(); err != nil {
			t.Fatalf("setup cmd %v: %v", c, err)
		}
	}
	// Write and commit an initial file so HEAD exists
	initial := filepath.Join(dir, "README.md")
	os.WriteFile(initial, []byte("# test\n"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
}

func TestGetStagedDiff_Empty(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	diff, err := GetStagedDiff()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff, got %q", diff)
	}
}

func TestGetStagedDiff_WithStagedFile(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	// Stage a new file
	newFile := filepath.Join(dir, "hello.go")
	os.WriteFile(newFile, []byte("package main\n"), 0644)
	exec.Command("git", "-C", dir, "add", "hello.go").Run()

	diff, err := GetStagedDiff()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(diff, "hello.go") {
		t.Errorf("expected diff to contain hello.go, got:\n%s", diff)
	}
}

func TestGetRepoRoot(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	root, err := GetRepoRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Root should be a real absolute path
	if !filepath.IsAbs(root) {
		t.Errorf("expected absolute path, got %q", root)
	}
}

func TestGetRepoRoot_NotARepo(t *testing.T) {
	chdir(t, t.TempDir()) // empty dir, not a git repo

	_, err := GetRepoRoot()
	if err == nil {
		t.Error("expected error outside git repo")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	branch, err := GetCurrentBranch()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default branch is typically "main" or "master"
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestGetGitInfo(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	info, err := GetGitInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.CommitHash == "" || info.CommitHash == "staged" {
		t.Errorf("expected real commit hash, got %q", info.CommitHash)
	}
	if info.Author.Name != "Tester" {
		t.Errorf("author name: got %q, want Tester", info.Author.Name)
	}
	if info.Author.Email != "test@test.com" {
		t.Errorf("author email: got %q, want test@test.com", info.Author.Email)
	}
}

func TestParseHunkStart(t *testing.T) {
	tests := []struct {
		hunk string
		want int
	}{
		{"@@ -1,3 +1,5 @@", 1},
		{"@@ -0,0 +1 @@", 1},
		{"@@ -10,7 +25,12 @@ func Foo() {", 25},
		{"@@ -1 +1 @@", 1},
	}
	for _, tt := range tests {
		got := parseHunkStart(tt.hunk)
		if got != tt.want {
			t.Errorf("parseHunkStart(%q) = %d, want %d", tt.hunk, got, tt.want)
		}
	}
}

func TestAnnotateLineNumbers(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
index 000..111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+
+// new comment
 func main() {}
`
	annotated := AnnotateLineNumbers(diff)

	// Added lines should have +NNN: prefix
	if !strings.Contains(annotated, "+2:") {
		t.Errorf("expected +2: in annotated diff, got:\n%s", annotated)
	}
	if !strings.Contains(annotated, "+3:") {
		t.Errorf("expected +3: in annotated diff, got:\n%s", annotated)
	}
	// Original diff header lines should be preserved
	if !strings.Contains(annotated, "diff --git") {
		t.Error("expected diff header to be preserved")
	}
}

// ─── GetLocalConfig ──────────────────────────────────────────────────────────

func TestGetLocalConfig_ExistingKey(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)
	// Set a local config key.
	exec.Command("git", "-C", dir, "config", "user.testkey", "hello").Run()

	val, err := GetLocalConfig("user.testkey")
	if err != nil {
		t.Fatalf("GetLocalConfig: %v", err)
	}
	if val != "hello" {
		t.Errorf("got %q, want %q", val, "hello")
	}
}

func TestGetLocalConfig_MissingKey(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	val, err := GetLocalConfig("no.such.key")
	if err != nil {
		t.Fatalf("GetLocalConfig for missing key returned error: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for missing key, got %q", val)
	}
}

// ─── refExists ───────────────────────────────────────────────────────────────

func TestRefExists_Exists(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	if !refExists("HEAD") {
		t.Error("HEAD should exist in a fresh repo")
	}
}

func TestRefExists_NotExists(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	if refExists("refs/heads/this-branch-does-not-exist") {
		t.Error("non-existent ref should return false")
	}
}

// ─── GetPRDiff fallback paths ─────────────────────────────────────────────────

func TestGetPRDiff_FallsBackToStagedWithSingleCommit(t *testing.T) {
	// With only one commit there's no HEAD~1 and no remotes, so GetPRDiff
	// falls all the way through to GetStagedDiff (empty = "").
	dir := newTempRepo(t)
	chdir(t, dir)

	diff, err := GetPRDiff("")
	if err != nil {
		t.Fatalf("GetPRDiff: %v", err)
	}
	_ = diff // we just verify no error; staged diff may be empty
}

func TestGetPRDiff_FallsBackToHEAD1(t *testing.T) {
	dir := newTempRepo(t)
	chdir(t, dir)

	// Add a second commit so HEAD~1 exists.
	secondFile := filepath.Join(dir, "second.txt")
	os.WriteFile(secondFile, []byte("second\n"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "second").Run()

	diff, err := GetPRDiff("")
	if err != nil {
		t.Fatalf("GetPRDiff: %v", err)
	}
	if !strings.Contains(diff, "second.txt") {
		t.Errorf("expected diff to mention second.txt; got: %q", diff)
	}
}

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"refs/pull/42/merge", "42"},
		{"refs/pull/100/head", "100"},
		{"refs/heads/main", ""},
		{"", ""},
	}
	for _, tt := range tests {
		os.Setenv("GITHUB_REF", tt.ref)
		got := extractPRNumber()
		if got != tt.want {
			t.Errorf("extractPRNumber with GITHUB_REF=%q: got %q, want %q", tt.ref, got, tt.want)
		}
	}
	os.Unsetenv("GITHUB_REF")
}
