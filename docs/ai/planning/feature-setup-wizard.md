---
phase: planning
title: Setup Wizard - Planning
description: Task breakdown for implementing the interactive setup wizard
---

# Planning: Setup Wizard

## Milestones

- [x] Milestone 1: Requirements & design documentation
- [x] Milestone 2: Implementation of wizard flow
- [x] Milestone 3: Tests and validation

## Task Breakdown

### Phase 1: Prompt Helpers

- [x] Task 1.1: Add `promptBool(reader, label, defaultVal)` helper to `setup.go`
- [x] Task 1.2: Add `promptInt(reader, label, defaultVal)` helper to `setup.go`
- [x] Task 1.3: Extract `promptPassword(label, current)` from inline API key code
- [x] Task 1.4: Add `promptStringRequired(reader, label, current)` — re-prompts until non-empty
- [x] Task 1.5: Add `promptPasswordRequired(label, current)` — re-prompts until non-empty
- [x] Task 1.6: Add `printSetupSummary(cfg)` to render the confirmation table

### Phase 2: Wizard Flow

- [x] Task 2.1: Rewrite `runSetup` — Step 1: Feature flags (EnableAIReview, EnableSonarQube)
- [x] Task 2.2: Rewrite `runSetup` — Step 2: AI Gateway (conditional on EnableAIReview)
- [x] Task 2.3: Rewrite `runSetup` — Step 3: Gateway Behaviour (conditional on EnableAIReview)
- [x] Task 2.4: Rewrite `runSetup` — Step 4: SonarQube settings (conditional on EnableSonarQube)
- [x] Task 2.5: Rewrite `runSetup` — Summary display + confirmation prompt
- [x] Task 2.6: Add `--project` flag support via `setupProjectFlag` (separate from `configProjectFlag`)

### Phase 3: Tests

- [x] Task 3.1: Unit tests for `promptBool` with various inputs (6 tests)
- [x] Task 3.2: Unit tests for `promptInt` with valid/invalid inputs (3 tests)
- [x] Task 3.3: Integration test: full wizard flow with mocked stdin (AI-only)
- [x] Task 3.4: Integration test: full wizard flow with SonarQube enabled
- [x] Task 3.5: Integration test: both disabled — skips steps 2-4
- [x] Task 3.6: Integration test: abort at summary — no config written
- [x] Task 3.7: Verify existing tests still pass (all 12 packages green)

## Dependencies

- All prompt helpers (Phase 1) must be done before wizard flow (Phase 2)
- Phase 2 tasks are sequential (each step builds on the previous)
- Phase 3 can start after Phase 2 is complete

## Risks & Mitigation

| Risk | Mitigation |
|---|---|
| `term.ReadPassword` doesn't work in CI (no TTY) | Test with mocked stdin; skip password masking in tests |
| Users confused by too many prompts | Group into logical steps with clear headers |
| Breaking existing `setup` behaviour | The command signature is unchanged; only the interactive flow expands |
