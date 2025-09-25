# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Smart Code Review GitHub Action** that automatically detects project language/framework and runs appropriate code review tools with reviewdog integration. The action combines traditional linting tools with AI-powered code review using an AI Gateway service.

## Architecture

- **Composite Action**: Uses `action.yml` configuration with shell script execution
- **Two-stage Process**:
  1. `detect-language.sh` - Detects language and runs appropriate linters
  2. `ai-review.sh` - Performs AI-powered review via gateway and formats output for reviewdog
- **Language Support**: Node.js/TS, Python, Java, Go, .NET with corresponding linters
- **AI Gateway Integration**: Uses external AI gateway service instead of direct API calls

## Key Commands

### Testing Scripts Locally
```bash
# Test language detection
bash scripts/detect-language.sh

# Test AI review (requires environment variables)
export AI_GATEWAY_URL="https://your-gateway.com/api/review"
export AI_GATEWAY_API_KEY="your-api-key"
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

- `GITHUB_TOKEN`: Required for reviewdog PR comments (automatically provided in GitHub Actions)
- `AI_GATEWAY_URL`: Required for AI Gateway service endpoint
- `AI_GATEWAY_API_KEY`: Required for AI Gateway authentication
- `AI_MODEL`: Optional AI model selection (default: gemini-2.0-flash)
- `AI_PROVIDER`: Optional AI provider selection (default: google)

**Important**: The script automatically sets `REVIEWDOG_GITHUB_API_TOKEN=$GITHUB_TOKEN` for reviewdog integration.

## Dependencies

The action automatically installs:
- **reviewdog**: Downloaded via official install script to `$HOME/bin`

External tools used:
- `curl`: API calls and downloads
- `jq`: JSON processing (ensure available in environment)
- `git`: Diff generation and repository operations

## Testing Strategy

### **GitHub Actions Testing (Recommended)**
1. **Repository Setup**: Ensure proper permissions in workflow and repository settings
2. **Create PR**: Push changes and create pull request to trigger action
3. **Check Results**: Review AI-powered comments posted directly to PR

### **Local Testing**
1. **Script Testing**: Run with environment variables - uses local reporter automatically
2. **With GitHub API**: Use Personal Access Token for full GitHub integration testing
3. **Without API**: Review output saved to `ai-output.json`

### **Required Permissions**
- **Workflow**: `contents: read`, `pull-requests: write`, `checks: write`
- **Repository Settings**: Enable "Read and write permissions" for GitHub Actions
- **Local Testing**: Personal Access Token with `repo` and `pull_requests` scopes

## File Structure

- `action.yml`: GitHub Action configuration and inputs
- `scripts/detect-language.sh`: Language detection logic
- `scripts/ai-review.sh`: AI review implementation with AI Gateway integration
- `README.md`: User-facing documentation
- `REQUIREMENT.md`: Detailed Vietnamese requirements specification