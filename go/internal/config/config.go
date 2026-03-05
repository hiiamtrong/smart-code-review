// Package config handles reading and writing the ai-review configuration file.
// The config file uses a shell-sourceable KEY="VALUE" format for backwards
// compatibility with existing bash-version installations.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Config holds all ai-review settings.
type Config struct {
	// AI Gateway
	AIGatewayURL    string
	AIGatewayAPIKey string

	// Model
	AIModel    string
	AIProvider string

	// Feature flags
	EnableAIReview  bool
	EnableSonarQube bool

	// Gateway behaviour
	BlockOnGatewayError bool
	GatewayTimeoutSec   int

	// SonarQube
	SonarHostURL       string
	SonarToken         string
	SonarProjectKey    string
	SonarBlockHotspots bool
	SonarFilterChanged bool

	// Semgrep
	EnableSemgrep bool
	SemgrepRules  string
}

// Defaults returns a Config populated with default values.
func Defaults() *Config {
	return &Config{
		AIModel:             "gemini-2.0-flash",
		AIProvider:          "google",
		EnableAIReview:      true,
		EnableSonarQube:     false,
		EnableSemgrep:       false,
		SemgrepRules:        "auto",
		BlockOnGatewayError: true,
		GatewayTimeoutSec:   120,
		SonarBlockHotspots:  true,
		SonarFilterChanged:  true,
	}
}

// ConfigDir returns the platform-appropriate config directory.
//
//	Unix:    $HOME/.config/ai-review/
//	Windows: %APPDATA%\ai-review\
func ConfigDir() string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "ai-review")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ai-review")
}

// FilePath returns the full path to the config file.
func FilePath() string {
	return filepath.Join(ConfigDir(), "config")
}

// Load reads the config file and returns a Config with full layered
// resolution (defaults ← global ← project ← git-local ← env).
//
// Deprecated: Use LoadMerged() directly for clarity.  Load now delegates
// to LoadMerged to ensure per-project config is always applied.
func Load() (*Config, error) {
	return LoadMerged()
}

// LoadWithRepoOverrides loads config with per-repo overrides.
//
// Deprecated: Use LoadMerged() which includes project config, git-local,
// and env layers automatically.
func LoadWithRepoOverrides() (*Config, error) {
	return LoadMerged()
}

// Save writes cfg back to the config file in shell KEY="VALUE" format.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path := FilePath()
	content := formatShellConfig(cfg)
	return os.WriteFile(path, []byte(content), 0600)
}

// GetField returns the string representation of a named config field.
func GetField(cfg *Config, key string) string {
	switch strings.ToUpper(key) {
	case "AI_GATEWAY_URL":
		return cfg.AIGatewayURL
	case "AI_GATEWAY_API_KEY":
		return cfg.AIGatewayAPIKey
	case "AI_MODEL":
		return cfg.AIModel
	case "AI_PROVIDER":
		return cfg.AIProvider
	case "ENABLE_AI_REVIEW":
		return boolToStr(cfg.EnableAIReview)
	case "ENABLE_SONARQUBE_LOCAL":
		return boolToStr(cfg.EnableSonarQube)
	case "BLOCK_ON_GATEWAY_ERROR":
		return boolToStr(cfg.BlockOnGatewayError)
	case "GATEWAY_TIMEOUT_SEC":
		return strconv.Itoa(cfg.GatewayTimeoutSec)
	case "SONAR_HOST_URL":
		return cfg.SonarHostURL
	case "SONAR_TOKEN":
		return cfg.SonarToken
	case "SONAR_PROJECT_KEY":
		return cfg.SonarProjectKey
	case "SONAR_BLOCK_ON_HOTSPOTS":
		return boolToStr(cfg.SonarBlockHotspots)
	case "SONAR_FILTER_CHANGED_LINES_ONLY":
		return boolToStr(cfg.SonarFilterChanged)
	case "ENABLE_SEMGREP":
		return boolToStr(cfg.EnableSemgrep)
	case "SEMGREP_RULES":
		return cfg.SemgrepRules
	default:
		return ""
	}
}

// SetField updates a named config field from a string value.
func SetField(cfg *Config, key, value string) error {
	switch strings.ToUpper(key) {
	case "AI_GATEWAY_URL":
		cfg.AIGatewayURL = value
	case "AI_GATEWAY_API_KEY":
		cfg.AIGatewayAPIKey = value
	case "AI_MODEL":
		cfg.AIModel = value
	case "AI_PROVIDER":
		cfg.AIProvider = value
	case "ENABLE_AI_REVIEW":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("ENABLE_AI_REVIEW must be true/false")
		}
		cfg.EnableAIReview = b
	case "ENABLE_SONARQUBE_LOCAL":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("ENABLE_SONARQUBE_LOCAL must be true/false")
		}
		cfg.EnableSonarQube = b
	case "BLOCK_ON_GATEWAY_ERROR":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("BLOCK_ON_GATEWAY_ERROR must be true/false")
		}
		cfg.BlockOnGatewayError = b
	case "GATEWAY_TIMEOUT_SEC":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("GATEWAY_TIMEOUT_SEC must be an integer")
		}
		cfg.GatewayTimeoutSec = n
	case "SONAR_HOST_URL":
		cfg.SonarHostURL = value
	case "SONAR_TOKEN":
		cfg.SonarToken = value
	case "SONAR_PROJECT_KEY":
		cfg.SonarProjectKey = value
	case "SONAR_BLOCK_ON_HOTSPOTS":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("SONAR_BLOCK_ON_HOTSPOTS must be true/false")
		}
		cfg.SonarBlockHotspots = b
	case "SONAR_FILTER_CHANGED_LINES_ONLY":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("SONAR_FILTER_CHANGED_LINES_ONLY must be true/false")
		}
		cfg.SonarFilterChanged = b
	case "ENABLE_SEMGREP":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("ENABLE_SEMGREP must be true/false")
		}
		cfg.EnableSemgrep = b
	case "SEMGREP_RULES":
		cfg.SemgrepRules = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// ─── internal helpers ────────────────────────────────────────────────────────

// parseShellConfig reads KEY="VALUE" (or KEY='VALUE') lines from r.
// Lines starting with # and blank lines are ignored.
func parseShellConfig(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes (single or double)
		val = strings.Trim(val, `"'`)
		result[key] = val
	}
	return result, scanner.Err()
}

func applyValues(cfg *Config, m map[string]string) {
	for k, v := range m {
		_ = SetField(cfg, k, v)
	}
}

func formatShellConfig(cfg *Config) string {
	var sb strings.Builder
	sb.WriteString("# AI Review Configuration\n")
	sb.WriteString("# Generated by ai-review — do not edit manually while ai-review is running\n\n")

	write := func(key, val string) {
		sb.WriteString(fmt.Sprintf("%s=%q\n", key, val))
	}
	writeBool := func(key string, val bool) {
		write(key, boolToStr(val))
	}
	writeInt := func(key string, val int) {
		write(key, strconv.Itoa(val))
	}

	write("AI_GATEWAY_URL", cfg.AIGatewayURL)
	write("AI_GATEWAY_API_KEY", cfg.AIGatewayAPIKey)
	write("AI_MODEL", cfg.AIModel)
	write("AI_PROVIDER", cfg.AIProvider)
	writeBool("ENABLE_AI_REVIEW", cfg.EnableAIReview)
	writeBool("ENABLE_SONARQUBE_LOCAL", cfg.EnableSonarQube)
	writeBool("BLOCK_ON_GATEWAY_ERROR", cfg.BlockOnGatewayError)
	writeInt("GATEWAY_TIMEOUT_SEC", cfg.GatewayTimeoutSec)
	write("SONAR_HOST_URL", cfg.SonarHostURL)
	write("SONAR_TOKEN", cfg.SonarToken)
	write("SONAR_PROJECT_KEY", cfg.SonarProjectKey)
	writeBool("SONAR_BLOCK_ON_HOTSPOTS", cfg.SonarBlockHotspots)
	writeBool("SONAR_FILTER_CHANGED_LINES_ONLY", cfg.SonarFilterChanged)
	writeBool("ENABLE_SEMGREP", cfg.EnableSemgrep)
	write("SEMGREP_RULES", cfg.SemgrepRules)

	return sb.String()
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

