# Smart Code Review GitHub Action

🤖 **AI-Powered Code Review with Language Detection and Linting Integration**

This GitHub Action automatically detects your project's language/framework and combines traditional linting tools with AI-powered code review using [reviewdog](https://github.com/reviewdog/reviewdog) for seamless PR integration.

## ✨ Features

- **🔍 Language Detection**: Automatically detects project type based on configuration files
- **🤖 AI-Powered Review**: Intelligent code analysis using configurable AI models
- **🛠️ Traditional Linting**: Integrates with popular linters for each language
- **📝 Smart Diff Analysis**: Reviews only changes in PRs or since last push
- **💬 Inline Comments**: Posts review comments directly on PR lines
- **🔧 Configurable**: Flexible AI model and provider selection

## 📚 Supported Languages & Tools

| Language               | Linters                     | AI Review |
| ---------------------- | --------------------------- | --------- |
| **Node.js/TypeScript** | ESLint                      | ✅         |
| **Python**             | ruff → flake8 → pylint      | ✅         |
| **Java**               | Checkstyle (Maven/Gradle)   | ✅         |
| **Go**                 | staticcheck → golangci-lint | ✅         |
| **.NET**               | dotnet format               | ✅         |

## 🚀 Quick Start

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

## ⚙️ Configuration

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

## 🏗️ How It Works

1. **Language Detection**: Analyzes project files to identify the primary language
2. **Traditional Linting**: Runs appropriate linters (ESLint, ruff, etc.)
3. **Smart Diff Analysis**:
   - **PRs**: Reviews all changes against base branch
   - **Direct pushes**: Reviews only unpushed changes
4. **AI Analysis**: Sends diff to AI Gateway for intelligent review
5. **PR Integration**: Posts findings as inline comments via reviewdog

## 🔧 Local Development & Testing

### Prerequisites

```bash
# Required tools
- bash
- git
- curl
- jq
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

## 📖 AI Gateway Integration

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

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test locally
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/hiiamtrong/smart-code-review/issues)
- **Discussions**: [GitHub Discussions](https://github.com/hiiamtrong/smart-code-review/discussions)

---

Made with ❤️ by [hiiamtrong](https://github.com/hiiamtrong)