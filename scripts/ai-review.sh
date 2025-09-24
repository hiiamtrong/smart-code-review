#!/usr/bin/env bash
set -e

# Get diff since last push
echo "🔍 Getting diff since last push..."

# Get the current branch (handle GitHub Actions detached HEAD)
if [[ -n "$GITHUB_HEAD_REF" ]]; then
  # Pull request - use the PR head branch
  CURRENT_BRANCH="$GITHUB_HEAD_REF"
elif [[ -n "$GITHUB_REF_NAME" ]]; then
  # Direct push - use the ref name
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
  # Try origin/<current-branch> first (for GitHub Actions PR scenario)
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

DIFF_LINES=$(echo "$DIFF" | wc -l)
DIFF_CHARS=$(echo "$DIFF" | wc -c)
echo "📊 Diff size: $DIFF_LINES lines, $DIFF_CHARS characters"

# Check if we have diff content to review
if [[ -z "$DIFF" || "$DIFF" == "No changes detected" ]]; then
  echo "ℹ️ No changes to review, skipping AI analysis"
  exit 0
fi

# Check if diff is too large for API (OpenAI has token limits)
if [[ $DIFF_CHARS -gt 50000 ]]; then
  echo "⚠️ Diff is very large ($DIFF_CHARS chars), truncating for API..."
  DIFF=$(echo "$DIFF" | head -c 40000)
  echo "📊 Truncated to: $(echo "$DIFF" | wc -c) characters"
fi

# Check if gateway URL is set
if [[ -z "$AI_GATEWAY_URL" ]]; then
  echo "⚠️ AI_GATEWAY_URL not set, skipping AI review"
  exit 0
fi

echo "ℹ️ Using AI Gateway at $AI_GATEWAY_URL"

echo "🤖 Sending to AI for review..."

# Write diff to a temporary file for reference
echo "$DIFF" > ai-diff.txt

# Generate diff with line numbers for AI analysis
echo "🔢 Generating diff with line numbers..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/showlinenum.awk" ]]; then
  NUMBERED_DIFF=$(echo "$DIFF" | awk -f "$SCRIPT_DIR/showlinenum.awk")
  echo "$NUMBERED_DIFF" > ai-diff-with-lines.txt
  echo "✅ Diff with line numbers generated"
  echo "🔍 First 20 lines of numbered diff:"
  head -20 ai-diff-with-lines.txt

  # Use numbered diff for AI analysis
  DIFF_FOR_AI="$NUMBERED_DIFF"
  echo "✅ Using numbered diff for AI analysis"
else
  echo "⚠️ showlinenum.awk not found at $SCRIPT_DIR/showlinenum.awk, using raw diff"
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

JSON_PAYLOAD=$(jq -n \
  --arg git_diff "$DIFF_FOR_AI" \
  --arg language "$LANGUAGE" \
  --arg ai_model "${AI_MODEL:-gemini-2.0-flash}" \
  --arg ai_provider "${AI_PROVIDER:-gemini}" \
  '{
    "ai_model": $ai_model,
    "ai_provider": $ai_provider,
    "git_diff": $git_diff,
    "language": $language,
    "review_mode": "string"
  }')

# Validate the JSON payload
if ! echo "$JSON_PAYLOAD" | jq empty 2>/dev/null; then
  echo "❌ Generated invalid JSON payload"
  exit 1
fi

API_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "$AI_GATEWAY_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $AI_GATEWAY_API_KEY" \
  -X POST \
  -d "$JSON_PAYLOAD")

# Split response and status
HTTP_STATUS=$(echo "$API_RESPONSE" | tail -n1 | sed 's/HTTP_STATUS://')
API_BODY=$(echo "$API_RESPONSE" | sed '$d')

echo "🔍 API Status: $HTTP_STATUS"

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

# Try to extract JSON from markdown code blocks if present
if [[ "$REVIEW_JSON" == *'```json'* ]]; then
  echo "🔧 Extracting JSON from markdown code block..."
  REVIEW_JSON=$(echo "$REVIEW_JSON" | sed -n '/```json/,/```/p' | sed '1d;$d')
fi

# Remove any non-JSON prefix/suffix
if [[ "$REVIEW_JSON" == *'['* ]]; then
  # Extract from first [ to last ]
  REVIEW_JSON=$(echo "$REVIEW_JSON" | sed -n '/\[/,/\]/p' | tr -d '\n' | sed 's/.*\(\[.*\]\).*/\1/')
fi

if echo "$REVIEW_JSON" | jq empty 2>/dev/null; then
  echo "✅ Valid JSON format received"

  # Validate the diagnostic format structure
  if echo "$REVIEW_JSON" | jq -e '.source and .diagnostics' >/dev/null 2>&1; then
    echo "✅ Proper diagnostic format detected"
    # Save the diagnostic object directly (no conversion needed)
    echo "$REVIEW_JSON" > ai-output.jsonl
  else
    echo "⚠️ Converting array format to diagnostic format..."
    echo "🔍 Examining structure..."
    echo "$REVIEW_JSON" | jq '.[0] | keys' 2>/dev/null || echo "Not an array format"

    # Convert old array format to new diagnostic format if needed
    if echo "$REVIEW_JSON" | jq -e 'type == "array"' >/dev/null 2>&1; then
      echo "✅ Converting from array format"
      CONVERTED_JSON=$(echo "$REVIEW_JSON" | jq '{
        "source": {"name": "ai-review", "url": ""},
        "severity": "ERROR",
        "diagnostics": [.[] | {
          "message": (if .message | type == "object" then .message.text else .message end),
          "location": .location,
          "severity": .severity,
          "code": {"value": "ai-review", "url": ""}
        }]
      }')
      echo "$CONVERTED_JSON" > ai-output.jsonl
    else
      echo "🔄 Treating as single object, wrapping in diagnostic format"
      CONVERTED_JSON=$(echo "$REVIEW_JSON" | jq '{
        "source": {"name": "ai-review", "url": ""},
        "severity": .severity // "INFO",
        "diagnostics": [{
          "message": (if .message | type == "object" then .message.text else (.message // "AI Review")),
          "location": (.location // {"path": "README.md", "range": {"start": {"line": 1, "column": 1}, "end": {"line": 1, "column": 1}}}),
          "severity": (.severity // "INFO"),
          "code": {"value": "ai-review", "url": ""}
        }]
      }')
      echo "$CONVERTED_JSON" > ai-output.jsonl
    fi
  fi
else
  echo "⚠️ Invalid JSON format, creating fallback format..."
  echo "📄 Raw AI response (first 200 chars):"
  echo "$REVIEW_JSON" | head -c 200
  echo ""

  # Fallback: create a single diagnostic object
  REVIEW_ESCAPED=$(echo "$REVIEW_JSON" | sed 's/\\/\\\\/g' | sed 's/"/\\"/g' | tr '\n' '\r' | sed 's/\r/\\n/g')
  cat > ai-output.jsonl << EOF
{
  "source": {"name": "ai-review", "url": ""},
  "severity": "INFO",
  "diagnostics": [{
    "message": "🤖 AI Code Review: $REVIEW_ESCAPED",
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

# Display the review for logging
echo "📝 AI Review Output:"
if [[ -f ai-output.jsonl ]]; then
  # Pretty print the JSON for logging
  echo "$REVIEW_JSON" | jq '.' 2>/dev/null || echo "$REVIEW_JSON"

  # Line number verification if showlinenum output exists
  if [[ -f ai-diff-with-lines.txt ]]; then
    echo ""
    echo "🔍 Line Number Verification:"
    echo "Comparing AI reported line numbers with actual diff line numbers..."

    # Extract line numbers from AI output and show corresponding lines from numbered diff
    if echo "$REVIEW_JSON" | jq -e '.diagnostics' >/dev/null 2>&1; then
      echo "$REVIEW_JSON" | jq -r '.diagnostics[] | "Line \(.location.range.start.line): \(.message)"' | while read -r line; do
        echo "AI: $line"
        line_num=$(echo "$line" | sed 's/Line \([0-9]*\):.*/\1/')
        if [[ -n "$line_num" && "$line_num" -gt 0 ]]; then
          echo "Diff context around line $line_num:"
          grep -n "^$line_num:" ai-diff-with-lines.txt | head -3 || echo "  (line not found in numbered diff)"
        fi
        echo ""
      done
    fi
  fi
else
  echo "No review output generated"
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

    # Use reviewdog with the structured JSON response
    echo "🔍 Checking AI review content:"
    if [[ -f ai-output.jsonl ]]; then
      cat ai-output.jsonl
      echo ""
      echo "🚀 Running reviewdog with structured AI response..."

      # Use the diagnostic format file
      INPUT_FILE="ai-output.jsonl"
      echo "✅ Using reviewdog diagnostic format"

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
    echo "🔍 AI Review Summary:"
    cat ai-output.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local 2>/dev/null || echo "Review saved to ai-output.jsonl"
  fi
fi

# Cleanup temporary files
rm -f ai-output-lines.jsonl 2>/dev/null
