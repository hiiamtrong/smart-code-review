#!/usr/bin/env bash
# AI-REVIEW-HOOK
# Pre-commit hook for AI-powered code review
set -e

VERSION="1.23.3"

# Source platform abstraction layer
_PRECOMMIT_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$_PRECOMMIT_SCRIPT_DIR/../lib/platform.sh" ]]; then
  source "$_PRECOMMIT_SCRIPT_DIR/../lib/platform.sh"
elif [[ -f "$_PRECOMMIT_SCRIPT_DIR/platform.sh" ]]; then
  source "$_PRECOMMIT_SCRIPT_DIR/platform.sh"
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

  # Load global config
  source "$CONFIG_FILE"

  # Set defaults
  ENABLE_AI_REVIEW="${ENABLE_AI_REVIEW:-true}"
  AI_MODEL="${AI_MODEL:-gemini-2.0-flash}"
  AI_PROVIDER="${AI_PROVIDER:-google}"

  # Only require AI credentials if AI review is enabled
  if [[ "$ENABLE_AI_REVIEW" == "true" ]]; then
    if [[ -z "$AI_GATEWAY_URL" ]]; then
      log_error "AI_GATEWAY_URL not configured"
      exit 1
    fi
    if [[ -z "$AI_GATEWAY_API_KEY" ]]; then
      log_error "AI_GATEWAY_API_KEY not configured"
      exit 1
    fi
  fi
  ENABLE_SONARQUBE_LOCAL="${ENABLE_SONARQUBE_LOCAL:-false}"
  SONAR_BLOCK_ON_HOTSPOTS="${SONAR_BLOCK_ON_HOTSPOTS:-true}"
  SONAR_FILTER_CHANGED_LINES_ONLY="${SONAR_FILTER_CHANGED_LINES_ONLY:-true}"
  
  # Load project-specific config from git config (overrides global config)
  local project_key=$(git config --local aireview.sonarProjectKey 2>/dev/null)

  if [[ -n "$project_key" ]]; then
    SONAR_PROJECT_KEY="$project_key"
  fi
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

# Add line numbers to diff using showlinenum.awk (run AFTER filtering)
format_diff() {
  local showlinenum="$CONFIG_DIR/hooks/showlinenum.awk"
  if command -v gawk &>/dev/null && [[ -f "$showlinenum" ]]; then
    local formatted
    formatted=$(echo "$DIFF" | gawk -f "$showlinenum" show_header=0 show_path=1 2>/dev/null) || true
    if [[ -n "$formatted" ]]; then
      DIFF="$formatted"
    fi
  fi
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

  # Read ignore patterns (strip CR for Windows CRLF compatibility)
  local patterns=()
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
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
    line="${line%$'\r'}"
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
# Call AI Gateway (with SSE Streaming)
# ============================================

call_ai_gateway() {
  # Collect git info
  local commit_hash=$(git rev-parse HEAD 2>/dev/null || echo "staged")
  local branch_name=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
  local author_name=$(git config user.name 2>/dev/null || echo "unknown")
  local author_email=$(git config user.email 2>/dev/null || echo "unknown")
  local repo_url=$(git remote get-url origin 2>/dev/null || echo "local")

  # Save diff to temp file
  local diff_file
  if type safe_mktemp &>/dev/null; then
    diff_file=$(safe_mktemp "ai-diff")
  else
    diff_file=$(mktemp 2>/dev/null || mktemp -t ai-diff 2>/dev/null)
  fi
  echo "$DIFF" > "$diff_file"

  # Create JSON payload with streaming enabled
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
      "stream": true,
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

  # Temp files for processing (must be accessible outside subshell)
  local _mk
  if type safe_mktemp &>/dev/null; then _mk="safe_mktemp"; else _mk="mktemp"; fi
  local result_file=$($_mk "ai-result" 2>/dev/null || mktemp)
  local diagnostics_file=$($_mk "ai-diag" 2>/dev/null || mktemp)
  local text_buffer_file=$($_mk "ai-text" 2>/dev/null || mktemp)
  local has_diagnostics_file=$($_mk "ai-hasdiag" 2>/dev/null || mktemp)
  local api_error_file=$($_mk "ai-apierr" 2>/dev/null || mktemp)

  echo ""
  print_separator
  log_info "AI is reviewing your code..."
  echo ""

  # Initialize files
  : > "$diagnostics_file"
  : > "$text_buffer_file"
  echo "0" > "$has_diagnostics_file"
  echo "0" > "$api_error_file"

  # Store file paths in temp file for subshell access
  local current_event=""

  # Make streaming API request with SSE
  # Use a temp file to capture the SSE stream for cross-platform compatibility
  # (process substitution < <(...) is unreliable on some Windows Git Bash versions)
  local sse_stream_file
  if type safe_mktemp &>/dev/null; then
    sse_stream_file=$(safe_mktemp "ai-sse")
  else
    sse_stream_file=$(mktemp 2>/dev/null || mktemp -t ai-sse 2>/dev/null)
  fi

  curl -sN "$AI_GATEWAY_URL/review" \
    -H "X-API-Key: $AI_GATEWAY_API_KEY" \
    -H "Accept: text/event-stream" \
    -X POST \
    -F "metadata=$json_payload" \
    -F "git_diff=@$diff_file" > "$sse_stream_file" 2>/dev/null || true

  while IFS= read -r line; do
    # Skip empty lines and carriage returns
    line="${line%$'\r'}"
    [[ -z "$line" ]] && continue

    # Parse event type line
    if [[ "$line" =~ ^event:\ (.*)$ ]]; then
      current_event="${BASH_REMATCH[1]}"
      continue
    fi

    # Parse data line
    if [[ "$line" =~ ^data:\ (.*)$ ]]; then
      local data="${BASH_REMATCH[1]}"

      # Skip empty data
      [[ -z "$data" ]] && continue

      # Try to parse as JSON
      if ! echo "$data" | jq empty 2>/dev/null; then
        continue
      fi

      case "$current_event" in
        "progress")
          # Show progress info
          local event_type=$(echo "$data" | jq -r '.type // ""' 2>/dev/null)
          local total_chunks=$(echo "$data" | jq -r '.total_chunks // 0' 2>/dev/null)
          local chunk_num=$(echo "$data" | jq -r '.chunk // 0' 2>/dev/null)

          case "$event_type" in
            "start")
              echo -e "${CYAN}  ▸${NC} Analyzing $total_chunks chunk(s)..."
              ;;
            "chunk_start")
              echo -e "${CYAN}  ▸${NC} Processing chunk $chunk_num/$total_chunks..."
              ;;
            "chunk_complete")
              # Clear spinner and show completion
              printf "\r%-60s\r" ""
              echo -e "${CYAN}  ▸${NC} Chunk $chunk_num complete"

              # Parse accumulated JSON if no diagnostics were received from diagnostic events
              if [[ "$(cat "$has_diagnostics_file")" == "0" ]] && [[ -s "$text_buffer_file" ]]; then
                local accumulated_json=$(cat "$text_buffer_file")
                if echo "$accumulated_json" | jq empty 2>/dev/null; then
                  # Extract diagnostics from accumulated JSON
                  local diag_array=$(echo "$accumulated_json" | jq -c '.diagnostics // []' 2>/dev/null)
                  if [[ "$diag_array" != "[]" && "$diag_array" != "null" ]]; then
                    echo ""
                    echo "$accumulated_json" | jq -c '.diagnostics[]' 2>/dev/null | while read -r diag; do
                      if [[ -n "$diag" ]]; then
                        local sev=$(echo "$diag" | jq -r '.severity // "INFO"')
                        local msg=$(echo "$diag" | jq -r '.message // ""')
                        local f=$(echo "$diag" | jq -r '.location.path // "unknown"')
                        local ln=$(echo "$diag" | jq -r '.location.range.start.line // 0')
                        print_issue "$sev" "$f" "$ln" "$msg"
                        echo ""
                        echo "$diag" >> "$diagnostics_file"
                      fi
                    done
                    echo "1" > "$has_diagnostics_file"
                  fi
                  # Extract overview if available
                  local text_overview=$(echo "$accumulated_json" | jq -r '.overview // ""' 2>/dev/null)
                  if [[ -n "$text_overview" && "$text_overview" != "null" ]]; then
                    echo ""
                    echo -e "${BOLD}Overview:${NC}"
                    echo "$text_overview"
                  fi
                fi
              fi
              # Clear buffer for next chunk
              : > "$text_buffer_file"
              ;;
          esac
          ;;

        "text")
          # Accumulate JSON text fragments
          local text=$(echo "$data" | jq -r '.text // ""' 2>/dev/null)
          if [[ -n "$text" ]]; then
            # Append to text buffer
            printf "%s" "$text" >> "$text_buffer_file"
            # Show spinner only if no diagnostics shown yet
            if [[ "$(cat "$has_diagnostics_file")" == "0" ]]; then
              local spinners=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
              local spinner_idx=$((RANDOM % 10))
              printf "\r${CYAN}  ${spinners[$spinner_idx]}${NC} AI analyzing..."
            fi
          fi
          ;;

        "diagnostic")
          # Clear the spinner line completely
          printf "\r%-60s\r" ""

          # Mark that we have diagnostics
          echo "1" > "$has_diagnostics_file"

          # Show issue as it's found and collect it
          local severity=$(echo "$data" | jq -r '.severity // "INFO"' 2>/dev/null)
          local message=$(echo "$data" | jq -r '.message // ""' 2>/dev/null)
          local file=$(echo "$data" | jq -r '.location.path // "unknown"' 2>/dev/null)
          local line_num=$(echo "$data" | jq -r '.location.range.start.line // 0' 2>/dev/null)

          if [[ -n "$message" ]]; then
            print_issue "$severity" "$file" "$line_num" "$message"
            echo ""

            # Append diagnostic to JSONL file
            echo "$data" >> "$diagnostics_file"
          fi
          ;;

        "complete")
          # Clear spinner line
          printf "\r%-60s\r" ""

          # Final summary - show overview and save result
          local overview=$(echo "$data" | jq -r '.overview // ""' 2>/dev/null)
          local total=$(echo "$data" | jq -r '.total_diagnostics // 0' 2>/dev/null)
          local max_severity=$(echo "$data" | jq -r '.severity // "INFO"' 2>/dev/null)

          if [[ -n "$overview" && "$overview" != "null" ]]; then
            echo ""
            echo -e "${BOLD}Overview:${NC}"
            echo "$overview"
          fi

          # Convert JSONL to array and build final result
          local diags="[]"
          if [[ -s "$diagnostics_file" ]]; then
            diags=$(jq -s '.' "$diagnostics_file" 2>/dev/null || echo "[]")
          fi

          jq -n \
            --argjson diagnostics "$diags" \
            --arg overview "$overview" \
            --arg severity "$max_severity" \
            '{
              "source": {"name": "ai-review", "url": ""},
              "diagnostics": $diagnostics,
              "overview": $overview,
              "max_severity": $severity
            }' > "$result_file"
          ;;

        "error")
          # Clear spinner line and show error
          printf "\r%-60s\r" ""
          local error_msg=$(echo "$data" | jq -r '.message // .error // "Unknown error"' 2>/dev/null)
          log_error "AI review error: $error_msg"
          # Mark API error occurred
          echo "1" > "$api_error_file"
          ;;

        *)
          # Unknown event - check if it contains diagnostics array (non-streaming fallback)
          if echo "$data" | jq -e '.diagnostics' &>/dev/null; then
            echo "$data" > "$result_file"
          fi
          ;;
      esac
    fi
  done < "$sse_stream_file"

  # Cleanup SSE stream temp file
  rm -f "$sse_stream_file"

  # Check if API error occurred
  local had_api_error=$(cat "$api_error_file" 2>/dev/null || echo "0")

  # Cleanup temp files
  rm -f "$diff_file"
  rm -f "$diagnostics_file"
  rm -f "$text_buffer_file"
  rm -f "$has_diagnostics_file"
  rm -f "$api_error_file"

  # If API error occurred, block the commit
  if [[ "$had_api_error" == "1" ]]; then
    rm -f "$result_file"
    echo ""
    log_error "Commit blocked - AI review service error"
    echo "        Use 'git commit --no-verify' to bypass"
    exit 1
  fi

  # Check if we got a result
  if [[ -s "$result_file" ]]; then
    REVIEW_JSON=$(cat "$result_file")
    rm -f "$result_file"
  else
    # Fallback: try non-streaming request if SSE failed
    rm -f "$result_file"
    log_warn "Streaming failed, retrying with standard request..."
    call_ai_gateway_sync
    return
  fi
}

# Fallback non-streaming API call
call_ai_gateway_sync() {
  # Collect git info
  local commit_hash=$(git rev-parse HEAD 2>/dev/null || echo "staged")
  local branch_name=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
  local author_name=$(git config user.name 2>/dev/null || echo "unknown")
  local author_email=$(git config user.email 2>/dev/null || echo "unknown")
  local repo_url=$(git remote get-url origin 2>/dev/null || echo "local")

  # Save diff to temp file
  local diff_file
  if type safe_mktemp &>/dev/null; then
    diff_file=$(safe_mktemp "ai-sync-diff")
  else
    diff_file=$(mktemp 2>/dev/null || mktemp -t ai-sync-diff 2>/dev/null)
  fi
  echo "$DIFF" > "$diff_file"

  # Create JSON payload (no streaming)
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
    echo ""
    log_error "Commit blocked - AI review service unavailable"
    echo "        Use 'git commit --no-verify' to bypass"
    exit 1
  fi

  REVIEW_JSON="$api_body"
}

# ============================================
# Parse and Display Results
# ============================================

display_results() {
  if [[ -z "$REVIEW_JSON" ]]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_success "AI Review: No issues found!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    if [[ "$ENABLE_SONARQUBE_LOCAL" == "true" ]]; then
      log_success "All checks passed! Commit proceeding..."
    fi
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
    local _ai_elapsed=$((SECONDS - ${_AI_STEP_START:-$SECONDS}))
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_success "AI Review: No issues found!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "AI Review completed in ${_ai_elapsed}s"
    echo ""
    if [[ "$ENABLE_SONARQUBE_LOCAL" == "true" ]]; then
      log_success "All checks passed! Commit proceeding..."
    fi
    exit 0
  fi

  # Count by severity
  local error_count=$(echo "$diagnostics" | jq '[.[] | select(.severity == "ERROR")] | length')
  local warning_count=$(echo "$diagnostics" | jq '[.[] | select(.severity == "WARNING")] | length')
  local info_count=$(echo "$diagnostics" | jq '[.[] | select(.severity == "INFO")] | length')

  # Check if this is from streaming (issues already displayed) or sync call
  local is_streamed=$(echo "$REVIEW_JSON" | jq -r '.max_severity // ""' 2>/dev/null)

  if [[ -z "$is_streamed" ]]; then
    # Non-streaming response - display issues now
    echo ""
    print_separator

    echo "$diagnostics" | jq -c '.[]' | tr -d '\r' | while read -r issue; do
      local severity=$(echo "$issue" | jq -r '.severity // "INFO"')
      local message=$(echo "$issue" | jq -r '.message // "No message"')
      local file=$(echo "$issue" | jq -r '.location.path // "unknown"')
      local line=$(echo "$issue" | jq -r '.location.range.start.line // 0')

      print_issue "$severity" "$file" "$line" "$message"
      echo ""
    done
  fi

  local _ai_elapsed=$((SECONDS - ${_AI_STEP_START:-$SECONDS}))
  print_separator
  echo ""
  echo -e "${BOLD}AI Review Summary:${NC} $error_count errors, $warning_count warnings, $info_count info"
  log_info "AI Review completed in ${_ai_elapsed}s"
  echo ""

  # Determine exit code based on errors
  if [[ "$error_count" -gt 0 ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_error "COMMIT BLOCKED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    log_error "AI Review found errors that must be fixed."
    echo ""
    echo "Next steps:"
    echo "  1. Fix the AI Review errors shown above"
    echo "  2. Run 'git commit' again"
    if [[ "$ENABLE_SONARQUBE_LOCAL" == "true" ]]; then
      echo "  3. SonarQube will run again, then AI Review"
    fi
    echo ""
    echo "Or bypass: git commit --no-verify"
    echo ""
    exit 1
  elif [[ "$warning_count" -gt 0 ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_success "Commit allowed (with warnings)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    log_warn "Consider fixing the warnings above"
    if [[ "$ENABLE_SONARQUBE_LOCAL" == "true" ]]; then
      log_success "All checks passed! Commit proceeding..."
    fi
    exit 0
  else
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_success "All checks passed!"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    if [[ "$ENABLE_SONARQUBE_LOCAL" == "true" ]]; then
      log_success "SonarQube OK AI Review OK Commit proceeding..."
    else
      log_success "AI Review OK Commit proceeding..."
    fi
    exit 0
  fi
}

# ============================================
# Run SonarQube Analysis (if enabled)
# ============================================

run_sonarqube_analysis() {
  if [[ "$ENABLE_SONARQUBE_LOCAL" != "true" ]]; then
    return 0
  fi

  # Check if SonarQube credentials are configured
  if [[ -z "$SONAR_HOST_URL" || -z "$SONAR_TOKEN" ]]; then
    log_warn "SonarQube credentials not configured, skipping"
    return 0
  fi

  # Export variables for sonarqube-review.sh
  export SONAR_HOST_URL
  export SONAR_TOKEN
  export SONAR_PROJECT_KEY
  export SONAR_BLOCK_ON_HOTSPOTS
  export SONAR_FILTER_CHANGED_LINES_ONLY
  export SONAR_QUIET="true"

  # Find the sonarqube-review.sh script in hooks directory
  local sonar_script="$CONFIG_DIR/hooks/sonarqube-review.sh"

  if [[ ! -f "$sonar_script" ]]; then
    log_warn "SonarQube script not found, skipping"
    return 0
  fi

  print_separator
  echo -e "  ${BOLD}STEP ${sonar_step}/${total_steps}: SonarQube Static Analysis${NC}"
  print_separator
  echo ""

  # Run SonarQube directly (real-time output, no buffering)
  local sonar_exit_code=0
  local _sonar_start=$SECONDS
  bash "$sonar_script" 2>&1 || sonar_exit_code=$?
  local _sonar_elapsed=$((SECONDS - _sonar_start))

  if [[ $sonar_exit_code -eq 0 ]]; then
    log_info "SonarQube completed in ${_sonar_elapsed}s"
    return 0
  elif [[ $sonar_exit_code -eq 1 ]]; then
    echo ""
    print_separator
    log_error "COMMIT BLOCKED - SonarQube found errors (${_sonar_elapsed}s)"
    print_separator
    echo ""
    echo "  Fix the errors listed above, then commit again."
    echo "  Bypass: git commit --no-verify"
    echo ""
    exit 1
  else
    log_warn "SonarQube failed to run after ${_sonar_elapsed}s, continuing with AI review"
  fi
}

# ============================================
# Cleanup Temporary Files
# ============================================

cleanup_temp_files() {
  # Move output files to temp directory (not in project)
  local temp_dir="$CONFIG_DIR/temp"
  mkdir -p "$temp_dir" 2>/dev/null || true
  
  # List of temporary files to clean up
  local temp_files=(
    "ai-output.jsonl"
    "sonarqube-output.jsonl"
    "combined-output.jsonl"
    "ai-overview.txt"
    "sonarqube-overview.txt"
    "combined-overview.txt"
  )
  
  for file in "${temp_files[@]}"; do
    if [[ -f "$file" ]]; then
      mv "$file" "$temp_dir/" 2>/dev/null || rm -f "$file" 2>/dev/null || true
    fi
  done
  
  # Clean up SonarQube output capture file
  rm -f "$CONFIG_DIR/temp/sonar-output.txt" 2>/dev/null || true

  # Comprehensive cleanup of ALL SonarQube-generated files
  rm -rf .scannerwork 2>/dev/null || true
  rm -rf .sonar 2>/dev/null || true
  rm -f .sonar_lock 2>/dev/null || true
  rm -f report-task.txt 2>/dev/null || true
  rm -f sonar-report.json 2>/dev/null || true
  
  # Clean up any .sonar* files in current directory
  # Use rm with glob instead of find for better Windows Git Bash compatibility
  for f in .sonar*; do
    [[ -f "$f" ]] && rm -f "$f" 2>/dev/null || true
  done
  
  # Clean up auto-generated sonar-project.properties (has "auto-generated" comment)
  if [[ -f "sonar-project.properties" ]]; then
    if grep -q "auto-generated" "sonar-project.properties" 2>/dev/null; then
      rm -f sonar-project.properties 2>/dev/null || true
    fi
  fi
  
  # Clean up .sonarignore if it was auto-created (empty or only comments)
  if [[ -f ".sonarignore" ]]; then
    if ! grep -v "^#" ".sonarignore" | grep -q "[^[:space:]]" 2>/dev/null; then
      rm -f .sonarignore 2>/dev/null || true
    fi
  fi
}

# ============================================
# Main
# ============================================

main() {
  # Ensure cleanup runs even if script exits early
  trap cleanup_temp_files EXIT
  
  load_config

  echo ""
  echo -e "${BOLD}AI Review${NC} v${VERSION} - Pre-commit code review"
  echo ""

  # Determine which steps are enabled
  local sonar_enabled="$ENABLE_SONARQUBE_LOCAL"
  local ai_enabled="$ENABLE_AI_REVIEW"

  # If both are disabled, nothing to do
  if [[ "$sonar_enabled" != "true" && "$ai_enabled" != "true" ]]; then
    log_info "Both AI Review and SonarQube are disabled. Nothing to check."
    log_info "Enable: ai-review config set ENABLE_AI_REVIEW true"
    exit 0
  fi

  # Determine step numbering
  local total_steps=0
  local sonar_step=0
  local ai_step=0
  if [[ "$sonar_enabled" == "true" ]]; then
    total_steps=$((total_steps + 1))
    sonar_step=$total_steps
  fi
  if [[ "$ai_enabled" == "true" ]]; then
    total_steps=$((total_steps + 1))
    ai_step=$total_steps
  fi

  # Step: Run SonarQube (if enabled)
  if [[ "$sonar_enabled" == "true" ]]; then
    run_sonarqube_analysis

    if [[ "$ai_enabled" == "true" ]]; then
      echo ""
      print_separator
      echo -e "  ${BOLD}STEP ${ai_step}/${total_steps}: AI-Powered Code Review${NC}"
      print_separator
      echo ""
    fi
  fi

  # Step: Run AI Review (if enabled)
  if [[ "$ai_enabled" == "true" ]]; then
    _AI_STEP_START=$SECONDS
    get_staged_diff
    filter_ignored_files
    format_diff
    detect_language
    call_ai_gateway
    display_results
  else
    log_info "AI Review disabled (enable: ai-review config set ENABLE_AI_REVIEW true)"
  fi
}

main
