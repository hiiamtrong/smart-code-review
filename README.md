# AI Review — Pre-commit Code Quality Hook

Automatically runs Semgrep, SonarQube, and AI code review on your staged changes before each commit.

## Important Notes

### SonarQube Artifacts

- **`.scannerwork/` directory**: Created in your repo root during SonarQube analysis
- **Automatically cleaned up**: Removed after analysis completes
- **Already in `.gitignore`**: Won't be committed to your repository
- **Server-side settings**: Working directory location is controlled by SonarQube server settings and cannot be overridden from client-side

If you see `.scannerwork/` in your repo, it will be removed automatically after the pre-commit hook completes. If the analysis is interrupted (e.g., you cancel it), you can manually clean it up:

```bash
rm -rf .scannerwork
```

### Configuration Files

- **`.aireviewignore`**: Exclude files from all analysis tools (AI, Semgrep, SonarQube)
- **SonarQube properties**: Auto-generated in `~/.config/ai-review/projects/<id>/sonar-project.properties`
- **No files in repo root**: All configuration is stored outside your repository to keep it clean

## Prerequisites

Install these before setting up:

- **Java 17+** — required for SonarQube scanner
  - macOS: `brew install openjdk@17`
  - Linux: `sudo apt install openjdk-17-jdk`
  - Windows: `winget install EclipseAdoptium.Temurin.17.JDK`
  - Verify: `java -version`
- **SonarQube Scanner CLI**
  - macOS: `brew install sonar-scanner`
  - Others: [download](https://docs.sonarsource.com/sonarqube/latest/analyzing-source-code/scanners/sonarscanner/)
  - Verify: `sonar-scanner --version`
- **Semgrep** *(optional, only if enabling Semgrep analysis)*
  - `pip install semgrep` or `brew install semgrep`
  - Verify: `semgrep --version`

## Installation

**Step 1 — Install the CLI:**

```bash
# macOS / Linux
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash

# Windows (PowerShell)
irm https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.ps1 | iex
```

Restart terminal after install.

**Step 2 — Configure (once per machine):**

```bash
ai-review setup
```

**Step 3 — Install hook into a project:**

```bash
cd /path/to/your-repo
ai-review install
```

You will be prompted for the SonarQube Project Key — defaults to the repo directory name.

## Usage

Commit as normal. The hook runs automatically:

```bash
git add .
git commit -m "feat: your change"
```

To bypass in emergencies:

```bash
git commit --no-verify -m "hotfix"
```

## Configuration

```bash
ai-review config show                          # view current config
ai-review config set KEY VALUE                 # update a value
ai-review uninstall                            # remove hook from repo
ai-review update                               # update to latest version
```

Common settings:

| Key | Default | Description |
|-----|---------|-------------|
| `ENABLE_AI_REVIEW` | `true` | Enable AI gateway review |
| `ENABLE_SONARQUBE_LOCAL` | `true` | Enable SonarQube analysis |
| `ENABLE_SEMGREP` | `false` | Enable Semgrep analysis |
| `SONAR_BLOCK_ON_HOTSPOTS` | `true` | Block commit on security hotspots |
| `SONAR_FILTER_CHANGED_LINES_ONLY` | `true` | Only report issues on changed lines |
| `BLOCK_ON_GATEWAY_ERROR` | `true` | Block commit if AI gateway is unreachable |

## Ignore files

Create `.aireviewignore` in your repo root (same syntax as `.gitignore`):

```gitignore
dist/
build/
node_modules/
vendor/
*.min.js
*.md
```

## Troubleshooting

| Error | Fix |
|-------|-----|
| `java: command not found` | Install Java 17, restart terminal |
| `sonar-scanner: command not found` | Install SonarQube Scanner CLI (see Prerequisites) |
| `You're not authorized` | Run `ai-review install` again with the correct project key |
| `AI Gateway error: 404` | Check `AI_GATEWAY_URL` in `ai-review config show` |
| Hook not running | Run `ai-review install` in the repo |
