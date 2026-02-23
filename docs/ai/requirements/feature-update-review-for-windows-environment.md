---
phase: requirements
title: Requirements - Update Review for Windows Environment
description: Full Windows compatibility for the smart-code-review tool
---

# Requirements - Update Review for Windows Environment

## Problem Statement
**What problem are we solving?**

- Internal Sotatek developers on Windows cannot reliably use the smart-code-review tool
- `install.sh` immediately fails on Git Bash with "Unsupported operating system" (no MINGW/MSYS detection)
- SonarQube scanner downloads the wrong binary for Windows (no Windows-specific zip case)
- GNU tool dependencies (mktemp, sed, awk, etc.) are not validated on Windows
- CRLF line endings cause silent failures in `while read` loops and here-documents
- Process substitution `< <(...)` is unreliable in some Git Bash versions
- Output capture and display breaks in Windows shells (partially fixed in recent commits)

## Goals & Objectives
**What do we want to achieve?**

- **Primary**: All review scripts work end-to-end on Windows via Git Bash
- **Primary**: PowerShell and CMD users can install and invoke the tool via wrappers
- **Primary**: WSL users have a seamless Linux-native experience
- **Secondary**: Centralized platform abstraction layer to simplify future OS-specific changes
- **Secondary**: Comprehensive Windows setup documentation for the internal team
- **Non-goals**: Native PowerShell reimplementation of all bash scripts; support for Windows versions below 10

## User Stories & Use Cases
**How will users interact with the solution?**

- As a Windows developer using Git Bash, I want to run `bash install.sh` so that the tool installs successfully
- As a Windows developer using PowerShell, I want to run `install.ps1` so that the tool sets up with all dependencies
- As a Windows developer, I want to run `ai-review install` in any shell (Git Bash, PowerShell, CMD) so that the pre-commit hook activates
- As a Windows developer, I want `git commit` to trigger the pre-commit review without errors so that I get code feedback
- As a Windows developer, I want SonarQube analysis to download and run the correct Windows scanner binary
- As a WSL user, I want the standard Linux installation to work unchanged
- As a macOS/Linux user, I want all existing functionality to remain unaffected

## Success Criteria
**How will we know when we're done?**

- `bash install.sh` completes on Windows Git Bash without errors
- `powershell install.ps1` completes on Windows PowerShell without errors
- Pre-commit hook triggers and produces review output on all 4 shells
- SonarQube scanner downloads the `windows-x64` zip and runs successfully
- ANSI colors display correctly (or degrade gracefully) in all terminals
- No regressions on macOS or Linux
- SETUP_GUIDE.md documents Windows installation for all shell environments

## Constraints & Assumptions
**What limitations do we need to work within?**

- Git Bash (from Git for Windows) is a required dependency - we don't rewrite scripts in PowerShell
- Scripts must remain backward compatible with macOS and Linux
- Developers have Git for Windows 2.40+ installed
- Java 11+ is required for SonarQube (documented separately)
- Windows 10+ is the minimum supported version

## Questions & Open Items
**What do we still need to clarify?**

- All key questions have been resolved during planning
