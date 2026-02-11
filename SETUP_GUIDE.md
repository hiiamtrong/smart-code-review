# Guide: Pre-commit Code Review

Quick guide for setting up AI + SonarQube code review on your local machine.

---

## Prerequisites

Before starting, make sure you have:

- **Git** installed (includes Git Bash on Windows)
- **A SonarQube account** on [https://sonarqube.sotatek.works/](https://sonarqube.sotatek.works/)
- **Terminal/Command Line** access
- **Java 11+** (required for SonarQube scanner when using local SonarQube)

---

## Step 0: Install Java (required for SonarQube)

SonarQube scanner requires Java 11 or higher on all platforms. Install before enabling SonarQube.

### Linux

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install openjdk-18-jdk

# Fedora/RHEL
sudo dnf install java-18-openjdk-devel

# Arch
sudo pacman -S jdk18-openjdk
```

### macOS

```bash
brew install openjdk@18
# Follow the post-install instructions to add Java to your PATH
```

### Windows

**Option A – Winget:**
```powershell
winget install EclipseAdoptium.Temurin.18.JDK
# or
winget install Microsoft.OpenJDK.18
```

**Option B – Chocolatey:**
```powershell
choco install temurin18
```

**Option C – Manual:** Download from [Adoptium](https://adoptium.net/temurin/releases/?version=18&os=windows&arch=x64) and run the installer. During setup, enable **"Add to PATH"**.

**Note:** If `java` is not found after install, add the Java `bin` folder to your PATH manually (System Properties → Environment Variables → Path). The typical path is:
`C:\Program Files\Eclipse Adoptium\jdk-18.x.x-hotspot\bin`

### Verify installation

```bash
# Restart your terminal first, then:
java -version
```

---

## Step 1: Install AI Review Tool

### macOS / Linux / WSL

```bash
# Download and install the ai-review CLI
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash
```

### Windows

**From PowerShell (recommended):**
```powershell
cd C:\path\to\smart-code-review
powershell -ExecutionPolicy Bypass -File scripts/local/install.ps1
```

**From Git Bash:** Use forward slashes for the path:
```bash
powershell -ExecutionPolicy Bypass -File scripts/local/install.ps1
```

**One-line install (when published):**
```powershell
irm https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.ps1 | iex
```

This will install the `ai-review` command and add it to your PATH. Restart your terminal after installation.

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

# Only report issues on changed lines (default: true)
ai-review config SONAR_FILTER_CHANGED_LINES_ONLY true
```

**Note:** By default, SonarQube only reports issues on lines you changed in this commit. Issues in unchanged lines (existing code) are filtered out and won't block your commit. To see all issues in changed files (old behavior), set `SONAR_FILTER_CHANGED_LINES_ONLY` to `false`.

**Automatic Cleanup:** The hook automatically cleans up temporary SonarQube files (`.scannerwork/`, `sonar-project.properties`, etc.) after each commit, whether it succeeds or is blocked. Your project directory stays clean.

### Update to Latest Version

```bash
ai-review update
```

### Uninstall from a Project

```bash
cd /path/to/your/project
ai-review uninstall
```

### Windows: Running enable-local-sonarqube

On Windows, the `enable-local-sonarqube` command may not be found in Git Bash. Use one of these:

```bash
# Option 1: Run the script directly (Git Bash)
bash "$HOME/.config/ai-review/hooks/enable-local-sonarqube.sh"

# Option 2: Use PowerShell or CMD (if PATH is set)
enable-local-sonarqube
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

### Issue: "SonarQube analysis failed" (no error details shown)

**Root Cause:** Usually Java is not installed or not in PATH. The SonarQube scanner requires Java 11+.

**Solution:**
1. Install Java (see Step 0 above)
2. Restart your terminal after installing
3. Verify: `java -version`
4. To see the full error, run the script manually:
   ```bash
   bash "$HOME/.config/ai-review/hooks/sonarqube-review.sh"
   ```

### Issue: "java: command not found" on Windows

**Solution:**
1. Install Java (see Step 0 above)
2. Close all terminals and open a new one
3. If still not found, add Java to PATH manually:
   - System Properties → Environment Variables → Path
   - Add: `C:\Program Files\Eclipse Adoptium\jdk-18.x.x-hotspot\bin` (match your version)
4. Or temporarily in Git Bash: `export PATH="/c/Program Files/Eclipse Adoptium/jdk-18.0.2.101-hotspot/bin:$PATH"`

### Issue: "enable-local-sonarqube: command not found" (Windows)

**Solution:** Run the script directly:
```bash
bash "$HOME/.config/ai-review/hooks/enable-local-sonarqube.sh"
```

---

## What Gets Scanned?

### Files Scanned

- **On Main/Master/Develop Branch:** Only staged files (what you're about to commit)
- **On Feature Branch:** All files changed from base branch + staged files
- **No Changed Files:** Entire project (full scan)

### Issues Reported

**By default, only issues on lines you changed are reported.** This means:
- You edit line 50 in `file.js` → Issue on line 50 is reported
- Issue already exists on line 10 (unchanged) → Filtered out, not reported

This ensures you're only responsible for fixing issues in your changes, not pre-existing technical debt.

To see all issues in changed files (including unchanged lines), set:
```bash
ai-review config SONAR_FILTER_CHANGED_LINES_ONLY false
```

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

