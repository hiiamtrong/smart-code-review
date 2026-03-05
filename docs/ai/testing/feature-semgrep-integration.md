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

## Test Files

- [`go/internal/semgrep/semgrep_test.go`](go/internal/semgrep/semgrep_test.go)
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
