---
phase: testing
title: Testing - Update Review for Windows Environment
description: Testing strategy for Windows compatibility
---

# Testing - Update Review for Windows Environment

## Test Coverage Goals

- All platform utility functions tested on Windows Git Bash
- Installation flow tested on Git Bash and PowerShell
- Pre-commit hook tested end-to-end on Windows
- SonarQube scanner download tested for Windows binary
- Regression testing on macOS and Linux

## Unit Tests

### scripts/lib/platform.sh
- [ ] `detect_platform()` returns `windows` on MINGW/MSYS/CYGWIN
- [ ] `detect_platform()` returns `macos` on Darwin
- [ ] `detect_platform()` returns `linux` on Linux
- [ ] `detect_platform()` returns `wsl` on WSL
- [ ] `safe_mktemp()` creates a valid temp file on all platforms
- [ ] `safe_sed_inplace()` performs in-place edit on all platforms
- [ ] `check_color_support()` detects terminal capabilities
- [ ] `strip_cr()` removes carriage returns from input
- [ ] `check_required_tools()` reports missing tools clearly
- [ ] `normalize_path()` converts backslashes to forward slashes

## Integration Tests

### Installation Flow
- [ ] `bash install.sh` completes on Windows Git Bash
- [ ] `powershell install.ps1` completes on Windows PowerShell
- [ ] Dependencies (jq, curl, git) validated and installed
- [ ] Config directory created at correct location
- [ ] Hook template copied to correct location

### Pre-commit Flow
- [ ] `git commit` triggers pre-commit hook on Windows
- [ ] AI review produces output without errors
- [ ] SonarQube analysis runs with correct scanner binary
- [ ] Output displays with colors (or graceful fallback)

### SonarQube Scanner
- [ ] Downloads `sonar-scanner-cli-*-windows-x64.zip` on Windows
- [ ] Falls back to PowerShell `Expand-Archive` if `unzip` unavailable
- [ ] Scanner binary path resolves to `bin/sonar-scanner.bat` on Windows

## End-to-End Tests

### Git Bash E2E
- [ ] Install tool via `bash install.sh`
- [ ] Configure credentials
- [ ] Enable pre-commit hook in a test repo
- [ ] Make a commit and verify review output
- [ ] Enable SonarQube and verify analysis

### PowerShell E2E
- [ ] Install tool via `install.ps1`
- [ ] Use `ai-review.cmd` wrapper
- [ ] Use `enable-local-sonarqube.cmd` wrapper

### CMD E2E
- [ ] Use `ai-review.cmd` wrapper from CMD
- [ ] Verify output displays correctly

### WSL E2E
- [ ] Standard Linux install flow works unchanged

### Regression
- [ ] macOS installation and commit flow unchanged
- [ ] Linux installation and commit flow unchanged

## Manual Testing

- [ ] Verify ANSI colors in Git Bash
- [ ] Verify ANSI colors in PowerShell (or graceful degradation)
- [ ] Verify ANSI colors in CMD (or graceful degradation)
- [ ] Test with paths containing spaces
- [ ] Test with non-ASCII filenames
