package config

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testConfigSubdir  = ".config"
	testAppName       = "ai-review"
	testFromEnv       = "from-env"
	fmtLoadMergedErr  = "LoadMerged: %v"
)

func TestDefaultsAsMap_AllKeys(t *testing.T) {
	m := DefaultsAsMap()
	if len(m) != 13 {
		t.Errorf("DefaultsAsMap: got %d keys, want 13", len(m))
	}
	for _, k := range allConfigKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("DefaultsAsMap missing key %q", k)
		}
	}
}

func TestDefaultsAsMap_Values(t *testing.T) {
	m := DefaultsAsMap()
	cases := map[string]string{
		"AI_MODEL":             "gemini-2.0-flash",
		"AI_PROVIDER":          "google",
		"ENABLE_AI_REVIEW":     "true",
		"ENABLE_SONARQUBE_LOCAL": "false",
		"GATEWAY_TIMEOUT_SEC":  "120",
	}
	for k, want := range cases {
		if m[k] != want {
			t.Errorf("DefaultsAsMap[%q] = %q, want %q", k, m[k], want)
		}
	}
}

func TestLoadGlobalRaw_NoFile(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	m, err := LoadGlobalRaw()
	if err != nil {
		t.Fatalf("LoadGlobalRaw: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil map when no file, got %v", m)
	}
}

func TestLoadGlobalRaw_WithFile(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	configDir := filepath.Join(dir, testConfigSubdir, testAppName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}

	content := `AI_MODEL="gpt-4o"
AI_PROVIDER="openai"
ENABLE_AI_REVIEW="false"
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	m, err := LoadGlobalRaw()
	if err != nil {
		t.Fatalf("LoadGlobalRaw: %v", err)
	}
	if m["AI_MODEL"] != "gpt-4o" {
		t.Errorf("AI_MODEL: got %q, want gpt-4o", m["AI_MODEL"])
	}
	if m["ENABLE_AI_REVIEW"] != "false" {
		t.Errorf("ENABLE_AI_REVIEW: got %q, want false", m["ENABLE_AI_REVIEW"])
	}
}

func TestLoadEnvRaw_StringKeys(t *testing.T) {
	t.Setenv("AI_MODEL", "env-model")
	t.Setenv("AI_PROVIDER", "env-provider")

	m := loadEnvRaw()
	if m["AI_MODEL"] != "env-model" {
		t.Errorf("AI_MODEL: got %q", m["AI_MODEL"])
	}
	if m["AI_PROVIDER"] != "env-provider" {
		t.Errorf("AI_PROVIDER: got %q", m["AI_PROVIDER"])
	}
}

func TestLoadEnvRaw_BoolKeys(t *testing.T) {
	t.Setenv("ENABLE_AI_REVIEW", "false")
	t.Setenv("BLOCK_ON_GATEWAY_ERROR", "true")

	m := loadEnvRaw()
	if m["ENABLE_AI_REVIEW"] != "false" {
		t.Errorf("ENABLE_AI_REVIEW: got %q", m["ENABLE_AI_REVIEW"])
	}
	if m["BLOCK_ON_GATEWAY_ERROR"] != "true" {
		t.Errorf("BLOCK_ON_GATEWAY_ERROR: got %q", m["BLOCK_ON_GATEWAY_ERROR"])
	}
}

func TestLoadEnvRaw_InvalidBool(t *testing.T) {
	t.Setenv("ENABLE_AI_REVIEW", "not-a-bool")

	m := loadEnvRaw()
	if _, ok := m["ENABLE_AI_REVIEW"]; ok {
		t.Error("invalid bool should not be included in map")
	}
}

func TestLoadEnvRaw_TimeoutSec(t *testing.T) {
	t.Setenv("GATEWAY_TIMEOUT_SEC", "30")

	m := loadEnvRaw()
	if m["GATEWAY_TIMEOUT_SEC"] != "30" {
		t.Errorf("GATEWAY_TIMEOUT_SEC: got %q", m["GATEWAY_TIMEOUT_SEC"])
	}
}

func TestLoadEnvRaw_InvalidTimeout(t *testing.T) {
	t.Setenv("GATEWAY_TIMEOUT_SEC", "abc")

	m := loadEnvRaw()
	if _, ok := m["GATEWAY_TIMEOUT_SEC"]; ok {
		t.Error("invalid int should not be included in map")
	}
}

func TestLoadMerged_DefaultsOnly(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf(fmtLoadMergedErr, err)
	}
	if cfg.AIModel != "gemini-2.0-flash" {
		t.Errorf("default AI_MODEL: got %q", cfg.AIModel)
	}
	if !cfg.EnableAIReview {
		t.Error("default ENABLE_AI_REVIEW should be true")
	}
}

func TestLoadMerged_GlobalOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	configDir := filepath.Join(dir, testConfigSubdir, testAppName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := `AI_MODEL="claude-3.5-sonnet"
GATEWAY_TIMEOUT_SEC="60"
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf(fmtLoadMergedErr, err)
	}
	if cfg.AIModel != "claude-3.5-sonnet" {
		t.Errorf("AI_MODEL: got %q, want claude-3.5-sonnet", cfg.AIModel)
	}
	if cfg.GatewayTimeoutSec != 60 {
		t.Errorf("GatewayTimeoutSec: got %d, want 60", cfg.GatewayTimeoutSec)
	}
	// Default values should still be present.
	if !cfg.EnableAIReview {
		t.Error("ENABLE_AI_REVIEW should still default to true")
	}
}

func TestLoadMerged_EnvOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	configDir := filepath.Join(dir, testConfigSubdir, testAppName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := `AI_MODEL="from-file"
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_MODEL", testFromEnv)

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf(fmtLoadMergedErr, err)
	}
	if cfg.AIModel != testFromEnv {
		t.Errorf("env should override file: got %q", cfg.AIModel)
	}
}

func TestLoadMergedWithSources_Labels(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	configDir := filepath.Join(dir, testConfigSubdir, testAppName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := `AI_MODEL="from-global"
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_PROVIDER", testFromEnv)

	sources, err := LoadMergedWithSources()
	if err != nil {
		t.Fatalf("LoadMergedWithSources: %v", err)
	}

	if sources["AI_MODEL"].Source != "global" {
		t.Errorf("AI_MODEL source: got %q, want global", sources["AI_MODEL"].Source)
	}
	if sources["AI_PROVIDER"].Source != "env" {
		t.Errorf("AI_PROVIDER source: got %q, want env", sources["AI_PROVIDER"].Source)
	}
	if sources["ENABLE_AI_REVIEW"].Source != "default" {
		t.Errorf("ENABLE_AI_REVIEW source: got %q, want default", sources["ENABLE_AI_REVIEW"].Source)
	}
}

func TestLoadMerged_BooleanOverride(t *testing.T) {
	// This tests the critical boolean merge case: project sets
	// ENABLE_AI_REVIEW=false should override global's default true.
	dir := t.TempDir()
	setTestHome(t, dir)

	// Global config has ENABLE_AI_REVIEW=true (same as default).
	configDir := filepath.Join(dir, testConfigSubdir, testAppName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	globalContent := `ENABLE_AI_REVIEW="true"
AI_MODEL="global-model"
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(globalContent), 0600); err != nil {
		t.Fatal(err)
	}

	// We can't easily test the project layer here without a git repo,
	// so we test via env var override (same merge mechanism).
	t.Setenv("ENABLE_AI_REVIEW", "false")

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf(fmtLoadMergedErr, err)
	}
	if cfg.EnableAIReview {
		t.Error("ENABLE_AI_REVIEW should be false (env override)")
	}
	if cfg.AIModel != "global-model" {
		t.Errorf("AI_MODEL should still come from global: got %q", cfg.AIModel)
	}
}
