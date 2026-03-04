---
phase: implementation
title: Per-Project Configuration — Implementation Guide
description: Technical implementation notes for per-project config
---

# Per-Project Configuration — Implementation Guide

## Development Setup

**Prerequisites:**
- Go 1.22+
- Existing `go/` directory structure

**No new dependencies required** — uses only stdlib (`crypto/sha256`, `encoding/hex`, `path/filepath`, `os`).

## Code Structure

### New Files

```
go/internal/config/
├── config.go           # existing — add minor helpers
├── gitconfig.go        # existing — unchanged
├── project.go          # NEW — project identification + I/O
├── project_test.go     # NEW — unit tests
├── merge.go            # NEW — layered config merge
└── merge_test.go       # NEW — unit tests
```

### Key Implementation Details

#### `project.go` — Project Identification

```go
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

// ProjectID computes a stable, filesystem-safe identifier for a repo root path.
func ProjectID(repoRoot string) string {
    // Canonicalize: resolve symlinks + clean path
    canonical, err := filepath.EvalSymlinks(repoRoot)
    if err != nil {
        canonical = repoRoot
    }
    canonical = filepath.Clean(canonical)

    h := sha256.Sum256([]byte(canonical))
    return hex.EncodeToString(h[:])[:12]
}

// ProjectConfigDir returns the per-project config directory for the current git repo.
// Returns ("", nil) if not inside a git repo.
func ProjectConfigDir() (string, error) {
    repoRoot := detectRepoRoot()
    if repoRoot == "" {
        return "", nil
    }
    id := ProjectID(repoRoot)
    return filepath.Join(ConfigDir(), "projects", id), nil
}

// LoadProject loads the per-project config file.
// Returns (nil, nil) if not in a repo or no project config exists.
func LoadProject() (*Config, error) {
    dir, err := ProjectConfigDir()
    if err != nil || dir == "" {
        return nil, err
    }

    path := filepath.Join(dir, "config")
    f, err := os.Open(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil // no project config — not an error
        }
        return nil, fmt.Errorf("open project config: %w", err)
    }
    defer f.Close()

    cfg := &Config{} // empty, not defaults
    values, err := parseShellConfig(f)
    if err != nil {
        return nil, fmt.Errorf("parse project config: %w", err)
    }
    applyValues(cfg, values)
    return cfg, nil
}

// SaveProject writes cfg to the per-project config directory.
func SaveProject(cfg *Config) error {
    repoRoot := detectRepoRoot()
    if repoRoot == "" {
        return fmt.Errorf("not inside a git repository")
    }

    dir, _ := ProjectConfigDir()
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("create project config dir: %w", err)
    }

    // Write config
    path := filepath.Join(dir, "config")
    content := formatShellConfig(cfg)
    if err := os.WriteFile(path, []byte(content), 0600); err != nil {
        return err
    }

    // Write repo-path metadata for discoverability
    metaPath := filepath.Join(dir, "repo-path")
    return os.WriteFile(metaPath, []byte(repoRoot+"\n"), 0600)
}

func detectRepoRoot() string {
    out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}
```

#### `merge.go` — Config Merging

```go
package config

// LoadMerged loads config with full resolution:
// defaults ← global ← project ← git-local ← env
func LoadMerged() (*Config, error) {
    cfg := Defaults()

    // Layer 1: global config
    globalCfg, _ := loadGlobalOnly()
    if globalCfg != nil {
        mergeConfig(cfg, globalCfg)
    }

    // Layer 2: project config
    projectCfg, _ := LoadProject()
    if projectCfg != nil {
        mergeConfig(cfg, projectCfg)
    }

    // Layer 3: git-local overrides (existing mechanism)
    applyGitLocalOverrides(cfg)

    // Layer 4: env vars (always highest priority)
    applyEnvVars(cfg)

    return cfg, nil
}

// mergeConfig copies non-zero fields from src to dst.
func mergeConfig(dst, src *Config) {
    if src.AIGatewayURL != "" { dst.AIGatewayURL = src.AIGatewayURL }
    if src.AIGatewayAPIKey != "" { dst.AIGatewayAPIKey = src.AIGatewayAPIKey }
    if src.AIModel != "" { dst.AIModel = src.AIModel }
    if src.AIProvider != "" { dst.AIProvider = src.AIProvider }
    // ... booleans need special handling (track "was explicitly set")
}
```

**Important: boolean merge challenge.**
Booleans default to `false` in Go — we cannot distinguish "explicitly set to false" from "not set". Two approaches:

1. **Option A: Track explicitly-set fields** — add a `SetFields map[string]bool` to Config. When parsing, record which keys were present. Only merge fields that are in SetFields.
2. **Option B: Parse project config as raw map** — merge at the string level before converting to Config.

**Recommended: Option B** — simpler, reuses existing `parseShellConfig()`, no struct changes.

```go
// mergeValues merges raw key-value maps, later maps override earlier ones.
func mergeValues(layers ...map[string]string) map[string]string {
    result := make(map[string]string)
    for _, m := range layers {
        for k, v := range m {
            result[k] = v
        }
    }
    return result
}
```

## Integration Points

### Replacing `Load()` calls

All command files that call `config.Load()` or `config.LoadWithRepoOverrides()` should switch to `config.LoadMerged()`:

| Call site | Current | New |
|-----------|---------|-----|
| `runhook.go:35` | `LoadWithRepoOverrides()` | `LoadMerged()` |
| `cireview.go:39` | `Load()` | `LoadMerged()` |
| `setup.go:28` | `Load()` | `LoadMerged()` |
| `status.go:29` | `Load()` | `LoadMerged()` |
| `install.go:31` | `Load()` | `LoadMerged()` |

### Deprecation

Keep `Load()` and `LoadWithRepoOverrides()` working but internally delegate to `LoadMerged()`:

```go
// Load is deprecated. Use LoadMerged for full per-project support.
func Load() (*Config, error) {
    return LoadMerged()
}
```

## Error Handling

- **No project config**: silent fallback to global config (not an error)
- **Corrupt project config**: log warning, fall back to global config
- **No git repo**: skip project layer entirely
- **Project dir not writable**: return error from `SaveProject()` only

## Security Notes

- All config files use 0600 permissions (existing pattern)
- Project config dirs use 0700 permissions
- API keys stored in project config are protected by filesystem permissions, never committed to repo
- `repo-path` file is informational only, does not affect config resolution
