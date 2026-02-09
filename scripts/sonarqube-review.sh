#!/usr/bin/env bash
set -e

# SonarQube Code Review Integration
# This script runs SonarQube analysis and converts results to reviewdog format

echo "🔍 SonarQube Code Review Starting..."

# Configuration (can be overridden by environment variables)
SONAR_HOST_URL="${SONAR_HOST_URL:-http://localhost:9000}"
SONAR_TOKEN="${SONAR_TOKEN:-}"
SONAR_PROJECT_KEY="${SONAR_PROJECT_KEY:-}"
SONAR_PROJECT_NAME="${SONAR_PROJECT_NAME:-$(basename $(git rev-parse --show-toplevel))}"
SONAR_PROJECT_VERSION="${SONAR_PROJECT_VERSION:-1.0}"
SONAR_SOURCES="${SONAR_SOURCES:-.}"

# Default exclusions
DEFAULT_EXCLUSIONS="**/node_modules/**,**/dist/**,**/build/**,**/target/**,**/vendor/**,**/*.test.js,**/*.spec.ts,**/*.test.ts,**/*.spec.js"

# Load exclusions from .sonarignore file if it exists
SONAR_EXCLUSIONS="$DEFAULT_EXCLUSIONS"
if [[ -f ".sonarignore" ]]; then
  echo "📋 Loading exclusions from .sonarignore..."
  # Read .sonarignore and convert to comma-separated list
  CUSTOM_EXCLUSIONS=$(grep -v '^#' .sonarignore | grep -v '^[[:space:]]*$' | sed 's/^/\*\*\//g' | tr '\n' ',' | sed 's/,$//')
  if [[ -n "$CUSTOM_EXCLUSIONS" ]]; then
    SONAR_EXCLUSIONS="$DEFAULT_EXCLUSIONS,$CUSTOM_EXCLUSIONS"
    echo "   ✅ Added custom exclusions from .sonarignore"
  fi
fi

# Check required variables
if [[ -z "$SONAR_TOKEN" ]]; then
  echo "⚠️  SONAR_TOKEN not set, skipping SonarQube analysis"
  echo "   Set SONAR_TOKEN to enable SonarQube integration"
  exit 0
fi

if [[ -z "$SONAR_PROJECT_KEY" ]]; then
  # Auto-generate project key from repo name
  SONAR_PROJECT_KEY=$(basename $(git rev-parse --show-toplevel) | sed 's/[^a-zA-Z0-9_-]/_/g')
  echo "📋 Auto-generated project key: $SONAR_PROJECT_KEY"
fi

echo "📊 SonarQube Configuration:"
echo "   Host: $SONAR_HOST_URL"
echo "   Project: $SONAR_PROJECT_KEY"
echo "   Sources: $SONAR_SOURCES"
echo "   Exclusions: $(echo "$SONAR_EXCLUSIONS" | cut -c1-60)..."

# Detect SonarQube Scanner
SONAR_SCANNER=""
if command -v sonar-scanner &> /dev/null; then
  SONAR_SCANNER="sonar-scanner"
elif command -v sonar-scanner-cli &> /dev/null; then
  SONAR_SCANNER="sonar-scanner-cli"
elif [[ -f "$HOME/.sonar/sonar-scanner/bin/sonar-scanner" ]]; then
  SONAR_SCANNER="$HOME/.sonar/sonar-scanner/bin/sonar-scanner"
else
  echo "📥 Installing SonarQube Scanner..."
  # Install SonarQube Scanner
  SCANNER_VERSION="5.0.1.3006"
  SCANNER_DIR="$HOME/.sonar/sonar-scanner"
  
  mkdir -p "$HOME/.sonar"
  cd "$HOME/.sonar"
  
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}-linux.zip"
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}-macosx.zip"
  else
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}.zip"
  fi
  
  if [[ ! -d "$SCANNER_DIR" ]]; then
    echo "Downloading SonarQube Scanner..."
    curl -sSL "https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/${SCANNER_ZIP}" -o scanner.zip
    unzip -q scanner.zip
    mv sonar-scanner-${SCANNER_VERSION}* sonar-scanner
    rm scanner.zip
  fi
  
  SONAR_SCANNER="$SCANNER_DIR/bin/sonar-scanner"
  cd - > /dev/null
fi

# Scanner detected (silent)

# Get git diff for changed files (for PR analysis)
CHANGED_FILES=""
if [[ -n "$GITHUB_BASE_REF" ]]; then
  # Pull request - get changed files
  echo "📌 PR detected: analyzing changed files only"
  if git rev-parse --verify origin/$GITHUB_BASE_REF >/dev/null 2>&1; then
    CHANGED_FILES=$(git diff --name-only origin/$GITHUB_BASE_REF...HEAD | tr '\n' ',')
  fi
fi

# Create sonar-project.properties if it doesn't exist
if [[ ! -f "sonar-project.properties" ]]; then
  echo "📝 Creating sonar-project.properties..."
  cat > sonar-project.properties << EOF
# SonarQube Configuration (auto-generated)
sonar.projectKey=$SONAR_PROJECT_KEY
sonar.projectName=$SONAR_PROJECT_NAME
sonar.projectVersion=$SONAR_PROJECT_VERSION
sonar.sources=$SONAR_SOURCES
sonar.exclusions=$SONAR_EXCLUSIONS

# Language-specific settings
sonar.sourceEncoding=UTF-8

# SCM settings
sonar.scm.provider=git
EOF
  
  # Add language-specific configurations
  if [[ -f "package.json" ]]; then
    echo "sonar.javascript.lcov.reportPaths=coverage/lcov.info" >> sonar-project.properties
    echo "sonar.typescript.lcov.reportPaths=coverage/lcov.info" >> sonar-project.properties
  elif [[ -f "pom.xml" || -f "build.gradle" ]]; then
    echo "sonar.java.binaries=target/classes,build/classes" >> sonar-project.properties
  elif [[ -f "go.mod" ]]; then
    echo "sonar.go.coverage.reportPaths=coverage.out" >> sonar-project.properties
  fi
  
  echo "✅ Created sonar-project.properties"
fi

# Run SonarQube analysis
echo "🚀 Running SonarQube analysis..."

# Use -Dsonar.login for compatibility with older SonarQube versions
# (Modern versions accept tokens via login parameter)
SONAR_OPTS="-Dsonar.host.url=$SONAR_HOST_URL -Dsonar.login=$SONAR_TOKEN"

# Reduce log verbosity (only show WARN and ERROR)
SONAR_OPTS="$SONAR_OPTS -Dsonar.log.level=WARN -Dsonar.verbose=false"

# Detect if running in CI/CD or local pre-commit
if [[ -n "$GITHUB_ACTIONS" || -n "$CI" ]]; then
  # CI/CD Mode: Use SCM integration for blame information
  
  # Add PR-specific parameters if in GitHub Actions PR
  if [[ -n "$GITHUB_BASE_REF" && -n "$GITHUB_HEAD_REF" ]]; then
    PR_NUMBER=$(echo "$GITHUB_REF" | sed -n 's/refs\/pull\/\([0-9]*\)\/merge/\1/p')
    if [[ -n "$PR_NUMBER" ]]; then
      SONAR_OPTS="$SONAR_OPTS -Dsonar.pullrequest.key=$PR_NUMBER"
      SONAR_OPTS="$SONAR_OPTS -Dsonar.pullrequest.branch=$GITHUB_HEAD_REF"
      SONAR_OPTS="$SONAR_OPTS -Dsonar.pullrequest.base=$GITHUB_BASE_REF"
    fi
  fi
else
  # Local Pre-commit Mode: Disable SCM to avoid blame warnings
  SONAR_OPTS="$SONAR_OPTS -Dsonar.scm.disabled=true"
  
  # Force full analysis of all code (not just new code)
  SONAR_OPTS="$SONAR_OPTS -Dsonar.qualitygate.wait=false"
  
  # Differential Analysis: Only scan changed files
  echo "🔍 Scanning changed files..."
  
  # Get current branch
  CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
  
  # Get base branch from git config (user-configured)
  BASE_BRANCH=$(git config --local aireview.baseBranch 2>/dev/null)
  
  # If not configured, auto-detect main branch name (could be main, master, or develop)
  if [[ -z "$BASE_BRANCH" ]]; then
    if git rev-parse --verify main >/dev/null 2>&1; then
      BASE_BRANCH="main"
    elif git rev-parse --verify master >/dev/null 2>&1; then
      BASE_BRANCH="master"
    elif git rev-parse --verify develop >/dev/null 2>&1; then
      BASE_BRANCH="develop"
    fi
  fi
  
  # Legacy variable for backward compatibility
  MAIN_BRANCH="$BASE_BRANCH"
  
  FILES_TO_SCAN=""
  
  # Check if we're on the base branch
  if [[ "$CURRENT_BRANCH" == "$BASE_BRANCH" ]]; then
    # On base branch: Only scan staged files (pre-commit)
    FILES_TO_SCAN=$(git diff --cached --name-only --diff-filter=ACMRTUXB 2>/dev/null | tr '\n' ',')
    
  else
    # On feature branch: Scan all changed files compared to base + staged files
    if [[ -n "$BASE_BRANCH" ]]; then
      # Get files changed from base branch
      BRANCH_FILES=$(git diff "$BASE_BRANCH"...HEAD --name-only --diff-filter=ACMRTUXB 2>/dev/null | tr '\n' ',')
      
      # Get staged files
      STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACMRTUXB 2>/dev/null | tr '\n' ',')
      
      # Combine both
      FILES_TO_SCAN="${BRANCH_FILES}${STAGED_FILES}"
    else
      FILES_TO_SCAN=$(git diff --cached --name-only --diff-filter=ACMRTUXB 2>/dev/null | tr '\n' ',')
    fi
  fi
  
  # Remove duplicates and clean up
  FILES_TO_SCAN=$(echo "$FILES_TO_SCAN" | tr ',' '\n' | sort -u | grep -v '^$' | tr '\n' ',' | sed 's/,$//')
  
  if [[ -n "$FILES_TO_SCAN" ]]; then
    # Count files
    FILE_COUNT=$(echo "$FILES_TO_SCAN" | tr ',' '\n' | wc -l)
    echo "   → $FILE_COUNT file(s) to analyze"
    
    # Add inclusions to SonarQube
    SONAR_OPTS="$SONAR_OPTS -Dsonar.inclusions=$FILES_TO_SCAN"
  fi
fi

# Run scanner (suppress INFO logs, show only WARN/ERROR)
echo "🔄 Running analysis..."
SCANNER_LOG=$(mktemp)
$SONAR_SCANNER $SONAR_OPTS > "$SCANNER_LOG" 2>&1
SCANNER_EXIT=$?

# Filter and show only warnings and errors
grep -E "(WARN|ERROR|FAIL)" "$SCANNER_LOG" || true

rm -f "$SCANNER_LOG"

if [[ $SCANNER_EXIT -ne 0 ]]; then
  echo "❌ SonarQube analysis failed"
  exit 1
fi

# Wait a moment for SonarQube to process results
sleep 3

# Fetch issues via API (Bugs, Vulnerabilities, Code Smells)
ISSUES_JSON=$(curl -s -u "$SONAR_TOKEN:" \
  "$SONAR_HOST_URL/api/issues/search?componentKeys=$SONAR_PROJECT_KEY&resolved=false&ps=500" || echo "")

if [[ -z "$ISSUES_JSON" || "$ISSUES_JSON" == "null" ]]; then
  echo "⚠️  Could not fetch issues from SonarQube"
  echo "Creating empty report..."
  cat > sonarqube-output.jsonl << EOF
{
  "source": {"name": "sonarqube", "url": "$SONAR_HOST_URL"},
  "diagnostics": []
}
EOF
  exit 0
fi

# Fetch Security Hotspots (separate API)
HOTSPOTS_JSON=$(curl -s -u "$SONAR_TOKEN:" \
  "$SONAR_HOST_URL/api/hotspots/search?projectKey=$SONAR_PROJECT_KEY&status=TO_REVIEW&ps=500" || echo "")

HOTSPOT_COUNT=0
if [[ -n "$HOTSPOTS_JSON" && "$HOTSPOTS_JSON" != "null" ]]; then
  HOTSPOT_COUNT=$(echo "$HOTSPOTS_JSON" | jq '.hotspots | length' 2>/dev/null || echo "0")
fi

# Convert SonarQube issues to reviewdog diagnostic format
DIAGNOSTICS=$(echo "$ISSUES_JSON" | jq -r '[.issues[] | {
  message: (.message + " [" + .rule + "]"),
  location: {
    path: (.component | sub("^[^:]+:"; "")),
    range: {
      start: {
        line: (.line // 1),
        column: 1
      },
      end: {
        line: (.line // 1),
        column: 100
      }
    }
  },
  severity: (
    if .severity == "BLOCKER" or .severity == "CRITICAL" or .severity == "MAJOR" then "ERROR"
    elif .severity == "MINOR" then "WARNING"
    else "INFO"
    end
  ),
  code: {
    value: .rule,
    url: (.rule | if . then "'$SONAR_HOST_URL'/coding_rules?open=" + . else "" end)
  },
  suggestions: []
}]' 2>/dev/null || echo "[]")

# Create final reviewdog format
cat > sonarqube-output.jsonl << EOF
{
  "source": {
    "name": "sonarqube",
    "url": "$SONAR_HOST_URL/dashboard?id=$SONAR_PROJECT_KEY"
  },
  "diagnostics": $DIAGNOSTICS
}
EOF

# Validate output
if ! jq empty sonarqube-output.jsonl 2>/dev/null; then
  echo "❌ Failed to create valid reviewdog format"
  exit 1
fi

ISSUE_COUNT=$(echo "$DIAGNOSTICS" | jq 'length' 2>/dev/null || echo "0")

# Count by severity
ERROR_COUNT=$(echo "$DIAGNOSTICS" | jq '[.[] | select(.severity == "ERROR")] | length' 2>/dev/null || echo "0")
WARNING_COUNT=$(echo "$DIAGNOSTICS" | jq '[.[] | select(.severity == "WARNING")] | length' 2>/dev/null || echo "0")
INFO_COUNT=$(echo "$DIAGNOSTICS" | jq '[.[] | select(.severity == "INFO")] | length' 2>/dev/null || echo "0")

echo "📊 Found $ISSUE_COUNT issues from SonarQube"
echo "   🔴 Errors: $ERROR_COUNT | 🟡 Warnings: $WARNING_COUNT | 🔵 Info: $INFO_COUNT"

if [[ "$HOTSPOT_COUNT" -gt 0 ]]; then
  echo "   🔐 Security Hotspots: $HOTSPOT_COUNT (require review)"
fi

# Display issues in terminal
if [[ "$ISSUE_COUNT" -gt 0 ]]; then
  echo ""
  echo "────────────────────────────────────────"
  
  # Display ERROR issues
  if [[ "$ERROR_COUNT" -gt 0 ]]; then
    echo "$DIAGNOSTICS" | jq -r '.[] | select(.severity == "ERROR") | 
      "🔴 [ERROR] " + .message + "\n        " + .location.path + ":" + (.location.range.start.line | tostring)'
    echo ""
  fi
  
  # Display WARNING issues
  if [[ "$WARNING_COUNT" -gt 0 ]]; then
    echo "$DIAGNOSTICS" | jq -r '.[] | select(.severity == "WARNING") | 
      "🟡 [WARN] " + .message + "\n        " + .location.path + ":" + (.location.range.start.line | tostring)'
    echo ""
  fi
  
  # Display INFO issues (only first 3 to avoid clutter)
  if [[ "$INFO_COUNT" -gt 0 ]]; then
    echo "$DIAGNOSTICS" | jq -r '.[] | select(.severity == "INFO") | 
      "🔵 [INFO] " + .message + "\n        " + .location.path + ":" + (.location.range.start.line | tostring)' | head -12
    if [[ "$INFO_COUNT" -gt 3 ]]; then
      echo "... and $(($INFO_COUNT - 3)) more info issues"
    fi
    echo ""
  fi
  
  echo "────────────────────────────────────────"
  echo ""
  echo "📋 Summary: $ERROR_COUNT errors, $WARNING_COUNT warnings, $INFO_COUNT info"
  if [[ "$HOTSPOT_COUNT" -gt 0 ]]; then
    echo "           $HOTSPOT_COUNT security hotspots"
  fi
  echo "🔗 View details: $SONAR_HOST_URL/dashboard?id=$SONAR_PROJECT_KEY"
  
  # Create overview summary
  cat > sonarqube-overview.txt << EOF
## 📊 SonarQube Analysis Results

**Project:** $SONAR_PROJECT_KEY

**Summary:**
- 🔴 Errors: $ERROR_COUNT
- 🟡 Warnings: $WARNING_COUNT
- 🔵 Info: $INFO_COUNT

**Total Issues:** $ISSUE_COUNT

[View Full Report on SonarQube]($SONAR_HOST_URL/dashboard?id=$SONAR_PROJECT_KEY)
EOF
fi

# Display Security Hotspots separately
if [[ "$HOTSPOT_COUNT" -gt 0 ]]; then
  if [[ "$ISSUE_COUNT" -eq 0 ]]; then
    echo ""
  fi
  echo "────────────────────────────────────────"
  echo "🔐 Security Hotspots (Require Manual Review)"
  echo "────────────────────────────────────────"
  echo ""
  
  # Display hotspots
  echo "$HOTSPOTS_JSON" | jq -r '.hotspots[] | 
    "🔐 [SECURITY] " + .message + "\n        " + (.component | sub("^[^:]+:"; "")) + ":" + (.line // 0 | tostring) + 
    "\n        Priority: " + .vulnerabilityProbability + " | Rule: " + .ruleKey + "\n"' | head -20
  
  if [[ "$HOTSPOT_COUNT" -gt 5 ]]; then
    echo "... and $(($HOTSPOT_COUNT - 5)) more hotspots"
    echo ""
  fi
  
  echo "────────────────────────────────────────"
  echo ""
  echo "🔐 Total Security Hotspots: $HOTSPOT_COUNT"
  echo "🔗 Review hotspots: $SONAR_HOST_URL/security_hotspots?id=$SONAR_PROJECT_KEY"
  echo ""
fi

echo "✅ SonarQube review completed"

# Cleanup: Move output files to temp directory (not in project)
TEMP_DIR="$HOME/.config/ai-review/temp"
mkdir -p "$TEMP_DIR"

if [[ -f "sonarqube-output.jsonl" ]]; then
  mv sonarqube-output.jsonl "$TEMP_DIR/" 2>/dev/null || true
fi

if [[ -f "sonarqube-overview.txt" ]]; then
  mv sonarqube-overview.txt "$TEMP_DIR/" 2>/dev/null || true
fi

# ============================================
# Comprehensive SonarQube Cleanup
# ============================================
echo "🧹 Cleaning up SonarQube generated files..."

# Remove all known SonarQube-generated files and directories
rm -rf .scannerwork 2>/dev/null || true
rm -rf .sonar 2>/dev/null || true
rm -f .sonar_lock 2>/dev/null || true
rm -f report-task.txt 2>/dev/null || true
rm -f sonar-report.json 2>/dev/null || true

# Clean up auto-generated sonar-project.properties
if [[ -f "sonar-project.properties" ]] && grep -q "auto-generated" "sonar-project.properties" 2>/dev/null; then
  rm -f sonar-project.properties 2>/dev/null || true
fi

# Clean up any .sonar* files in current directory
find . -maxdepth 1 -name ".sonar*" -type f -delete 2>/dev/null || true

echo "✅ Cleanup complete"

# Check for blocking issues
BLOCK_ON_HOTSPOTS="${SONAR_BLOCK_ON_HOTSPOTS:-true}"

# Exit with error if there are blocking issues (ERROR level)
if [[ "$ERROR_COUNT" -gt 0 ]]; then
  echo ""
  echo "🚫 SonarQube found $ERROR_COUNT error(s) - Please fix before committing"
  exit 1
fi

# Exit with error if there are Security Hotspots and blocking is enabled
if [[ "$BLOCK_ON_HOTSPOTS" == "true" && "$HOTSPOT_COUNT" -gt 0 ]]; then
  echo ""
  echo "🔐 SonarQube found $HOTSPOT_COUNT Security Hotspot(s) that require review"
  echo ""
  echo "Security Hotspots are security-sensitive code that needs manual review."
  echo "Review them at: $SONAR_HOST_URL/security_hotspots?id=$SONAR_PROJECT_KEY"
  echo ""
  echo "Options:"
  echo "  • Review and resolve hotspots on SonarQube dashboard"
  echo "  • Disable blocking: ai-review config set SONAR_BLOCK_ON_HOTSPOTS false"
  echo "  • Bypass once: git commit --no-verify"
  exit 1
fi

exit 0

