---
phase: requirements
title: Requirements - Toggle AI Review
description: Allow users to enable/disable AI review during setup, install, and via config command
---

# Requirements - Toggle AI Review

## Problem Statement

Currently, AI review **always runs** if `AI_GATEWAY_URL` and `AI_GATEWAY_API_KEY` are configured. There is no way to disable it without removing credentials. SonarQube has a toggle (`ENABLE_SONARQUBE_LOCAL`), but AI review does not.

**Who is affected?**
- Developers who want SonarQube-only analysis without AI review overhead
- Teams testing SonarQube integration without AI gateway costs
- Developers who want to temporarily disable AI review for faster commits

**Current workaround:** Remove `AI_GATEWAY_URL`/`AI_GATEWAY_API_KEY` from config, or use `git commit --no-verify` (skips everything).

## Goals & Objectives

**Primary goals:**
- Add `ENABLE_AI_REVIEW` config variable (true/false, default: true)
- Prompt during `ai-review setup` to enable/disable AI review
- Allow toggling via `ai-review config set ENABLE_AI_REVIEW true/false`

**Non-goals:**
- Per-project AI review toggle (only global config for now)
- Granular AI review settings (e.g., severity threshold)

## User Stories & Use Cases

- As a developer, I want to disable AI review during `ai-review setup` so that I only use SonarQube analysis
- As a developer, I want to run `ai-review config set ENABLE_AI_REVIEW false` so that I can temporarily disable AI review without losing my credentials
- As a developer, I want the pre-commit hook to skip AI review when disabled but still run SonarQube

## Success Criteria

- `ai-review setup` prompts "Enable AI Review? [Y/n]" (default: yes)
- `ai-review config set ENABLE_AI_REVIEW false` disables AI review
- Pre-commit hook skips AI review step when `ENABLE_AI_REVIEW=false`
- Pre-commit hook still runs SonarQube when AI review is disabled
- `ai-review config show` displays AI review status
- Backward compatible: existing configs without `ENABLE_AI_REVIEW` default to `true`

## Constraints & Assumptions

- Follow the same pattern as `ENABLE_SONARQUBE_LOCAL`
- Default to `true` for backward compatibility
- Config stored in `$HOME/.config/ai-review/config`

## Questions & Open Items

- None - straightforward feature following established patterns
