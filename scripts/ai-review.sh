#!/usr/bin/env bash
set -e

# Get diff for review
echo "🔍 Getting diff for review..."

# Determine comparison strategy based on environment
if [[ -n "$GITHUB_BASE_REF" ]]; then
  # Pull request - compare with base branch
  BASE_BRANCH="$GITHUB_BASE_REF"
  echo "📌 PR detected: comparing with base branch $BASE_BRANCH"
  if git rev-parse --verify origin/$BASE_BRANCH >/dev/null 2>&1; then
    echo "📊 Comparing with origin/$BASE_BRANCH"
    DIFF=$(git diff origin/$BASE_BRANCH...HEAD)
  else
    echo "⚠️ Base branch origin/$BASE_BRANCH not found"
    DIFF=$(git diff origin/main...HEAD 2>/dev/null || git diff origin/master...HEAD 2>/dev/null || echo "No changes detected")
  fi
else
  # Not a PR - get the current branch and compare with remote
  if [[ -n "$GITHUB_REF_NAME" ]]; then
    # Direct push in GitHub Actions
    CURRENT_BRANCH="$GITHUB_REF_NAME"
  else
    # Local environment
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  fi
  echo "📌 Current branch: $CURRENT_BRANCH"

  # Try to get the remote tracking branch
  REMOTE_BRANCH=$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "")

  if [[ -n "$REMOTE_BRANCH" ]]; then
    # Compare with remote tracking branch (changes since last push)
    echo "📊 Comparing with remote tracking: $REMOTE_BRANCH"
    DIFF=$(git diff $REMOTE_BRANCH...HEAD)
  else
    # Try origin/<current-branch> first
    if git rev-parse --verify origin/$CURRENT_BRANCH >/dev/null 2>&1; then
      echo "📊 Comparing with origin/$CURRENT_BRANCH"
      DIFF=$(git diff origin/$CURRENT_BRANCH...HEAD)
    # Fallback: try origin/main or origin/master
    elif git rev-parse --verify origin/main >/dev/null 2>&1; then
      echo "📊 Comparing with origin/main"
      DIFF=$(git diff origin/main...HEAD)
    elif git rev-parse --verify origin/master >/dev/null 2>&1; then
      echo "📊 Comparing with origin/master"
      DIFF=$(git diff origin/master...HEAD)
    elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
      # Local testing - diff against previous commit
      echo "📊 Comparing with previous commit"
      DIFF=$(git diff HEAD~1)
    else
      # Check for staged changes first
      if git diff --cached --name-only | head -1 >/dev/null 2>&1; then
        echo "📝 Found staged changes, reviewing..."
        DIFF=$(git diff --cached)
      elif git ls-files --others --exclude-standard | head -1 >/dev/null; then
        # New repository - get all untracked files as diff
        echo "📝 New repository detected, reviewing all untracked files..."
        DIFF=""
        for file in $(git ls-files --others --exclude-standard); do
          if [[ -f "$file" ]]; then
            echo "Adding $file to review..."
            DIFF="${DIFF}\n--- /dev/null\n+++ $file\n$(cat "$file" | sed 's/^/+/')"
          fi
        done
      else
        # Fallback - get working directory changes
        DIFF=$(git diff HEAD 2>/dev/null || echo "No changes detected")
      fi
    fi
  fi
fi

DIFF_LINES=$(echo "$DIFF" | wc -l)
DIFF_CHARS=$(echo "$DIFF" | wc -c)
echo "📊 Diff size: $DIFF_LINES lines, $DIFF_CHARS characters"

# Check if we have diff content to review
if [[ -z "$DIFF" || "$DIFF" == "No changes detected" ]]; then
  echo "ℹ️ No changes to review, skipping AI analysis"
  exit 0
fi

# Filter out ignored files based on .aireviewignore
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IGNORE_FILE="$(dirname "$SCRIPT_DIR")/.aireviewignore"

if [[ -f "$IGNORE_FILE" ]]; then
  echo "🔍 Applying ignore patterns from .aireviewignore..."
  FILTERED_DIFF=$(bash "$SCRIPT_DIR/filter-ignored-files.sh" "$DIFF" "$IGNORE_FILE")
  
  # Check if all files were filtered out
  if [[ -z "$FILTERED_DIFF" || "$FILTERED_DIFF" == "No changes detected" ]]; then
    echo "ℹ️ All changes were filtered out by .aireviewignore, skipping AI analysis"
    exit 0
  fi
  
  DIFF="$FILTERED_DIFF"
  FILTERED_LINES=$(echo "$DIFF" | wc -l)
  FILTERED_CHARS=$(echo "$DIFF" | wc -c)
  echo "📊 Filtered diff size: $FILTERED_LINES lines, $FILTERED_CHARS characters"
else
  echo "ℹ️ No .aireviewignore file found, reviewing all changes"
fi

# Check if gateway URL is set
if [[ -z "$AI_GATEWAY_URL" ]]; then
  echo "⚠️ AI_GATEWAY_URL not set, skipping AI review"
  exit 0
fi

echo "ℹ️ Using AI Gateway at $AI_GATEWAY_URL"

echo "🤖 Sending to AI for review..."

# Generate diff with line numbers for AI analysis
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/showlinenum.awk" ]]; then
  # Use temp file to avoid pipe buffer issues with large diffs
  TEMP_DIFF_INPUT=$(mktemp)
  echo "$DIFF" > "$TEMP_DIFF_INPUT"
  NUMBERED_DIFF=$(awk -f "$SCRIPT_DIR/showlinenum.awk" "$TEMP_DIFF_INPUT")
  rm -f "$TEMP_DIFF_INPUT"
  DIFF_FOR_AI="$NUMBERED_DIFF"
else
  DIFF_FOR_AI="$DIFF"
fi

# Detect language from git diff
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
fi

echo "📝 Detected language: $LANGUAGE"

# Use gateway API
echo "📡 Making API request via gateway..."

# Collect git and repository information
COMMIT_HASH=$(git rev-parse HEAD)
REPO_URL="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY}"

# Extract git user information
AUTHOR_NAME=$(git log -1 --format='%an')
AUTHOR_EMAIL=$(git log -1 --format='%ae')
COMMITTER_NAME=$(git log -1 --format='%cn')
COMMITTER_EMAIL=$(git log -1 --format='%ce')

# Extract PR number and branch name properly from GitHub environment
if [[ -n "$GITHUB_REF" && "$GITHUB_REF" =~ refs/pull/([0-9]+) ]]; then
  PR_NUMBER="${BASH_REMATCH[1]}"
  # For PRs, use GITHUB_HEAD_REF which contains the actual branch name
  BRANCH_NAME="${GITHUB_HEAD_REF:-${CURRENT_BRANCH:-$GITHUB_REF_NAME}}"
elif [[ -n "$GITHUB_EVENT_PATH" && -f "$GITHUB_EVENT_PATH" ]]; then
  PR_NUMBER=$(jq -r '.number // empty' "$GITHUB_EVENT_PATH" 2>/dev/null || echo "")
  BRANCH_NAME="${GITHUB_HEAD_REF:-${CURRENT_BRANCH:-$GITHUB_REF_NAME}}"
else
  PR_NUMBER=""
  BRANCH_NAME="${CURRENT_BRANCH:-$GITHUB_REF_NAME}"
fi

echo "📋 Repository info:"
echo "   - Repo: $GITHUB_REPOSITORY"
echo "   - Commit: ${COMMIT_HASH:0:8}"
echo "   - Branch: $BRANCH_NAME"
echo "   - PR: ${PR_NUMBER}"
echo "   - Author: $AUTHOR_NAME <$AUTHOR_EMAIL>"

# Save diff to temporary file for upload
DIFF_FILE=$(mktemp)
echo "$DIFF_FOR_AI" > "$DIFF_FILE"

# Create JSON payload without git_diff (will be sent as file)
JSON_PAYLOAD=$(jq -n \
  --arg language "$LANGUAGE" \
  --arg ai_model "${AI_MODEL:-gemini-2.0-flash}" \
  --arg ai_provider "${AI_PROVIDER:-google}" \
  --arg commit_hash "$COMMIT_HASH" \
  --arg branch_name "$BRANCH_NAME" \
  --arg pr_number "$PR_NUMBER" \
  --arg repo_url "$REPO_URL" \
  --arg author_name "$AUTHOR_NAME" \
  --arg author_email "$AUTHOR_EMAIL" \
  --arg committer_name "$COMMITTER_NAME" \
  --arg committer_email "$COMMITTER_EMAIL" \
  '{
    "ai_model": $ai_model,
    "ai_provider": $ai_provider,
    "language": $language,
    "review_mode": "file",
    "git_info": {
      "commit_hash": $commit_hash,
      "branch_name": $branch_name,
      "pr_number": $pr_number,
      "repo_url": $repo_url,
      "author": {
        "name": $author_name,
        "email": $author_email
      },
      "committer": {
        "name": $committer_name,
        "email": $committer_email
      }
    }
  }')

# Validate the JSON payload
if ! echo "$JSON_PAYLOAD" | jq empty 2>/dev/null; then
  echo "❌ Generated invalid JSON payload"
  rm -f "$DIFF_FILE"
  exit 1
fi

# Print payload info (without the large diff content)
echo "📄 Request metadata:"
echo "$JSON_PAYLOAD" | jq '.' 2>/dev/null || echo "Failed to parse JSON payload"
echo "📎 Diff file size: $(wc -c < "$DIFF_FILE") bytes"

# Send request with multipart/form-data (file upload)
API_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$AI_GATEWAY_URL/review" \
  -H "X-API-Key: $AI_GATEWAY_API_KEY" \
  -X POST \
  -F "metadata=$JSON_PAYLOAD" \
  -F "git_diff=@$DIFF_FILE")

# Split response and status
HTTP_STATUS=$(echo "$API_RESPONSE" | tail -n1 | sed 's/HTTP_STATUS://')
API_BODY=$(echo "$API_RESPONSE" | sed '$d')

# Cleanup diff file
rm -f "$DIFF_FILE"

# Check HTTP status
if [[ "$HTTP_STATUS" != "200" ]]; then
  echo "❌ API request failed with status $HTTP_STATUS"
  echo "📄 Response body:"
  echo "$API_BODY" | head -10
  exit 1
fi

# Extract the content from the response (Gateway returns diagnostic format directly)
REVIEW_JSON=$(echo "$API_BODY" | jq -c '.' 2>/dev/null)

# Check if API call was successful
if [[ -z "$REVIEW_JSON" || "$REVIEW_JSON" == "null" ]]; then
  echo "❌ Failed to extract AI review content"
  echo "📄 Raw API response (first 500 chars):"
  echo "$API_BODY" | head -c 500
  echo ""
  echo "🔧 Parsed content:"
  echo "$API_BODY" | jq '.candidates[0].content.parts[0].text' 2>/dev/null || echo "Failed to parse JSON"
  exit 1
fi

echo "✅ AI review completed"

# Validate and clean the JSON response
echo "🔍 Validating AI response format..."

# Check if the response is already in proper reviewdog diagnostic format
if echo "$REVIEW_JSON" | jq -e '.source and .diagnostics' >/dev/null 2>&1; then
  echo "✅ Proper reviewdog diagnostic format detected"
  echo "$REVIEW_JSON" > ai-output.jsonl

  # Extract overview for separate comment if it exists
  OVERVIEW=$(echo "$REVIEW_JSON" | jq -r '.overview // empty' 2>/dev/null)
  if [[ -n "$OVERVIEW" && "$OVERVIEW" != "null" ]]; then
    echo "$OVERVIEW" > ai-overview.txt
    echo "📝 Overview extracted for separate comment"
  fi
elif echo "$REVIEW_JSON" | jq -e 'type == "array" and length > 0' >/dev/null 2>&1; then
  echo "✅ Converting array format to reviewdog diagnostic format"
  # Convert array to proper reviewdog format
  CONVERTED_JSON=$(echo "$REVIEW_JSON" | jq '{
    "source": {"name": "ai-review", "url": ""},
    "diagnostics": [.[] | {
      "message": (if .message then .message else "Code review issue" end),
      "location": .location,
      "severity": (.severity // "INFO"),
      "code": {"value": "ai-review", "url": ""}
    }]
  }')
  echo "$CONVERTED_JSON" > ai-output.jsonl
  echo "🔧 Converted to proper format"
else
  echo "⚠️ Unexpected format, creating fallback..."
  echo "📄 Raw response format:"
  echo "$REVIEW_JSON" | jq type 2>/dev/null || echo "Not valid JSON"

  # Create fallback format
  cat > ai-output.jsonl << EOF
{
  "source": {"name": "ai-review", "url": ""},
  "diagnostics": [{
    "message": "AI Review completed but response format was unexpected",
    "location": {
      "path": "README.md",
      "range": {"start": {"line": 1, "column": 1}, "end": {"line": 1, "column": 1}}
    },
    "severity": "INFO",
    "code": {"value": "ai-review", "url": ""}
  }]
}
EOF
fi

# Validate final output format
if [[ -f ai-output.jsonl ]] && echo "$(cat ai-output.jsonl)" | jq empty 2>/dev/null; then
  echo "✅ Final reviewdog format validated"
else
  echo "❌ Failed to create valid reviewdog format"
  exit 1
fi


# Check if GitHub token is available for reviewdog
if [[ -n "$GITHUB_TOKEN" ]]; then
  echo "🚀 Posting review via reviewdog..."
  # Set the reviewdog environment variable and post to GitHub
  export REVIEWDOG_GITHUB_API_TOKEN="$GITHUB_TOKEN"

  # Check if we're in a GitHub Actions environment
  if [[ -n "$GITHUB_REPOSITORY" && -n "$GITHUB_EVENT_PATH" ]]; then
    # GitHub Actions environment - use github-pr-review reporter
    echo "📋 Repository: $GITHUB_REPOSITORY"
    echo "📝 Event: $(basename "$GITHUB_EVENT_PATH")"

    # Post overview comment first if available and we have a PR number
    if [[ -f ai-overview.txt && -n "$PR_NUMBER" ]]; then
      echo "💬 Managing overview comment for PR #$PR_NUMBER..."

      # First, find and delete any existing AI overview comments
      echo "🔍 Looking for existing AI overview comments..."
      EXISTING_COMMENTS=$(curl -s \
        -H "Authorization: token $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$GITHUB_REPOSITORY/issues/$PR_NUMBER/comments")

      # Find comment IDs that contain our AI overview marker
      if echo "$EXISTING_COMMENTS" | jq -e '. | length > 0' >/dev/null 2>&1; then
        OVERVIEW_COMMENT_IDS=$(echo "$EXISTING_COMMENTS" | jq -r '.[] | select(.body | contains("🤖 AI Code Review Overview")) | .id' 2>/dev/null || echo "")

        if [[ -n "$OVERVIEW_COMMENT_IDS" ]]; then
          echo "🗑️ Deleting existing AI overview comments..."
          while IFS= read -r comment_id; do
            if [[ -n "$comment_id" && "$comment_id" != "null" ]]; then
              curl -s -X DELETE \
                -H "Authorization: token $GITHUB_TOKEN" \
                -H "Accept: application/vnd.github.v3+json" \
                "https://api.github.com/repos/$GITHUB_REPOSITORY/issues/comments/$comment_id" \
                > /dev/null 2>&1
              echo "🗑️ Deleted comment ID: $comment_id"
            fi
          done <<< "$OVERVIEW_COMMENT_IDS"
        else
          echo "ℹ️ No existing AI overview comments found"
        fi
      fi

      OVERVIEW_CONTENT=$(cat ai-overview.txt)

      # Create overview comment with formatting
      OVERVIEW_COMMENT="## 🤖 AI Code Review Overview

$OVERVIEW_CONTENT

---
*This overview was generated by AI code review. Individual issues are commented inline below.*"

      # Post new overview comment via GitHub API
      echo "💬 Posting new overview comment..."
      curl -s -X POST \
        -H "Authorization: token $GITHUB_TOKEN" \
        -H "Content-Type: application/json" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$GITHUB_REPOSITORY/issues/$PR_NUMBER/comments" \
        -d "$(jq -n --arg body "$OVERVIEW_COMMENT" '{body: $body}')" \
        > /dev/null 2>&1 && echo "✅ Overview comment posted" || echo "⚠️ Failed to post overview comment"
    fi

    # Use reviewdog with the structured JSON response
    if [[ -f ai-output.jsonl ]]; then
      echo "🚀 Running reviewdog..."

      # Use the diagnostic format file
      INPUT_FILE="ai-output.jsonl"

      # Try github-pr-review reporter with diagnostic input
      if ! cat "$INPUT_FILE" | $HOME/bin/reviewdog \
        -f=rdjson \
        -name="ai-review" \
        -reporter=github-pr-review \
        -filter-mode=nofilter \
        -fail-on-error=false \
        -level=info; then

        echo "⚠️ github-pr-review failed, trying github-pr-check reporter..."
        cat "$INPUT_FILE" | $HOME/bin/reviewdog \
          -f=rdjson \
          -name="ai-review" \
          -reporter=github-pr-check \
          -filter-mode=nofilter \
          -fail-on-error=false
      fi
    else
      echo "❌ No review output file found"
    fi
  else
    # Local testing - use local reporter to avoid API issues
    echo "⚠️ Local testing detected, using local reporter"
    if [[ -f ai-output.jsonl ]]; then
      cat ai-output.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local
    fi
  fi
else
  echo "ℹ️ No GITHUB_TOKEN available, using local output"
  echo "📄 Review JSON output saved to ai-output.jsonl"
  # Show the review locally
  if [[ -f ai-output.jsonl ]]; then
    cat ai-output.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local 2>/dev/null || echo "Review saved to ai-output.jsonl"
  fi
fi

# Cleanup temporary files
rm -f ai-diff.txt ai-diff-with-lines.txt ai-output-lines.jsonl ai-overview.txt "$TEMP_DIFF_INPUT" "$DIFF_FILE" 2>/dev/null
