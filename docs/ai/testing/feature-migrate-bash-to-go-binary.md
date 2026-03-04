---
phase: testing
title: Testing Strategy — Migrate Bash Scripts to Go Binary
description: Test plan for the ai-review Go binary migration
---

# Testing Strategy

## Test Coverage Goals

- **Unit test coverage target:** 90%+ of all `internal/` packages
- **Integration test scope:** AI Gateway client (with mock server), git operations (with temp repos), full pre-commit flow end-to-end
- **End-to-end test scenarios:** install → configure → commit (with and without errors) → uninstall on each OS
- **Parity tests:** each bash integration test in `scripts/local/tests/` must have an equivalent Go test

## Unit Tests

### internal/config (85% coverage)
- [x] `ParseShellConfig`: parse standard `KEY="VALUE"` format
- [x] `ParseShellConfig`: skip comment lines and blank lines
- [x] `ParseShellConfig`: handle single-quoted values
- [x] `ParseShellConfig`: handle values with equals inside value
- [x] `Save`/`Load`: roundtrip recovers same values
- [x] `Load`: missing config file succeeds (returns defaults + env vars) — no error in CI
- [x] `Load`: env vars overlay file values (CI mode)
- [x] `Load`: env-var-only config (no file) — CI runner support
- [x] `GetField`: all known keys returned correctly
- [x] `SetField`: all settable keys update correctly; unknown key returns error
- [x] Config file permissions: 0600
- [x] `GATEWAY_TIMEOUT_SEC` env var overrides default int timeout
- [ ] `ConfigDir`: Windows path (APPDATA) — deferred (requires Windows runner)
- [ ] `LoadWithRepoOverrides`: git local config overrides — deferred (requires git repo setup)

### internal/filter (96.4% coverage) ✅
- [x] `FilterDiff`: single file matching pattern removed
- [x] `FilterDiff`: unmatched file preserved
- [x] `FilterDiff`: `**/*.min.js` nested path
- [x] `FilterDiff`: `node_modules/` prefix
- [x] `FilterDiff`: empty patterns returns unchanged diff
- [x] `FilterDiff`: all files ignored → empty + correct count
- [x] `FilterDiff`: mixed ignored/kept files
- [x] `LoadIgnorePatterns`: skips comments and blanks
- [x] `LoadIgnorePatterns`: missing file returns empty slice

### internal/language (89.7% coverage) ✅
- [x] `DetectFromDiff`: `.ts`, `.tsx` → typescript
- [x] `DetectFromDiff`: `.js`, `.jsx` → javascript
- [x] `DetectFromDiff`: `.py`, `.java`, `.go`, `.cs`, `.rb`, `.php`
- [x] `DetectFromDiff`: unknown extension → unknown
- [x] `DetectFromProject`: package.json, go.mod, requirements.txt

### internal/gateway (85.3% coverage) ✅
- [x] `parseSSEStream`: diagnostic, text, complete, error, unknown events
- [x] `parseSSEStream`: empty data lines skipped
- [x] `StreamingReview`: multipart body, X-API-Key header, non-200 error
- [x] `SyncReview`: non-streaming JSON fallback

### internal/display (0% coverage)
- [ ] `PrintIssue` severity variants — deferred (stdout capture required, low business risk)

### internal/installer (75% coverage)
- [x] `GetHooksDir`: default `.git/hooks`, husky detection
- [x] `WritePreCommitHook`: creates marker, executable, refuses foreign hook
- [x] `RemovePreCommitHook`: removes if marker present, skips if not
- [x] `IsHookInstalled`: true/false cases
- [ ] `GetHooksDir`: `core.hooksPath` git config path — deferred

### internal/git (70.1% coverage)
- [x] `GetStagedDiff`: empty and non-empty
- [x] `GetGitInfo`: author, email, branch, commit hash
- [x] `GetRepoRoot`: from subdirectory
- [x] `AnnotateLineNumbers`: line numbering logic
- [ ] `GetPRDiff`: requires remote — deferred
- [ ] `GetLocalConfig`, `GetRemoteTrackingBranch` — deferred (require git remote setup)

### internal/sonarqube (78% coverage)
- [x] `ParseStagedLineRanges`: basic, deletion hunk, no count, multi-file, empty
- [x] `dedupedDirs`: basic dedup, subdir removal, root files
- [x] `sanitizeKey`: special character replacement
- [x] `sonarToSeverity`: BLOCKER/CRITICAL/MAJOR/MINOR/INFO
- [x] `filterByChangedLines`: keeps, drops, wrong file
- [x] `convertIssues`: strips project key prefix, zero line → 1
- [x] `AutoGenerateProperties`: creates file, skips existing, JS/Go project detection
- [x] `readTaskID`: found, missing file, no taskID line
- [x] `fetchTaskStatus`: success (HTTP mock), network error
- [x] `WaitForTask`: no taskID path, task succeeds (HTTP mock)
- [x] `FetchResults`: HTTP mock with issues + hotspots; `Truncated=false` for < 500 results
- [x] `FetchResults`: `Truncated=true` when page limit (500) is hit
- [ ] `FindScanner`: requires sonar-scanner binary — deferred
- [ ] `RunAnalysis`: requires sonar-scanner binary — deferred

### internal/reviewdog (71.4% coverage)
- [x] `WriteRDJSON`: empty, single, multiple diagnostics; default source name; creates dirs
- [x] `WriteOverview`: basic write, empty overview
- [x] `DeleteExistingOverviewComments`: deletes sentinel-containing comments, skips others
- [x] `PostOverviewComment`: posts with sentinel, HTTP 4xx returns error
- [ ] `InvokeReviewdog`: requires reviewdog binary — deferred

### internal/updater (0% coverage)
- [ ] `FetchLatest`: HTTP mock for GitHub Releases API — deferred
- [ ] `extractFromTarGz`, `extractFromZip`: unit testable — deferred
- [ ] `ReplaceCurrentBinary`: replaces running binary — E2E only

## Integration Tests

- [ ] **Full SSE streaming**: spin up an `httptest.Server` that emits a pre-recorded SSE stream; assert `StreamingReview` returns correct `ReviewResult` with all diagnostics
- [ ] **SSE → sync fallback**: server returns non-200 on first request, then normal JSON on second; assert `run-hook` successfully processes result
- [ ] **Gateway timeout**: server hangs; assert context deadline triggers correct error message and exit code 1
- [ ] **Filter + gateway**: diff with mixed ignored/non-ignored files; assert only non-ignored files are sent to gateway
- [ ] **Pre-commit hook integration** (using `os/exec` + temp git repo):
  - Create temp git repo, write pre-commit hook, stage a file, run `git commit` → assert exit code and stdout
  - Stage no files → exit 0 silently
  - All files ignored → exit 0 with message
  - Gateway returns ERROR severity → exit 1, message contains "COMMIT BLOCKED"
  - Gateway returns WARNING → exit 0 with warnings shown
- [ ] **Config roundtrip**: write config file with `WriteShellConfig`, read back with `ParseShellConfig`, assert all values match

## End-to-End Tests

**Test file:** [`go/e2e/e2e_test.go`](go/e2e/e2e_test.go) — `//go:build e2e` tag; binary compiled once in `TestMain`.

**Run E2E tests:**
```bash
cd go
go test -tags=e2e -v -timeout 120s ./e2e/
```

- [x] **E2E: Install flow** — `ai-review install` on temp repo → hook file written, `status` shows installed (`TestInstallAndStatus`)
- [x] **E2E: Uninstall** — `ai-review uninstall` removes hook, `status` shows not installed (`TestInstallAndUninstall`)
- [x] **E2E: Help / version** — `ai-review --help` and `ai-review --version` exit 0 (`TestHelp`, `TestVersion`)
- [x] **E2E: run-hook no staged files** — exit 0 silently (`TestRunHook_NoStagedFiles`)
- [x] **E2E: run-hook AI disabled** — `ENABLE_AI_REVIEW=false` → exit 0 (`TestRunHook_AIDisabled`)
- [x] **E2E: run-hook all files ignored** — exit 0 with "skipped" message (`TestRunHook_AllFilesIgnored`)
- [x] **E2E: run-hook gateway WARNING** — exit 0, output shows issue (`TestRunHook_GatewayReturnsWarning`)
- [x] **E2E: run-hook gateway ERROR** — `BLOCK_ON_ERROR=true` → exit 1, "COMMIT BLOCKED" (`TestRunHook_GatewayReturnsError`)
- [x] **E2E: run-hook gateway network error** — `BLOCK_ON_ERROR=false` → exit 0 (`TestRunHook_GatewayError_NoBlock`)
- [x] **E2E: run-hook gateway network error + block** — `BLOCK_ON_ERROR=true` → exit 1 (`TestRunHook_GatewayError_Block`)
- [x] **E2E: run-hook gateway timeout** — `GATEWAY_TIMEOUT_SEC=2`, `BLOCK_ON_ERROR=false` → exit 0 (`TestRunHook_GatewayTimeout`)
- [x] **E2E: run-hook gateway timeout + block** — `BLOCK_ON_ERROR=true` → exit 1 (`TestRunHook_GatewayTimeout_Block`)
- [x] **E2E: run-hook no gateway configured** — missing `AI_GATEWAY_URL` → exit 0 with skip message (`TestRunHook_NoGatewayConfigured`)
- [ ] **E2E: Pre-commit with real gateway** (optional, requires `AI_GATEWAY_URL` env) — stage test file → commit → AI review runs → result displayed
- [ ] **E2E: ci-review** (in GitHub Actions): workflow triggers on PR → `ai-review ci-review` runs → `ai-output.jsonl` written → reviewdog invoked

## Test Data

**Golden files** (`go/testdata/`):
- `sample.diff` — a multi-file git diff for filter and language detection tests
- `sample_filtered.diff` — expected output after applying `testdata/aireviewignore`
- `sse_stream.txt` — a recorded SSE response to replay in gateway tests
- `sse_stream_with_error.txt` — SSE stream with an `error` event

**Mock gateway** (`internal/gateway/testserver_test.go`):
```go
func newMockGateway(t *testing.T, responseFile string) *httptest.Server {
    data, _ := os.ReadFile(responseFile)
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.Write(data)
    }))
}
```

**Temp git repos** for installer and hook tests:
```go
func newTempGitRepo(t *testing.T) string {
    dir := t.TempDir()
    exec.Command("git", "-C", dir, "init").Run()
    exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
    exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
    return dir
}
```

## Test Reporting & Coverage

**Run unit + integration tests:**
```bash
cd go
go test ./... -v -race -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out | grep total
```

**Run E2E tests** (builds binary, spawns subprocesses):
```bash
cd go
go test -tags=e2e -v -timeout 120s ./e2e/
```

**Coverage thresholds:**
- `internal/filter`: 95%+ (critical correctness)
- `internal/gateway`: 85%+ (complex SSE logic)
- `internal/config`: 90%+
- `internal/language`: 95%+
- `internal/installer`: 85%+
- Overall: 90%+

**CI coverage gate** (in `go/Makefile` or GitHub Actions):
```bash
COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
if (( $(echo "$COVERAGE < 80" | bc -l) )); then echo "Coverage below 80%"; exit 1; fi
```

## Manual Testing

**Cross-platform smoke tests** (must run before each release):
- [ ] macOS (arm64): `brew` install → configure → hook install → commit with changes
- [ ] macOS (amd64): same
- [ ] Ubuntu 22.04: binary from GitHub Releases → configure → hook install → commit
- [ ] Windows 11 (native CMD/PowerShell): `install.ps1` → configure → hook install → commit
- [ ] Windows Git Bash: existing `install.sh` → configure → hook install → commit

**Manual regression checklist:**
- [ ] `ai-review status` shows correct installed/not-installed state
- [ ] `ai-review config` masks API key correctly
- [ ] Hook correctly handles husky (`.husky/pre-commit` + `_/husky.sh`)
- [ ] Colors render correctly in each terminal environment
- [ ] `git commit --no-verify` still bypasses the hook
- [ ] Large diff (>10k lines) doesn't hang or crash

## Performance Testing

- [ ] Measure binary startup time: `time ai-review help` should be < 100ms
- [ ] Measure pre-commit hook time (excl. network): should be < 200ms before gateway call
- [ ] Compare with bash version: record baseline from `scripts/local/pre-commit.sh` timing

## Bug Tracking

- Issues tracked in GitHub Issues with label `migrate-go-binary`
- Severity: P1 (blocks commit wrong), P2 (output differs from bash), P3 (cosmetic)
- Regression tests added for every P1/P2 bug fix
