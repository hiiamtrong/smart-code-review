---
phase: requirements
title: Migrate from Bash Scripts to Go Binary
description: Replace the existing bash script toolchain with a single cross-platform Go binary
---

# Requirements & Problem Understanding

## Problem Statement
**What problem are we solving?**

- The current toolchain consists of ~7 bash scripts (`ai-review`, `pre-commit.sh`, `install.sh`, `ai-review.sh`, `detect-language.sh`, `filter-ignored-files.sh`, `sonarqube-review.sh`) plus a `platform.sh` abstraction layer. Managing cross-platform compatibility in bash is fragile and complex.
- Who is affected: developers installing `ai-review` on Windows (Git Bash/MSYS), macOS, and Linux. Windows users in particular experience issues with `mktemp`, `sed -i`, CRLF line endings, `jq` availability, and ANSI color codes.
- Current situation/workaround: A dedicated `scripts/lib/platform.sh` file with wrappers (`safe_mktemp`, `safe_sed_inplace`, `safe_unzip`, `apply_color_settings`, `strip_cr`) patches over OS differences. External dependencies (`jq`, `curl`, `gawk`) must be installed separately, and the installer attempts auto-install via `brew`/`apt`/`winget`/`choco`/`scoop`.

## Goals & Objectives
**What do we want to achieve?**

**Primary goals:**
- Replace all bash scripts with a single, statically-compiled Go binary (`ai-review`)
- Zero external runtime dependencies (no `jq`, `curl`, `gawk`, `awk` required by end users)
- Identical CLI interface (`ai-review setup|install|uninstall|config|status|update|help`)
- Preserve all existing behavior: pre-commit hook, AI Gateway streaming SSE, SonarQube integration, `.aireviewignore` filtering, `reviewdog` output format

**Secondary goals:**
- Improve startup time (Go binary vs bash + subprocess overhead)
- Enable proper unit testing of core logic
- Simplify the installer to a single binary download + PATH setup
- Enable future features: auto-update, richer TUI, progress bars, retry logic

**Non-goals:**
- Changing the AI Gateway API contract
- Changing the `.aireviewignore` file format
- Changing the `reviewdog` JSONL output format
- Rewriting the gateway server side
- Changing SonarQube scanner invocation (still shells out to `sonar-scanner`)

## User Stories & Use Cases

- **As a Windows developer**, I want to install `ai-review` without needing Git Bash or WSL, so that I can use it in native PowerShell/CMD.
- **As a macOS/Linux developer**, I want `ai-review install` to work without needing `jq` or `gawk` pre-installed, so that setup is frictionless.
- **As a developer**, I want the pre-commit hook to run as fast as possible, so that my commit workflow is not disrupted.
- **As a CI/CD engineer**, I want `ai-review` to run in GitHub Actions without extra `apt install` steps, so that the workflow YAML is simpler.
- **As a project maintainer**, I want to add unit tests for the diff filtering and language detection logic, so that regressions are caught automatically.

**Key workflows:**
1. Install: `curl -sSL .../install.sh | bash` → downloads single Go binary → adds to PATH
2. Setup: `ai-review setup` → interactive credential configuration
3. Repo hook install: `ai-review install` → writes pre-commit hook that calls `ai-review run-hook`
4. Pre-commit: git triggers hook → binary reads staged diff → filters ignored files → calls AI Gateway (SSE) → displays results → blocks commit on ERROR
5. CI review: GitHub Actions calls `ai-review ci-review` → reads PR diff → posts reviewdog output

**Edge cases:**
- Empty diff (no staged changes) → exit 0 silently
- All files filtered by `.aireviewignore` → exit 0 with message
- AI Gateway unreachable → configurable: block commit or warn and allow
- Windows: no ANSI support in old CMD → detect and strip color codes
- Large diff (>100k chars) → chunk handling (already done in bash via SSE streaming)
- New repo with no remote → fallback to staged diff

## Success Criteria
**How will we know when we're done?**

- [ ] Single Go binary (`ai-review`) passes all existing bash integration tests
- [ ] Binary runs on macOS (arm64, amd64), Linux (amd64, arm64), Windows (amd64) without additional dependencies
- [ ] All CLI commands (`setup`, `install`, `uninstall`, `config`, `status`, `update`, `help`) behave identically to the bash version
- [ ] Pre-commit hook produces identical output format (colors, summary, exit codes)
- [ ] AI Gateway SSE streaming works end-to-end
- [ ] `.aireviewignore` filtering passes existing test cases
- [ ] Unit test coverage ≥ 80% on core packages
- [ ] Binary size ≤ 15 MB (uncompressed)
- [ ] Install/uninstall roundtrip works on all three OSes

## Constraints & Assumptions

**Technical constraints:**
- Go version ≥ 1.22 (for range-over-int, improved stdlib)
- Must use CGO-free build (`CGO_ENABLED=0`) for fully static binaries
- Must preserve `~/.config/ai-review/config` file format (shell-sourceable `KEY="VALUE"` pairs) for backwards compatibility — or provide migration
- Pre-commit hook file written into `.git/hooks/pre-commit` (or `.husky/`) must remain a shell script that delegates to the binary
- `reviewdog` is still an external tool; the binary prepares JSONL and pipes to it (or invokes it directly)
- SonarQube scanner (`sonar-scanner`) is still an external tool

**Assumptions:**
- Go toolchain available in the build environment (CI, not required by end users)
- The AI Gateway API contract (`/review` multipart POST, SSE response) remains stable
- Config file at `~/.config/ai-review/config` may need a parser that reads `KEY="VALUE"` shell syntax

## Questions & Open Items

- **Config format migration**: Should the Go binary continue reading the shell-sourceable `KEY="VALUE"` config, or migrate to a proper format (TOML/YAML/JSON)? If migrating, provide automatic conversion on first run.
- **reviewdog invocation**: Should the binary shell out to `$HOME/bin/reviewdog` (current behavior) or bundle reviewdog's output posting logic?
- **Windows installer**: Replace `install.sh` with a PowerShell script + Go binary, or keep install.sh but have it download the Go binary?
- **Auto-update**: Should `ai-review update` download a new binary from GitHub releases, replacing itself?
- **Hook delegation**: The pre-commit hook script should be minimal — just `ai-review run-hook` — or should it remain a full bash script?
