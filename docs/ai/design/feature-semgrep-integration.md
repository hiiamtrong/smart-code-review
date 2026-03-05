---
phase: design
title: Semgrep Integration Design
description: Architecture and component design for Semgrep integration
---

# Design: Semgrep Integration

## Architecture Overview

```mermaid
graph TD
    Hook[pre-commit hook] --> RunHook[runHook]
    RunHook --> SG[hookRunSemgrep<br/>Stage 1]
    RunHook --> SQ[hookRunSonarQube<br/>Stage 2]
    RunHook --> AI[hookRunAIReview<br/>Stage 3]

    SG --> FindBin[FindSemgrep<br/>binary discovery]
    SG --> RunScan[ScanFiles<br/>semgrep --json]
    SG --> Parse[parseOutput<br/>JSON → Diagnostics]

    RunScan -->|staged files| Semgrep[semgrep CLI]
    Semgrep -->|JSON output| Parse
    Parse --> Diag[gateway.Diagnostic]

    SQ --> Diag
    AI --> Diag
    Diag --> Display[display.PrintIssueWithSource]
```

**Execution order in pre-commit hook (fail-fast, fastest first):**
1. **Semgrep (local scan)** — if enabled, fastest stage
2. SonarQube (server-based) — if enabled and not blocked
3. AI Gateway review — if not blocked

## Data Models

### Config additions (`config.Config`)

```go
// Semgrep
EnableSemgrep    bool   // ENABLE_SEMGREP
SemgrepRules     string // SEMGREP_RULES (e.g. "auto", "p/default", ".semgrep.yml")
```

### Semgrep JSON output structure (parsed)

```go
type semgrepOutput struct {
    Results []semgrepResult `json:"results"`
    Errors  []semgrepError  `json:"errors"`
}

type semgrepResult struct {
    CheckID string         `json:"check_id"`
    Path    string         `json:"path"`
    Start   semgrepPos     `json:"start"`
    End     semgrepPos     `json:"end"`
    Extra   semgrepExtra   `json:"extra"`
}

type semgrepPos struct {
    Line int `json:"line"`
    Col  int `json:"col"`
}

type semgrepExtra struct {
    Message  string `json:"message"`
    Severity string `json:"severity"` // ERROR, WARNING, INFO
    Metadata map[string]interface{} `json:"metadata"`
}
```

## Component Breakdown

### New package: `go/internal/semgrep/`

| File | Responsibility |
|------|---------------|
| `semgrep.go` | Binary discovery, scan execution, output parsing, diagnostic conversion |
| `semgrep_test.go` | Unit tests with fixture JSON |

### Key functions

```go
// FindSemgrep returns the path to the semgrep binary.
func FindSemgrep() (string, error)

// ScanFiles runs semgrep on the given files and returns diagnostics.
func ScanFiles(bin string, cfg SemgrepConfig, files []string) ([]gateway.Diagnostic, error)
```

### Modified files

| File | Change |
|------|--------|
| `config/config.go` | Add `EnableSemgrep`, `SemgrepRules` fields |
| `cmd/runhook.go` | Add `hookRunSemgrep()` as Stage 1 (before SonarQube and AI) |
| `cmd/setup.go` | Add Semgrep configuration step in setup wizard |
| `cmd/install.go` | Auto-detect pre-commit.com framework; inject `repo: local` hook |
| `installer/installer.go` | Add `InjectPreCommitConfig`, `RemovePreCommitConfig`, `DetectPreCommitFramework` |
| `display/display.go` | Add `PrintIssueWithSource`, `PrintStageHeader`, `PrintStageSummary`, `StageSummary` |

## pre-commit.com Framework Compatibility

When the user runs `ai-review install` in a repo that has `.pre-commit-config.yaml`:

1. Detect the framework via `DetectPreCommitFramework()` — checks for `.pre-commit-config.yaml`
2. Instead of writing to `.git/hooks/pre-commit`, inject a `repo: local` block into `.pre-commit-config.yaml`
3. The local hook uses `language: system`, `always_run: true`, `pass_filenames: false`
4. `ai-review uninstall` removes the block cleanly; also checks the hook file for dual-method cleanup

This allows ai-review to coexist with pre-commit.com managed hooks (e.g. `trailing-whitespace`, `end-of-file-fixer`).

## Design Decisions

1. **Scan staged files only**: Pass explicit file paths to `semgrep --json <files>` instead of scanning the full project. This keeps pre-commit fast (< 10s).

2. **Use `--json` output**: Semgrep's JSON output is stable and well-documented. We parse it into `gateway.Diagnostic` for unified display.

3. **Rules configuration**: Default to `auto` (Semgrep auto-detects language and applies relevant rules from the registry). Users can override with specific rulesets or local config files.

4. **Severity mapping**: Semgrep's `ERROR`/`WARNING`/`INFO` maps directly to our severity levels. No transformation needed.

5. **Independent from SonarQube**: Semgrep and SonarQube can both be enabled. They serve different purposes — Semgrep for local pattern matching, SonarQube for server-tracked issues.

6. **No server dependency**: Semgrep CLI runs entirely locally. Registry rules are cached after first download.

## Non-Functional Requirements

- **Performance**: Scanning 5-10 staged files should complete in < 10 seconds
- **Reliability**: If Semgrep fails or is not installed, skip gracefully (warn, don't block)
- **Security**: Semgrep rules are read-only; no data is sent to external servers (unless using registry rules, which only downloads rule definitions)
