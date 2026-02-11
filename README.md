# Smart Code Review GitHub Action

**AI-Powered Code Review with Language Detection and Linting Integration**

This GitHub Action automatically detects your project's language/framework and combines traditional linting tools with AI-powered code review using [reviewdog](https://github.com/reviewdog/reviewdog) for seamless PR integration.

## Features

- **Language Detection**: Automatically detects project type based on configuration files
- **AI-Powered Review**: Intelligent code analysis using configurable AI models
- **SonarQube Integration**: Combine static analysis with AI review ([Setup Guide](SONARQUBE_INTEGRATION.md))
- **Traditional Linting**: Integrates with popular linters for each language
- **Smart Diff Analysis**: Reviews only changes in PRs or since last push
- **Inline Comments**: Posts review comments directly on PR lines
- **Configurable**: Flexible AI model and provider selection

## Supported Languages & Tools

| Language               | Linters                     | AI Review |
| ---------------------- | --------------------------- | --------- |
| **Node.js/TypeScript** | ESLint                      | Yes       |
| **Python**             | ruff → flake8 → pylint      | Yes       |
| **Java**               | Checkstyle (Maven/Gradle)   | Yes       |
| **Go**                 | staticcheck → golangci-lint | Yes       |
| **.NET**               | dotnet format               | Yes       |

## Quick Start

### Basic Usage

```yaml
name: Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  code-review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      checks: write
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run Smart Code Review
        uses: hiiamtrong/smart-code-review@v1
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          ai_gateway_url: ${{ secrets.AI_GATEWAY_URL }}
          ai_gateway_api_key: ${{ secrets.AI_GATEWAY_API_KEY }}
```

### Advanced Configuration

```yaml
      - name: Run Smart Code Review
        uses: hiiamtrong/smart-code-review@v1
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
          ai_gateway_url: ${{ secrets.AI_GATEWAY_URL }}
          ai_gateway_api_key: ${{ secrets.AI_GATEWAY_API_KEY }}
          ai_model: "claude-3-sonnet"     # Optional: AI model selection
          ai_provider: "anthropic"        # Optional: AI provider selection
```

## Configuration

### Ignore Files from Review

Create a `.aireviewignore` file in your repository root to exclude files from AI review:

```gitignore
# Dependencies and lock files
package-lock.json
yarn.lock
*.lock

# Build outputs
dist/*
build/*
*.min.js

# Generated files
*.generated.*

# Documentation
*.md
docs/*
```

The syntax is similar to `.gitignore`:
- Use `*` for wildcards: `*.test.js`
- Use `**` for directory matching: `**/fixtures/*`
- Use `#` for comments
- One pattern per line

### Required Inputs

| Input                | Description                  | Example                                  |
| -------------------- | ---------------------------- | ---------------------------------------- |
| `github_token`       | GitHub token for PR comments | `${{ secrets.GITHUB_TOKEN }}`            |
| `ai_gateway_url`     | AI Gateway service endpoint  | `https://gateway.example.com/api/review` |
| `ai_gateway_api_key` | API key for AI Gateway       | `${{ secrets.AI_GATEWAY_API_KEY }}`      |

### Optional Inputs

| Input         | Description     | Default            |
| ------------- | --------------- | ------------------ |
| `ai_model`    | AI model to use | `gemini-2.0-flash` |
| `ai_provider` | AI provider     | `google`           |

### Required Repository Settings

1. **Workflow Permissions**:
   ```yaml
   permissions:
     contents: read
     pull-requests: write
     checks: write
   ```

2. **Repository Settings**:
   - Go to Settings → Actions → General
   - Set "Workflow permissions" to "Read and write permissions"

### Required Secrets

Add these secrets to your repository (Settings → Secrets and variables → Actions):

- `AI_GATEWAY_URL`: Your AI Gateway service endpoint
- `AI_GATEWAY_API_KEY`: Authentication key for your AI Gateway

## How It Works

1. **Language Detection**: Analyzes project files to identify the primary language
2. **Traditional Linting**: Runs appropriate linters (ESLint, ruff, etc.)
3. **Smart Diff Analysis**:
   - **PRs**: Reviews all changes against base branch
   - **Direct pushes**: Reviews only unpushed changes
4. **AI Analysis**: Sends diff to AI Gateway for intelligent review
5. **PR Integration**: Posts findings as inline comments via reviewdog

## Local Development & Testing

### Prerequisites

```bash
# Required tools
- bash
- git
- curl
- jq

# When using SonarQube locally (optional)
- Java 11+ (SonarQube scanner requirement)
```

### Testing Locally

```bash
# Set environment variables
export AI_GATEWAY_URL="https://your-gateway.com/api/review"
export AI_GATEWAY_API_KEY="your-api-key"
export GITHUB_TOKEN="your-token"

# Test language detection
bash scripts/detect-language.sh

# Test AI review
bash scripts/ai-review.sh
```

## Local Installation (Git Hook)

You can install AI Review as a git pre-commit hook to automatically review code before each commit.

### Installation

**macOS / Linux / WSL:**
```bash
# Option 1: Run installer directly from repo
git clone https://github.com/hiiamtrong/smart-code-review.git
cd smart-code-review
bash scripts/local/install.sh

# Option 2: One-line install (when published)
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash
```

**Windows:**
```powershell
# Option 1: Run installer from repo (PowerShell)
cd C:\path\to\smart-code-review
powershell -ExecutionPolicy Bypass -File scripts/local/install.ps1
```

```bash
# From Git Bash - use forward slashes for paths
powershell -ExecutionPolicy Bypass -File scripts/local/install.ps1
```

```powershell
# Option 2: One-line install (when published)
irm https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.ps1 | iex
```

**Windows + SonarQube:** Java 11+ is required. Install via `winget install EclipseAdoptium.Temurin.18.JDK` or `choco install temurin18`. See [SETUP_GUIDE.md](SETUP_GUIDE.md) for details.

The installer will:
1. Install required dependencies (jq)
2. Prompt for your AI Gateway credentials
3. Set up the CLI tool and hook scripts
4. Add `~/.local/bin` to your PATH

**Optional:** For `ai-review diff` command, install gawk:
- macOS: `brew install gawk`
- Ubuntu: `sudo apt-get install gawk`

### Optional: Enable SonarQube in Local Hooks

By default, local pre-commit hooks run **AI review only** (fast, 2-10 seconds).

To optionally enable SonarQube locally (slower, 30-60 seconds):

```bash
# macOS/Linux: Enable SonarQube in pre-commit hooks
bash scripts/local/enable-local-sonarqube.sh
```

```bash
# Windows (Git Bash): If command not found, run script directly
bash "$HOME/.config/ai-review/hooks/enable-local-sonarqube.sh"
```

Follow prompts to enter:
- SonarQube URL: https://sonarqube.sotatek.works
- SonarQube Token: (from dashboard)

**Windows:** Requires Java 11+ installed (see [SETUP_GUIDE.md](SETUP_GUIDE.md)).

**Configuration:**
```bash
# View current config
ai-review config

# Disable SonarQube locally (back to fast mode)
ai-review config set ENABLE_SONARQUBE_LOCAL false

# Only report issues on lines you changed (default: true)
ai-review config set SONAR_FILTER_CHANGED_LINES_ONLY true

# Or run the script again and choose "Disable"
bash scripts/local/enable-local-sonarqube.sh
```

**Changed Lines Filtering:** By default, SonarQube only reports issues on the exact lines you changed. Issues in unchanged code (existing/legacy issues) are filtered out and won't block your commit. This ensures you're only responsible for fixing issues in your changes, not pre-existing technical debt.

**Automatic Cleanup:** Temporary SonarQube files are automatically removed after each commit.

### Enable Hook in a Repository

After installation, navigate to any git repository and run:

```bash
ai-review install
```

This installs the pre-commit hook that will review your staged changes before each commit.

### CLI Commands

| Command | Description |
|---------|-------------|
| `ai-review install` | Install hook in current repository |
| `ai-review uninstall` | Remove hook from current repository |
| `ai-review config` | View current configuration |
| `ai-review config set KEY VALUE` | Update a config value |
| `ai-review config edit` | Open config in editor |
| `ai-review diff` | Show staged diff with line numbers |
| `ai-review diff --all` | Show all changes (staged + unstaged) |
| `ai-review status` | Check installation status |
| `ai-review update` | Update to latest version |
| `ai-review help` | Show help message |

### How It Works

When you run `git commit`, the hook will:

1. Get your staged changes (`git diff --cached`)
2. Filter out files matching `.aireviewignore` patterns
3. Send the diff to your AI Gateway for review
4. Display results with severity levels:
   - **ERROR**: Blocks the commit (fix required)
   - **WARNING**: Allows commit but shows warnings
   - **INFO**: Informational suggestions

### Example Output

```
AI Review analyzing your changes...
Reviewing 45 lines of changes

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
ERROR: SQL Injection vulnerability detected
   src/utils/db.js:42

WARNING: Missing error handling
   src/api/handler.js:87
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Summary: 1 errors, 1 warnings, 0 info

Commit blocked - please fix ERROR issues first
   Use git commit --no-verify to bypass (not recommended)
```

### Bypassing the Hook

To skip AI review for a single commit:

```bash
git commit --no-verify -m "your message"
```

### Configuration

Config is stored at `~/.config/ai-review/config`:

```bash
AI_GATEWAY_URL="https://your-gateway.com/api"
AI_GATEWAY_API_KEY="your-api-key"
AI_MODEL="gemini-2.0-flash"
AI_PROVIDER="google"
```

### Uninstalling

```bash
# Remove from current repository
ai-review uninstall

# Remove completely (delete config and CLI)
rm -rf ~/.config/ai-review ~/.local/bin/ai-review
```

### Windows Troubleshooting

- **Java not found:** Install Java 11+ (`winget install EclipseAdoptium.Temurin.18.JDK` or `choco install temurin18`). Restart terminal. Add to PATH if needed.
- **enable-local-sonarqube not found:** Run `bash "$HOME/.config/ai-review/hooks/enable-local-sonarqube.sh"`
- **Path errors in Git Bash:** Use forward slashes (e.g. `scripts/local/install.ps1`)

See [SETUP_GUIDE.md](SETUP_GUIDE.md) for full Windows setup and troubleshooting.

## AI Gateway Integration

This action expects your AI Gateway to:

### Request Format
```json
{
  "ai_model": "gemini-2.0-flash",
  "ai_provider": "google",
  "git_diff": "diff content...",
  "language": "javascript",
  "review_mode": "string"
}
```

### Response Format
The gateway should return reviewdog-compatible diagnostic format:

```json
{
  "source": {"name": "ai-review", "url": ""},
  "diagnostics": [
    {
      "message": "Issue description",
      "location": {
        "path": "file.js",
        "range": {
          "start": {"line": 10, "column": 5},
          "end": {"line": 10, "column": 15}
        }
      },
      "severity": "ERROR",
      "code": {"value": "issue-type", "url": ""}
    }
  ]
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test locally
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/hiiamtrong/smart-code-review/issues)
- **Discussions**: [GitHub Discussions](https://github.com/hiiamtrong/smart-code-review/discussions)

---

Made by [hiiamtrong](https://github.com/hiiamtrong)