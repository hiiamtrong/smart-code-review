---
phase: planning
title: Planning — Migrate Bash Scripts to Go Binary
description: Task breakdown for replacing bash scripts with a Go binary
---

# Project Planning & Task Breakdown

## Milestones

- [x] **M1: Go project scaffold + config + CLI skeleton** — binary compiles, `setup`/`config`/`status`/`help` commands work
- [x] **M2: Core hook logic** — `run-hook` command fully replaces `pre-commit.sh` (diff, filter, language detect, AI Gateway SSE)
- [x] **M3: Installer + hook management** — `install`/`uninstall`/`update` commands; simplified `install.sh`/`install.ps1`
- [x] **M4: CI review command** — `ci-review` replaces `ai-review.sh` + `detect-language.sh` for GitHub Actions
- [x] **M5: SonarQube integration** — `sonarqube` subcommand replaces `sonarqube-review.sh`
- [x] **M6: Cross-platform release + goreleaser** — binaries for all platforms, GitHub Release pipeline
- [x] **M7: Migration & deprecation** — update docs, README, GitHub Action workflow; deprecate bash scripts

## Task Breakdown

### Phase 1: Foundation (M1) ✅
- [x] **1.1** Initialize Go module: `go mod init github.com/hiiamtrong/smart-code-review`
  - `go/` directory at repo root; `go.mod` with Go 1.25 (actual installed version)
- [x] **1.2** Add cobra CLI skeleton
  - `go/cmd/ai-review/main.go` entry point
  - Root command with version flag (`-v/--version`)
  - All subcommands registered: `setup`, `config`, `status`, `install`, `uninstall`, `update`, `run-hook` (hidden), `ci-review` (hidden)
- [x] **1.3** Implement `internal/config` (renamed from `pkg/config` per design)
  - Custom `parseShellConfig(io.Reader)` — reads shell `KEY="VALUE"` format (no viper)
  - `Save()` writes 0600-permission file; `Load()` + `LoadWithRepoOverrides()`
  - `GetField`/`SetField` for `config get/set` commands
  - Windows path: `%APPDATA%\ai-review\config`
  - 6 unit tests — all passing
- [x] **1.4** `cmd/setup` — interactive credential wizard with masked password input
- [x] **1.5** `cmd/config` — print all values (API key masked), `get KEY`, `set KEY VALUE`
- [x] **1.6** `cmd/status` — config + hook presence check
- [x] **Bonus**: `internal/display`, `internal/git`, `internal/installer` scaffolded (needed for compilation)

### Phase 2: Core Hook Logic (M2) ✅
- [x] **2.1** Implement `internal/git`
  - `GetStagedDiff()`, `GetPRDiff(baseBranch)` with origin/main fallback chain
  - `GetCurrentBranch()`, `GetRemoteTrackingBranch()`, `GetRepoRoot()`, `GetLocalConfig(key)`
  - `GetGitInfo()` — author, email, commit hash, repo URL, PR number from GITHUB_REF
  - `AnnotateLineNumbers(diff)` — pure Go port of showlinenum.awk
  - 9 unit tests — all passing
- [x] **2.2** Implement `internal/filter`
  - `LoadIgnorePatterns(path)`, `FilterDiff(diff, patterns)` — returns filtered diff + ignored file count
  - Pattern matching via `doublestar.Match` with `**/` prefix fallback for extension patterns
  - 12 unit tests — all passing
- [x] **2.3** Implement `internal/language`
  - `DetectFromDiff(diff)` — check file extensions in diff headers
  - `DetectFromProject(root)` — check `package.json`, `go.mod`, `requirements.txt`, etc.
  - Returns: `typescript|javascript|python|java|go|csharp|ruby|php|unknown`
  - 21 unit tests — all passing
- [x] **2.4** Implement `internal/display`
  - `LogInfo`, `LogWarn`, `LogError`, `LogSuccess`, `PrintIssue`, `PrintSummary`, `PrintHeader`, `Divider`
  - Uses `github.com/fatih/color` — ANSI handled via go-isatty (Windows-safe)
- [x] **2.5** Implement `internal/gateway` — streaming SSE client
  - `StreamingReview(ctx, cfg, payload, onDiagnostic)` — multipart POST, parse `text/event-stream`
  - `sse.go`: `parseSSEStream` with 1MB `bufio.Scanner` buffer; handles `progress`, `text`, `diagnostic`, `complete`, `error` events
  - `SyncReview(ctx, cfg, payload)` — non-streaming fallback
  - Auto-fallback to sync if SSE `error` event received
  - 16 unit/integration tests (httptest.Server) — all passing
- [x] **2.6** Implement `cmd/run-hook` (pre-commit logic)
  - Load config → validate credentials (warn + skip if not configured)
  - `GetStagedDiff` → `FilterDiff` → `AnnotateLineNumbers`
  - `DetectLanguage` from diff, fallback to project files
  - `StreamingReview` with live `onDiagnostic` callback printing each issue as it streams
  - Sync-fallback path also prints and counts diagnostics
  - Display overview + summary; exit 1 on ERROR diagnostics
  - Gateway error: warn + skip if `BlockOnGatewayError=false`; exit 1 if `true` (default)
  - SonarQube hook: TODO(M5) — `ENABLE_SONARQUBE_LOCAL` check placeholder

### Phase 3: Installer & Hook Management (M3) ✅
- [x] **3.1** Implement `internal/installer`
  - `GetHooksDir(repoRoot)` — detect husky, `core.hooksPath`, default `.git/hooks/`
  - `WritePreCommitHook(hooksDir)` — refuses to overwrite non-our hooks
  - `RemovePreCommitHook(hooksDir)` — removes only if `# AI-REVIEW-HOOK` marker present
  - `IsHookInstalled(hooksDir)` bool
- [x] **3.2** Implement `cmd/install` — install hook in current repo
- [x] **3.3** Implement `cmd/uninstall` — remove hook from current repo
- [x] **3.4** Implement `cmd/update` — self-update via `internal/updater`
  - `FetchLatest()` — queries GitHub Releases API for latest tag + asset URL
  - `ReplaceCurrentBinary(url)` — downloads archive, extracts binary, atomic `rename` on Unix
  - Windows: batch file trampoline (`.update.bat`) for in-place replacement
- [x] **3.5** Simplify `scripts/local/install.sh`
  - Detect OS/arch → fetch latest tag → download `.tar.gz` → `tar xz` → `install -m 755`
  - Removed all bash script copying; no `jq` dependency
- [x] **3.6** Rewrite `scripts/local/install.ps1` for Windows
  - `Invoke-RestMethod` to get latest tag; `Invoke-WebRequest` + `Expand-Archive` for zip
  - `[Environment]::SetEnvironmentVariable` to add `BinDir` to User PATH permanently
- [x] **3.7** Write `internal/installer` tests — 13 tests passing
  - Husky detection, default `.git/hooks/`, creates dir, executable bit
  - Refuses foreign hook, overwrites own hook, remove/skip/not-found cases

### Phase 4: CI Review Command (M4) ✅
- [x] **4.1** Implement `cmd/ci-review` — GitHub Actions PR review
  - Reads `GITHUB_BASE_REF`, `GITHUB_TOKEN`, `GITHUB_REPOSITORY` env vars
  - `GetPRDiff(baseBranch)` with origin/main → origin/master → HEAD~1 fallback chain
  - Detects language from diff then project files
  - Calls `SyncReview` (not streaming — CI doesn't need live output)
  - Writes `ai-output.jsonl` (rdjson) and `ai-overview.txt`
  - Posts overview as PR comment via GitHub API (deletes old before posting)
  - Invokes `reviewdog`; exits 1 if ERROR diagnostics present
  - `--output`, `--overview`, `--reporter` flags for CI flexibility
- [x] **4.2** Implement `internal/reviewdog`
  - `WriteRDJSON(result, path)` — newline-delimited JSON, one diagnostic per line
  - `WriteOverview(result, path)` — plain-text file
  - `InvokeReviewdog(inputFile, reporter)` — exec with stdin piped from rdjson file
  - `PostOverviewComment(token, repo, prNumber, overview)` — GitHub Issues API
  - `DeleteExistingOverviewComments(token, repo, prNumber)` — finds by sentinel marker

### Phase 5: SonarQube Integration (M5)
- [x] **5.1** Implement `internal/sonarqube`
  - `RunAnalysis(cfg SonarConfig) error` — exec `sonar-scanner` with appropriate args
  - `FetchResults(cfg, changedLineRanges) (*Result, error)` — call SonarQube API, filter to changed lines
  - `AutoGenerateProperties(root string) (string, error)` — generate `sonar-project.properties` if missing
- [x] **5.2** Integrate SonarQube into `cmd/run-hook`
  - If `ENABLE_SONARQUBE_LOCAL=true`, run SonarQube before AI review
  - Display results in same format as bash version
  - Block commit on SonarQube errors

### Phase 6: Release Pipeline (M6)
- [x] **6.1** Add `.goreleaser.yml`
  - Builds: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
  - Archives: `.tar.gz` for Unix, `.zip` for Windows
  - Checksums file
  - GitHub Release notes from git log
- [x] **6.2** Update `release.yml` to use goreleaser
  - Added `actions/setup-go@v5` + `goreleaser/goreleaser-action@v6` steps
  - Replaced `ncipollo/release-action` with goreleaser
- [x] **6.3** Update `action.yml` to use `ai-review ci-review` instead of shell scripts

### Phase 7: Migration & Cleanup (M7)
- [x] **7.1** Update `README.md` — new install instructions, commands (removed jq/curl prereqs, updated CLI table, SonarQube config)
- [x] **7.2** Update `action.yml` to download Go binary via `install.sh` instead of running bash scripts
- [x] **7.3** Deprecation notice in bash scripts (`scripts/ai-review.sh`, `scripts/detect-language.sh`)
- [ ] **7.4** Remove bash scripts after 1 version cycle (optional, can keep for reference)

## Dependencies

```
Phase 1 (Foundation) ✅
  └─► Phase 2 (Hook Logic) ✅     # needs config to be ready
        └─► Phase 3 (Installer)   # needs run-hook to be complete — partial
        └─► Phase 4 (CI Review)   # parallel with Phase 3, needs internal/git, internal/filter, internal/gateway
              └─► Phase 5 (Sonar) # needs base hook logic
                    └─► Phase 6 (Release) # all features complete
                          └─► Phase 7 (Cleanup)
```

**External dependencies:**
- `reviewdog` binary: still external (not bundled), invoked by the Go binary
- `sonar-scanner` CLI: still external
- GitHub Releases: needed for `update` command and CI binary download

## Risks & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| SSE streaming logic differs subtly from bash | ~~Medium~~ **Resolved** | High | 16 integration tests covering all event types; sync fallback verified |
| Config file format incompatibility | Low | Medium | Go parser for `KEY="VALUE"` is straightforward; test with existing config files |
| Windows path handling bugs | Medium | Medium | Use `filepath` package throughout; CI test on Windows runner |
| `.aireviewignore` glob semantics differ | ~~Low~~ **Resolved** | Medium | 12 table-driven tests; `**/` prefix fallback matches bash behavior |
| Binary size exceeds 15 MB | Low | Low | Use `-ldflags="-s -w"` + UPX compression if needed |
| sonar-scanner invocation breaks | Medium | Medium | Keep the sonar-scanner exec logic minimal; test in Docker with real sonar |
| Breaking change in pre-commit hook format | Low | Medium | Keep `# AI-REVIEW-HOOK` marker; `uninstall` checks for it before removing |
| `os.Exit(1)` in `cmd/run-hook` prevents unit testing | Low | Low | Extract exit-code logic to a testable helper in future; currently acceptable |

## Resources Needed

- Go 1.22+ development environment
- goreleaser (for local release testing)
- Access to AI Gateway (for SSE integration tests)
- GitHub Actions Windows/Linux/macOS runners (for cross-platform CI)
- Docker with SonarQube (for sonar integration tests)
