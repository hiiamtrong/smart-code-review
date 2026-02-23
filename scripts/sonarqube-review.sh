#!/usr/bin/env bash
set -e

# SonarQube Code Review Integration
# This script runs SonarQube analysis and converts results to reviewdog format

# Source platform abstraction layer
_SONAR_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$_SONAR_SCRIPT_DIR/lib/platform.sh" ]]; then
  source "$_SONAR_SCRIPT_DIR/lib/platform.sh"
elif [[ -f "$_SONAR_SCRIPT_DIR/platform.sh" ]]; then
  source "$_SONAR_SCRIPT_DIR/platform.sh"
elif [[ -f "$HOME/.config/ai-review/hooks/platform.sh" ]]; then
  source "$HOME/.config/ai-review/hooks/platform.sh"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Apply color settings if platform.sh is loaded
if type apply_color_settings &>/dev/null; then
  apply_color_settings
fi

# ============================================
# Helper Functions
# ============================================

log_error() {
  echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_success() {
  echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

print_separator() {
  echo -e "${CYAN}────────────────────────────────────────${NC}"
}

print_issue() {
  local severity="$1"
  local file="$2"
  local line="$3"
  local message="$4"

  case "$severity" in
    ERROR)
      echo -e "${RED}[ERROR]${NC} $message"
      ;;
    WARNING)
      echo -e "${YELLOW}[WARN]${NC} $message"
      ;;
    *)
      echo -e "${BLUE}[INFO]${NC} $message"
      ;;
  esac
  echo -e "        ${BOLD}$file:$line${NC}"
}

# ============================================
# Configuration
# ============================================

# Quiet mode: less verbose when called from pre-commit hook
QUIET="${SONAR_QUIET:-false}"

# Configuration (can be overridden by environment variables)
SONAR_HOST_URL="${SONAR_HOST_URL:-http://localhost:9000}"
SONAR_TOKEN="${SONAR_TOKEN:-}"
SONAR_PROJECT_KEY="${SONAR_PROJECT_KEY:-}"
SONAR_PROJECT_NAME="${SONAR_PROJECT_NAME:-$(basename "$(git rev-parse --show-toplevel)")}"
SONAR_PROJECT_VERSION="${SONAR_PROJECT_VERSION:-1.0}"
SONAR_SOURCES="${SONAR_SOURCES:-.}"
SONAR_FILTER_CHANGED_LINES_ONLY="${SONAR_FILTER_CHANGED_LINES_ONLY:-true}"

# Default exclusions
DEFAULT_EXCLUSIONS="**/node_modules/**,**/dist/**,**/build/**,**/target/**,**/vendor/**,**/*.test.js,**/*.spec.ts,**/*.test.ts,**/*.spec.js"

# Load exclusions from .gitignore and .sonarignore
SONAR_EXCLUSIONS="$DEFAULT_EXCLUSIONS"

# Load from .gitignore (files already ignored by git shouldn't be scanned)
if [[ -f ".gitignore" ]]; then
  GITIGNORE_EXCLUSIONS=$(grep -v '^#' .gitignore | grep -v '^[[:space:]]*$' | grep -v '^!' \
    | grep -v -E '^(node_modules|dist|build|target|vendor)' \
    | sed 's|/$|/**|' | sed 's|^/|**/|' \
    | grep -v '^[[:space:]]*$' | tr '\n' ',' | sed 's/,$//')
  if [[ -n "$GITIGNORE_EXCLUSIONS" ]]; then
    SONAR_EXCLUSIONS="$SONAR_EXCLUSIONS,$GITIGNORE_EXCLUSIONS"
  fi
fi

# Load from .sonarignore (additional SonarQube-specific exclusions)
if [[ -f ".sonarignore" ]]; then
  CUSTOM_EXCLUSIONS=$(grep -v '^#' .sonarignore | grep -v '^[[:space:]]*$' | sed 's/^/\*\*\//g' | tr '\n' ',' | sed 's/,$//')
  if [[ -n "$CUSTOM_EXCLUSIONS" ]]; then
    SONAR_EXCLUSIONS="$SONAR_EXCLUSIONS,$CUSTOM_EXCLUSIONS"
  fi
fi

# Check required variables
if [[ -z "$SONAR_TOKEN" ]]; then
  log_warn "SONAR_TOKEN not set, skipping SonarQube analysis"
  exit 0
fi

if [[ -z "$SONAR_PROJECT_KEY" ]]; then
  SONAR_PROJECT_KEY=$(basename "$(git rev-parse --show-toplevel)" | sed 's/[^a-zA-Z0-9_-]/_/g')
fi

log_info "Project: ${BOLD}$SONAR_PROJECT_KEY${NC} → $SONAR_HOST_URL"

# ============================================
# Detect & Install Scanner
# ============================================

SONAR_SCANNER=""

# Determine scanner binary name based on platform
_SCANNER_BIN="sonar-scanner"
if type is_windows &>/dev/null && is_windows; then
  _SCANNER_BIN="sonar-scanner.bat"
fi

if command -v sonar-scanner &> /dev/null; then
  SONAR_SCANNER="sonar-scanner"
elif command -v sonar-scanner-cli &> /dev/null; then
  SONAR_SCANNER="sonar-scanner-cli"
elif [[ -f "$HOME/.sonar/sonar-scanner/bin/$_SCANNER_BIN" ]]; then
  SONAR_SCANNER="$HOME/.sonar/sonar-scanner/bin/$_SCANNER_BIN"
elif [[ -f "$HOME/.sonar/sonar-scanner/bin/sonar-scanner" ]]; then
  SONAR_SCANNER="$HOME/.sonar/sonar-scanner/bin/sonar-scanner"
else
  log_info "Installing SonarQube Scanner..."
  SCANNER_VERSION="6.2.1.4610"
  SCANNER_DIR="$HOME/.sonar/sonar-scanner"

  mkdir -p "$HOME/.sonar"
  cd "$HOME/.sonar"

  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}-linux-x64.zip"
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}-macosx-x64.zip"
  elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "mingw"* ]] || [[ "$OSTYPE" == "cygwin"* ]]; then
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}-windows-x64.zip"
  else
    SCANNER_ZIP="sonar-scanner-cli-${SCANNER_VERSION}.zip"
  fi

  if [[ ! -d "$SCANNER_DIR" ]]; then
    log_info "Downloading SonarQube Scanner ($SCANNER_ZIP)..."
    DOWNLOAD_URL="https://binaries.sonarsource.com/Distribution/sonar-scanner-cli/${SCANNER_ZIP}"
    curl -sSL "$DOWNLOAD_URL" -o scanner.zip

    # Verify download is a valid zip (zip files start with PK magic bytes)
    if [[ ! -f scanner.zip ]] || [[ $(wc -c < scanner.zip) -lt 1000 ]]; then
      log_error "Download failed or file too small"
      rm -f scanner.zip
      cd - > /dev/null
      exit 1
    fi

    ZIP_HEADER=$(head -c 2 scanner.zip 2>/dev/null || true)
    if [[ "$ZIP_HEADER" != "PK" ]]; then
      log_error "Downloaded file is not a valid zip (possibly HTML error page)"
      log_error "URL: $DOWNLOAD_URL"
      rm -f scanner.zip
      cd - > /dev/null
      exit 1
    fi

    # On Windows, prefer PowerShell Expand-Archive (more reliable than Git Bash unzip)
    if command -v powershell.exe &>/dev/null; then
      log_info "Extracting scanner..."
      WIN_ZIP=$(cygpath -w "$HOME/.sonar/scanner.zip" 2>/dev/null || echo "$HOME/.sonar/scanner.zip")
      WIN_DEST=$(cygpath -w "$HOME/.sonar" 2>/dev/null || echo "$HOME/.sonar")
      powershell.exe -NoProfile -Command "Expand-Archive -Path '$WIN_ZIP' -DestinationPath '$WIN_DEST' -Force"
    elif type safe_unzip &>/dev/null; then
      safe_unzip scanner.zip "$HOME/.sonar"
    elif command -v unzip &>/dev/null; then
      unzip -q scanner.zip
    else
      log_error "Cannot extract scanner: neither unzip nor PowerShell available"
      rm -f scanner.zip
      cd - > /dev/null
      exit 1
    fi

    mv sonar-scanner-${SCANNER_VERSION}* sonar-scanner 2>/dev/null || true
    rm -f scanner.zip
    log_success "Scanner installed"
  fi

  SONAR_SCANNER="$SCANNER_DIR/bin/$_SCANNER_BIN"
  # Fallback to non-.bat if .bat doesn't exist
  if [[ ! -f "$SONAR_SCANNER" ]] && [[ -f "$SCANNER_DIR/bin/sonar-scanner" ]]; then
    SONAR_SCANNER="$SCANNER_DIR/bin/sonar-scanner"
  fi
  cd - > /dev/null
fi

# ============================================
# PR Analysis (CI/CD)
# ============================================

CHANGED_FILES=""
if [[ -n "$GITHUB_BASE_REF" ]]; then
  log_info "PR detected: analyzing changed files only"
  if git rev-parse --verify origin/$GITHUB_BASE_REF >/dev/null 2>&1; then
    CHANGED_FILES=$(git diff --name-only origin/$GITHUB_BASE_REF...HEAD | tr '\n' ',')
  fi
fi

# ============================================
# Create sonar-project.properties
# ============================================

if [[ ! -f "sonar-project.properties" ]]; then
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

fi

# ============================================
# Run SonarQube Analysis
# ============================================

# Pass token via environment variable (avoids exposing it in process list)
export SONAR_TOKEN="$SONAR_TOKEN"
SONAR_OPTS="-Dsonar.host.url=$SONAR_HOST_URL -Dsonar.token=$SONAR_TOKEN"

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

  # Differential Analysis: Only scan staged files (current commit)
  FILES_TO_SCAN=$(git diff --cached --name-only --diff-filter=ACMRTUXB 2>/dev/null | tr '\n' ',')

  # Remove duplicates and clean up
  FILES_TO_SCAN=$(echo "$FILES_TO_SCAN" | tr ',' '\n' | sort -u | grep -v '^$' | tr '\n' ',' | sed 's/,$//')

  if [[ -n "$FILES_TO_SCAN" ]]; then
    FILE_COUNT=$(echo "$FILES_TO_SCAN" | tr ',' '\n' | wc -l)
    log_info "Scanning $FILE_COUNT changed file(s)..."
    SONAR_OPTS="$SONAR_OPTS -Dsonar.inclusions=$FILES_TO_SCAN"
  else
    log_info "Scanning all files..."
  fi
fi

# Run scanner (suppress all logs except errors)
if type safe_mktemp &>/dev/null; then
  SCANNER_LOG=$(safe_mktemp "sonar-scanner")
else
  SCANNER_LOG=$(mktemp 2>/dev/null || mktemp -t sonar-scanner 2>/dev/null)
fi

echo "Running SonarQube scanner... (this may take 30-60 seconds)"
echo ""

# Run with timeout to prevent hanging forever (2 minutes max)
# Check if timeout command exists (Linux/MacOS have it, Windows Git Bash might not)
if command -v timeout &> /dev/null; then
  # Linux/MacOS with timeout command
  if timeout 120 $SONAR_SCANNER $SONAR_OPTS > "$SCANNER_LOG" 2>&1; then
    SCANNER_EXIT=0
  else
    SCANNER_EXIT=$?
    if [[ $SCANNER_EXIT -eq 124 ]]; then
      echo ""
      log_error "SonarQube scanner timed out after 2 minutes"
      echo "This might indicate:"
      echo "  - Network issues connecting to SonarQube server"
      echo "  - Too many files to scan"
      echo "  - SonarQube server is slow/unresponsive"
      echo ""
      echo "Suggestion: Disable local SonarQube and use CI/CD instead"
      echo "  Run: ai-review config set ENABLE_SONARQUBE_LOCAL false"
      rm -f "$SCANNER_LOG"
      exit 1
    fi
  fi
else
  # No timeout command available (macOS/Windows) - run directly
  log_info "Running scanner (press Ctrl+C to cancel)..."
  if $SONAR_SCANNER $SONAR_OPTS > "$SCANNER_LOG" 2>&1; then
    SCANNER_EXIT=0
  else
    SCANNER_EXIT=$?
  fi
fi

# Only show real errors (not Java version warnings)
grep -E "(ERROR|FAIL)" "$SCANNER_LOG" | grep -v "Java 17 scanner" || true

rm -f "$SCANNER_LOG"

if [[ $SCANNER_EXIT -ne 0 ]]; then
  log_error "SonarQube analysis failed"
  exit 1
fi

# ============================================
# Wait for Analysis Results
# ============================================
TASK_ID=$(grep 'ceTaskId=' "$HOME/.config/ai-review/temp/report-task.txt" 2>/dev/null | sed 's/.*ceTaskId=//' | tr -d '[:space:]' || true)
MAX_WAIT=60
WAIT_ELAPSED=0
if [[ -n "$TASK_ID" ]]; then
  while [[ $WAIT_ELAPSED -lt $MAX_WAIT ]]; do
    TASK_STATUS=$(curl -s -u "$SONAR_TOKEN:" "$SONAR_HOST_URL/api/ce/task?id=$TASK_ID" | jq -r '.task.status // ""' 2>/dev/null)
    if [[ "$TASK_STATUS" == "SUCCESS" || "$TASK_STATUS" == "FAILED" || "$TASK_STATUS" == "CANCELED" ]]; then
      break
    fi
    sleep 2
    WAIT_ELAPSED=$((WAIT_ELAPSED + 2))
  done
else
  # Fallback: wait a few seconds if task ID not found
  sleep 5
fi

# ============================================
# Fetch & Process Results
# ============================================

# Fetch issues via API (Bugs, Vulnerabilities, Code Smells)
ISSUES_JSON=$(curl -s -u "$SONAR_TOKEN:" \
  "$SONAR_HOST_URL/api/issues/search?componentKeys=$SONAR_PROJECT_KEY&resolved=false&ps=500" || echo "")

if [[ -z "$ISSUES_JSON" || "$ISSUES_JSON" == "null" ]]; then
  log_warn "Could not fetch issues from SonarQube"
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

# ============================================
# Filter Issues by Changed Lines Only
# ============================================

# Get changed line ranges from git diff
get_changed_lines() {
  # Pre-commit: only staged changes matter
  local diff_command="git diff --cached -U0"
  
  # Parse diff to extract changed lines
  # Format: file|start_line|line_count (one per hunk)
  # Note: Uses POSIX awk (compatible with BSD awk on macOS, mawk on Linux)
  $diff_command | awk '
    /^diff --git/ {
      # Extract filename from "diff --git a/path b/path"
      split($0, parts, " ")
      # parts[4] is "b/path"
      current_file = substr(parts[4], 3)
    }
    /^@@/ {
      # Parse hunk header: @@ -old_start,old_count +new_start,new_count @@
      # We care about +new_start,new_count
      for (i = 1; i <= NF; i++) {
        if (substr($i, 1, 1) == "+") {
          plus_part = substr($i, 2)
          split(plus_part, nums, ",")
          start_line = nums[1] + 0
          line_count = (nums[2] != "" ? nums[2] + 0 : 1)
          if (current_file != "" && start_line > 0) {
            print current_file "|" start_line "|" line_count
          }
          break
        }
      }
    }
  '
}

# Filter diagnostics to only include issues on changed lines (single jq pass)
filter_diagnostics_by_changed_lines() {
  local diagnostics="$1"
  local changed_lines="$2"

  # If no changed lines info or diagnostics is empty, return as-is
  if [[ -z "$changed_lines" || "$diagnostics" == "[]" ]]; then
    echo "$diagnostics"
    return
  fi

  # Build ranges as JSON array: [{"f":"file","s":start,"e":end}, ...]
  local ranges_json
  ranges_json=$(echo "$changed_lines" | awk -F'|' '{printf "{\"f\":\"%s\",\"s\":%d,\"e\":%d}\n", $1, $2, $2+$3-1}' | jq -s '.')

  # Filter in a single jq pass
  echo "$diagnostics" | jq --argjson ranges "$ranges_json" \
    '[.[] | . as $issue | select(any($ranges[]; .f == $issue.location.path and $issue.location.range.start.line >= .s and $issue.location.range.start.line <= .e))]'
}

# Apply filtering (only if enabled)
if [[ "$SONAR_FILTER_CHANGED_LINES_ONLY" == "true" ]]; then
  CHANGED_LINES=$(get_changed_lines)
  
  if [[ -n "$CHANGED_LINES" ]]; then
    ORIGINAL_COUNT=$(echo "$DIAGNOSTICS" | jq 'length' 2>/dev/null || echo "0")
    DIAGNOSTICS=$(filter_diagnostics_by_changed_lines "$DIAGNOSTICS" "$CHANGED_LINES")
    FILTERED_COUNT=$(echo "$DIAGNOSTICS" | jq 'length' 2>/dev/null || echo "0")
    
    if [[ "$ORIGINAL_COUNT" -gt "$FILTERED_COUNT" ]]; then
      log_info "Filtered out $(($ORIGINAL_COUNT - $FILTERED_COUNT)) issue(s) from unchanged lines"
    fi
  fi
fi

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
  log_error "Failed to create valid reviewdog format"
  exit 1
fi

# ============================================
# Display Results
# ============================================

ISSUE_COUNT=$(echo "$DIAGNOSTICS" | jq 'length' 2>/dev/null || echo "0")

# Count by severity
ERROR_COUNT=$(echo "$DIAGNOSTICS" | jq '[.[] | select(.severity == "ERROR")] | length' 2>/dev/null || echo "0")
WARNING_COUNT=$(echo "$DIAGNOSTICS" | jq '[.[] | select(.severity == "WARNING")] | length' 2>/dev/null || echo "0")
INFO_COUNT=$(echo "$DIAGNOSTICS" | jq '[.[] | select(.severity == "INFO")] | length' 2>/dev/null || echo "0")

# Display issues in terminal
if [[ "$ISSUE_COUNT" -gt 0 ]]; then
  echo ""
  print_separator

  # Display ERROR issues
  if [[ "$ERROR_COUNT" -gt 0 ]]; then
    echo "$DIAGNOSTICS" | jq -c '.[] | select(.severity == "ERROR")' | tr -d '\r' | while read -r issue; do
      local_msg=$(echo "$issue" | jq -r '.message // ""')
      local_file=$(echo "$issue" | jq -r '.location.path // "unknown"')
      local_line=$(echo "$issue" | jq -r '.location.range.start.line // 0')
      print_issue "ERROR" "$local_file" "$local_line" "$local_msg"
      echo ""
    done
  fi

  # Display WARNING issues
  if [[ "$WARNING_COUNT" -gt 0 ]]; then
    echo "$DIAGNOSTICS" | jq -c '.[] | select(.severity == "WARNING")' | tr -d '\r' | while read -r issue; do
      local_msg=$(echo "$issue" | jq -r '.message // ""')
      local_file=$(echo "$issue" | jq -r '.location.path // "unknown"')
      local_line=$(echo "$issue" | jq -r '.location.range.start.line // 0')
      print_issue "WARNING" "$local_file" "$local_line" "$local_msg"
      echo ""
    done
  fi

  # Display INFO issues (only first 3 to avoid clutter)
  if [[ "$INFO_COUNT" -gt 0 ]]; then
    echo "$DIAGNOSTICS" | jq -c '[.[] | select(.severity == "INFO")] | .[:3] | .[]' | tr -d '\r' | while read -r issue; do
      local_msg=$(echo "$issue" | jq -r '.message // ""')
      local_file=$(echo "$issue" | jq -r '.location.path // "unknown"')
      local_line=$(echo "$issue" | jq -r '.location.range.start.line // 0')
      print_issue "INFO" "$local_file" "$local_line" "$local_msg"
      echo ""
    done
    if [[ "$INFO_COUNT" -gt 3 ]]; then
      echo "        ... and $(($INFO_COUNT - 3)) more info issues"
      echo ""
    fi
  fi

  print_separator
fi

# Display Security Hotspots separately
if [[ "$HOTSPOT_COUNT" -gt 0 ]]; then
  if [[ "$ISSUE_COUNT" -eq 0 ]]; then
    echo ""
  fi
  print_separator
  echo -e "${BOLD}Security Hotspots (Require Manual Review)${NC}"
  print_separator
  echo ""

  # Display hotspots
  echo "$HOTSPOTS_JSON" | jq -c '.hotspots[:5] | .[]' 2>/dev/null | tr -d '\r' | while read -r hotspot; do
    local_msg=$(echo "$hotspot" | jq -r '.message // ""')
    local_file=$(echo "$hotspot" | jq -r '.component | sub("^[^:]+:"; "")' 2>/dev/null)
    local_line=$(echo "$hotspot" | jq -r '.line // 0')
    local_priority=$(echo "$hotspot" | jq -r '.vulnerabilityProbability // ""')
    local_rule=$(echo "$hotspot" | jq -r '.ruleKey // ""')
    print_issue "WARNING" "$local_file" "$local_line" "$local_msg"
    echo -e "        Priority: ${BOLD}$local_priority${NC} | Rule: $local_rule"
    echo ""
  done

  if [[ "$HOTSPOT_COUNT" -gt 5 ]]; then
    echo "        ... and $(($HOTSPOT_COUNT - 5)) more hotspots"
    echo ""
  fi

  print_separator
fi

# Final summary line
if [[ "$ISSUE_COUNT" -eq 0 && "$HOTSPOT_COUNT" -eq 0 ]]; then
  log_success "SonarQube: No issues found"
else
  echo ""
  echo -e "${BOLD}SonarQube:${NC} $ERROR_COUNT errors, $WARNING_COUNT warnings, $INFO_COUNT info"
  if [[ "$HOTSPOT_COUNT" -gt 0 ]]; then
    echo "          $HOTSPOT_COUNT security hotspots"
  fi
fi

# ============================================
# Cleanup & Exit
# ============================================

# Note: Cleanup of SonarQube-generated files is handled by pre-commit.sh cleanup_temp_files()
# Save report-task.txt for polling before cleanup
TEMP_DIR="$HOME/.config/ai-review/temp"
mkdir -p "$TEMP_DIR"

if [[ -f "report-task.txt" ]]; then
  cp report-task.txt "$TEMP_DIR/" 2>/dev/null || true
fi

# Check for blocking issues
BLOCK_ON_HOTSPOTS="${SONAR_BLOCK_ON_HOTSPOTS:-true}"

# Exit with error if there are blocking issues (ERROR level)
if [[ "$ERROR_COUNT" -gt 0 ]]; then
  echo ""
  log_error "SonarQube found $ERROR_COUNT error(s) - Please fix before committing"
  exit 1
fi

# Exit with error if there are Security Hotspots and blocking is enabled
if [[ "$BLOCK_ON_HOTSPOTS" == "true" && "$HOTSPOT_COUNT" -gt 0 ]]; then
  echo ""
  log_error "SonarQube found $HOTSPOT_COUNT Security Hotspot(s) that require review"
  echo ""
  echo "        Security Hotspots are security-sensitive code that needs manual review."
  log_info "Review them at: $SONAR_HOST_URL/security_hotspots?id=$SONAR_PROJECT_KEY"
  echo ""
  echo "        Options:"
  echo "          - Review and resolve hotspots on SonarQube dashboard"
  echo "          - Disable blocking: ai-review config set SONAR_BLOCK_ON_HOTSPOTS false"
  echo "          - Bypass once: git commit --no-verify"
  exit 1
fi

exit 0
