# Quick Reference Card

## Initial Setup (One Time)

```bash
# 1. Install
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash

# 2. Configure
ai-review setup

# 3. Create project in SonarQube UI
# Go to https://sonarqube.sotatek.works/
# Click "Create Project" → Enter project key (e.g., "my-project")
# IMPORTANT: Choose "Reference branch" for New Code definition

# 4. In your project (use SAME key from step 3)
cd /path/to/project
ai-review install
```

---

## Daily Usage

```bash
# Normal commit (review runs automatically)
git add .
git commit -m "your message"

# Emergency bypass (use sparingly!)
git commit --no-verify -m "emergency fix"
```

---

## Common Commands

```bash
# View configuration
ai-review config

# Update token
ai-review config SONAR_TOKEN <new-token>

# Update tool
ai-review update

# Uninstall from project
ai-review uninstall
```

---

## Get SonarQube Token

1. Go to https://sonarqube.sotatek.works/
2. Profile → My Account → Security
3. Generate Token → Copy it
4. `ai-review config SONAR_TOKEN <paste-token>`

---

## Exclude Files

**`.aireviewignore`** - Exclude from AI review
```gitignore
node_modules/
dist/
*.test.js
```

**`.sonarignore`** - Exclude from SonarQube
```gitignore
.idea/
.vscode/
*.min.js
```

---

## Quick Troubleshooting

| Issue | Solution |
|-------|----------|
| "Not authorized" | **Create project in SonarQube UI first** with same key! |
| "405 Error" | Check AI Gateway URL with team lead |
| Hook not running | `ai-review install` again |
| Too many issues | Only stage files you changed |
| Wrong project key | `git config --local aireview.sonarProjectKey "correct-key"` |

---

## What Gets Scanned?

- **On main/master**: Only staged files
- **On feature branch**: Changed files from base branch
- **No changes**: Full project scan

---

## Help

- Full guide: `SETUP_GUIDE.md`
- SonarQube: https://sonarqube.sotatek.works/
- Ask your team lead for AI Gateway credentials

