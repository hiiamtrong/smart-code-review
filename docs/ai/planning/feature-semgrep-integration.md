---
phase: planning
title: Semgrep Integration Planning
description: Task breakdown for implementing Semgrep integration
---

# Planning: Semgrep Integration

## Milestones

- [x] Milestone 1: Core Semgrep package (binary discovery, scan, parse)
- [x] Milestone 2: Hook integration (runhook.go wiring)
- [x] Milestone 3: Config & setup wizard
- [x] Milestone 4: Tests & documentation

## Task Breakdown

### Phase 1: Core Semgrep Package

- [x] Task 1.1: Create `go/internal/semgrep/semgrep.go` with `FindSemgrep()` binary discovery
- [x] Task 1.2: Implement `ScanFiles()` — run `semgrep --json --config <rules> <files>` and capture output
- [x] Task 1.3: Implement JSON output parser — convert `semgrepResult` to `gateway.Diagnostic`
- [x] Task 1.4: Add `SemgrepConfig` struct with `Rules` field

### Phase 2: Hook Integration

- [x] Task 2.1: Add `hookRunSemgrep()` in `cmd/runhook.go`
- [x] Task 2.2: Wire `hookRunSemgrep()` into `runHook()` between SonarQube and AI review
- [x] Task 2.3: Reuse `extractStagedFiles()` for Semgrep input

### Phase 3: Config & Setup

- [x] Task 3.1: Add `EnableSemgrep` and `SemgrepRules` to `config.Config`
- [x] Task 3.2: Add environment variable mapping (`ENABLE_SEMGREP`, `SEMGREP_RULES`)
- [x] Task 3.3: Add Semgrep step to setup wizard in `cmd/setup.go`

### Phase 4: Tests

- [x] Task 4.1: Unit tests for `FindSemgrep()` (found, not found)
- [x] Task 4.2: Unit tests for JSON parsing with fixture data
- [x] Task 4.3: Unit tests for severity mapping
- [x] Task 4.4: Unit tests for empty file list, absolute path conversion, invalid JSON

## Dependencies

- Phase 2 depends on Phase 1 (core package must exist first)
- Phase 3 depends on Phase 1 (config fields needed for SemgrepConfig)
- Phase 4 can partially run in parallel with Phase 2-3

## Risks & Mitigation

| Risk | Mitigation |
|------|-----------|
| Semgrep not installed on dev machines | Graceful skip with warning message |
| Semgrep CLI version differences | Parse only stable JSON fields; test against v1.x output format |
| Registry rules require internet | Support local `.semgrep.yml` as fallback |
| Slow scan on large file sets | Only scan staged files, not full project |
