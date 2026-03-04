package config

import (
	"fmt"
	"os"
	"strconv"
)

// ConfigSource tracks the origin of a config value.
type ConfigSource struct {
	Value  string
	Source string // "default", "global", "project", "git-local", "env"
}

// AllConfigKeys returns the canonical ordered list of config key names.
func AllConfigKeys() []string { return allConfigKeys }

var allConfigKeys = []string{
	"AI_GATEWAY_URL",
	"AI_GATEWAY_API_KEY",
	"AI_MODEL",
	"AI_PROVIDER",
	"ENABLE_AI_REVIEW",
	"ENABLE_SONARQUBE_LOCAL",
	"BLOCK_ON_GATEWAY_ERROR",
	"GATEWAY_TIMEOUT_SEC",
	"SONAR_HOST_URL",
	"SONAR_TOKEN",
	"SONAR_PROJECT_KEY",
	"SONAR_BLOCK_ON_HOTSPOTS",
	"SONAR_FILTER_CHANGED_LINES_ONLY",
}

// LoadMerged loads the fully merged config with resolution order:
//
//	defaults ← global ← project ← git-local ← env
//
// It replaces Load() and LoadWithRepoOverrides() as the primary entry point.
func LoadMerged() (*Config, error) {
	merged, _ := loadMergedRaw()

	cfg := Defaults()
	applyValues(cfg, merged)
	return cfg, nil
}

// LoadMergedWithSources returns the merged value and source label for every
// config key.  The map is keyed by config KEY name (e.g. "AI_MODEL").
func LoadMergedWithSources() (map[string]ConfigSource, error) {
	_, sources := loadMergedRaw()
	return sources, nil
}

// loadMergedRaw performs the actual layered merge and returns both the merged
// map and the per-key source labels.
func loadMergedRaw() (map[string]string, map[string]ConfigSource) {
	merged := DefaultsAsMap()
	sources := make(map[string]ConfigSource, len(allConfigKeys))
	for k, v := range merged {
		sources[k] = ConfigSource{Value: v, Source: "default"}
	}

	// Layer 1: global config file (no env overlay).
	if globalMap, err := LoadGlobalRaw(); err == nil && globalMap != nil {
		for k, v := range globalMap {
			merged[k] = v
			sources[k] = ConfigSource{Value: v, Source: "global"}
		}
	}

	// Layer 2: per-project config file (partial — only overridden keys).
	if projectMap, err := LoadProjectRaw(); err == nil && projectMap != nil {
		for k, v := range projectMap {
			merged[k] = v
			sources[k] = ConfigSource{Value: v, Source: "project"}
		}
	}

	// Layer 3: git config --local aireview.* (all 13 keys).
	for k, v := range loadGitLocalAll() {
		merged[k] = v
		sources[k] = ConfigSource{Value: v, Source: "git-local"}
	}

	// Layer 4: environment variables (highest priority).
	for k, v := range loadEnvRaw() {
		merged[k] = v
		sources[k] = ConfigSource{Value: v, Source: "env"}
	}

	return merged, sources
}

// defaultsAsMap returns the compiled defaults as a raw key-value map.
func DefaultsAsMap() map[string]string {
	return map[string]string{
		"AI_GATEWAY_URL":                  "",
		"AI_GATEWAY_API_KEY":              "",
		"AI_MODEL":                        "gemini-2.0-flash",
		"AI_PROVIDER":                     "google",
		"ENABLE_AI_REVIEW":                "true",
		"ENABLE_SONARQUBE_LOCAL":          "false",
		"BLOCK_ON_GATEWAY_ERROR":          "true",
		"GATEWAY_TIMEOUT_SEC":             "120",
		"SONAR_HOST_URL":                  "",
		"SONAR_TOKEN":                     "",
		"SONAR_PROJECT_KEY":               "",
		"SONAR_BLOCK_ON_HOTSPOTS":         "true",
		"SONAR_FILTER_CHANGED_LINES_ONLY": "true",
	}
}

// loadGlobalRaw reads the global config file as a raw key-value map, without
// applying env vars or git-local overrides.
func LoadGlobalRaw() (map[string]string, error) {
	path := FilePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open global config: %w", err)
	}
	defer f.Close()
	return parseShellConfig(f)
}

// loadEnvRaw reads all config-relevant environment variables and returns
// them as a raw key-value map.  Only non-empty env vars are included.
func loadEnvRaw() map[string]string {
	result := make(map[string]string)

	stringKeys := []string{
		"AI_GATEWAY_URL", "AI_GATEWAY_API_KEY",
		"AI_MODEL", "AI_PROVIDER",
		"SONAR_HOST_URL", "SONAR_TOKEN", "SONAR_PROJECT_KEY",
	}
	for _, k := range stringKeys {
		if v := os.Getenv(k); v != "" {
			result[k] = v
		}
	}

	boolKeys := []string{
		"ENABLE_AI_REVIEW", "ENABLE_SONARQUBE_LOCAL",
		"BLOCK_ON_GATEWAY_ERROR",
		"SONAR_BLOCK_ON_HOTSPOTS", "SONAR_FILTER_CHANGED_LINES_ONLY",
	}
	for _, k := range boolKeys {
		if v := os.Getenv(k); v != "" {
			// Validate it parses as a bool before adding.
			if _, err := strconv.ParseBool(v); err == nil {
				result[k] = v
			}
		}
	}

	if v := os.Getenv("GATEWAY_TIMEOUT_SEC"); v != "" {
		if _, err := strconv.Atoi(v); err == nil {
			result["GATEWAY_TIMEOUT_SEC"] = v
		}
	}

	return result
}
