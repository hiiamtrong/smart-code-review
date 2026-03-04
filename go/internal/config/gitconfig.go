package config

import (
	"os/exec"
	"strings"
)

// gitLocalKeyMap maps config KEY names to their git-local aireview.* equivalents.
// Existing keys (sonarProjectKey, enableSonarQube) keep their original names
// for backwards compatibility.
var gitLocalKeyMap = map[string]string{
	"AI_GATEWAY_URL":                  "aireview.gatewayUrl",
	"AI_GATEWAY_API_KEY":              "aireview.gatewayApiKey",
	"AI_MODEL":                        "aireview.model",
	"AI_PROVIDER":                     "aireview.provider",
	"ENABLE_AI_REVIEW":                "aireview.enableAiReview",
	"ENABLE_SONARQUBE_LOCAL":          "aireview.enableSonarQube",
	"BLOCK_ON_GATEWAY_ERROR":          "aireview.blockOnGatewayError",
	"GATEWAY_TIMEOUT_SEC":             "aireview.gatewayTimeoutSec",
	"SONAR_HOST_URL":                  "aireview.sonarHostUrl",
	"SONAR_TOKEN":                     "aireview.sonarToken",
	"SONAR_PROJECT_KEY":               "aireview.sonarProjectKey",
	"SONAR_BLOCK_ON_HOTSPOTS":         "aireview.sonarBlockHotspots",
	"SONAR_FILTER_CHANGED_LINES_ONLY": "aireview.sonarFilterChanged",
}

// gitLocalConfigImpl reads a per-repo git config value using os/exec directly.
// This avoids an import cycle with internal/git. Returns "" on any error.
func gitLocalConfigImpl(key string) string {
	out, err := exec.Command("git", "config", "--local", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// loadGitLocalAll reads all aireview.* keys from git local config in a
// single exec call and returns a raw map keyed by the config KEY names
// (e.g. "AI_MODEL", not "aireview.model").
// Returns an empty map if not in a git repo or no keys are set.
func loadGitLocalAll() map[string]string {
	result := make(map[string]string)

	// Build reverse mapping: aireview.* → config KEY
	reverse := make(map[string]string, len(gitLocalKeyMap))
	for k, v := range gitLocalKeyMap {
		reverse[v] = k
	}

	// Batch read all local git config entries.
	out, err := exec.Command("git", "config", "--local", "--list").Output()
	if err != nil {
		return result
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		gitKey := strings.TrimSpace(parts[0])
		gitVal := strings.TrimSpace(parts[1])

		if configKey, ok := reverse[gitKey]; ok {
			result[configKey] = gitVal
		}
	}
	return result
}
