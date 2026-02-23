---
phase: planning
title: Planning - Toggle AI Review
description: Task breakdown for AI review toggle feature
---

# Planning - Toggle AI Review

## Milestones

- [ ] Milestone 1: Config and CLI support for ENABLE_AI_REVIEW
- [ ] Milestone 2: Pre-commit hook respects the toggle
- [ ] Milestone 3: Documentation updated

## Task Breakdown

### Phase 1: CLI & Config
- [ ] Task 1.1: Add `ENABLE_AI_REVIEW` prompt to `cmd_setup()` in `scripts/local/ai-review`
- [ ] Task 1.2: Add `ENABLE_AI_REVIEW` to config help text in `scripts/local/ai-review`

### Phase 2: Pre-commit Hook
- [ ] Task 2.1: Add `ENABLE_AI_REVIEW` default to `load_config()` in `scripts/local/pre-commit.sh`
- [ ] Task 2.2: Add toggle check before AI review execution in `pre-commit.sh`
- [ ] Task 2.3: Update output messages to show AI review status

### Phase 3: Documentation
- [ ] Task 3.1: Update SETUP_GUIDE.md with AI review toggle info
- [ ] Task 3.2: Update README.md

## Dependencies

- No external dependencies
- All tasks are sequential within each phase
- Phases can be done in order 1 → 2 → 3

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing configs | Medium | Default to `true` when variable is absent |
| Confusing UX with two toggles | Low | Clear prompts and help text |
