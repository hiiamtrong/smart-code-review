---
phase: requirements
title: Setup Wizard - Interactive Configuration
description: Replace the minimal setup flow with a full interactive wizard that covers all config options
---

# Requirements: Setup Wizard

## Problem Statement

The current `ai-review setup` command only asks for 4 fields (AI Gateway URL, API Key, Model, Provider) and silently enables AI review by default. Users have no visibility into:

- Whether AI review or SonarQube review is enabled
- SonarQube connection settings (host URL, token, project key)
- Gateway behaviour settings (timeout, block-on-error)
- Advanced SonarQube options (block hotspots, filter changed lines)

Users must manually run `ai-review config set KEY VALUE` for each remaining field, which is error-prone and undiscoverable.

## Goals & Objectives

**Primary goals:**
- Provide a step-by-step interactive wizard that walks users through ALL config options
- Group related settings into logical sections (AI Review, SonarQube, Gateway Behaviour)
- Conditionally show SonarQube settings only when SonarQube is enabled
- Show sensible defaults and allow users to accept them by pressing Enter

**Secondary goals:**
- Support both first-time setup and re-configuration (pre-fill existing values)
- Show a summary of all settings before saving
- Support `--project` flag to save wizard output to project config instead of global

**Non-goals:**
- Changing the config file format (still shell KEY="VALUE")
- Adding a TUI framework (keep it simple stdin/stdout prompts)
- Removing the existing `config set` command (wizard is additive)

## User Stories & Use Cases

1. **As a new user**, I want `ai-review setup` to ask me everything I need so that I don't have to discover config keys manually.

2. **As a user enabling SonarQube**, I want the wizard to ask for SonarQube host, token, and project key only after I say "yes" to enabling SonarQube, so that irrelevant questions are skipped.

3. **As an existing user re-running setup**, I want my current values shown as defaults so that I only change what I need.

4. **As a project maintainer**, I want `ai-review setup --project` to save the wizard output as a per-project config override.

## Wizard Flow

```
ai-review setup

  AI Review Setup
  ════════════════════════════════════════

  ── Step 1: Feature Flags ──
  Enable AI Review? [Y/n]: Y
  Enable SonarQube Review? [y/N]: y

  ── Step 2: AI Gateway ──               ← only if AI Review enabled
  AI Gateway URL (required): https://gateway.example.com/api
  AI Gateway API Key [****]: <masked input>
  AI Model [gemini-2.0-flash]:
  AI Provider [google]:

  ── Step 3: Gateway Behaviour ──        ← only if AI Review enabled
  Block commit on gateway error? [Y/n]:
  Gateway timeout (seconds) [120]:

  ── Step 4: SonarQube Settings ──       ← only if SonarQube enabled
  SonarQube Host URL (required): https://sonar.example.com
  SonarQube Token (required): <masked input>
  SonarQube Project Key (required): my-project
  Block commit on security hotspots? [Y/n]:
  Filter changed lines only? [Y/n]:

  ── Summary ──
  ENABLE_AI_REVIEW         true
  ENABLE_SONARQUBE_LOCAL   true
  AI_GATEWAY_URL           https://gateway.example.com/api
  AI_GATEWAY_API_KEY       ****
  AI_MODEL                 gemini-2.0-flash
  AI_PROVIDER              google
  BLOCK_ON_GATEWAY_ERROR   true
  GATEWAY_TIMEOUT_SEC      120
  SONAR_HOST_URL           https://sonar.example.com
  SONAR_TOKEN              ****
  SONAR_PROJECT_KEY        my-project
  SONAR_BLOCK_ON_HOTSPOTS  true
  SONAR_FILTER_CHANGED_LINES_ONLY true

  Save configuration? [Y/n]: Y
  ✓ Configuration saved to ~/.config/ai-review/config
```

## Success Criteria

1. `ai-review setup` prompts for all config fields in logical order
2. Steps 2-3 (AI Gateway, Gateway Behaviour) are skipped when AI Review is disabled
3. Step 4 (SonarQube) is skipped when SonarQube Review is disabled
4. Existing config values are shown as defaults; pressing Enter keeps them
5. Required fields without existing values cannot be left empty — re-prompt until a value is provided
6. Sensitive fields (API key, Sonar token) use masked input
7. SonarQube Token is required when SonarQube is enabled
8. A summary is displayed before saving; user can abort
9. `--project` flag saves to project config instead of global
10. All existing tests continue to pass
11. New unit tests cover the wizard flow with mocked stdin

## Constraints & Assumptions

- **No external TUI libraries** — use only `bufio.Reader` and `term.ReadPassword` (already in use)
- **Backwards compatible** — existing `config set/get` commands unchanged
- The wizard replaces the current `runSetup` function body, keeping the same cobra command
- Boolean prompts use `[Y/n]` (default yes) or `[y/N]` (default no) convention

## Questions & Open Items

- None — the user has clearly specified the desired behaviour
