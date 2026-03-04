package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectInfo describes a stored per-project config.
type ProjectInfo struct {
	ID         string
	RepoPath   string
	ConfigPath string
}

// ProjectID computes a stable, filesystem-safe identifier for a repo root path.
// It canonicalises the path (resolves symlinks, cleans separators) then returns
// the first 12 hex chars of its SHA-256 hash.
func ProjectID(repoRoot string) string {
	canonical, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		canonical = repoRoot
	}
	canonical = filepath.Clean(canonical)

	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])[:12]
}

// ProjectConfigDir returns the per-project config directory for the current
// git repo.  Returns ("", nil) if not inside a git repo.
func ProjectConfigDir() (string, error) {
	repoRoot := detectRepoRoot()
	if repoRoot == "" {
		return "", nil
	}
	id := ProjectID(repoRoot)
	return filepath.Join(ConfigDir(), "projects", id), nil
}

// LoadProjectRaw reads the per-project config as a raw key-value map.
// Returns (nil, nil) if not in a repo or no project config exists.
func LoadProjectRaw() (map[string]string, error) {
	dir, err := ProjectConfigDir()
	if err != nil || dir == "" {
		return nil, err
	}

	path := filepath.Join(dir, "config")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open project config: %w", err)
	}
	defer f.Close()

	values, err := parseShellConfig(f)
	if err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	return values, nil
}

// SaveProjectField writes a single key-value pair to the current project's
// config, preserving any existing keys.  Creates the project directory and
// repo-path metadata file if they do not exist yet.
func SaveProjectField(key, value string) error {
	repoRoot := detectRepoRoot()
	if repoRoot == "" {
		return fmt.Errorf("not inside a git repository")
	}

	dir, err := ProjectConfigDir()
	if err != nil {
		return err
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create project config dir: %w", err)
	}

	// Read existing project config (if any).
	configPath := filepath.Join(dir, "config")
	existing := make(map[string]string)
	if f, err := os.Open(configPath); err == nil {
		existing, _ = parseShellConfig(f)
		f.Close()
	}

	// Update the single key.
	existing[strings.ToUpper(key)] = value

	// Write back only the keys that are present.
	if err := writePartialConfig(configPath, existing); err != nil {
		return err
	}

	// Write repo-path metadata for discoverability.
	metaPath := filepath.Join(dir, "repo-path")
	return os.WriteFile(metaPath, []byte(repoRoot+"\n"), 0600)
}

// RemoveProject deletes a project config directory by ID.
// If id is empty, the current repo's project ID is used.
func RemoveProject(id string) error {
	if id == "" {
		repoRoot := detectRepoRoot()
		if repoRoot == "" {
			return fmt.Errorf("not inside a git repository and no project ID given")
		}
		id = ProjectID(repoRoot)
	}

	dir := filepath.Join(ConfigDir(), "projects", id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("project config not found: %s", id)
	}
	return os.RemoveAll(dir)
}

// ListProjects returns all stored per-project configs.
func ListProjects() ([]ProjectInfo, error) {
	projectsDir := filepath.Join(ConfigDir(), "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var projects []ProjectInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		configPath := filepath.Join(projectsDir, id, "config")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue // no config file — skip
		}

		repoPath := ""
		if b, err := os.ReadFile(filepath.Join(projectsDir, id, "repo-path")); err == nil {
			repoPath = strings.TrimSpace(string(b))
		}

		projects = append(projects, ProjectInfo{
			ID:         id,
			RepoPath:   repoPath,
			ConfigPath: configPath,
		})
	}
	return projects, nil
}

// ── internal helpers ────────────────────────────────────────────────────────

// detectRepoRoot returns the git repository root or "" if not in a repo.
func detectRepoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// writePartialConfig writes only the given key-value pairs to path.
func writePartialConfig(path string, m map[string]string) error {
	var sb strings.Builder
	sb.WriteString("# AI Review Project Configuration\n")
	sb.WriteString("# Only overridden keys are stored here; others fall through to global config.\n\n")

	for k, v := range m {
		sb.WriteString(fmt.Sprintf("%s=%q\n", k, v))
	}
	return os.WriteFile(path, []byte(sb.String()), 0600)
}
