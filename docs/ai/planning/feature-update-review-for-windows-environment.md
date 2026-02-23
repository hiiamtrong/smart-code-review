---
phase: planning
title: Planning - Update Review for Windows Environment
description: Task breakdown and implementation order for Windows compatibility
---

# Planning - Update Review for Windows Environment

## Milestones

- [x] Milestone 1: Platform abstraction layer complete and sourced by all scripts
- [x] Milestone 2: Installation works on Windows (Git Bash + PowerShell)
- [x] Milestone 3: Full review pipeline works on Windows (pre-commit + SonarQube + AI review)
- [x] Milestone 4: Documentation updated and tested

## Task Breakdown

### Phase 1: Foundation
- [x] Task 1.1: Create `scripts/lib/platform.sh` with all utility functions
- [x] Task 1.2: Add platform.sh sourcing to all scripts

### Phase 2: Core Fixes
- [x] Task 2.1: Fix `install.sh` - add MINGW/MSYS/CYGWIN detection, Windows jq install
- [x] Task 2.2: Fix `sonarqube-review.sh` - Windows scanner binary, unzip fallback, mktemp
- [x] Task 2.3: Fix `pre-commit.sh` - mktemp, CRLF stripping, process substitution, trap
- [x] Task 2.4: Fix `ai-review.sh` - mktemp, CRLF stripping
- [x] Task 2.5: Fix `detect-language.sh` - minor compatibility check
- [x] Task 2.6: Fix `filter-ignored-files.sh` - CRLF stripping in read loops
- [x] Task 2.7: Fix `enable-local-sonarqube.sh` - sed -i wrapper, Windows cases

### Phase 3: Integration & Polish
- [x] Task 3.1: Update `install.ps1` to copy platform.sh, verify wrappers
- [x] Task 3.2: Update SETUP_GUIDE.md with Windows instructions
- [x] Task 3.3: Update README.md with Windows support info
- [ ] Task 3.4: End-to-end testing across all shells

## Dependencies

- Task 1.1 must be completed before all Phase 2 tasks
- Phase 2 tasks are independent of each other
- Phase 3 depends on Phase 2 completion
- No external dependencies

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking macOS/Linux | High | Platform detection + conditional blocks; test on both |
| Git Bash version differences | Medium | Document minimum version (2.40+); test with common versions |
| CRLF corruption | Medium | `strip_cr()` utility in all read loops |
| Missing GNU tools | Medium | `check_required_tools()` at startup with clear install instructions |
| Scanner path differences | Medium | Platform-aware path construction |
