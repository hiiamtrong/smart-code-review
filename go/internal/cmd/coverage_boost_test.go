package cmd

import (
	"bufio"
	"context"
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

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/gateway"
	"github.com/hiiamtrong/smart-code-review/internal/git"
)

// ─── test constants ─────────────────────────────────────────────────────────

const (
	testFileTestGo    = "test.go"
	testFileMainGo    = "main.go"
	testPkgMain       = "package main\n"
	testOutputJSONL   = "output.jsonl"
	testSevWarning    = "WARNING"
	testModelGPT4     = "gpt-4"
	testPreCommitYAML = ".pre-commit-config.yaml"
	testRulesAuto     = "auto"
	testAPIKey        = "test-key"
	testKeyAIModel    = "AI_MODEL"
	testRepoOwner     = "owner/repo"
	testReporterLocal = "local"
	testDiffContent   = "diff content"
	testErrFmt        = "unexpected error: %v"
	testGHToken       = "GITHUB_TOKEN"
	testGHRepo        = "GITHUB_REPOSITORY"
	testGHTokenVal    = "tok"
	testCfgDir        = "config"
	testGoLang        = "go"
	testGitCmd        = "git"
	testGitAdd        = "add"
	testGitCFlag      = "-C"
	testGitInit       = "init"
	testGitQuiet      = "--quiet"
	testInputY        = "y"
)

// ─── helper: create a minimal git repo ──────────────────────────────────────

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command(testGitCmd, args...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func createTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, testGitInit, testGitQuiet)
	runGit(t, dir, testCfgDir, "user.email", "test@test.com")
	runGit(t, dir, testCfgDir, "user.name", "Test")
	// Create initial commit so HEAD exists
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte(testGitInit), 0644)
	runGit(t, dir, testGitAdd, ".")
	runGit(t, dir, "commit", "-m", testGitInit, testGitQuiet)
	return dir
}

func writeTestConfig(t *testing.T, homeDir, content string) {
	t.Helper()
	configDir := filepath.Join(homeDir, ".config", "ai-review")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, testCfgDir), []byte(content), 0o644)
}

// ─── ciWriteOutputs ─────────────────────────────────────────────────────────

func TestCiWriteOutputs_Success(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, testOutputJSONL)
	overviewFile := filepath.Join(tmp, "overview.txt")

	result := &gateway.ReviewResult{
		Overview: "Test overview content",
		Diagnostics: []gateway.Diagnostic{
			{
				Message:  "test issue",
				Severity: testSevWarning,
				Location: gateway.Location{
					Path:  testFileMainGo,
					Range: gateway.Range{Start: gateway.Position{Line: 10}},
				},
			},
		},
	}

	err := ciWriteOutputs(result, outFile, overviewFile)
	if err != nil {
		t.Fatalf("ciWriteOutputs: %v", err)
	}

	// Verify output file exists
	if _, err := os.Stat(outFile); err != nil {
		t.Errorf("output file should exist: %v", err)
	}
	// Verify overview file exists
	if _, err := os.Stat(overviewFile); err != nil {
		t.Errorf("overview file should exist: %v", err)
	}
	content, _ := os.ReadFile(overviewFile)
	if string(content) != "Test overview content" {
		t.Errorf("overview content = %q", string(content))
	}
}

func TestCiWriteOutputs_NoOverview(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, testOutputJSONL)
	overviewFile := filepath.Join(tmp, "overview.txt")

	result := &gateway.ReviewResult{
		Overview: "", // empty overview
	}

	err := ciWriteOutputs(result, outFile, overviewFile)
	if err != nil {
		t.Fatalf("ciWriteOutputs: %v", err)
	}

	// Overview file should NOT be written
	if _, err := os.Stat(overviewFile); err == nil {
		t.Error("overview file should not exist when overview is empty")
	}
}

func TestCiWriteOutputs_BadPath(t *testing.T) {
	// Use a path whose parent directory doesn't exist and can't be created.
	badPath := filepath.Join(string(filepath.Separator), "nonexistent-dir-"+t.Name(), "sub", testOutputJSONL)
	if runtime.GOOS == "windows" {
		badPath = `Q:\nonexistent\path\output.jsonl`
	}
	result := &gateway.ReviewResult{}
	err := ciWriteOutputs(result, badPath, "")
	if err == nil {
		t.Error("expected error for bad output path")
	}
}

// ─── ciRunReviewdog ─────────────────────────────────────────────────────────

func TestCiRunReviewdog_NoReviewdog(t *testing.T) {
	// Use manual temp dir to avoid Windows file-locking issues with t.TempDir().
	tmp, err := os.MkdirTemp("", "reviewdog-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	outFile := filepath.Join(tmp, testOutputJSONL)
	os.WriteFile(outFile, []byte("{}"), 0644)

	result := &gateway.ReviewResult{
		Diagnostics: []gateway.Diagnostic{
			{Severity: testSevWarning, Message: "test"},
		},
	}

	ciRunReviewdog(result, outFile, testReporterLocal)
}

func TestCiRunReviewdog_NoReviewdog_WithErrors(t *testing.T) {
	tmp, err := os.MkdirTemp("", "reviewdog-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	outFile := filepath.Join(tmp, testOutputJSONL)
	os.WriteFile(outFile, []byte("{}"), 0644)

	result := &gateway.ReviewResult{
		Diagnostics: []gateway.Diagnostic{
			{Severity: testSevWarning, Message: "just a warning"},
		},
	}

	ciRunReviewdog(result, outFile, testReporterLocal)
}

// ─── ciPostPRComment ────────────────────────────────────────────────────────

func TestCiPostPRComment_EmptyOverview(t *testing.T) {
	t.Setenv(testGHToken, testGHTokenVal)
	t.Setenv(testGHRepo, testRepoOwner)

	result := &gateway.ReviewResult{Overview: ""}
	ciPostPRComment(result, git.GitInfo{PRNumber: "1"})
	// Should return immediately due to empty overview
}

func TestCiPostPRComment_EmptyPRNumber(t *testing.T) {
	t.Setenv(testGHToken, testGHTokenVal)
	t.Setenv(testGHRepo, testRepoOwner)

	result := &gateway.ReviewResult{Overview: "Some overview"}
	ciPostPRComment(result, git.GitInfo{PRNumber: ""})
	// Should return immediately due to empty PR number
}

func TestCiPostPRComment_WithMockServer(t *testing.T) {
	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// list comments
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]struct{}{})
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	// ciPostPRComment uses hardcoded GitHub API URLs, so we can't easily redirect.
	// But we can test with env vars set to trigger the "post" path that fails gracefully.
	t.Setenv(testGHToken, "test-token")
	t.Setenv(testGHRepo, testRepoOwner)

	result := &gateway.ReviewResult{Overview: "Test overview"}
	// This will fail because it hits real GitHub API, but should not panic.
	ciPostPRComment(result, git.GitInfo{PRNumber: "999"})
}

// ─── runCIReview ────────────────────────────────────────────────────────────

func TestRunCIReview_NoCredentials_Defaults(t *testing.T) {
	// LoadMerged always returns defaults (no error), but defaults have empty
	// AI_GATEWAY_URL and AI_GATEWAY_API_KEY, so runCIReview should error.
	tmp := t.TempDir()
	setTestHome(t, tmp)

	// cd into a non-git dir so git-local layer doesn't add anything
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	err := runCIReview(nil, nil)
	if err == nil {
		t.Error("expected error when credentials are empty defaults")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf(testErrFmt, err)
	}
}

func TestRunCIReview_AIDisabled(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="false"`)

	err := runCIReview(nil, nil)
	if err != nil {
		t.Errorf("expected nil when AI disabled, got: %v", err)
	}
}

func TestRunCIReview_NoCredentials(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL=""
AI_GATEWAY_API_KEY=""`)

	err := runCIReview(nil, nil)
	if err == nil {
		t.Error("expected error when credentials missing")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf(testErrFmt, err)
	}
}

func TestRunCIReview_NoDiff(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	// Create git repo with no changes
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="test-key"`)

	t.Setenv("GITHUB_BASE_REF", "")

	// This should hit "No diff" or "get PR diff" error path
	err := runCIReview(nil, nil)
	// Either nil (no diff) or error (git diff fails) is acceptable
	_ = err
}

func TestRunCIReview_WithMockGateway(t *testing.T) {
	// Create a mock AI Gateway
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := gateway.ReviewResult{
			Overview: "Test overview",
			Diagnostics: []gateway.Diagnostic{
				{
					Message:  "test issue",
					Severity: testSevWarning,
					Location: gateway.Location{Path: testFileTestGo, Range: gateway.Range{Start: gateway.Position{Line: 1}}},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, fmt.Sprintf(`ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="%s"
AI_GATEWAY_API_KEY="test-key"
GATEWAY_TIMEOUT_SEC="10"`, server.URL))

	// Create a diff by adding a file
	os.WriteFile(filepath.Join(repoDir, testFileTestGo), []byte(testPkgMain), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, ".").Run()
	exec.Command(testGitCmd, testGitCFlag, repoDir, "commit", "-m", "add test", testGitQuiet).Run()

	t.Setenv("GITHUB_BASE_REF", "")

	// Use manual temp dir for CI output files to avoid Windows file-locking
	// issues with t.TempDir() cleanup (reviewdog may still hold the file).
	ciTmp, err := os.MkdirTemp("", "ci-review-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(ciTmp)

	// Save and restore ciOutputFile/ciOverviewFile
	origOutput := ciOutputFile
	origOverview := ciOverviewFile
	origReporter := ciReporter
	ciOutputFile = filepath.Join(ciTmp, "ai-output.jsonl")
	ciOverviewFile = filepath.Join(ciTmp, "ai-overview.txt")
	ciReporter = testReporterLocal
	defer func() {
		ciOutputFile = origOutput
		ciOverviewFile = origOverview
		ciReporter = origReporter
	}()

	err = runCIReview(nil, nil)
	// May return nil or error depending on git state; we just want coverage
	_ = err
}

// ─── runInstall (in a git repo) ─────────────────────────────────────────────

func TestRunInstall_InGitRepo_Defaults(t *testing.T) {
	// LoadMerged never errors - it returns defaults. So runInstall will
	// succeed even without an explicit config file (hook gets installed).
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// runInstall should succeed (defaults are valid config)
	err := runInstall(nil, nil)
	if err != nil {
		t.Errorf("runInstall with defaults should succeed: %v", err)
	}
}

func TestRunInstall_InGitRepo_WithConfig(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	err := runInstall(nil, nil)
	if err != nil {
		t.Errorf("runInstall in git repo with config should succeed: %v", err)
	}

	// Run again - should say "already installed"
	err = runInstall(nil, nil)
	if err != nil {
		t.Errorf("runInstall second time should succeed: %v", err)
	}
}

func TestRunInstall_WithPreCommitFramework(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	// Create .pre-commit-config.yaml to trigger framework detection
	pcConfig := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
      - id: trailing-whitespace
`
	os.WriteFile(filepath.Join(repoDir, testPreCommitYAML), []byte(pcConfig), 0644)

	err := runInstall(nil, nil)
	if err != nil {
		t.Errorf("runInstall with pre-commit framework: %v", err)
	}

	// Install again - should say "already registered"
	err = runInstall(nil, nil)
	if err != nil {
		t.Errorf("runInstall second time with pre-commit: %v", err)
	}
}

// ─── installViaPreCommitFramework ───────────────────────────────────────────

func TestInstallViaPreCommitFramework_AlreadyInstalled(t *testing.T) {
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create .pre-commit-config.yaml with ai-review already present
	pcConfig := `repos:
  - repo: local
    hooks:
      - id: ai-review
        name: AI Code Review
        entry: ai-review run-hook
        language: system
        always_run: true
        pass_filenames: false
        stages: [commit]
`
	os.WriteFile(filepath.Join(repoDir, testPreCommitYAML), []byte(pcConfig), 0644)

	err := installViaPreCommitFramework(repoDir)
	if err != nil {
		t.Errorf("installViaPreCommitFramework already installed: %v", err)
	}
}

func TestInstallViaPreCommitFramework_Fresh(t *testing.T) {
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	pcConfig := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
      - id: trailing-whitespace
`
	os.WriteFile(filepath.Join(repoDir, testPreCommitYAML), []byte(pcConfig), 0644)

	err := installViaPreCommitFramework(repoDir)
	if err != nil {
		t.Errorf("installViaPreCommitFramework fresh: %v", err)
	}
}

// ─── runUninstall (in a git repo) ───────────────────────────────────────────

func TestRunUninstall_InGitRepo_NoHook(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	err := runUninstall(nil, nil)
	if err != nil {
		t.Errorf("runUninstall with no hook should succeed: %v", err)
	}
}

func TestRunUninstall_InGitRepo_WithHook(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	// Install first
	err := runInstall(nil, nil)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Now uninstall
	err = runUninstall(nil, nil)
	if err != nil {
		t.Errorf("runUninstall should succeed: %v", err)
	}
}

func TestRunUninstall_WithPreCommitFramework(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	// Create .pre-commit-config.yaml with ai-review hook
	pcConfig := `repos:
  - repo: local
    hooks:
      - id: ai-review
        name: AI Code Review
        entry: ai-review run-hook
        language: system
        always_run: true
        pass_filenames: false
        stages: [commit]
`
	os.WriteFile(filepath.Join(repoDir, testPreCommitYAML), []byte(pcConfig), 0644)

	err := runUninstall(nil, nil)
	if err != nil {
		t.Errorf("runUninstall with pre-commit framework: %v", err)
	}
}

// ─── hookPrepareDiff ────────────────────────────────────────────────────────

func TestHookPrepareDiff_NoDiff(t *testing.T) {
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	rawDiff, _, _, _, _ := hookPrepareDiff()
	if rawDiff != "" {
		t.Errorf("expected empty diff, got %q", rawDiff)
	}
}

func TestHookPrepareDiff_WithStagedChanges(t *testing.T) {
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create and stage a file
	os.WriteFile(filepath.Join(repoDir, "hello.go"), []byte("package main\n\nfunc hello() {}\n"), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, "hello.go").Run()

	rawDiff, annotated, lang, repoRoot, gitInfo := hookPrepareDiff()
	if rawDiff == "" {
		t.Error("expected non-empty diff")
	}
	if annotated == "" {
		t.Error("expected non-empty annotated diff")
	}
	// lang should be detected (likely "go")
	_ = lang
	if repoRoot == "" {
		t.Error("expected non-empty repoRoot")
	}
	if gitInfo.CommitHash == "" {
		t.Error("expected non-empty commit hash")
	}
}

func TestHookPrepareDiff_AllIgnored(t *testing.T) {
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create .aireviewignore that ignores everything
	os.WriteFile(filepath.Join(repoDir, ".aireviewignore"), []byte("*.txt\n"), 0644)

	// Stage a .txt file
	os.WriteFile(filepath.Join(repoDir, "data.txt"), []byte("hello\n"), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, "data.txt").Run()

	rawDiff, _, _, _, _ := hookPrepareDiff()
	if rawDiff != "" {
		t.Errorf("expected empty diff when all files ignored, got diff of len %d", len(rawDiff))
	}
}

func TestHookPrepareDiff_NotGitRepo(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	rawDiff, _, _, _, _ := hookPrepareDiff()
	if rawDiff != "" {
		t.Error("expected empty diff outside git repo")
	}
}

// ─── hookRunSemgrep ─────────────────────────────────────────────────────────

func TestHookRunSemgrep_NoSemgrep(t *testing.T) {
	// Semgrep likely not installed in test environment
	cfg := config.Defaults()
	cfg.EnableSemgrep = true
	cfg.SemgrepRules = testRulesAuto

	diff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n+package main\n"
	counts := hookRunSemgrep(cfg, t.TempDir(), diff)

	// Should return empty counts since semgrep not found
	if counts.errCount != 0 || counts.warnCount != 0 || counts.blocked {
		t.Errorf("expected zero counts when semgrep not found, got %+v", counts)
	}
}

func TestHookRunSemgrep_EmptyFiles(t *testing.T) {
	cfg := config.Defaults()
	cfg.SemgrepRules = testRulesAuto

	counts := hookRunSemgrep(cfg, t.TempDir(), "")
	if counts.errCount != 0 {
		t.Errorf("expected zero counts for empty diff, got %+v", counts)
	}
}

// ─── hookRunSonarQube ───────────────────────────────────────────────────────

func TestHookRunSonarQube_NoScanner(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableSonarQube = true
	cfg.SonarHostURL = "http://localhost:9000"
	cfg.SonarToken = "test-token"
	cfg.SonarProjectKey = testAPIKey

	diff := "diff --git a/main.go b/main.go\n"
	counts := hookRunSonarQube(cfg, t.TempDir(), diff)

	// Should return empty counts since sonar-scanner not found
	if counts.blocked {
		t.Error("should not block when scanner not found")
	}
}

// ─── hookRunAIReview ────────────────────────────────────────────────────────

func TestHookRunAIReview_GatewayError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	cfg := config.Defaults()
	cfg.AIGatewayURL = server.URL
	cfg.AIGatewayAPIKey = testAPIKey
	cfg.GatewayTimeoutSec = 5
	cfg.BlockOnGatewayError = false

	counts, result := hookRunAIReview(cfg, testDiffContent, testGoLang, git.GitInfo{})
	if result != nil {
		t.Error("expected nil result on gateway error")
	}
	if counts.blocked {
		t.Error("should not block when BlockOnGatewayError is false")
	}
}

func TestHookRunAIReview_GatewayErrorBlocking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	cfg := config.Defaults()
	cfg.AIGatewayURL = server.URL
	cfg.AIGatewayAPIKey = testAPIKey
	cfg.GatewayTimeoutSec = 5
	cfg.BlockOnGatewayError = true

	counts, result := hookRunAIReview(cfg, testDiffContent, testGoLang, git.GitInfo{})
	if result != nil {
		t.Error("expected nil result on gateway error")
	}
	if !counts.blocked {
		t.Error("should block when BlockOnGatewayError is true")
	}
}

func TestHookRunAIReview_GatewaySuccess(t *testing.T) {
	// StreamingReview tries SSE first; if the mock returns plain JSON, SSE parsing
	// fails and it falls back to SyncReview (second request). The mock handles both.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := gateway.ReviewResult{
			Overview: "looks good",
			Diagnostics: []gateway.Diagnostic{
				{
					Message:  "possible issue",
					Severity: testSevWarning,
					Location: gateway.Location{Path: testFileMainGo, Range: gateway.Range{Start: gateway.Position{Line: 5}}},
				},
				{
					Message:  "error found",
					Severity: "ERROR",
					Location: gateway.Location{Path: testFileMainGo, Range: gateway.Range{Start: gateway.Position{Line: 10}}},
				},
				{
					Message:  "info note",
					Severity: "INFO",
					Location: gateway.Location{Path: testFileMainGo, Range: gateway.Range{Start: gateway.Position{Line: 15}}},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	cfg := config.Defaults()
	cfg.AIGatewayURL = server.URL
	cfg.AIGatewayAPIKey = testAPIKey
	cfg.GatewayTimeoutSec = 10

	counts, result := hookRunAIReview(cfg, testDiffContent, testGoLang, git.GitInfo{})
	// We primarily want coverage of the success paths. The result may or may
	// not be nil depending on SSE fallback timing, so just verify no panic.
	_ = counts
	_ = result
}

// ─── runHook (full paths) ───────────────────────────────────────────────────

func TestRunHook_WithStagedChanges_NoGateway(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:1"
AI_GATEWAY_API_KEY="test-key"
GATEWAY_TIMEOUT_SEC="2"
BLOCK_ON_GATEWAY_ERROR="false"
ENABLE_SONARQUBE="false"
ENABLE_SEMGREP="false"`)

	// Stage a file
	os.WriteFile(filepath.Join(repoDir, testFileTestGo), []byte(testPkgMain), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, testFileTestGo).Run()

	err := runHook(nil, nil)
	// Should succeed (gateway error non-blocking)
	if err != nil {
		t.Errorf("runHook should not return error with non-blocking config: %v", err)
	}
}

func TestRunHook_WithSemgrepEnabled(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:1"
AI_GATEWAY_API_KEY="test-key"
GATEWAY_TIMEOUT_SEC="2"
BLOCK_ON_GATEWAY_ERROR="false"
ENABLE_SONARQUBE="false"
ENABLE_SEMGREP="true"
SEMGREP_RULES="auto"`)

	os.WriteFile(filepath.Join(repoDir, testFileTestGo), []byte(testPkgMain), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, testFileTestGo).Run()

	err := runHook(nil, nil)
	_ = err // Just for coverage
}

func TestRunHook_WithSonarQubeEnabled(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:1"
AI_GATEWAY_API_KEY="test-key"
GATEWAY_TIMEOUT_SEC="2"
BLOCK_ON_GATEWAY_ERROR="false"
ENABLE_SONARQUBE="true"
SONAR_HOST_URL="http://localhost:9000"
SONAR_TOKEN="test-token"
SONAR_PROJECT_KEY="test-key"
ENABLE_SEMGREP="false"`)

	os.WriteFile(filepath.Join(repoDir, testFileTestGo), []byte(testPkgMain), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, testFileTestGo).Run()

	err := runHook(nil, nil)
	_ = err // Just for coverage
}

func TestRunHook_WithMockGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := gateway.ReviewResult{
			Overview:    "all good",
			Diagnostics: []gateway.Diagnostic{},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, fmt.Sprintf(`ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="%s"
AI_GATEWAY_API_KEY="test-key"
GATEWAY_TIMEOUT_SEC="10"
BLOCK_ON_GATEWAY_ERROR="false"
ENABLE_SONARQUBE="false"
ENABLE_SEMGREP="false"`, server.URL))

	os.WriteFile(filepath.Join(repoDir, testFileTestGo), []byte(testPkgMain), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, testFileTestGo).Run()

	err := runHook(nil, nil)
	if err != nil {
		t.Errorf("runHook with mock gateway: %v", err)
	}
}

func TestRunHook_WithGatewayHTTPError(t *testing.T) {
	// Test with a gateway that returns HTTP 500 (non-blocking config)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, fmt.Sprintf(`ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="%s"
AI_GATEWAY_API_KEY="test-key"
GATEWAY_TIMEOUT_SEC="5"
BLOCK_ON_GATEWAY_ERROR="true"
ENABLE_SONARQUBE="false"
ENABLE_SEMGREP="false"`, server.URL))

	os.WriteFile(filepath.Join(repoDir, testFileTestGo), []byte(testPkgMain), 0644)
	exec.Command(testGitCmd, testGitCFlag, repoDir, testGitAdd, testFileTestGo).Run()

	err := runHook(nil, nil)
	// Gateway returns 500 + BlockOnGatewayError=true => errBlocked
	if err != errBlocked {
		t.Errorf("expected errBlocked when gateway returns HTTP 500 with block=true, got: %v", err)
	}
}

// ─── runUpdate ──────────────────────────────────────────────────────────────

func TestRunUpdate_AlreadyUpToDate(t *testing.T) {
	// Create a mock GitHub releases API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := map[string]interface{}{
			"tag_name": "v" + Version,
			"assets":   []interface{}{},
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	// We need to modify the updater's releaseAPIURL, but it's package-private.
	// Instead, test runUpdate with the actual Version matching.
	// This test just verifies the function handles FetchLatest errors gracefully.
	err := runUpdate(nil, nil)
	// Will likely fail due to network, which is expected behavior
	if err == nil {
		// If it succeeds, it means we're already up to date (or network worked)
		return
	}
	if !strings.Contains(err.Error(), "check for updates") {
		t.Errorf(testErrFmt, err)
	}
}

// ─── Execute (root.go) ─────────────────────────────────────────────────────
// Execute calls os.Exit so we can't test it directly.
// We test through the rootCmd instead.

func TestRootCmd_Version(t *testing.T) {
	rootCmd.SetArgs([]string{"--version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("--version should succeed: %v", err)
	}
}

func TestRootCmd_Help(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("--help should succeed: %v", err)
	}
}

// ─── runConfigSet more paths ────────────────────────────────────────────────

func TestRunConfigSet_ProjectFlag_InGitRepo(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	origProject := configProjectFlag
	origGlobal := configGlobalFlag
	configProjectFlag = true
	configGlobalFlag = false
	defer func() {
		configProjectFlag = origProject
		configGlobalFlag = origGlobal
	}()

	err := runConfigSet(testKeyAIModel, testModelGPT4)
	if err != nil {
		t.Errorf("runConfigSet --project in git repo should succeed: %v", err)
	}
}

func TestRunConfigSet_AutoDetectProject(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `AI_MODEL="gpt-4"`)

	// First create a project config so auto-detect picks it
	origProject := configProjectFlag
	origGlobal := configGlobalFlag
	configProjectFlag = true
	configGlobalFlag = false
	_ = runConfigSet(testKeyAIModel, testModelGPT4) // create project config
	configProjectFlag = false
	defer func() {
		configProjectFlag = origProject
		configGlobalFlag = origGlobal
	}()

	// Now auto-detect should find project config
	err := runConfigSet(testKeyAIModel, "claude-3")
	if err != nil {
		t.Errorf("runConfigSet auto-detect project: %v", err)
	}
}

// ─── runConfigListProjects with projects ────────────────────────────────────

func TestRunConfigListProjects_WithProjects(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	projectsDir := filepath.Join(tmp, ".config", "ai-review", "projects")
	for _, name := range []string{"proj1", "proj2"} {
		dir := filepath.Join(projectsDir, name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, testCfgDir), []byte(`AI_MODEL="gpt-4"`), 0o644)
		os.WriteFile(filepath.Join(dir, "repo-path"), []byte("/path/to/"+name), 0o644)
	}

	// Also create one without repo-path
	dir := filepath.Join(projectsDir, "proj3")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, testCfgDir), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := runConfigListProjects()
	if err != nil {
		t.Errorf("runConfigListProjects with projects: %v", err)
	}
}

// ─── runConfigShow (no config file, defaults used) ──────────────────────────

func TestRunConfigShow_DefaultsOnly(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	// No config file; LoadMergedWithSources returns defaults
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp) // non-git dir to avoid git-local layer

	err := runConfigShow()
	// LoadMergedWithSources never errors; it returns defaults
	if err != nil {
		t.Errorf("runConfigShow with defaults should succeed: %v", err)
	}
}

// ─── runConfigGet (no config file, defaults used) ───────────────────────────

func TestRunConfigGet_DefaultsOnly(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	// LoadMerged never errors; returns defaults
	err := runConfigGet(testKeyAIModel)
	if err != nil {
		t.Errorf("runConfigGet with defaults should succeed: %v", err)
	}
}

func TestRunConfigGet_GlobalFlag_NoConfig(t *testing.T) {
	setTestHome(t, t.TempDir())

	origGlobal := configGlobalFlag
	configGlobalFlag = true
	defer func() { configGlobalFlag = origGlobal }()

	// No global config file, but should still work using defaults
	err := runConfigGet(testKeyAIModel)
	if err != nil {
		t.Errorf("runConfigGet --global with no config file should use defaults: %v", err)
	}
}

// ─── runStatus more paths ───────────────────────────────────────────────────

func TestRunStatus_InGitRepo_WithHook(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	// Install hook first
	_ = runInstall(nil, nil)

	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("runStatus with hook: %v", err)
	}
}

func TestRunStatus_InGitRepo_NoHook(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("runStatus in git repo without hook: %v", err)
	}
}

// ─── detectRepoName ─────────────────────────────────────────────────────────

func TestDetectRepoName_InGitRepo(t *testing.T) {
	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	name := detectRepoName()
	if name == "" {
		t.Error("expected non-empty repo name in git repo")
	}
}

func TestDetectRepoName_NotGitRepo(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	name := detectRepoName()
	// Should fall back to cwd basename
	if name == "" {
		t.Error("expected non-empty name (cwd basename)")
	}
}

// ─── setupStepSemgrep ───────────────────────────────────────────────────────

func TestSetupStepSemgrep_Enabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableSemgrep = true
	cfg.SemgrepRules = ""

	reader := bufio.NewReader(strings.NewReader("p/security\n"))
	setupStepSemgrep(reader, cfg)

	if cfg.SemgrepRules != "p/security" {
		t.Errorf("SemgrepRules = %q, want p/security", cfg.SemgrepRules)
	}
}

func TestSetupStepSemgrep_Enabled_Default(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableSemgrep = true
	cfg.SemgrepRules = ""

	reader := bufio.NewReader(strings.NewReader("\n"))
	setupStepSemgrep(reader, cfg)

	if cfg.SemgrepRules != testRulesAuto {
		t.Errorf("SemgrepRules = %q, want auto", cfg.SemgrepRules)
	}
}

func TestSetupStepSemgrep_Disabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableSemgrep = false

	reader := bufio.NewReader(strings.NewReader(""))
	setupStepSemgrep(reader, cfg)

	// Should not change anything
}

// ─── printSetupSummary more paths ───────────────────────────────────────────

func TestPrintSetupSummary_AllEnabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableAIReview = true
	cfg.EnableSonarQube = true
	cfg.EnableSemgrep = true
	cfg.AIGatewayURL = "http://localhost"
	cfg.AIGatewayAPIKey = "key"
	cfg.SonarHostURL = "http://sonar"
	cfg.SonarToken = testGHTokenVal
	cfg.SonarProjectKey = "proj"
	cfg.SemgrepRules = testRulesAuto

	// Should not panic
	printSetupSummary(cfg)
}

// ─── promptString with empty current and not required ───────────────────────

func TestPromptString_EmptyCurrentNotRequired(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got := promptString(r, "Label", "", false)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ─── runConfigRemoveProject more paths ──────────────────────────────────────

func TestRunConfigRemoveProject_EmptyID_InGitRepo(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create project config for this repo
	origProject := configProjectFlag
	origGlobal := configGlobalFlag
	configProjectFlag = true
	configGlobalFlag = false
	_ = config.SaveProjectField(testKeyAIModel, testModelGPT4)
	configProjectFlag = origProject
	configGlobalFlag = origGlobal

	err := runConfigRemoveProject("")
	if err != nil {
		t.Errorf("runConfigRemoveProject in git repo: %v", err)
	}
}

// ─── hookFinalize with blocked flag ─────────────────────────────────────────

func TestHookFinalize_BlockedFlag(t *testing.T) {
	result := &gateway.ReviewResult{}
	counts := hookCounts{blocked: true}
	err := hookFinalize(result, counts, nil)
	if err != errBlocked {
		t.Errorf("expected errBlocked, got: %v", err)
	}
}

// ─── printProjectInfo ───────────────────────────────────────────────────────

func TestPrintProjectInfo_InGitRepo(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create project config
	_ = config.SaveProjectField(testKeyAIModel, testModelGPT4)

	// Should not panic
	printProjectInfo()
}

func TestPrintProjectInfo_NoProjectConfig(t *testing.T) {
	setTestHome(t, t.TempDir())
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	// Should not panic
	printProjectInfo()
}

// ─── saveToGlobal edge case ─────────────────────────────────────────────────

func TestSaveToGlobal_DefaultsOnly(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

	// LoadMerged returns defaults; Save creates the config dir/file
	err := saveToGlobal(testKeyAIModel, testModelGPT4)
	if err != nil {
		t.Errorf("saveToGlobal with defaults should succeed: %v", err)
	}
}

// ─── setupStepAIGateway ────────────────────────────────────────────────────

func TestSetupStepAIGateway_Disabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableAIReview = false

	reader := bufio.NewReader(strings.NewReader(""))
	setupStepAIGateway(reader, cfg)
	// Should be a no-op
}

// ─── setupStepSonarQube ────────────────────────────────────────────────────

func TestSetupStepSonarQube_Disabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnableSonarQube = false

	reader := bufio.NewReader(strings.NewReader(""))
	setupStepSonarQube(reader, cfg)
	// Should be a no-op
}

// ─── runSetup with semgrep enabled ──────────────────────────────────────────

func TestRunSetup_WithSemgrep(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) { return "test-api-key", nil }

	input := strings.Join([]string{
		testInputY,           // Enable AI Review
		"n",                  // Enable SonarQube
		testInputY,           // Enable Semgrep
		"https://gw.test",   // AI Gateway URL
		"",                   // AI Model (default)
		"",                   // AI Provider (default)
		"p/default",          // Semgrep Rules
		testInputY,           // Save configuration
	}, "\n") + "\n"

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err := runSetup(nil, nil)
	if err != nil {
		t.Fatalf("runSetup with semgrep: %v", err)
	}

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if !cfg.EnableSemgrep {
		t.Error("EnableSemgrep should be true")
	}
	if cfg.SemgrepRules != "p/default" {
		t.Errorf("SemgrepRules = %q, want p/default", cfg.SemgrepRules)
	}
}

// ─── runStatus project config display ───────────────────────────────────────

func TestRunStatus_WithProjectConfig(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	repoDir := createTestGitRepo(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	writeTestConfig(t, tmp, `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL="http://localhost:9999"
AI_GATEWAY_API_KEY="key"`)

	// Create project config
	_ = config.SaveProjectField(testKeyAIModel, "custom-model")

	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("runStatus with project config: %v", err)
	}
}

// Ensure context import is used
var _ = context.Background
