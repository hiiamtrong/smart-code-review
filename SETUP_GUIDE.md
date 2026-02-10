# Guide: Pre-commit Code Review

Quick guide for setting up AI + SonarQube code review on your local machine.

---

## Prerequisites

Before starting, make sure you have:

- Git installed
- A SonarQube account on [https://sonarqube.sotatek.works/](https://sonarqube.sotatek.works/)
- Terminal/Command Line access

---

## Step 1: Install AI Review Tool

Open your terminal and run:

```bash
# Download and install the ai-review CLI
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash
```

This will install the `ai-review` command to your system.

---

## Step 2: Get Your SonarQube Token

1. Go to [https://sonarqube.sotatek.works/](https://sonarqube.sotatek.works/)
2. Log in with your account
3. Click your **profile icon** (top right) → **My Account**
4. Go to **Security** tab
5. Under **Generate Tokens**:
   - Name: `ai-review-local` (or any name you like)
   - Type: **User Token**
   - Expires in: **No expiration** (or choose a duration)
6. Click **Generate**
7. **Copy the token** (you won't see it again!)

---

## Step 3: Configure AI Review

Run the setup command:

```bash
ai-review setup
```

You'll be prompted for:

### 1. AI Gateway Configuration
```
AI Gateway URL: https://dashboard.code-review.sotatek.works/api/review
AI Gateway API Key: [Ask your team lead for this]
AI Model (optional): [Press Enter to use default]
AI Provider (optional): [Press Enter to use default]
```

### 2. SonarQube Configuration
```
Enable SonarQube for local commits? (y/n): y
SonarQube Host URL: https://sonarqube.sotatek.works
SonarQube Token: [Paste the token you copied in Step 2]
```

### 3. Security Hotspots
```
Block commits on Security Hotspots? (y/n): y
```

Configuration saved!

---

## Step 4: Create Project in SonarQube

**Before installing the hook**, create your project in SonarQube:

1. Go to [https://sonarqube.sotatek.works/](https://sonarqube.sotatek.works/)
2. Click **Create Project** (+ icon or Projects → Create Project)
3. Choose **Manually**
4. Fill in:
   - **Project display name**: `My Awesome Project` (human-readable name)
   - **Project key**: `my-awesome-project` (lowercase, numbers, hyphens, underscores only)
5. Click **Set Up**
6. **IMPORTANT: Configure "New Code" Definition**
   - You'll see "Set up new code for project" screen
   - Select **"Reference branch"** (recommended for projects using feature branches)
   - This tells SonarQube to compare your changes against your main branch
   - The main branch will be automatically set as the reference
7. Click **Create project**

**Remember your Project Key!** You'll need it in the next step.

**Why Reference Branch?**
- The pre-commit hook uses differential analysis (only scans changed files)
- SonarQube's "Reference branch" setting aligns with this approach
- You'll only see issues in code you actually changed, not old issues

---

## Step 5: Install in Your Project

Navigate to your project and run:

```bash
cd /path/to/your/project
ai-review install
```

You'll be asked:

1. **SonarQube Project Key**: Enter the SAME key you created in Step 4
   - Example: `my-awesome-project`
   - **Must match exactly** with what you created in SonarQube UI!

2. **Base Branch**: Which branch to compare against
   - Press Enter to accept the detected branch (usually `main` or `master`)
   - Or type a custom branch like `develop`

Pre-commit hook installed!

---

## How to Use

### Normal Workflow

Just commit as usual:

```bash
git add .
git commit -m "your commit message"
```

The review will run automatically:

```
Scanning changed files...
   -> 3 file(s) to analyze
Running analysis...
Found 5 issues from SonarQube
   Errors: 2 | Warnings: 3 | Info: 0

AI Review analyzing your changes...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
AI Review Results
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### If Issues Found

**Commit will be blocked** if:
- SonarQube finds **errors**
- SonarQube finds **security hotspots** (if enabled)

You must fix the issues before committing.

### Bypass Review (Emergency Only)

If you need to commit without review:

```bash
git commit --no-verify -m "emergency fix"
```

**Use sparingly!** This skips all checks.

---

## Managing Your Setup

### View Current Configuration

```bash
ai-review config
```

### Update Configuration

```bash
ai-review config <key> <value>
```

Examples:
```bash
# Change SonarQube URL
ai-review config SONAR_HOST_URL https://sonarqube.sotatek.works

# Update SonarQube token
ai-review config SONAR_TOKEN <your-new-token>

# Disable hotspot blocking
ai-review config SONAR_BLOCK_ON_HOTSPOTS false
```

### Update to Latest Version

```bash
ai-review update
```

### Uninstall from a Project

```bash
cd /path/to/your/project
ai-review uninstall
```

---

## Excluding Files from Analysis

### Exclude from AI Review

Create `.aireviewignore` in your project root:

```gitignore
# Build outputs
dist/
build/

# Dependencies
node_modules/
vendor/

# Test files
*.test.js
__tests__/
```

### Exclude from SonarQube

Create `.sonarignore` in your project root:

```gitignore
# IDE directories
.idea/
.vscode/
.cursor/

# Generated files
*.min.js
*.bundle.js

# Documentation
docs/
*.md
```

---

## Troubleshooting

### Issue: "You're not authorized to run analysis"

**Root Cause:** The project doesn't exist in SonarQube yet.

**Solution:**
1. **Create the project in SonarQube UI first** (see Step 4 above):
   - Go to [https://sonarqube.sotatek.works/](https://sonarqube.sotatek.works/)
   - Click **Create Project** → **Manually**
   - Project key: Use the **exact same key** you entered during `ai-review install`
   - Example: If you used `my-project` during install, create project with key `my-project`

2. If project already exists, verify your token has "Execute Analysis" permission:
   - Go to SonarQube → My Account → Security
   - Check token permissions
   - Re-generate token if needed:
     ```bash
     ai-review config SONAR_TOKEN <new-token>
     ```

3. If you misspelled the project key during install:
   ```bash
   # Update to correct key
   git config --local aireview.sonarProjectKey "correct-project-key"
   ```

### Issue: "AI Gateway 405 Method Not Allowed"

**Solution:** Ask your team lead for the correct AI Gateway URL and API key.

### Issue: "Missing blame information" warnings

**Solution:** This is normal and already handled! The tool disables SCM integration for pre-commit to avoid this.

### Issue: Too many issues found

**Solution:** The scan only checks files you changed, but if you're on the main branch with no staged files, it scans everything. To scan only specific files:

```bash
# Stage only the files you want to scan
git add file1.js file2.js
git commit -m "message"
```

### Issue: Hook not running

**Solution:**
1. Check if hook is installed:
   ```bash
   ls -la .git/hooks/pre-commit
   ```
2. Reinstall if needed:
   ```bash
   ai-review install
   ```

---

## What Gets Scanned?

### On Main/Master/Develop Branch
- **Only staged files** (what you're about to commit)

### On Feature Branch
- **All files changed from base branch** + staged files

### No Changed Files
- **Entire project** (full scan)

---

## Best Practices

1. **Commit frequently** - Smaller commits = fewer issues to fix
2. **Review before staging** - Check your code before `git add`
3. **Fix issues immediately** - Don't accumulate technical debt
4. **Use `.sonarignore`** - Exclude unnecessary files for faster scans
5. **Keep token secure** - Don't share or commit your SonarQube token

---

## Getting Help

- **View this guide**: https://github.com/hiiamtrong/smart-code-review/blob/main/SETUP_GUIDE.md
- **Ask your team lead** for AI Gateway credentials

---

## You're All Set!

Your local environment is now configured with:
- AI-powered code review
- SonarQube static analysis
- Automatic pre-commit scanning

Happy coding!

