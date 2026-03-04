---
phase: requirements
title: Per-Project Configuration
description: Allow each project to have its own ai-review configuration stored in the user's config home
---

# Per-Project Configuration

> **Note:** This feature supersedes the non-goal in [feature-toggle-ai-review](feature-toggle-ai-review.md) which stated _"Per-project AI review toggle (only global config for now)"_.

## Problem Statement

Currently ai-review has a single global config file (`~/.config/ai-review/config`) shared across all repositories. This means:

- All repos use the same AI Gateway URL, API key, model, and provider
- Enabling/disabling AI review or SonarQube is an all-or-nothing setting
- Developers working on multiple projects with different endpoints or models must manually re-run `ai-review setup` each time they switch repos

There is a partial per-repo override via `git config --local aireview.*`, but it only covers `sonarProjectKey` and `enableSonarQube` — not credentials, model, or other settings.

## Goals & Objectives

**Primary goals:**
- Allow every config key to be set on a per-project basis
- Config stored in the user's config home directory (`~/.config/ai-review/projects/<id>/config`) to avoid leaking secrets into repos
- Clear, predictable config resolution order: **env vars > git-local > per-project config > global config > defaults**

**Secondary goals:**
- `ai-review config` subcommand gains `--project` awareness (show/set project-level values)
- `ai-review status` displays which values come from which layer (global vs project)
- Seamless experience: when running inside a git repo with a project config, it applies automatically

**Non-goals:**
- Sharing config via committed files in the repo (user explicitly chose not to share)
- Config inheritance between projects
- Remote/cloud config sync

## User Stories & Use Cases

1. **As a developer working on multiple projects**, I want each project to use a different AI Gateway endpoint and API key, so that I don't need to re-run `ai-review setup` when switching repos.

2. **As a developer**, I want to enable SonarQube only on certain projects (where a SonarQube server is available), while keeping AI-only review on others.

3. **As a developer**, I want to use different AI models per project (e.g., `gemini-2.0-flash` for a fast project, `gpt-4o` for a critical codebase).

4. **As a developer**, I want `ai-review config` to show me which config values are active for the current project and where they come from.

5. **As a developer**, I want `ai-review config set --project KEY VALUE` to set a project-level override without touching my global config.

### Edge Cases

- **Repo moved/renamed on disk**: project ID is based on the repo root path at config-creation time. If the repo moves, the old project config becomes orphaned. User can run `config list-projects` to find stale entries and `config remove-project` to clean up, then re-create config at the new location.
- **Symlinked repo root**: project ID must canonicalize the path (resolve symlinks) before hashing, so two symlinks to the same directory produce the same project ID.
- **Running from a subdirectory**: `git rev-parse --show-toplevel` always returns the repo root regardless of cwd, so project detection works from any subdirectory.
- **Monorepos with nested git repos**: each nested `.git` directory is a separate repo root and gets its own project ID. The innermost repo root is used.

## Success Criteria

- [ ] Running `ai-review config set --project AI_MODEL gpt-4o` inside repo A saves a project-level config
- [ ] Running `ai-review run-hook` inside repo A uses `gpt-4o`; inside repo B (no project config) uses the global model
- [ ] `ai-review config` (no args) inside a project shows merged config with source indicators
- [ ] `ai-review status` shows project config path when active
- [ ] All config keys supported by `config get/set` are overridable per project (currently 13 keys — see list below)
- [ ] Existing `git config --local aireview.*` overrides remain functional (backwards compatible)
- [ ] Config resolution order is respected: env > git-local > project > global > defaults

### Config Keys (exhaustive list)

| Key | Type | Default |
|-----|------|---------|
| `AI_GATEWAY_URL` | string | (empty) |
| `AI_GATEWAY_API_KEY` | string | (empty) |
| `AI_MODEL` | string | `gemini-2.0-flash` |
| `AI_PROVIDER` | string | `google` |
| `ENABLE_AI_REVIEW` | bool | `true` |
| `ENABLE_SONARQUBE_LOCAL` | bool | `false` |
| `BLOCK_ON_GATEWAY_ERROR` | bool | `true` |
| `GATEWAY_TIMEOUT_SEC` | int | `120` |
| `SONAR_HOST_URL` | string | (empty) |
| `SONAR_TOKEN` | string | (empty) |
| `SONAR_PROJECT_KEY` | string | (empty) |
| `SONAR_BLOCK_ON_HOTSPOTS` | bool | `true` |
| `SONAR_FILTER_CHANGED_LINES_ONLY` | bool | `true` |

## Constraints & Assumptions

- **Storage**: per-project config uses the same shell `KEY="VALUE"` format as global config
- **Project identification**: SHA-256 hash of canonical repo root path, truncated to 12 hex chars — filesystem-safe and collision-resistant
- **No repo-committed files**: secrets stay in `~/.config/ai-review/`, never in the repo
- **Backwards compatibility**: existing global config and git-local overrides continue to work unchanged
- **Platform support**: must work on macOS, Linux, and Windows (same paths as global config)

## Questions & Open Items

- [x] ~~Project identifier: use a hash of the repo root path, or a sanitized directory name?~~ **Resolved:** SHA-256 hash, first 12 hex chars. Collision-safe and filesystem-safe.
- [ ] Should `ai-review setup` gain a `--project` flag to run the interactive wizard for project config? (Deferred — can be added later without breaking changes)
- [x] ~~Should there be an `ai-review config list-projects` to show all projects with overrides?~~ **Resolved:** Yes, included in design as `config list-projects` and `config remove-project`.
