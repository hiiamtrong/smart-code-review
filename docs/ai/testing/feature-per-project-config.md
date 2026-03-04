---
phase: testing
title: Per-Project Configuration — Testing Strategy
description: Test plan for per-project config feature
---

# Per-Project Configuration — Testing Strategy

## Test Coverage Goals

- 100% coverage of new functions in `project.go` and `merge.go`
- Integration tests for CLI `config` command with `--project`/`--global` flags
- E2E test for full workflow: set project config → run-hook uses it

## Unit Tests

### `internal/config/project_test.go`

- [ ] `TestProjectID_Deterministic` — same path always produces same ID
- [ ] `TestProjectID_DifferentPaths` — different paths produce different IDs
- [ ] `TestProjectID_Canonicalization` — paths with trailing slashes, `/../`, symlinks produce same ID as canonical path
- [ ] `TestProjectID_Length` — ID is exactly 12 hex characters
- [ ] `TestSaveProject_CreatesDir` — creates `projects/<id>/` directory with 0700 perms
- [ ] `TestSaveProject_WritesConfig` — config file has correct KEY="VALUE" content
- [ ] `TestSaveProject_WritesRepoPath` — `repo-path` file contains the repo root
- [ ] `TestLoadProject_ReadsConfig` — round-trip: save then load returns same values
- [ ] `TestLoadProject_NoProjectConfig` — returns (nil, nil) when no config file
- [ ] `TestLoadProject_NotInRepo` — returns (nil, nil) when not in a git repo
- [ ] `TestLoadProject_CorruptFile` — returns error for malformed config

### `internal/config/merge_test.go`

- [ ] `TestLoadMerged_DefaultsOnly` — no config files → returns compiled defaults
- [ ] `TestLoadMerged_GlobalOverridesDefaults` — global config values override defaults
- [ ] `TestLoadMerged_ProjectOverridesGlobal` — project config values override global
- [ ] `TestLoadMerged_EnvOverridesAll` — env vars override project and global
- [ ] `TestLoadMerged_GitLocalOverridesProject` — `git config --local` overrides project config
- [ ] `TestLoadMerged_PartialProjectConfig` — project config with only some keys; rest comes from global
- [ ] `TestLoadMerged_BooleanOverride` — project sets `ENABLE_AI_REVIEW=false` overrides global `true`
- [ ] `TestLoadMergedWithSources_Labels` — each field has correct source label
- [ ] `TestListProjects_MultipleProjects` — lists all project dirs with repo paths
- [ ] `TestListProjects_EmptyProjectsDir` — returns empty list
- [ ] `TestListProjects_MissingRepoPathFile` — handles missing `repo-path` gracefully
- [ ] `TestRemoveProject_DeletesDir` — removes project config directory

## Integration Tests

### `internal/cmd/cmd_test.go` (extend existing)

- [ ] `TestConfigCommand_ShowsMergedConfig` — output includes source annotations
- [ ] `TestConfigSetGlobalFlag` — `config set KEY VAL --global` writes to global config only
- [ ] `TestConfigSetProjectFlag` — `config set KEY VAL --project` writes to project config only
- [ ] `TestConfigGetGlobalFlag` — `config get KEY --global` reads from global only
- [ ] `TestConfigListProjects` — `config list-projects` shows all project configs
- [ ] `TestConfigRemoveProject` — `config remove-project` deletes project config

## End-to-End Tests

### `go/e2e/` (extend existing)

- [ ] `TestE2E_ProjectConfigWorkflow`:
  1. Create temp git repo (`git init`)
  2. Set global config with model X
  3. Run `ai-review config set --project AI_MODEL Y`
  4. Verify `ai-review config get AI_MODEL` returns Y (project override)
  5. Run outside the repo → verify model is X (global)
  6. Run `ai-review config list-projects` → shows the temp repo
  7. Run `ai-review config remove-project` → project config deleted
  8. Verify `ai-review config get AI_MODEL` returns X (fallen back to global)

## Test Data

- **Temp git repos**: use `t.TempDir()` + `git init` for isolated repos
- **Config files**: generate via `formatShellConfig()` or write manually
- **Environment**: use `t.Setenv()` for env var overrides (auto-restored)

## Test Reporting & Coverage

```bash
# Run all config tests
cd go && go test ./internal/config/ -v -cover

# Run with coverage report
cd go && go test ./internal/config/ -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Target: 100% coverage on `project.go` and `merge.go`**

## Manual Testing

- [ ] Install binary, run `ai-review config` in a repo → shows global values
- [ ] Run `ai-review config set --project AI_MODEL gpt-4o` → creates project config
- [ ] Run `ai-review config` again → shows `gpt-4o (project)` annotation
- [ ] Switch to another repo → shows global model value
- [ ] Run `ai-review config list-projects` → shows project with path
- [ ] Run `ai-review config remove-project` → project config removed
- [ ] Run `ai-review status` → shows project config path when active
