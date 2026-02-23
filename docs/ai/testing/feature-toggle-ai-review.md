---
phase: testing
title: Testing - Toggle AI Review
description: Test cases for AI review toggle
---

# Testing - Toggle AI Review

## Manual Testing Checklist

### Setup Flow
- [ ] `ai-review setup` prompts for AI review enable/disable
- [ ] Answering "n" saves `ENABLE_AI_REVIEW="false"` to config
- [ ] Answering "y" or Enter saves `ENABLE_AI_REVIEW="true"` to config
- [ ] Credentials are still collected when AI review is disabled

### Config Command
- [ ] `ai-review config show` displays `ENABLE_AI_REVIEW` status
- [ ] `ai-review config set ENABLE_AI_REVIEW false` disables AI review
- [ ] `ai-review config set ENABLE_AI_REVIEW true` enables AI review

### Pre-commit Hook
- [ ] With `ENABLE_AI_REVIEW=true`: AI review runs normally
- [ ] With `ENABLE_AI_REVIEW=false`: AI review is skipped, info message shown
- [ ] With `ENABLE_AI_REVIEW` absent from config: defaults to `true` (backward compatible)
- [ ] SonarQube still runs regardless of AI review toggle
- [ ] Both disabled: hook exits early with info message

### Edge Cases
- [ ] Existing config without `ENABLE_AI_REVIEW` defaults to enabled
- [ ] Config with `ENABLE_AI_REVIEW=""` defaults to enabled
