#!/usr/bin/env bash
set -e

# Lấy diff của PR
DIFF=$(git diff origin/main...HEAD)

# Gửi diff đến AI API (ví dụ OpenAI)
REVIEW=$(curl -s https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"gpt-4o-mini\",
    \"messages\": [
      {\"role\": \"system\", \"content\": \"Bạn là code reviewer, hãy nhận xét trực tiếp vào diff code.\"},
      {\"role\": \"user\", \"content\": \"$DIFF\"}
    ]
  }" | jq -r '.choices[0].message.content')

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

# Gửi vào reviewdog
cat ai-output.json | reviewdog -f=rdjson -name="ai-review" -reporter=github-pr-review
