# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Smart Code Review GitHub Action** that automatically detects project language/framework and runs appropriate code review tools with reviewdog integration. The action combines traditional linting tools with AI-powered code review using OpenAI's API.

## Architecture

- **Composite Action**: Uses `action.yml` configuration with shell script execution
- **Two-stage Process**:
  1. `detect-language.sh` - Detects language based on config files (package.json, requirements.txt, etc.)
  2. `ai-review.sh` - Performs AI-powered review and formats output for reviewdog
- **Language Support**: Node.js/TS, Python, Java, Go, .NET with corresponding linters

## Key Commands

### Testing Scripts Locally
```bash
# Test language detection
bash scripts/detect-language.sh

# Test AI review (requires environment variables)
export OPENAI_API_KEY="your-key"
export GITHUB_TOKEN="your-token"
bash scripts/ai-review.sh
```

### Development Setup
```bash
# Make scripts executable
chmod +x scripts/*.sh

# Validate shell script syntax
bash -n scripts/detect-language.sh
bash -n scripts/ai-review.sh
```

## Code Conventions

- Use `#!/usr/bin/env bash` and `set -e` in shell scripts
- Environment variables in UPPERCASE
- Informative echo messages with emoji prefixes (🔎, ➡️)
- Error handling with proper exit codes
- Use `$(dirname "$0")` for relative script paths

## Required Environment Variables

- `GITHUB_TOKEN`: Required for reviewdog PR comments
- `OPENAI_API_KEY`: Required for AI-powered code review functionality

## Dependencies

The action automatically installs:
- **reviewdog**: Downloaded via official install script to `$HOME/bin`

External tools used:
- `curl`: API calls and downloads
- `jq`: JSON processing (ensure available in environment)
- `git`: Diff generation and repository operations

## Testing Strategy

Since this is a GitHub Action:
1. **Manual Script Testing**: Run individual scripts with proper environment variables
2. **Integration Testing**: Test full workflow in a test repository with PR
3. **Local Action Testing**: Use `act` tool if available for local GitHub Actions testing

## File Structure

- `action.yml`: GitHub Action configuration and inputs
- `scripts/detect-language.sh`: Language detection logic
- `scripts/ai-review.sh`: AI review implementation with OpenAI integration
- `README.md`: User-facing documentation
- `REQUIREMENT.md`: Detailed Vietnamese requirements specification