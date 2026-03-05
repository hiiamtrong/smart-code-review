package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	testModelClaude = "claude-opus-4"
	msgLoad         = "Load: %v"
)

func TestParseShellConfig(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "double quoted values",
			input: `AI_GATEWAY_URL="https://example.com"` + "\n" + `AI_MODEL="gemini-2.0-flash"`,
			want:  map[string]string{"AI_GATEWAY_URL": "https://example.com", "AI_MODEL": "gemini-2.0-flash"},
		},
		{
			name:  "single quoted values",
			input: `AI_PROVIDER='google'`,
			want:  map[string]string{"AI_PROVIDER": "google"},
		},
		{
			name:  "skip comment lines",
			input: "# This is a comment\nAI_MODEL=\"gpt-4\"",
			want:  map[string]string{"AI_MODEL": "gpt-4"},
		},
		{
			name:  "skip blank lines",
			input: "\n\nAI_MODEL=\"gpt-4\"\n\n",
			want:  map[string]string{"AI_MODEL": "gpt-4"},
		},
		{
			name:  "value with equals sign inside",
			input: `URL="https://host/path?key=value"`,
			want:  map[string]string{"URL": "https://host/path?key=value"},
		},
		{
			name:  "boolean value",
			input: `ENABLE_AI_REVIEW="true"`,
			want:  map[string]string{"ENABLE_AI_REVIEW": "true"},
		},
		{
			name:  "empty input",
			input: "",
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseShellConfig(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for k, want := range tt.want {
				if got[k] != want {
					t.Errorf("key %q: got %q, want %q", k, got[k], want)
				}
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.AIModel != "gemini-2.0-flash" {
		t.Errorf("AIModel default: got %q", cfg.AIModel)
	}
	if !cfg.EnableAIReview {
		t.Error("EnableAIReview should default to true")
	}
	if cfg.EnableSonarQube {
		t.Error("EnableSonarQube should default to false")
	}
	if !cfg.BlockOnGatewayError {
		t.Error("BlockOnGatewayError should default to true")
	}
	if cfg.GatewayTimeoutSec != 120 {
		t.Errorf("GatewayTimeoutSec default: got %d", cfg.GatewayTimeoutSec)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	// Override config path via environment
	setTestHome(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".config", "ai-review"), 0700); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults()
	cfg.AIGatewayURL = "https://gateway.example.com"
	cfg.AIGatewayAPIKey = "secret-key-123"
	cfg.AIModel = testModelClaude
	cfg.EnableSonarQube = true

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf(msgLoad, err)
	}

	if loaded.AIGatewayURL != cfg.AIGatewayURL {
		t.Errorf("AIGatewayURL: got %q, want %q", loaded.AIGatewayURL, cfg.AIGatewayURL)
	}
	if loaded.AIGatewayAPIKey != cfg.AIGatewayAPIKey {
		t.Errorf("AIGatewayAPIKey: got %q, want %q", loaded.AIGatewayAPIKey, cfg.AIGatewayAPIKey)
	}
	if loaded.AIModel != cfg.AIModel {
		t.Errorf("AIModel: got %q, want %q", loaded.AIModel, cfg.AIModel)
	}
	if loaded.EnableSonarQube != cfg.EnableSonarQube {
		t.Errorf("EnableSonarQube: got %v, want %v", loaded.EnableSonarQube, cfg.EnableSonarQube)
	}
}

func TestSetField(t *testing.T) {
	cfg := Defaults()

	tests := []struct {
		key, value string
		check      func() bool
	}{
		{"AI_GATEWAY_URL", "https://x.com", func() bool { return cfg.AIGatewayURL == "https://x.com" }},
		{"ENABLE_AI_REVIEW", "false", func() bool { return !cfg.EnableAIReview }},
		{"GATEWAY_TIMEOUT_SEC", "60", func() bool { return cfg.GatewayTimeoutSec == 60 }},
		{"BLOCK_ON_GATEWAY_ERROR", "false", func() bool { return !cfg.BlockOnGatewayError }},
		{"ENABLE_SEMGREP", "true", func() bool { return cfg.EnableSemgrep }},
		{"SEMGREP_RULES", "p/default", func() bool { return cfg.SemgrepRules == "p/default" }},
	}

	for _, tt := range tests {
		if err := SetField(cfg, tt.key, tt.value); err != nil {
			t.Errorf("SetField(%q, %q): %v", tt.key, tt.value, err)
			continue
		}
		if !tt.check() {
			t.Errorf("SetField(%q, %q): field not updated", tt.key, tt.value)
		}
	}
}

func TestSetFieldUnknownKey(t *testing.T) {
	cfg := Defaults()
	if err := SetField(cfg, "UNKNOWN_KEY", "value"); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestGetField_allKeys(t *testing.T) {
	cfg := Defaults()
	cfg.AIGatewayURL = "https://gw.example.com"
	cfg.AIGatewayAPIKey = "secret"
	cfg.AIModel = testModelClaude
	cfg.AIProvider = "anthropic"
	cfg.EnableAIReview = true
	cfg.EnableSonarQube = true
	cfg.BlockOnGatewayError = false
	cfg.GatewayTimeoutSec = 60
	cfg.SonarHostURL = "https://sonar.example.com"
	cfg.SonarToken = "sonar-tok"
	cfg.SonarProjectKey = "proj-key"
	cfg.SonarBlockHotspots = false
	cfg.SonarFilterChanged = false
	cfg.EnableSemgrep = true
	cfg.SemgrepRules = "p/security-audit"

	cases := []struct{ key, want string }{
		{"AI_GATEWAY_URL", "https://gw.example.com"},
		{"AI_GATEWAY_API_KEY", "secret"},
		{"AI_MODEL", testModelClaude},
		{"AI_PROVIDER", "anthropic"},
		{"ENABLE_AI_REVIEW", "true"},
		{"ENABLE_SONARQUBE_LOCAL", "true"},
		{"BLOCK_ON_GATEWAY_ERROR", "false"},
		{"GATEWAY_TIMEOUT_SEC", "60"},
		{"SONAR_HOST_URL", "https://sonar.example.com"},
		{"SONAR_TOKEN", "sonar-tok"},
		{"SONAR_PROJECT_KEY", "proj-key"},
		{"SONAR_BLOCK_ON_HOTSPOTS", "false"},
		{"SONAR_FILTER_CHANGED_LINES_ONLY", "false"},
		{"ENABLE_SEMGREP", "true"},
		{"SEMGREP_RULES", "p/security-audit"},
	}
	for _, tc := range cases {
		got := GetField(cfg, tc.key)
		if got != tc.want {
			t.Errorf("GetField(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestGetField_unknownKey(t *testing.T) {
	cfg := Defaults()
	if got := GetField(cfg, "DOES_NOT_EXIST"); got != "" {
		t.Errorf("GetField unknown key: got %q, want empty", got)
	}
}

func TestLoadFromEnvVarsOnly(t *testing.T) {
	// Simulate CI: no config file, credentials come from env vars.
	dir := t.TempDir()
	setTestHome(t, dir) // ensures no config file exists
	t.Setenv("AI_GATEWAY_URL", "https://ci-gateway.example.com")
	t.Setenv("AI_GATEWAY_API_KEY", "ci-secret")
	t.Setenv("AI_MODEL", testModelClaude)
	defer func() {
		os.Unsetenv("AI_GATEWAY_URL")
		os.Unsetenv("AI_GATEWAY_API_KEY")
		os.Unsetenv("AI_MODEL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with no config file should not error: %v", err)
	}
	if cfg.AIGatewayURL != "https://ci-gateway.example.com" {
		t.Errorf("AIGatewayURL = %q, want env value", cfg.AIGatewayURL)
	}
	if cfg.AIGatewayAPIKey != "ci-secret" {
		t.Errorf("AIGatewayAPIKey = %q, want env value", cfg.AIGatewayAPIKey)
	}
	if cfg.AIModel != testModelClaude {
		t.Errorf("AIModel = %q, want env value", cfg.AIModel)
	}
}

func TestEnvVarsOverrideFile(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, ".config", "ai-review"), 0700); err != nil {
		t.Fatal(err)
	}

	cfg := Defaults()
	cfg.AIGatewayURL = "https://file-gateway.example.com"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_GATEWAY_URL", "https://env-gateway.example.com")
	defer os.Unsetenv("AI_GATEWAY_URL")

	loaded, err := Load()
	if err != nil {
		t.Fatalf(msgLoad, err)
	}
	if loaded.AIGatewayURL != "https://env-gateway.example.com" {
		t.Errorf("env var should override file: got %q", loaded.AIGatewayURL)
	}
}

func TestGatewayTimeoutSecEnvVar(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)
	t.Setenv("GATEWAY_TIMEOUT_SEC", "30")

	cfg, err := Load()
	if err != nil {
		t.Fatalf(msgLoad, err)
	}
	if cfg.GatewayTimeoutSec != 30 {
		t.Errorf("GatewayTimeoutSec = %d, want 30", cfg.GatewayTimeoutSec)
	}
}

func TestConfigFilePermissions(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	cfg := Defaults()
	cfg.AIGatewayAPIKey = "secret"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(FilePath())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Errorf("config file permissions: got %o, want 0600", info.Mode().Perm())
	}
}
