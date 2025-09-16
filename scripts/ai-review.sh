#!/usr/bin/env bash
set -e

# Lấy diff của PR với fallback options
echo "🔍 Getting diff for review..."

# Try different diff strategies
if git rev-parse --verify origin/main >/dev/null 2>&1; then
  # PR scenario - diff against origin/main
  DIFF=$(git diff origin/main...HEAD)
elif git rev-parse --verify HEAD~1 >/dev/null 2>&1; then
  # Local testing - diff against previous commit
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

# Check if OpenAI API key is set
if [[ -z "$OPENAI_API_KEY" ]]; then
  echo "⚠️ OPENAI_API_KEY not set, skipping AI review"
  exit 0
fi

# Basic API key validation
if [[ ! "$OPENAI_API_KEY" =~ ^sk-[a-zA-Z0-9] ]]; then
  echo "⚠️ OPENAI_API_KEY doesn't appear to be valid (should start with 'sk-')"
  echo "🔍 API Key format: ${OPENAI_API_KEY:0:10}..."
  # Continue anyway in case the format changed
fi

echo "🤖 Sending to AI for review..."

# Properly escape the diff content for JSON
echo "🔧 Escaping diff content for JSON..."
DIFF_ESCAPED=$(echo "$DIFF" | jq -Rs . | sed 's/^"//; s/"$//')

# Request OpenAI to return reviewdog-compatible JSON format
SYSTEM_PROMPT="You are a code reviewer. Analyze the git diff and return your feedback as a JSON array where each item follows this exact format: {\"source\": {\"name\": \"ai-review\", \"url\": \"\"}, \"severity\": \"INFO\"|\"WARNING\"|\"ERROR\", \"message\": {\"text\": \"Your specific feedback here\"}, \"location\": {\"path\": \"filename.ext\", \"range\": {\"start\": {\"line\": NUMBER, \"column\": 1}, \"end\": {\"line\": NUMBER, \"column\": 1}}}}. Focus on bugs, security issues, and code quality. Use severity: ERROR for bugs/security, WARNING for code quality, INFO for suggestions. Extract actual filenames and line numbers from the diff. Return ONLY the JSON array, no other text."

# Escape the system prompt for JSON
SYSTEM_PROMPT_ESCAPED=$(echo "$SYSTEM_PROMPT" | jq -Rs . | sed 's/^"//; s/"$//')

# Make the API call with better error handling
echo "📡 Making API request to OpenAI..."

# Create the JSON payload using jq for proper escaping
JSON_PAYLOAD=$(jq -n \
  --arg model "gpt-4o-mini" \
  --arg system_content "$SYSTEM_PROMPT" \
  --arg user_content "$DIFF" \
  '{
    "model": $model,
    "messages": [
      {"role": "system", "content": $system_content},
      {"role": "user", "content": $user_content}
    ]
  }')

# Validate the JSON payload
if ! echo "$JSON_PAYLOAD" | jq empty 2>/dev/null; then
  echo "❌ Generated invalid JSON payload"
  exit 1
fi

echo "✅ JSON payload validated"

API_RESPONSE=$(curl -s -w "\nHTTP_STATUS:%{http_code}" https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d "$JSON_PAYLOAD")

# Split response and status
HTTP_STATUS=$(echo "$API_RESPONSE" | tail -n1 | sed 's/HTTP_STATUS://')
API_BODY=$(echo "$API_RESPONSE" | sed '$d')

echo "🔍 API Status: $HTTP_STATUS"

# Check HTTP status
if [[ "$HTTP_STATUS" != "200" ]]; then
  echo "❌ OpenAI API request failed with status $HTTP_STATUS"
  echo "📄 Response body:"
  echo "$API_BODY" | head -10
  exit 1
fi

# Extract the content from the response
REVIEW_JSON=$(echo "$API_BODY" | jq -r '.choices[0].message.content' 2>/dev/null)

# Check if API call was successful
if [[ -z "$REVIEW_JSON" || "$REVIEW_JSON" == "null" ]]; then
  echo "❌ Failed to extract AI review content"
  echo "📄 Raw API response (first 500 chars):"
  echo "$API_BODY" | head -c 500
  echo ""
  echo "🔧 Parsed content:"
  echo "$API_BODY" | jq '.choices[0].message.content' 2>/dev/null || echo "Failed to parse JSON"
  exit 1
fi

echo "✅ AI review completed"

# Validate and clean the JSON response
echo "🔍 Validating AI response format..."
if echo "$REVIEW_JSON" | jq empty 2>/dev/null; then
  echo "✅ Valid JSON format received"
  echo "$REVIEW_JSON" > ai-output.jsonl
else
  echo "⚠️ Invalid JSON format, creating fallback format..."
  # Fallback: create a single general comment
  REVIEW_ESCAPED=$(echo "$REVIEW_JSON" | sed 's/\\/\\\\/g' | sed 's/"/\\"/g' | tr '\n' '\r' | sed 's/\r/\\n/g')
  echo "{\"source\":{\"name\":\"ai-review\",\"url\":\"\"},\"severity\":\"INFO\",\"message\":{\"text\":\"🤖 AI Code Review:\\n\\n$REVIEW_ESCAPED\"},\"location\":{\"path\":\"README.md\",\"range\":{\"start\":{\"line\":1,\"column\":1},\"end\":{\"line\":1,\"column\":1}}}}" > ai-output.jsonl
fi

# Display the review for logging
echo "📝 AI Review Output:"
if [[ -f ai-output.jsonl ]]; then
  # Pretty print the JSON for logging
  echo "$REVIEW_JSON" | jq '.' 2>/dev/null || echo "$REVIEW_JSON"
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

      # Convert JSON array to line-delimited JSON for reviewdog
      if echo "$REVIEW_JSON" | jq -c '.[]' > ai-output-lines.jsonl 2>/dev/null; then
        echo "✅ Converted to JSONL format"
        INPUT_FILE="ai-output-lines.jsonl"
      else
        echo "⚠️ Using single-line format"
        INPUT_FILE="ai-output.jsonl"
      fi

      # Try github-pr-review reporter with structured input
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
      if echo "$REVIEW_JSON" | jq -c '.[]' > ai-output-lines.jsonl 2>/dev/null; then
        cat ai-output-lines.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local
      else
        cat ai-output.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local
      fi
    fi
  fi
else
  echo "ℹ️ No GITHUB_TOKEN available, using local output"
  echo "📄 Review JSON output saved to ai-output.jsonl"
  # Show the review locally
  if [[ -f ai-output.jsonl ]]; then
    echo "🔍 AI Review Summary:"
    if echo "$REVIEW_JSON" | jq -c '.[]' > ai-output-lines.jsonl 2>/dev/null; then
      cat ai-output-lines.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local 2>/dev/null || echo "Review saved to files"
    else
      cat ai-output.jsonl | $HOME/bin/reviewdog -f=rdjson -name="ai-review" -reporter=local 2>/dev/null || echo "Review saved to files"
    fi
  fi
fi

# Cleanup temporary files
rm -f ai-output-lines.jsonl 2>/dev/null
