package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hiiamtrong/smart-code-review/internal/gateway"
	"github.com/hiiamtrong/smart-code-review/internal/git"
)

const (
	testNotSet      = "(not set)"
	testConfigDir   = ".config"
	testAppName     = "ai-review"
	testModelClaude3 = "claude-3"
)

// ─── formatConfigValue ───────────────────────────────────────────────────────

func TestFormatConfigValue_Sensitive(t *testing.T) {
	got := formatConfigValue("AI_GATEWAY_API_KEY", "my-secret")
	if got != "****" {
		t.Errorf("formatConfigValue(AI_GATEWAY_API_KEY) = %q, want %q", got, "****")
	}
}

func TestFormatConfigValue_SensitiveSonarToken(t *testing.T) {
	got := formatConfigValue("SONAR_TOKEN", "tok-123")
	if got != "****" {
		t.Errorf("formatConfigValue(SONAR_TOKEN) = %q, want %q", got, "****")
	}
}

func TestFormatConfigValue_SensitiveEmpty(t *testing.T) {
	got := formatConfigValue("AI_GATEWAY_API_KEY", "")
	if got != testNotSet {
		t.Errorf("formatConfigValue(AI_GATEWAY_API_KEY, \"\") = %q, want %q", got, testNotSet)
	}
}

func TestFormatConfigValue_NonSensitive(t *testing.T) {
	got := formatConfigValue("AI_MODEL", "gpt-4")
	if got != "gpt-4" {
		t.Errorf("formatConfigValue(AI_MODEL) = %q, want %q", got, "gpt-4")
	}
}

func TestFormatConfigValue_NonSensitiveEmpty(t *testing.T) {
	got := formatConfigValue("AI_MODEL", "")
	if got != testNotSet {
		t.Errorf("formatConfigValue(AI_MODEL, \"\") = %q, want %q", got, testNotSet)
	}
}

// ─── hookFinalize ───────────────────────────────────────────────────────────

func TestHookFinalize_NoIssues(t *testing.T) {
	result := &gateway.ReviewResult{}
	counts := hookCounts{}
	err := hookFinalize(result, counts, nil)
	if err != nil {
		t.Errorf("hookFinalize with no issues should return nil, got: %v", err)
	}
}

func TestHookFinalize_WithErrors(t *testing.T) {
	result := &gateway.ReviewResult{}
	counts := hookCounts{errCount: 2}
	err := hookFinalize(result, counts, nil)
	if err == nil {
		t.Error("hookFinalize with errors should return errBlocked")
	}
	if err != errBlocked {
		t.Errorf("expected errBlocked, got: %v", err)
	}
}

func TestHookFinalize_WarningsOnly(t *testing.T) {
	result := &gateway.ReviewResult{}
	counts := hookCounts{warnCount: 3}
	err := hookFinalize(result, counts, nil)
	if err != nil {
		t.Errorf("hookFinalize with warnings only should return nil, got: %v", err)
	}
}

func TestHookFinalize_InfoOnly(t *testing.T) {
	result := &gateway.ReviewResult{}
	counts := hookCounts{infoCount: 5}
	err := hookFinalize(result, counts, nil)
	if err != nil {
		t.Errorf("hookFinalize with info only should return nil, got: %v", err)
	}
}

func TestHookFinalize_WithOverview(t *testing.T) {
	result := &gateway.ReviewResult{Overview: "This is a review overview."}
	counts := hookCounts{warnCount: 1}
	err := hookFinalize(result, counts, nil)
	if err != nil {
		t.Errorf("hookFinalize with overview should return nil, got: %v", err)
	}
}

func TestHookFinalize_NilResult(t *testing.T) {
	counts := hookCounts{}
	err := hookFinalize(nil, counts, nil)
	if err != nil {
		t.Errorf("hookFinalize with nil result should return nil, got: %v", err)
	}
}

// ─── runConfigShow (with config) ────────────────────────────────────────────

func TestRunConfigShow_WithConfig(t *testing.T) {
	// Create a temp dir with a valid config file so LoadMergedWithSources succeeds.
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := `AI_GATEWAY_URL="http://localhost:8080"
AI_GATEWAY_API_KEY="test-key"
AI_MODEL="gpt-4"
AI_PROVIDER="openai"
ENABLE_AI_REVIEW="true"
ENABLE_SONARQUBE="false"
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runConfigShow()
	if err != nil {
		t.Errorf("runConfigShow with valid config should succeed, got: %v", err)
	}
}

// ─── runConfigGet ───────────────────────────────────────────────────────────

func TestRunConfigGet_UnknownKey(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := runConfigGet("NONEXISTENT_KEY")
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestRunConfigGet_KnownKey(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := runConfigGet("AI_MODEL")
	if err != nil {
		t.Errorf("runConfigGet(AI_MODEL) should succeed, got: %v", err)
	}
}

func TestRunConfigGet_GlobalFlag(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	// Set the global flag temporarily.
	origGlobal := configGlobalFlag
	configGlobalFlag = true
	defer func() { configGlobalFlag = origGlobal }()

	err := runConfigGet("AI_MODEL")
	if err != nil {
		t.Errorf("runConfigGet --global should succeed, got: %v", err)
	}
}

func TestRunConfigGet_GlobalFlag_UnknownKey(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	origGlobal := configGlobalFlag
	configGlobalFlag = true
	defer func() { configGlobalFlag = origGlobal }()

	err := runConfigGet("NONEXISTENT")
	if err == nil {
		t.Error("expected error for unknown key with --global")
	}
}

// ─── runConfigSet ───────────────────────────────────────────────────────────

func TestRunConfigSet_GlobalFlag(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	origGlobal := configGlobalFlag
	configGlobalFlag = true
	defer func() { configGlobalFlag = origGlobal }()

	err := runConfigSet("AI_MODEL", testModelClaude3)
	if err != nil {
		t.Errorf("runConfigSet --global should succeed, got: %v", err)
	}
}

func TestRunConfigSet_InvalidKey(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	err := runConfigSet("INVALID_KEY", "value")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestRunConfigSet_AutoDetectGlobal(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	// Reset flags to default.
	origGlobal := configGlobalFlag
	origProject := configProjectFlag
	configGlobalFlag = false
	configProjectFlag = false
	defer func() {
		configGlobalFlag = origGlobal
		configProjectFlag = origProject
	}()

	// No project config exists, so this should fall back to global.
	err := runConfigSet("AI_MODEL", testModelClaude3)
	if err != nil {
		t.Errorf("runConfigSet auto-detect global should succeed, got: %v", err)
	}
}

// ─── runConfigListProjects ──────────────────────────────────────────────────

func TestRunConfigListProjects_Empty(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)

	err := runConfigListProjects()
	if err != nil {
		t.Errorf("runConfigListProjects should succeed, got: %v", err)
	}
}

// ─── runConfigRemoveProject ─────────────────────────────────────────────────

func TestRunConfigRemoveProject_NoRepo(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(tmp)

	// No project config exists → error expected.
	err := runConfigRemoveProject("")
	if err == nil {
		t.Error("expected error when removing project config outside git repo")
	}
}

func TestRunConfigRemoveProject_WithID(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	// Create projects dir with a project
	projectsDir := filepath.Join(tmp, testConfigDir, testAppName, "projects")
	projectDir := filepath.Join(projectsDir, "test-project")
	os.MkdirAll(projectDir, 0o755)
	os.WriteFile(filepath.Join(projectDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)
	os.WriteFile(filepath.Join(projectDir, "repo-path"), []byte("/some/path"), 0o644)

	err := runConfigRemoveProject("test-project")
	if err != nil {
		t.Errorf("runConfigRemoveProject should succeed, got: %v", err)
	}
}

// ─── runConfig dispatcher ───────────────────────────────────────────────────

func TestRunConfig_ShowSubcommand(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	// No args → show config.
	err := runConfig(nil, []string{})
	if err != nil {
		t.Errorf("runConfig() should succeed, got: %v", err)
	}
}

func TestRunConfig_GetSubcommand(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := runConfig(nil, []string{"get", "AI_MODEL"})
	if err != nil {
		t.Errorf("runConfig get AI_MODEL should succeed, got: %v", err)
	}
}

func TestRunConfig_SetSubcommand(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	origGlobal := configGlobalFlag
	configGlobalFlag = true
	defer func() { configGlobalFlag = origGlobal }()

	err := runConfig(nil, []string{"set", "AI_MODEL", testModelClaude3})
	if err != nil {
		t.Errorf("runConfig set should succeed, got: %v", err)
	}
}

func TestRunConfig_ListProjectsSubcommand(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)

	err := runConfig(nil, []string{"list-projects"})
	if err != nil {
		t.Errorf("runConfig list-projects should succeed, got: %v", err)
	}
}

func TestRunConfig_RemoveProjectSubcommand(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(tmp)

	// Expect error since no project config exists.
	err := runConfig(nil, []string{"remove-project"})
	if err == nil {
		t.Error("expected error for remove-project with no project")
	}
}

func TestRunConfig_RemoveProjectWithID(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	projectsDir := filepath.Join(tmp, testConfigDir, testAppName, "projects")
	projectDir := filepath.Join(projectsDir, "myproj")
	os.MkdirAll(projectDir, 0o755)
	os.WriteFile(filepath.Join(projectDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := runConfig(nil, []string{"remove-project", "myproj"})
	if err != nil {
		t.Errorf("runConfig remove-project myproj should succeed, got: %v", err)
	}
}

// ─── runUninstall ───────────────────────────────────────────────────────────

func TestRunUninstall_NotAGitRepo(t *testing.T) {
	setTestHome(t, t.TempDir())
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(tmp)

	err := runUninstall(nil, nil)
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}

// ─── runStatus (with config) ────────────────────────────────────────────────

func TestRunStatus_WithConfig(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	configContent := `AI_GATEWAY_URL="http://localhost:8080"
AI_GATEWAY_API_KEY="test-key"
`
	os.WriteFile(filepath.Join(configDir, "config"), []byte(configContent), 0o644)

	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("runStatus with config should return nil, got: %v", err)
	}
}

func TestRunStatus_WithIncompleteCredentials(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	configContent := `AI_GATEWAY_URL="http://localhost:8080"
AI_GATEWAY_API_KEY=""
`
	os.WriteFile(filepath.Join(configDir, "config"), []byte(configContent), 0o644)

	err := runStatus(nil, nil)
	if err != nil {
		t.Errorf("runStatus with incomplete creds should return nil, got: %v", err)
	}
}

// ─── runHook (early exits) ──────────────────────────────────────────────────

func TestRunHook_NoConfig(t *testing.T) {
	setTestHome(t, t.TempDir())

	err := runHook(nil, nil)
	if err != nil {
		t.Errorf("runHook with no config should return nil, got: %v", err)
	}
}

func TestRunHook_AIReviewDisabled(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	configContent := `AI_GATEWAY_URL="http://localhost:8080"
AI_GATEWAY_API_KEY="test-key"
ENABLE_AI_REVIEW="false"
`
	os.WriteFile(filepath.Join(configDir, "config"), []byte(configContent), 0o644)

	err := runHook(nil, nil)
	if err != nil {
		t.Errorf("runHook with AI review disabled should return nil, got: %v", err)
	}
}

func TestRunHook_NoCredentials(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	configContent := `ENABLE_AI_REVIEW="true"
AI_GATEWAY_URL=""
AI_GATEWAY_API_KEY=""
`
	os.WriteFile(filepath.Join(configDir, "config"), []byte(configContent), 0o644)

	err := runHook(nil, nil)
	if err != nil {
		t.Errorf("runHook with empty credentials should return nil, got: %v", err)
	}
}

// ─── ciPostPRComment (no env vars) ──────────────────────────────────────────

func TestCiPostPRComment_NoEnvVars(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	result := &gateway.ReviewResult{
		Overview: "Test overview",
	}

	// With no GITHUB_TOKEN/GITHUB_REPOSITORY, ciPostPRComment returns immediately.
	ciPostPRComment(result, git.GitInfo{PRNumber: "123"})
}

// ─── saveToGlobal ───────────────────────────────────────────────────────────

func TestSaveToGlobal_Success(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := saveToGlobal("AI_MODEL", testModelClaude3)
	if err != nil {
		t.Errorf("saveToGlobal should succeed, got: %v", err)
	}
}

func TestSaveToGlobal_InvalidKey(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	configDir := filepath.Join(tmp, testConfigDir, testAppName)
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config"), []byte(`AI_MODEL="gpt-4"`), 0o644)

	err := saveToGlobal("INVALID_KEY", "value")
	if err == nil {
		t.Error("expected error for invalid key")
	}
}

// ─── runConfigSet with --project flag ───────────────────────────────────────

func TestRunConfigSet_ProjectFlag_NoGitRepo(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(tmp)

	origProject := configProjectFlag
	origGlobal := configGlobalFlag
	configProjectFlag = true
	configGlobalFlag = false
	defer func() {
		configProjectFlag = origProject
		configGlobalFlag = origGlobal
	}()

	// Expect error because we're not in a git repo.
	err := runConfigSet("AI_MODEL", testModelClaude3)
	if err == nil {
		t.Error("expected error for --project flag outside git repo")
	}
}
