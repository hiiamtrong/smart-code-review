//go:build e2e

// Package e2e contains end-to-end tests that build and invoke the ai-review
// binary as a real subprocess. Each test verifies complete user-facing behaviour
// (exit codes, stdout/stderr text, file-system side effects) rather than
// individual package internals.
//
// # Why a separate build tag?
//
// E2E tests compile the binary on the fly and start a live httptest.Server to
// act as the AI gateway. They take several seconds and must not run alongside
// the fast unit-test suite. The `e2e` build tag keeps them out of
// `go test ./...` and lets CI run them in a dedicated step:
//
//	go test -tags=e2e -v -timeout 120s ./e2e/
//
// # Architecture
//
//	TestMain ─── builds ./cmd/ai-review binary → aiBinary (package var)
//	Each test ─ newTempGitRepo() → stageFile() → sseServer/hangServer
//	          └─ run(t, repoDir, env, "run-hook") → check exit code + output
package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// aiBinary holds the path to the compiled binary, set once by TestMain.
var aiBinary string

// TestMain compiles the ai-review binary once before running all E2E tests.
// The binary is built into a temporary directory that is cleaned up after the
// test run finishes.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ai-review-e2e-bin-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	defer os.RemoveAll(tmp)

	aiBinary = filepath.Join(tmp, "ai-review")
	if runtime.GOOS == "windows" {
		aiBinary += ".exe"
	}

	// Build from go/ module root (parent of this e2e/ directory).
	buildCmd := exec.Command("go", "build", "-o", aiBinary, "./cmd/ai-review")
	buildCmd.Dir = ".." // go/e2e/../ = go/
	if out, err := buildCmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("build ai-review failed: %v\n%s", err, out))
	}

	os.Exit(m.Run())
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// runResult holds the output and exit code of a subprocess invocation.
type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// run executes the ai-review binary with args inside dir, merging extraEnv on
// top of the current process environment.
func run(t *testing.T, dir string, extraEnv []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(aiBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Logf("run non-exit error: %v", err)
		}
	}
	return runResult{
		Stdout:   outBuf.String(),
		Stderr:   errBuf.String(),
		ExitCode: exitCode,
	}
}

// newTempGitRepo initialises a git repo with a single initial commit so that
// git commands (diff, log, rev-parse) work correctly.
func newTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	gitCmds := [][]string{
		{"init"},
		{"config", "user.email", "e2e@test.com"},
		{"config", "user.name", "E2E Test"},
	}
	for _, args := range gitCmds {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).
			CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create an initial commit so HEAD exists (needed for git diff --cached).
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "initial commit"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).
			CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// stageFile writes content to name inside repoDir and stages it with git add.
func stageFile(t *testing.T, repoDir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0644); err != nil {
		t.Fatalf("stageFile write: %v", err)
	}
	if out, err := exec.Command("git", "-C", repoDir, "add", name).
		CombinedOutput(); err != nil {
		t.Fatalf("git add %s: %v\n%s", name, err, out)
	}
}

// baseEnv returns the minimum environment variables the binary needs to run
// without a config file on disk (pure CI mode).
// BLOCK_ON_GATEWAY_ERROR defaults to false so tests can control blocking
// behaviour per-test by overriding the env var.
func baseEnv(homeDir, gatewayURL string) []string {
	return []string{
		"HOME=" + homeDir,
		"AI_GATEWAY_URL=" + gatewayURL,
		"AI_GATEWAY_API_KEY=e2e-test-key",
		"AI_MODEL=gemini-2.0-flash",
		"ENABLE_AI_REVIEW=true",
		"BLOCK_ON_GATEWAY_ERROR=false",
	}
}

// sseServer starts an httptest.Server that returns a well-formed SSE response.
// If severity is non-empty a single diagnostic event with that severity is
// emitted before the complete event.  If severity is empty only the complete
// event is sent (no issues found).
func sseServer(t *testing.T, severity string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		if severity != "" {
			diag, _ := json.Marshal(map[string]interface{}{
				"message":  "test issue found by E2E mock",
				"severity": severity,
				"location": map[string]interface{}{
					"path": "main.go",
					"range": map[string]interface{}{
						"start": map[string]int{"line": 1, "column": 1},
						"end":   map[string]int{"line": 1, "column": 10},
					},
				},
			})
			fmt.Fprintf(w, "event: diagnostic\ndata: %s\n\n", diag)
		}

		maxSev := severity
		if maxSev == "" {
			maxSev = "INFO"
		}
		complete, _ := json.Marshal(map[string]interface{}{
			"overview":     "E2E mock review complete",
			"max_severity": maxSev,
			"source":       map[string]string{"name": "ai-review"},
		})
		fmt.Fprintf(w, "event: complete\ndata: %s\n\n", complete)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// errorServer starts an httptest.Server that always returns an SSE error event,
// simulating a gateway failure (model overloaded, auth error, etc.).
func errorServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: error\ndata: {\"message\":\"gateway unavailable\"}\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// hangingServer starts an httptest.Server that blocks indefinitely, simulating
// a network timeout scenario. The server is registered for cleanup so that when
// the test ends, active handlers are unblocked before the server shuts down.
func hangingServer(t *testing.T) *httptest.Server {
	t.Helper()
	// done is closed during cleanup to release any blocked handlers before
	// srv.Close() is called, preventing a deadlock in httptest.Server.Close().
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-r.Context().Done():
		}
	}))
	// Cleanup order matters: t.Cleanup runs LIFO.
	// Register srv.Close first (runs last), then close(done) (runs first).
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(done) })
	return srv
}

// ─── smoke tests ──────────────────────────────────────────────────────────────

// TestHelp verifies the binary runs and exits 0 for the help command.
func TestHelp(t *testing.T) {
	res := run(t, t.TempDir(), nil, "help")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0\nstdout: %s\nstderr: %s",
			res.ExitCode, res.Stdout, res.Stderr)
	}
	if !strings.Contains(res.Stdout+res.Stderr, "ai-review") {
		t.Errorf("help output missing 'ai-review':\n%s", res.Stdout)
	}
}

// TestVersion verifies that --version exits 0 and prints a version string.
// Note: version is a flag (--version), not a subcommand.
func TestVersion(t *testing.T) {
	res := run(t, t.TempDir(), nil, "--version")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0\nstdout: %s\nstderr: %s",
			res.ExitCode, res.Stdout, res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "ai-review") {
		t.Errorf("expected 'ai-review' in version output:\n%s", combined)
	}
}

// ─── install / uninstall / status ─────────────────────────────────────────────

// TestStatus_NotInstalled verifies that `ai-review status` shows the hook is
// not installed in a freshly created git repo.
func TestStatus_NotInstalled(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "")

	res := run(t, repoDir, baseEnv(homeDir, srv.URL), "status")
	if res.ExitCode != 0 {
		t.Fatalf("status exit code: got %d, want 0\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "not installed") && !strings.Contains(combined, "Hook not installed") {
		t.Errorf("expected 'not installed' in output:\n%s", combined)
	}
}

// TestInstallAndStatus installs the hook and verifies status reflects that.
func TestInstallAndStatus(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "")
	env := baseEnv(homeDir, srv.URL)

	// Install
	res := run(t, repoDir, env, "install")
	if res.ExitCode != 0 {
		t.Fatalf("install exit code: got %d\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}

	// Verify hook file exists and is executable
	hookPath := filepath.Join(repoDir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("hook file not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("hook not executable: mode %o", info.Mode())
	}

	// Status should show installed
	res = run(t, repoDir, env, "status")
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "installed") {
		t.Errorf("expected 'installed' in status output:\n%s", combined)
	}
}

// TestInstallAndUninstall verifies the hook is removed after uninstall.
func TestInstallAndUninstall(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "")
	env := baseEnv(homeDir, srv.URL)

	if res := run(t, repoDir, env, "install"); res.ExitCode != 0 {
		t.Fatalf("install failed: %s", res.Stdout+res.Stderr)
	}

	hookPath := filepath.Join(repoDir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("hook not found after install: %v", err)
	}

	if res := run(t, repoDir, env, "uninstall"); res.ExitCode != 0 {
		t.Fatalf("uninstall failed: %s", res.Stdout+res.Stderr)
	}

	if _, err := os.Stat(hookPath); err == nil {
		t.Error("hook file still exists after uninstall")
	}
}

// ─── run-hook behaviour ───────────────────────────────────────────────────────

// TestRunHook_NoStagedFiles verifies that run-hook exits 0 when nothing is staged.
func TestRunHook_NoStagedFiles(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "")

	res := run(t, repoDir, baseEnv(homeDir, srv.URL), "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "No staged") {
		t.Errorf("expected 'No staged' in output:\n%s", combined)
	}
}

// TestRunHook_AIDisabled verifies that ENABLE_AI_REVIEW=false causes an early exit 0.
func TestRunHook_AIDisabled(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "")

	env := append(baseEnv(homeDir, srv.URL), "ENABLE_AI_REVIEW=false")
	res := run(t, repoDir, env, "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0", res.ExitCode)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "disabled") {
		t.Errorf("expected 'disabled' in output:\n%s", combined)
	}
}

// TestRunHook_AllFilesIgnored verifies that when all staged files match
// .aireviewignore patterns the hook exits 0 without calling the gateway.
func TestRunHook_AllFilesIgnored(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()

	// Use a server that would fail the test if called — it should never be reached.
	called := false
	gatewayNeverCalled := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	t.Cleanup(gatewayNeverCalled.Close)

	// Write .aireviewignore that ignores all .go files
	if err := os.WriteFile(filepath.Join(repoDir, ".aireviewignore"), []byte("*.go\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Stage a .go file
	stageFile(t, repoDir, "main.go", "package main\n")

	res := run(t, repoDir, baseEnv(homeDir, gatewayNeverCalled.URL), "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0\n%s", res.ExitCode, res.Stdout+res.Stderr)
	}
	if called {
		t.Error("gateway was called even though all files were ignored")
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "ignored") {
		t.Errorf("expected 'ignored' in output:\n%s", combined)
	}
}

// TestRunHook_GatewayReturnsWarning verifies that WARNING diagnostics do not
// block the commit (exit 0).
func TestRunHook_GatewayReturnsWarning(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "WARNING")

	stageFile(t, repoDir, "main.go", "package main\n")

	res := run(t, repoDir, baseEnv(homeDir, srv.URL), "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (warnings should not block)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "WARNING") && !strings.Contains(combined, "warning") {
		t.Errorf("expected WARNING in output:\n%s", combined)
	}
}

// TestRunHook_GatewayReturnsError verifies that ERROR diagnostics block the
// commit (exit 1).
func TestRunHook_GatewayReturnsError(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := sseServer(t, "ERROR")

	stageFile(t, repoDir, "main.go", "package main\n")

	res := run(t, repoDir, baseEnv(homeDir, srv.URL), "run-hook")
	if res.ExitCode != 1 {
		t.Fatalf("exit code: got %d, want 1 (errors must block commit)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "blocked") && !strings.Contains(combined, "Blocked") {
		t.Errorf("expected 'blocked' in output:\n%s", combined)
	}
}

// TestRunHook_GatewayError_NoBlock verifies that a gateway error with
// BLOCK_ON_GATEWAY_ERROR=false causes exit 0.
func TestRunHook_GatewayError_NoBlock(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := errorServer(t)

	stageFile(t, repoDir, "main.go", "package main\n")

	env := append(baseEnv(homeDir, srv.URL), "BLOCK_ON_GATEWAY_ERROR=false")
	res := run(t, repoDir, env, "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (BLOCK_ON_GATEWAY_ERROR=false)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
}

// TestRunHook_GatewayError_Block verifies that a gateway error with
// BLOCK_ON_GATEWAY_ERROR=true causes exit 1.
func TestRunHook_GatewayError_Block(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := errorServer(t)

	stageFile(t, repoDir, "main.go", "package main\n")

	env := append(baseEnv(homeDir, srv.URL), "BLOCK_ON_GATEWAY_ERROR=true")
	res := run(t, repoDir, env, "run-hook")
	if res.ExitCode != 1 {
		t.Fatalf("exit code: got %d, want 1 (BLOCK_ON_GATEWAY_ERROR=true)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
}

// TestRunHook_GatewayTimeout verifies that a hanging gateway triggers the
// context deadline. With BLOCK_ON_GATEWAY_ERROR=false the hook still exits 0
// (does not block legitimate commits when the AI service is degraded).
func TestRunHook_GatewayTimeout(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := hangingServer(t)

	stageFile(t, repoDir, "main.go", "package main\n")

	// 2-second timeout so the test completes quickly.
	env := append(baseEnv(homeDir, srv.URL),
		"GATEWAY_TIMEOUT_SEC=2",
		"BLOCK_ON_GATEWAY_ERROR=false",
	)
	res := run(t, repoDir, env, "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (timeout + no block)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
}

// TestRunHook_GatewayTimeout_Block verifies that a hanging gateway with
// BLOCK_ON_GATEWAY_ERROR=true causes exit 1.
func TestRunHook_GatewayTimeout_Block(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	srv := hangingServer(t)

	stageFile(t, repoDir, "main.go", "package main\n")

	env := append(baseEnv(homeDir, srv.URL),
		"GATEWAY_TIMEOUT_SEC=2",
		"BLOCK_ON_GATEWAY_ERROR=true",
	)
	res := run(t, repoDir, env, "run-hook")
	if res.ExitCode != 1 {
		t.Fatalf("exit code: got %d, want 1 (timeout + BLOCK_ON_GATEWAY_ERROR=true)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
}

// TestRunHook_OutputContent verifies that the binary correctly renders the
// diagnostic message, file location, and overview returned by the gateway.
func TestRunHook_OutputContent(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()

	// Serve a richer SSE response with a specific message and multi-field location.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		diag, _ := json.Marshal(map[string]interface{}{
			"message":  "unused variable 'x' declared but not used",
			"severity": "WARNING",
			"location": map[string]interface{}{
				"path": "main.go",
				"range": map[string]interface{}{
					"start": map[string]int{"line": 42, "column": 5},
					"end":   map[string]int{"line": 42, "column": 6},
				},
			},
		})
		fmt.Fprintf(w, "event: diagnostic\ndata: %s\n\n", diag)

		complete, _ := json.Marshal(map[string]interface{}{
			"overview":     "Found 1 warning. Please review before merging.",
			"max_severity": "WARNING",
			"source":       map[string]string{"name": "ai-review"},
		})
		fmt.Fprintf(w, "event: complete\ndata: %s\n\n", complete)
	}))
	t.Cleanup(srv.Close)

	stageFile(t, repoDir, "main.go", "package main\nfunc main() { x := 1\n_ = x\n}\n")

	res := run(t, repoDir, baseEnv(homeDir, srv.URL), "run-hook")
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)

	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (WARNING should not block)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}

	combined := res.Stdout + res.Stderr

	// Diagnostic message should appear in output.
	if !strings.Contains(combined, "unused variable") {
		t.Errorf("expected diagnostic message in output:\n%s", combined)
	}
	// File path should appear.
	if !strings.Contains(combined, "main.go") {
		t.Errorf("expected file path 'main.go' in output:\n%s", combined)
	}
	// Line number should appear.
	if !strings.Contains(combined, "42") {
		t.Errorf("expected line number '42' in output:\n%s", combined)
	}
	// Overview should appear.
	if !strings.Contains(combined, "Found 1 warning") {
		t.Errorf("expected overview text in output:\n%s", combined)
	}
}

// TestRunHook_MultipleIssues verifies that when the gateway returns multiple
// diagnostics of different severities, all are rendered and the exit code
// reflects the highest severity (ERROR → exit 1).
func TestRunHook_MultipleIssues(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		issues := []map[string]interface{}{
			{
				"message":  "SQL injection risk: unsanitized input",
				"severity": "ERROR",
				"location": map[string]interface{}{
					"path": "db.go",
					"range": map[string]interface{}{
						"start": map[string]int{"line": 10, "column": 1},
						"end":   map[string]int{"line": 10, "column": 50},
					},
				},
			},
			{
				"message":  "missing error check on Close()",
				"severity": "WARNING",
				"location": map[string]interface{}{
					"path": "db.go",
					"range": map[string]interface{}{
						"start": map[string]int{"line": 20, "column": 1},
						"end":   map[string]int{"line": 20, "column": 20},
					},
				},
			},
		}
		for _, issue := range issues {
			data, _ := json.Marshal(issue)
			fmt.Fprintf(w, "event: diagnostic\ndata: %s\n\n", data)
		}

		complete, _ := json.Marshal(map[string]interface{}{
			"overview":     "Found 2 issues: 1 error, 1 warning.",
			"max_severity": "ERROR",
			"source":       map[string]string{"name": "ai-review"},
		})
		fmt.Fprintf(w, "event: complete\ndata: %s\n\n", complete)
	}))
	t.Cleanup(srv.Close)

	stageFile(t, repoDir, "db.go", "package main\n")

	res := run(t, repoDir, baseEnv(homeDir, srv.URL), "run-hook")
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)

	// ERROR severity must block commit.
	if res.ExitCode != 1 {
		t.Fatalf("exit code: got %d, want 1 (ERROR must block commit)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}

	combined := res.Stdout + res.Stderr

	if !strings.Contains(combined, "SQL injection") {
		t.Errorf("expected ERROR diagnostic in output:\n%s", combined)
	}
	if !strings.Contains(combined, "missing error check") {
		t.Errorf("expected WARNING diagnostic in output:\n%s", combined)
	}
	if !strings.Contains(combined, "db.go") {
		t.Errorf("expected file path 'db.go' in output:\n%s", combined)
	}
}

// ─── SonarQube integration ────────────────────────────────────────────────────

// sonarServer starts a mock SonarQube API that serves the three endpoints used
// by run-hook: task polling, issues search, and hotspots search.
//
//   - issues:    slice of raw sonarIssue-like maps (message, severity, component, line)
//   - hotspots:  number of hotspot entries to return (empty objects suffice)
func sonarServer(t *testing.T, issues []map[string]interface{}, hotspotCount int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/api/ce/task"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"task": map[string]string{"status": "SUCCESS"},
			})
		case strings.Contains(r.URL.Path, "/api/issues/search"):
			json.NewEncoder(w).Encode(map[string]interface{}{"issues": issues})
		case strings.Contains(r.URL.Path, "/api/hotspots/search"):
			spots := make([]map[string]interface{}, hotspotCount)
			for i := range spots {
				spots[i] = map[string]interface{}{"key": fmt.Sprintf("hotspot-%d", i)}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"hotspots": spots})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeSonarScanner writes a platform-appropriate fake scanner script to a temp
// directory and returns that directory.
//
//   - Unix/macOS: "sonar-scanner" shell script (#!/bin/sh)
//   - Windows:    "sonar-scanner.bat" batch file
//
// Both scripts create .scannerwork/report-task.txt with a ceTaskId line in the
// process working directory (= repoDir when run-hook invokes the scanner).
func fakeSonarScanner(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		// Windows batch file — FindScanner() looks for "sonar-scanner.bat" on Windows.
		// The echo redirect must have no space before > to avoid trailing spaces.
		script := "@echo off\r\n" +
			"if not exist .scannerwork mkdir .scannerwork\r\n" +
			"echo ceTaskId=fake-task-e2e>.scannerwork\\report-task.txt\r\n" +
			"exit /b 0\r\n"
		path := filepath.Join(dir, "sonar-scanner.bat")
		if err := os.WriteFile(path, []byte(script), 0644); err != nil {
			t.Fatalf("write fake sonar-scanner.bat: %v", err)
		}
	} else {
		script := "#!/bin/sh\nmkdir -p .scannerwork\nprintf 'ceTaskId=fake-task-e2e\\n' > .scannerwork/report-task.txt\nexit 0\n"
		path := filepath.Join(dir, "sonar-scanner")
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			t.Fatalf("write fake sonar-scanner: %v", err)
		}
	}
	return dir
}

// sonarEnv returns env vars needed to enable SonarQube local scanning.
// aiGatewayURL must point to a live httptest.Server — run-hook checks for a
// non-empty AI_GATEWAY_URL before it reaches the SonarQube block.
func sonarEnv(homeDir, sonarURL, aiGatewayURL, binDir string) []string {
	return []string{
		"HOME=" + homeDir,
		"AI_GATEWAY_URL=" + aiGatewayURL,
		"AI_GATEWAY_API_KEY=e2e-test-key",
		"AI_MODEL=gemini-2.0-flash",
		"ENABLE_AI_REVIEW=true",
		"BLOCK_ON_GATEWAY_ERROR=false",
		"ENABLE_SONARQUBE_LOCAL=true",
		"SONAR_TOKEN=test-token",
		"SONAR_HOST_URL=" + sonarURL,
		"SONAR_PROJECT_KEY=test-project",
		"SONAR_FILTER_CHANGED_LINES_ONLY=false",
		"SONAR_BLOCK_ON_HOTSPOTS=false",
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
}

// TestRunHook_SonarQube_WarningIssues verifies that SonarQube MINOR issues
// (→ WARNING severity) are displayed but do not block the commit.
func TestRunHook_SonarQube_WarningIssues(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	binDir := fakeSonarScanner(t)

	issues := []map[string]interface{}{
		{
			"message":   "Variable name 'x' should match pattern",
			"rule":      "go:S117",
			"severity":  "MINOR",
			"component": "test-project:main.go",
			"line":      5,
		},
	}
	sonarSrv := sonarServer(t, issues, 0)
	aiSrv := sseServer(t, "") // no AI issues — only SonarQube issues matter here

	stageFile(t, repoDir, "main.go", "package main\nfunc main() { x := 1; _ = x }\n")

	res := run(t, repoDir, sonarEnv(homeDir, sonarSrv.URL, aiSrv.URL, binDir), "run-hook")
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)

	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (MINOR/WARNING must not block)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "Variable name") {
		t.Errorf("expected SonarQube issue message in output:\n%s", combined)
	}
	if !strings.Contains(combined, "main.go") {
		t.Errorf("expected file path in output:\n%s", combined)
	}
}

// TestRunHook_SonarQube_ErrorBlocksCommit verifies that a BLOCKER/CRITICAL
// SonarQube issue (→ ERROR severity) blocks the commit with exit 1.
func TestRunHook_SonarQube_ErrorBlocksCommit(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	binDir := fakeSonarScanner(t)

	issues := []map[string]interface{}{
		{
			"message":   "SQL injection vulnerability detected",
			"rule":      "go:S3649",
			"severity":  "BLOCKER",
			"component": "test-project:db.go",
			"line":      12,
		},
	}
	sonarSrv := sonarServer(t, issues, 0)
	aiSrv := sseServer(t, "")

	stageFile(t, repoDir, "db.go", "package main\n")

	res := run(t, repoDir, sonarEnv(homeDir, sonarSrv.URL, aiSrv.URL, binDir), "run-hook")
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)

	if res.ExitCode != 1 {
		t.Fatalf("exit code: got %d, want 1 (BLOCKER must block commit)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "SQL injection") {
		t.Errorf("expected SonarQube issue message in output:\n%s", combined)
	}
	if !strings.Contains(combined, "blocked") && !strings.Contains(combined, "Blocked") {
		t.Errorf("expected 'blocked' in output:\n%s", combined)
	}
}

// TestRunHook_SonarQube_HotspotsBlock verifies that when hotspots are returned
// and SONAR_BLOCK_ON_HOTSPOTS=true, the commit is blocked with exit 1.
func TestRunHook_SonarQube_HotspotsBlock(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()
	binDir := fakeSonarScanner(t)

	sonarSrv := sonarServer(t, nil, 2) // 2 hotspots, no regular issues
	aiSrv := sseServer(t, "")

	stageFile(t, repoDir, "auth.go", "package main\n")

	env := append(sonarEnv(homeDir, sonarSrv.URL, aiSrv.URL, binDir), "SONAR_BLOCK_ON_HOTSPOTS=true")
	res := run(t, repoDir, env, "run-hook")
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)

	if res.ExitCode != 1 {
		t.Fatalf("exit code: got %d, want 1 (hotspots + SONAR_BLOCK_ON_HOTSPOTS=true)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "hotspot") {
		t.Errorf("expected 'hotspot' in output:\n%s", combined)
	}
}

// TestRunHook_SonarQube_ScannerNotFound verifies that when sonar-scanner is
// not in PATH, the hook falls through to AI review (or exits 0 gracefully if
// AI gateway is also not configured).
func TestRunHook_SonarQube_ScannerNotFound(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()

	// Use an empty temp dir as PATH — no sonar-scanner binary exists there.
	emptyBinDir := t.TempDir()

	sonarSrv := sonarServer(t, nil, 0)
	aiSrv := sseServer(t, "")

	stageFile(t, repoDir, "main.go", "package main\n")

	env := sonarEnv(homeDir, sonarSrv.URL, aiSrv.URL, emptyBinDir)
	res := run(t, repoDir, env, "run-hook")
	t.Logf("stdout:\n%s", res.Stdout)
	t.Logf("stderr:\n%s", res.Stderr)

	// Missing scanner must never block commits — exit 0.
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (missing scanner must not block)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
	combined := res.Stdout + res.Stderr
	if !strings.Contains(combined, "not found") && !strings.Contains(combined, "scanner") {
		t.Errorf("expected scanner-not-found warning in output:\n%s", combined)
	}
}

// TestRunHook_SonarQube_NoGatewayConfigured verifies that when no gateway URL is set the
// hook warns and exits 0 (missing config must never block commits).
func TestRunHook_NoGatewayConfigured(t *testing.T) {
	repoDir := newTempGitRepo(t)
	homeDir := t.TempDir()

	stageFile(t, repoDir, "main.go", "package main\n")

	// No gateway URL or API key.
	env := []string{"HOME=" + homeDir, "ENABLE_AI_REVIEW=true"}
	res := run(t, repoDir, env, "run-hook")
	if res.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0 (unconfigured should not block)\n%s",
			res.ExitCode, res.Stdout+res.Stderr)
	}
}
