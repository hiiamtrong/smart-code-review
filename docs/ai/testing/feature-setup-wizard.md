---
phase: testing
title: Setup Wizard - Testing Strategy
description: Test cases for the interactive setup wizard
---

# Testing: Setup Wizard

## Test Coverage Goals

- 100% coverage of new prompt helpers (`promptBool`, `promptInt`, `promptPassword`)
- Integration tests for the full wizard flow (AI-only and AI+SonarQube paths)
- `--project` flag save path tested

## Unit Tests

### promptBool
- [ ] Default true + empty input → returns true
- [ ] Default true + "n" input → returns false
- [ ] Default false + empty input → returns false
- [ ] Default false + "y" input → returns true
- [ ] Default false + "yes" input → returns true
- [ ] Case insensitive: "Y", "N" work

### promptInt
- [ ] Empty input → returns default
- [ ] Valid integer → returns parsed value
- [ ] Invalid input (letters) → returns default with warning

### promptStringRequired
- [ ] Non-empty input on first try → returns value
- [ ] Empty input with existing value → returns existing value
- [ ] Empty input with no existing value → re-prompts (test with value on second line)

### promptPasswordRequired
- [ ] Non-empty input → returns value
- [ ] Empty input with existing value → returns existing value
- [ ] Empty input with no existing value → re-prompts (test with value on second attempt)

### promptString (existing)
- [ ] Already covered in cmd_test.go

## Integration Tests

### Full wizard: AI-only (SonarQube disabled)
- [ ] Provide all inputs, SonarQube=no → config saved with AI fields, SonarQube fields at defaults
- [ ] Verify Steps 2-3 (AI Gateway, Gateway Behaviour) ARE shown
- [ ] Verify Step 4 (SonarQube) is NOT shown

### Full wizard: AI + SonarQube
- [ ] Provide all inputs, SonarQube=yes → config saved with all 13 fields
- [ ] Verify all 4 steps are shown

### Full wizard: Both disabled
- [ ] EnableAIReview=no, EnableSonarQube=no → only Step 1 shown, then summary
- [ ] Verify Steps 2-4 are all skipped

### Re-run wizard (existing config)
- [ ] Pre-populate config, run wizard with all-Enter → config unchanged

### Abort at summary
- [ ] Answer "n" at "Save configuration?" → config file not modified

### --project flag
- [ ] Run with --project in a git repo → project config created
- [ ] Run with --project outside git repo → error

## Test Data

Simulated stdin via `strings.NewReader` with newline-separated answers. Password function overridden via `readPasswordFn` package var.

## Manual Testing

- [ ] Run `ai-review setup` on macOS terminal — verify masked password input
- [ ] Run `ai-review setup` on Windows cmd — verify masked password input
- [ ] Run `ai-review setup --project` inside a git repo — verify project config created
