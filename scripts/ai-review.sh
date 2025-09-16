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

echo "📊 Diff size: $(echo "$DIFF" | wc -l) lines"

# Check if we have diff content to review
if [[ -z "$DIFF" || "$DIFF" == "No changes detected" ]]; then
  echo "ℹ️ No changes to review, skipping AI analysis"
  exit 0
fi

# Check if OpenAI API key is set
if [[ -z "$OPENAI_API_KEY" ]]; then
  echo "⚠️ OPENAI_API_KEY not set, skipping AI review"
  exit 0
fi

echo "🤖 Sending to AI for review..."

# Escape the diff content for JSON
DIFF_ESCAPED=$(echo "$DIFF" | sed 's/\\/\\\\/g' | sed 's/"/\\"/g' | tr '\n' '\r' | sed 's/\r/\\n/g')

# Gửi diff đến AI API (ví dụ OpenAI)
REVIEW=$(curl -s https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"gpt-4o-mini\",
    \"messages\": [
      {\"role\": \"system\", \"content\": \"You are a code reviewer. Analyze the diff and provide concise, actionable feedback focusing on bugs, security issues, and code quality improvements. Be specific about line numbers when possible.\"},
      {\"role\": \"user\", \"content\": \"$DIFF_ESCAPED\"}
    ]
  }" 2>/dev/null | jq -r '.choices[0].message.content' 2>/dev/null)

# Check if API call was successful
if [[ -z "$REVIEW" || "$REVIEW" == "null" ]]; then
  echo "❌ Failed to get AI review response"
  exit 1
fi

echo "✅ AI review completed"

# Convert output thành rdjson tối thiểu (ở đây mình giả lập 1 lỗi)
cat <<EOF > ai-output.json
{
  "source": "ai-review",
  "severity": "INFO",
  "message": "$REVIEW",
  "location": {
    "path": "GLOBAL",
    "range": {
      "start": { "line": 1 }
    }
  }
}
EOF

# Hiển thị kết quả review
echo "📝 AI Review Output:"
echo "$REVIEW"
# Gửi vào reviewdog
cat ai-output.json | reviewdog -f=rdjson -name="ai-review" -reporter=github-pr-review
