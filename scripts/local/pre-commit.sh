#!/usr/bin/env bash
# AI-REVIEW-HOOK
# Pre-commit hook for AI-powered code review
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Paths
CONFIG_DIR="$HOME/.config/ai-review"
CONFIG_FILE="$CONFIG_DIR/config"

# ============================================
# Helper Functions
# ============================================

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[OK]${NC} $1"
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
# Load Configuration
# ============================================

load_config() {
  if [[ ! -f "$CONFIG_FILE" ]]; then
    log_error "Configuration not found at $CONFIG_FILE"
    echo "Please run: ai-review setup"
    exit 1
  fi

  source "$CONFIG_FILE"

  if [[ -z "$AI_GATEWAY_URL" ]]; then
    log_error "AI_GATEWAY_URL not configured"
    exit 1
  fi

  if [[ -z "$AI_GATEWAY_API_KEY" ]]; then
    log_error "AI_GATEWAY_API_KEY not configured"
    exit 1
  fi

  # Set defaults
  AI_MODEL="${AI_MODEL:-gemini-2.0-flash}"
  AI_PROVIDER="${AI_PROVIDER:-google}"
}

# ============================================
# Get Staged Changes
# ============================================

get_staged_diff() {
  DIFF=$(git diff --cached)

  if [[ -z "$DIFF" ]]; then
    log_success "No staged changes to review"
    exit 0
  fi

  DIFF_LINES=$(echo "$DIFF" | wc -l)
  log_info "Reviewing $DIFF_LINES lines of changes"
}

# ============================================
# Filter Ignored Files
# ============================================

filter_ignored_files() {
  local repo_root=$(git rev-parse --show-toplevel)
  local ignore_file="$repo_root/.aireviewignore"

  if [[ ! -f "$ignore_file" ]]; then
    return
  fi

  log_info "Applying ignore patterns..."

  # Read ignore patterns
  local patterns=()
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    line=$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    [[ -n "$line" ]] && patterns+=("$line")
  done < "$ignore_file"

  if [[ ${#patterns[@]} -eq 0 ]]; then
    return
  fi

  # Filter diff
  local filtered_diff=""
  local current_file=""
  local current_block=""
  local in_block=false

  while IFS= read -r line; do
    if [[ "$line" =~ ^diff\ --git\ a/(.+)\ b/(.+)$ ]]; then
      # Save previous block if not ignored
      if [[ -n "$current_file" && "$in_block" == true ]]; then
        local should_ignore=false
        for pattern in "${patterns[@]}"; do
          local grep_pattern=$(echo "$pattern" | sed 's/\./\\./g' | sed 's/\*/.*/g')
          if echo "$current_file" | grep -qE "$grep_pattern"; then
            should_ignore=true
            break
          fi
        done
        [[ "$should_ignore" == false ]] && filtered_diff="${filtered_diff}${current_block}"
      fi
      current_file="${BASH_REMATCH[2]}"
      current_block="$line"$'\n'
      in_block=true
    elif [[ "$in_block" == true ]]; then
      current_block="${current_block}${line}"$'\n'
    fi
  done <<< "$DIFF"

  # Handle last block
  if [[ -n "$current_file" && "$in_block" == true ]]; then
    local should_ignore=false
    for pattern in "${patterns[@]}"; do
      local grep_pattern=$(echo "$pattern" | sed 's/\./\\./g' | sed 's/\*/.*/g')
      if echo "$current_file" | grep -qE "$grep_pattern"; then
        should_ignore=true
        break
      fi
    done
    [[ "$should_ignore" == false ]] && filtered_diff="${filtered_diff}${current_block}"
  fi

  if [[ -z "$filtered_diff" ]]; then
    log_success "All changes are in ignored files, skipping review"
    exit 0
  fi

  DIFF="$filtered_diff"
}

# ============================================
# Detect Language
# ============================================

detect_language() {
  LANGUAGE="unknown"
  if echo "$DIFF" | grep -q "\.ts\|\.tsx"; then
    LANGUAGE="typescript"
  elif echo "$DIFF" | grep -q "\.js\|\.jsx"; then
    LANGUAGE="javascript"
  elif echo "$DIFF" | grep -q "\.py"; then
    LANGUAGE="python"
  elif echo "$DIFF" | grep -q "\.java"; then
    LANGUAGE="java"
  elif echo "$DIFF" | grep -q "\.go"; then
    LANGUAGE="go"
  elif echo "$DIFF" | grep -q "\.cs"; then
    LANGUAGE="csharp"
  elif echo "$DIFF" | grep -q "\.rb"; then
    LANGUAGE="ruby"
  elif echo "$DIFF" | grep -q "\.php"; then
    LANGUAGE="php"
  fi
}

# ============================================
# Call AI Gateway
# ============================================

call_ai_gateway() {
  # Collect git info
  local commit_hash=$(git rev-parse HEAD 2>/dev/null || echo "staged")
  local branch_name=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
  local author_name=$(git config user.name 2>/dev/null || echo "unknown")
  local author_email=$(git config user.email 2>/dev/null || echo "unknown")
  local repo_url=$(git remote get-url origin 2>/dev/null || echo "local")

  # Save diff to temp file
  local diff_file=$(mktemp)
  echo "$DIFF" > "$diff_file"

  # Create JSON payload
  local json_payload=$(jq -n \
    --arg language "$LANGUAGE" \
    --arg ai_model "$AI_MODEL" \
    --arg ai_provider "$AI_PROVIDER" \
    --arg commit_hash "$commit_hash" \
    --arg branch_name "$branch_name" \
    --arg author_name "$author_name" \
    --arg author_email "$author_email" \
    --arg repo_url "$repo_url" \
    '{
      "ai_model": $ai_model,
      "ai_provider": $ai_provider,
      "language": $language,
      "review_mode": "file",
      "git_info": {
        "commit_hash": $commit_hash,
        "branch_name": $branch_name,
        "repo_url": $repo_url,
        "author": {
          "name": $author_name,
          "email": $author_email
        }
      }
    }')

  # Make API request
  local api_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$AI_GATEWAY_URL/review" \
    -H "X-API-Key: $AI_GATEWAY_API_KEY" \
    -X POST \
    -F "metadata=$json_payload" \
    -F "git_diff=@$diff_file" 2>/dev/null)

  # Cleanup
  rm -f "$diff_file"

  # Parse response
  local http_status=$(echo "$api_response" | tail -n1 | sed 's/HTTP_STATUS://')
  local api_body=$(echo "$api_response" | sed '$d')

  if [[ "$http_status" != "200" ]]; then
    log_error "API request failed (HTTP $http_status)"
    echo "$api_body" | head -5
    exit 0  # Don't block commit on API failure
  fi

  REVIEW_JSON="$api_body"
}

# ============================================
# Parse and Display Results
# ============================================

display_results() {
  if [[ -z "$REVIEW_JSON" ]]; then
    log_success "No issues found"
    exit 0
  fi

  # Check if valid JSON
  if ! echo "$REVIEW_JSON" | jq empty 2>/dev/null; then
    log_warn "Could not parse AI response"
    exit 0
  fi

  # Extract diagnostics
  local diagnostics=$(echo "$REVIEW_JSON" | jq -c '.diagnostics // []' 2>/dev/null)

  if [[ "$diagnostics" == "[]" || "$diagnostics" == "null" ]]; then
    log_success "No issues found"
    exit 0
  fi

  # Count by severity
  local error_count=$(echo "$diagnostics" | jq '[.[] | select(.severity == "ERROR")] | length')
  local warning_count=$(echo "$diagnostics" | jq '[.[] | select(.severity == "WARNING")] | length')
  local info_count=$(echo "$diagnostics" | jq '[.[] | select(.severity == "INFO")] | length')

  # Display issues
  echo ""
  print_separator

  echo "$diagnostics" | jq -c '.[]' | while read -r issue; do
    local severity=$(echo "$issue" | jq -r '.severity // "INFO"')
    local message=$(echo "$issue" | jq -r '.message // "No message"')
    local file=$(echo "$issue" | jq -r '.location.path // "unknown"')
    local line=$(echo "$issue" | jq -r '.location.range.start.line // 0')

    print_issue "$severity" "$file" "$line" "$message"
    echo ""
  done

  print_separator
  echo ""
  echo -e "${BOLD}Summary:${NC} $error_count errors, $warning_count warnings, $info_count info"
  echo ""

  # Determine exit code based on errors
  if [[ "$error_count" -gt 0 ]]; then
    log_error "Commit blocked - please fix ERROR issues first"
    echo "        Use 'git commit --no-verify' to bypass (not recommended)"
    exit 1
  elif [[ "$warning_count" -gt 0 ]]; then
    log_success "Commit allowed - but consider fixing warnings above"
    exit 0
  else
    log_success "Commit allowed"
    exit 0
  fi
}

# ============================================
# Main
# ============================================

main() {
  log_info "AI Review analyzing your changes..."
  echo ""

  load_config
  get_staged_diff
  filter_ignored_files
  detect_language
  call_ai_gateway
  display_results
}

main
