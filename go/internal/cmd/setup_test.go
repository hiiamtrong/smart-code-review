package cmd

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiiamtrong/smart-code-review/internal/config"
)

const (
	testGatewayURL = "https://gateway.example.com"
	testPromptLabel = "Enable?"
	msgRunSetup    = "runSetup: %v"
	msgLoadMerged  = "LoadMerged: %v"
)

// ─── promptBool ──────────────────────────────────────────────────────────────

func TestPromptBool_DefaultTrue_EmptyInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	if got := promptBool(r, testPromptLabel, true); !got {
		t.Error("expected true for empty input with default=true")
	}
}

func TestPromptBool_DefaultTrue_InputN(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("n\n"))
	if got := promptBool(r, testPromptLabel, true); got {
		t.Error("expected false for input 'n'")
	}
}

func TestPromptBool_DefaultFalse_EmptyInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	if got := promptBool(r, testPromptLabel, false); got {
		t.Error("expected false for empty input with default=false")
	}
}

func TestPromptBool_DefaultFalse_InputY(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("y\n"))
	if got := promptBool(r, testPromptLabel, false); !got {
		t.Error("expected true for input 'y'")
	}
}

func TestPromptBool_DefaultFalse_InputYes(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("yes\n"))
	if got := promptBool(r, testPromptLabel, false); !got {
		t.Error("expected true for input 'yes'")
	}
}

func TestPromptBool_CaseInsensitive(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("Y\n"))
	if got := promptBool(r, testPromptLabel, false); !got {
		t.Error("expected true for input 'Y'")
	}
}

// ─── promptInt ───────────────────────────────────────────────────────────────

func TestPromptInt_EmptyInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	if got := promptInt(r, "Timeout", 120); got != 120 {
		t.Errorf("expected 120, got %d", got)
	}
}

func TestPromptInt_ValidInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("60\n"))
	if got := promptInt(r, "Timeout", 120); got != 60 {
		t.Errorf("expected 60, got %d", got)
	}
}

func TestPromptInt_InvalidInput(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("abc\n"))
	if got := promptInt(r, "Timeout", 120); got != 120 {
		t.Errorf("expected default 120 for invalid input, got %d", got)
	}
}

// ─── promptString ────────────────────────────────────────────────────────────

func TestPromptString_EmptyInputReturnsCurrent(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	if got := promptString(r, "URL", "existing", true); got != "existing" {
		t.Errorf("expected 'existing', got %q", got)
	}
}

func TestPromptString_NewValueOverrides(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("new-value\n"))
	if got := promptString(r, "URL", "old", true); got != "new-value" {
		t.Errorf("expected 'new-value', got %q", got)
	}
}

// ─── promptStringRequired ────────────────────────────────────────────────────

func TestPromptStringRequired_FirstTry(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("https://example.com\n"))
	got := promptStringRequired(r, "URL", "")
	if got != "https://example.com" {
		t.Errorf("expected 'https://example.com', got %q", got)
	}
}

func TestPromptStringRequired_ExistingValueAccepted(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got := promptStringRequired(r, "URL", "https://existing.com")
	if got != "https://existing.com" {
		t.Errorf("expected existing value, got %q", got)
	}
}

func TestPromptStringRequired_RepromptsOnEmpty(t *testing.T) {
	// First line empty (no current value), second line has value
	r := bufio.NewReader(strings.NewReader("\nhttps://retry.com\n"))
	got := promptStringRequired(r, "URL", "")
	if got != "https://retry.com" {
		t.Errorf("expected 'https://retry.com' on retry, got %q", got)
	}
}

// ─── promptPassword / promptPasswordRequired ────────────────────────────────

func TestPromptPassword_NewValue(t *testing.T) {
	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) { return "new-secret", nil }

	got := promptPassword("API Key", "old-secret")
	if got != "new-secret" {
		t.Errorf("expected 'new-secret', got %q", got)
	}
}

func TestPromptPassword_EmptyKeepsCurrent(t *testing.T) {
	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) { return "", nil }

	got := promptPassword("API Key", "keep-me")
	if got != "keep-me" {
		t.Errorf("expected 'keep-me', got %q", got)
	}
}

func TestPromptPasswordRequired_RepromptsOnEmpty(t *testing.T) {
	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()

	calls := 0
	readPasswordFn = func() (string, error) {
		calls++
		if calls == 1 {
			return "", nil // first attempt empty
		}
		return "secret-on-retry", nil
	}

	got := promptPasswordRequired("Token", "")
	if got != "secret-on-retry" {
		t.Errorf("expected 'secret-on-retry', got %q", got)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls to readPasswordFn, got %d", calls)
	}
}

// ─── Integration: full wizard AI-only ────────────────────────────────────────

func TestRunSetup_AIOnly(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) { return "test-api-key", nil }

	// Simulate stdin: step1 (Y, n, n), step2 (URL, model, provider), summary (Y)
	input := strings.Join([]string{
		"y",    // Enable AI Review
		"n",    // Enable SonarQube
		"n",    // Enable Semgrep
		testGatewayURL, // AI Gateway URL
		"",                            // AI Model (accept default)
		"",                            // AI Provider (accept default)
		"y",                           // Save configuration
	}, "\n") + "\n"

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err := runSetup(nil, nil)
	if err != nil {
		t.Fatalf(msgRunSetup, err)
	}

	// Verify config was saved
	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf(msgLoadMerged, err)
	}
	if !cfg.EnableAIReview {
		t.Error("EnableAIReview should be true")
	}
	if cfg.EnableSonarQube {
		t.Error("EnableSonarQube should be false")
	}
	if cfg.AIGatewayURL != testGatewayURL {
		t.Errorf("AIGatewayURL = %q", cfg.AIGatewayURL)
	}
	if cfg.AIGatewayAPIKey != "test-api-key" {
		t.Errorf("AIGatewayAPIKey = %q", cfg.AIGatewayAPIKey)
	}
}

// ─── Integration: full wizard with SonarQube ─────────────────────────────────

func TestRunSetup_WithSonarQube(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()

	passwordCalls := 0
	readPasswordFn = func() (string, error) {
		passwordCalls++
		switch passwordCalls {
		case 1:
			return "api-key-123", nil
		case 2:
			return "sonar-token-456", nil
		default:
			return "", nil
		}
	}

	input := strings.Join([]string{
		"y",    // Enable AI Review
		"y",    // Enable SonarQube
		"n",    // Enable Semgrep
		testGatewayURL, // AI Gateway URL
		"custom-model",                // AI Model
		"openai",                      // AI Provider
		"https://sonar.example.com",   // SonarQube Host URL
		"my-project",                  // SonarQube Project Key
		"y",                           // Save configuration
	}, "\n") + "\n"

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err := runSetup(nil, nil)
	if err != nil {
		t.Fatalf(msgRunSetup, err)
	}

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf(msgLoadMerged, err)
	}
	if !cfg.EnableAIReview {
		t.Error("EnableAIReview should be true")
	}
	if !cfg.EnableSonarQube {
		t.Error("EnableSonarQube should be true")
	}
	if cfg.AIModel != "custom-model" {
		t.Errorf("AIModel = %q, want 'custom-model'", cfg.AIModel)
	}
	if cfg.AIProvider != "openai" {
		t.Errorf("AIProvider = %q, want 'openai'", cfg.AIProvider)
	}
	if cfg.SonarHostURL != "https://sonar.example.com" {
		t.Errorf("SonarHostURL = %q", cfg.SonarHostURL)
	}
	if cfg.SonarToken != "sonar-token-456" {
		t.Errorf("SonarToken = %q", cfg.SonarToken)
	}
	if cfg.SonarProjectKey != "my-project" {
		t.Errorf("SonarProjectKey = %q", cfg.SonarProjectKey)
	}
}

// ─── Integration: both disabled ──────────────────────────────────────────────

func TestRunSetup_BothDisabled(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	// No password prompts expected since both are disabled
	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) {
		t.Error("readPasswordFn should not be called when both features disabled")
		return "", nil
	}

	input := strings.Join([]string{
		"n", // Enable AI Review
		"n", // Enable SonarQube
		"n", // Enable Semgrep
		"y", // Save configuration
	}, "\n") + "\n"

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err := runSetup(nil, nil)
	if err != nil {
		t.Fatalf(msgRunSetup, err)
	}

	cfg, err := config.LoadMerged()
	if err != nil {
		t.Fatalf(msgLoadMerged, err)
	}
	if cfg.EnableAIReview {
		t.Error("EnableAIReview should be false")
	}
	if cfg.EnableSonarQube {
		t.Error("EnableSonarQube should be false")
	}
}

// ─── Integration: --project flag ─────────────────────────────────────────────

func TestRunSetup_ProjectFlag(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	// Create a git repo so SaveProjectField can determine project ID.
	repoDir := filepath.Join(dir, "repo")
	os.MkdirAll(repoDir, 0755)
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "--quiet")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644)
	runGit("add", ".")
	runGit("commit", "-m", "init", "--quiet")

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) { return "proj-key", nil }

	origFlag := setupProjectFlag
	setupProjectFlag = true
	defer func() { setupProjectFlag = origFlag }()

	input := strings.Join([]string{
		"n", // Enable AI Review
		"n", // Enable SonarQube
		"n", // Enable Semgrep
		"y", // Save configuration
	}, "\n") + "\n"

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err := runSetup(nil, nil)
	if err != nil {
		t.Fatalf("runSetup --project: %v", err)
	}

	projDir, _ := config.ProjectConfigDir()
	if projDir == "" {
		t.Fatal("ProjectConfigDir returned empty")
	}
	if _, err := os.Stat(filepath.Join(projDir, "config")); err != nil {
		t.Errorf("project config file should exist: %v", err)
	}
}

// ─── Integration: abort at summary ───────────────────────────────────────────

func TestRunSetup_AbortAtSummary(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	orig := readPasswordFn
	defer func() { readPasswordFn = orig }()
	readPasswordFn = func() (string, error) { return "test-key", nil }

	input := strings.Join([]string{
		"y",                           // Enable AI Review
		"n",                           // Enable SonarQube
		"n",                           // Enable Semgrep
		testGatewayURL, // AI Gateway URL
		"",                            // AI Model
		"",                            // AI Provider
		"n",                           // Save? NO
	}, "\n") + "\n"

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString(input)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	err := runSetup(nil, nil)
	if err != nil {
		t.Fatalf(msgRunSetup, err)
	}

	// Config file should NOT exist
	cfgPath := filepath.Join(dir, ".config", "ai-review", "config")
	if _, err := os.Stat(cfgPath); err == nil {
		t.Error("config file should not exist after abort")
	}
}
