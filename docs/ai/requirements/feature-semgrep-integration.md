---
phase: requirements
title: Semgrep Integration
description: Integrate Semgrep as a local static analysis tool in the pre-commit hook
---

# Requirements: Semgrep Integration

## Problem Statement

SonarQube Community Edition only supports one branch (main). Running `sonar-scanner` in the pre-commit hook overwrites the main branch analysis on the server, corrupting project metrics (e.g. `ncloc` drops, issues disappear). The current workaround is fetch-only mode — querying the SonarQube API for existing issues — but this **cannot detect new bugs** introduced by the current commit.

**Who is affected?** All developers using `ai-review` pre-commit hooks who want local static analysis to catch bugs before they reach the server.

**Current workaround:** SonarQube fetch-only queries existing server issues on staged files. No local scanning is performed.

## Goals & Objectives

**Primary goals:**
- Run Semgrep locally on staged files during pre-commit to detect new bugs, security issues, and code smells
- Convert Semgrep findings into the existing `gateway.Diagnostic` format for unified display
- Support configurable rule sets (registry rules, custom `.semgrep.yml`)
- Keep Semgrep optional — if not installed or not configured, skip gracefully

**Secondary goals:**
- Auto-detect Semgrep binary (PATH, common install locations)
- Allow severity mapping (Semgrep ERROR/WARNING/INFO to ai-review severity)
- Support scanning only staged files (not full project) for speed

**Non-goals:**
- Replacing SonarQube entirely (SonarQube fetch-only still provides value for server-tracked issues)
- Building a Semgrep rule management UI
- Semgrep AppSec Platform integration (SaaS dashboard)

## User Stories & Use Cases

1. **As a developer**, I want Semgrep to scan my staged files during pre-commit so that I catch bugs before they're committed.
2. **As a developer**, I want to use Semgrep's built-in registry rules (e.g. `p/security-audit`, `p/owasp-top-ten`) without writing custom rules.
3. **As a developer**, I want to add custom `.semgrep.yml` rules in my repo that are automatically picked up.
4. **As a team lead**, I want to block commits that have Semgrep ERROR-level findings.
5. **As a developer**, I want Semgrep results displayed in the same format as AI review and SonarQube findings.

## Success Criteria

- Semgrep runs on staged files only (not full project) in < 10 seconds for typical commits
- Findings are displayed using the existing `display.PrintIssue` format
- ERROR-level Semgrep findings block the commit (configurable)
- Setup wizard includes Semgrep configuration step
- Works alongside SonarQube (both can be enabled simultaneously)
- Graceful skip when Semgrep is not installed

## Constraints & Assumptions

- Semgrep CLI must be installed separately by the user (`pip install semgrep` or `brew install semgrep`)
- Semgrep OSS is free and open-source (LGPL-2.1)
- Semgrep outputs JSON via `--json` flag, which we parse
- Semgrep can scan specific files via command-line arguments
- No server/network required for local rules; registry rules need internet on first download (cached after)

## Questions & Open Items

- Should we auto-install Semgrep if not found? (Probably not — user's responsibility)
- Default rule set: use `auto` (Semgrep's auto-detect) or specific rulesets like `p/default`?
- Should we support `.semgrepignore` in addition to `.aireviewignore`?
