---
phase: testing
title: Semgrep Integration Testing
description: Testing strategy and results for Semgrep integration
---

# Testing: Semgrep Integration

## Test Coverage Summary

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/semgrep` | **97.9%** | Uncovered: Windows `semgrep.exe` name branch (untestable on macOS) |
| `internal/installer` | **92.9%** | Uncovered: write-after-open errors (unreachable without fs mocks) |
| `internal/config` | All Semgrep fields covered | `GetField`, `SetField`, `DefaultsAsMap`, env vars |
| `internal/cmd` (setup) | All setup wizard tests updated | Added Semgrep prompt to all 5 integration tests |

## Unit Tests: `internal/semgrep/`

### FindSemgrep (90.9%)
- [x] `TestFindSemgrep_NotFound` — PATH empty, HOME empty
- [x] `TestFindSemgrep_FoundInPath` — fake binary in PATH
- [x] `TestFindSemgrep_FoundInLocalBin` — fallback to `~/.local/bin/`

### ScanFiles (100%)
- [x] `TestScanFiles_EmptyFileList` — early return, no execution
- [x] `TestScanFiles_WithFindings` — fake binary outputs JSON, exits 1
- [x] `TestScanFiles_NoFindings` — fake binary outputs empty results, exits 0
- [x] `TestScanFiles_BinaryFailure` — exit code 2 = real error
- [x] `TestScanFiles_InvalidOutput` — non-JSON output
- [x] `TestScanFiles_DefaultRules` — empty Rules defaults to "auto"
- [x] `TestScanFiles_BinaryNotFound` — nonexistent binary path

### parseOutput (100%)
- [x] `TestParseOutput_Empty` — empty byte slice
- [x] `TestParseOutput_ValidResults` — 3 findings (ERROR, WARNING, INFO) with full field validation
- [x] `TestParseOutput_AbsolutePathConversion` — /abs/path → relative
- [x] `TestParseOutput_RelativePathKept` — relative path preserved
- [x] `TestParseOutput_EmptyRepoRoot` — absolute path preserved when no repoRoot
- [x] `TestParseOutput_InvalidJSON` — parse error
- [x] `TestParseOutput_NoResults` — valid JSON, 0 results

### mapSeverity (100%)
- [x] `TestMapSeverity` — 8 cases: ERROR, error, WARNING, warning, INFO, info, empty, UNKNOWN

## Unit Tests: `internal/config/`

- [x] `TestSetField` — includes `ENABLE_SEMGREP` and `SEMGREP_RULES`
- [x] `TestGetField_allKeys` — includes Semgrep fields
- [x] `TestDefaultsAsMap_AllKeys` — updated to dynamic key count

## Integration Tests: `internal/cmd/`

- [x] `TestRunSetup_AIOnly` — added Semgrep prompt (n)
- [x] `TestRunSetup_WithSonarQube` — added Semgrep prompt (n)
- [x] `TestRunSetup_BothDisabled` — added Semgrep prompt (n)
- [x] `TestRunSetup_ProjectFlag` — added Semgrep prompt (n)
- [x] `TestRunSetup_AbortAtSummary` — added Semgrep prompt (n)

## Unit Tests: `internal/installer/` (pre-commit.com compatibility)

### DetectPreCommitFramework (100%)
- [x] `TestDetectPreCommitFramework_Found` — `.pre-commit-config.yaml` exists
- [x] `TestDetectPreCommitFramework_NotFound` — file absent

### InjectPreCommitConfig (92.9%)
- [x] `TestInjectPreCommitConfig_Injects` — appends `repo: local` block, preserves original
- [x] `TestInjectPreCommitConfig_Idempotent` — second call is no-op
- [x] `TestInjectPreCommitConfig_FileNotFound` — error when no config file
- [x] `TestInjectPreCommitConfig_AppendError` — read-only file returns error

### RemovePreCommitConfig (92.3%)
- [x] `TestRemovePreCommitConfig_Removes` — strips injected block cleanly
- [x] `TestRemovePreCommitConfig_NotPresent` — no-op when not installed
- [x] `TestRemovePreCommitConfig_NoFile` — no-op when file absent
- [x] `TestRemovePreCommitConfig_ReadError` — unreadable file returns error

### IsPreCommitConfigInstalled (100%)
- [x] `TestIsPreCommitConfigInstalled_True` — after injection
- [x] `TestIsPreCommitConfigInstalled_False` — no config file

### Error paths
- [x] `TestWritePreCommitHook_MkdirAllError` — read-only parent dir
- [x] `TestWritePreCommitHook_AppendToReadonlyHook` — read-only foreign hook
- [x] `TestRemovePreCommitHook_ReadError` — unreadable hook file

## Test Files

- [`go/internal/semgrep/semgrep_test.go`](go/internal/semgrep/semgrep_test.go)
- [`go/internal/installer/installer_test.go`](go/internal/installer/installer_test.go)
- [`go/internal/config/config_test.go`](go/internal/config/config_test.go)
- [`go/internal/config/merge_test.go`](go/internal/config/merge_test.go)
- [`go/internal/cmd/setup_test.go`](go/internal/cmd/setup_test.go)

## Coverage Commands

```bash
go test ./internal/semgrep/ -coverprofile=cover.out -count=1
go tool cover -func=cover.out
go test ./... -count=1  # full suite
```

## Deferred Tests

- Windows-specific `FindSemgrep` test (requires CI with Windows runner)
- End-to-end `hookRunSemgrep` with real Semgrep binary (requires Semgrep installed)
- Integration test for `runInstall`/`runUninstall` with pre-commit framework (requires git repo mock)
- Write-after-open error paths in installer (requires filesystem mocking)
