#!/usr/bin/env bash
# Test suite for ai-review pre-commit hook
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_SCRIPT="$SCRIPT_DIR/../pre-commit.sh"

# Test directories
TEST_DIR=$(mktemp -d)
TEST_CONFIG_DIR="$TEST_DIR/.config/ai-review"
TEST_REPO_DIR="$TEST_DIR/test-repo"
MOCK_SERVER_PID=""

# Override HOME for testing
export HOME="$TEST_DIR"

# Cleanup function
cleanup() {
  # Kill mock server if running
  if [[ -n "$MOCK_SERVER_PID" ]]; then
    kill "$MOCK_SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# Test helper functions
print_test() {
  echo -e "${BLUE}TEST:${NC} $1"
}

pass() {
  echo -e "${GREEN}PASS${NC}: $1"
  TESTS_PASSED=$((TESTS_PASSED + 1))
}

fail() {
  echo -e "${RED}FAIL${NC}: $1"
  TESTS_FAILED=$((TESTS_FAILED + 1))
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if echo "$haystack" | grep -q "$needle"; then
    pass "$message"
  else
    fail "$message (expected to contain: '$needle')"
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if ! echo "$haystack" | grep -q "$needle"; then
    pass "$message"
  else
    fail "$message (should not contain: '$needle')"
  fi
}

assert_exit_code() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  if [[ "$expected" == "$actual" ]]; then
    pass "$message"
  else
    fail "$message (expected exit code: $expected, got: $actual)"
  fi
}

# ============================================
# Mock API Server
# ============================================

# Create a mock API response file
create_mock_response() {
  local severity="$1"
  local message="$2"

  cat > "$TEST_DIR/mock_response.json" << EOF
{
  "source": {"name": "ai-review", "url": ""},
  "diagnostics": [
    {
      "message": "$message",
      "location": {
        "path": "test.js",
        "range": {
          "start": {"line": 1, "column": 1},
          "end": {"line": 1, "column": 10}
        }
      },
      "severity": "$severity",
      "code": {"value": "test-issue", "url": ""}
    }
  ]
}
EOF
}

create_empty_response() {
  cat > "$TEST_DIR/mock_response.json" << EOF
{
  "source": {"name": "ai-review", "url": ""},
  "diagnostics": []
}
EOF
}

# Start a simple mock server using netcat (if available) or Python
start_mock_server() {
  local port="$1"

  # Use Python's http.server as mock
  if command -v python3 &> /dev/null; then
    # Create a simple mock server script
    cat > "$TEST_DIR/mock_server.py" << 'PYEOF'
import http.server
import json
import os
import sys

PORT = int(sys.argv[1])
RESPONSE_FILE = sys.argv[2]

class MockHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # Suppress logging

    def do_POST(self):
        # Read the response file
        with open(RESPONSE_FILE, 'r') as f:
            response = f.read()

        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(response.encode())

if __name__ == '__main__':
    server = http.server.HTTPServer(('127.0.0.1', PORT), MockHandler)
    server.serve_forever()
PYEOF

    python3 "$TEST_DIR/mock_server.py" "$port" "$TEST_DIR/mock_response.json" &
    MOCK_SERVER_PID=$!
    sleep 1  # Wait for server to start
    return 0
  fi

  echo "Python3 not found, skipping mock server tests"
  return 1
}

stop_mock_server() {
  if [[ -n "$MOCK_SERVER_PID" ]]; then
    kill "$MOCK_SERVER_PID" 2>/dev/null || true
    MOCK_SERVER_PID=""
  fi
}

# Create SSE mock response file
create_sse_response() {
  local severity="$1"
  local message="$2"

  cat > "$TEST_DIR/sse_response.txt" << EOF
event: progress
data: {"type":"start","total_chunks":1}

event: progress
data: {"type":"chunk_start","chunk":1,"total_chunks":1}

event: text
data: {"chunk":1,"text":"analyzing..."}

event: diagnostic
data: {"severity":"$severity","message":"$message","location":{"path":"test.js","range":{"start":{"line":1,"column":1},"end":{"line":1,"column":10}}},"code":{"value":"test-issue","url":""}}

event: progress
data: {"type":"chunk_complete","chunk":1}

event: complete
data: {"overview":"Review completed","total_diagnostics":1,"severity":"$severity"}

EOF
}

# Start SSE mock server
start_sse_mock_server() {
  local port="$1"

  if command -v python3 &> /dev/null; then
    cat > "$TEST_DIR/sse_mock_server.py" << 'PYEOF'
import http.server
import sys
import time

PORT = int(sys.argv[1])
RESPONSE_FILE = sys.argv[2]

class SSEHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def do_POST(self):
        with open(RESPONSE_FILE, 'r') as f:
            response = f.read()

        self.send_response(200)
        self.send_header('Content-Type', 'text/event-stream')
        self.send_header('Cache-Control', 'no-cache')
        self.send_header('Connection', 'keep-alive')
        self.end_headers()

        # Send SSE events with small delays to simulate streaming
        for line in response.split('\n'):
            self.wfile.write((line + '\n').encode())
            self.wfile.flush()
            if line.startswith('data:'):
                time.sleep(0.1)

if __name__ == '__main__':
    server = http.server.HTTPServer(('127.0.0.1', PORT), SSEHandler)
    server.handle_request()  # Handle one request then exit
PYEOF

    python3 "$TEST_DIR/sse_mock_server.py" "$port" "$TEST_DIR/sse_response.txt" &
    MOCK_SERVER_PID=$!
    sleep 0.5
    return 0
  fi

  echo "Python3 not found, skipping SSE mock server tests"
  return 1
}

# ============================================
# Setup
# ============================================

setup() {
  echo -e "\n${YELLOW}Setting up test environment...${NC}"

  # Create directories
  mkdir -p "$TEST_CONFIG_DIR/hooks"

  # Copy hook script
  cp "$HOOK_SCRIPT" "$TEST_CONFIG_DIR/hooks/pre-commit.sh"
  chmod +x "$TEST_CONFIG_DIR/hooks/pre-commit.sh"

  # Create test git repo
  mkdir -p "$TEST_REPO_DIR"
  cd "$TEST_REPO_DIR"
  git init --quiet
  git config user.email "test@example.com"
  git config user.name "Test User"

  # Create initial commit
  echo "# Test Repo" > README.md
  git add README.md
  git commit -m "Initial commit" --quiet

  echo -e "${GREEN}Test environment ready${NC}"
  echo ""
}

setup_config() {
  local url="$1"
  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_URL="$url"
AI_GATEWAY_API_KEY="test-api-key"
AI_MODEL="test-model"
AI_PROVIDER="test-provider"
EOF
}

# ============================================
# Hook Tests
# ============================================

test_hook_syntax() {
  print_test "Hook script syntax"
  bash -n "$HOOK_SCRIPT" 2>&1
  assert_exit_code "0" "$?" "pre-commit.sh syntax is valid"
}

test_hook_missing_config() {
  print_test "Hook with missing config"

  rm -f "$TEST_CONFIG_DIR/config"
  cd "$TEST_REPO_DIR"

  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || true
  assert_contains "$output" "Configuration not found" "shows config error"
}

test_hook_missing_url() {
  print_test "Hook with missing AI_GATEWAY_URL"

  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_API_KEY="test-key"
EOF

  cd "$TEST_REPO_DIR"
  echo "change" >> test.txt
  git add test.txt

  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || true
  assert_contains "$output" "AI_GATEWAY_URL not configured" "shows URL error"

  git reset HEAD test.txt --quiet
  rm -f test.txt
}

test_hook_missing_api_key() {
  print_test "Hook with missing AI_GATEWAY_API_KEY"

  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_URL="http://localhost:9999"
EOF

  cd "$TEST_REPO_DIR"
  echo "change" >> test.txt
  git add test.txt

  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || true
  assert_contains "$output" "AI_GATEWAY_API_KEY not configured" "shows API key error"

  git reset HEAD test.txt --quiet
  rm -f test.txt
}

test_hook_no_staged_changes() {
  print_test "Hook with no staged changes"

  setup_config "http://localhost:9999"
  cd "$TEST_REPO_DIR"

  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1)
  assert_contains "$output" "No staged changes" "skips when no staged changes"
}

test_hook_with_error_response() {
  print_test "Hook blocks commit on ERROR severity"

  # Start mock server
  if ! start_mock_server 19876; then
    echo -e "${YELLOW}Skipping (no Python3)${NC}"
    return
  fi

  create_mock_response "ERROR" "Security vulnerability detected"
  setup_config "http://127.0.0.1:19876"

  cd "$TEST_REPO_DIR"
  echo "const x = 1;" > test.js
  git add test.js

  local exit_code=0
  local output
  output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || exit_code=$?

  assert_exit_code "1" "$exit_code" "hook returns exit code 1 on ERROR"
  assert_contains "$output" "Commit blocked" "shows commit blocked message"
  assert_contains "$output" "ERROR" "shows ERROR severity"

  git reset HEAD test.js --quiet
  rm -f test.js
  stop_mock_server
}

test_hook_with_warning_response() {
  print_test "Hook allows commit on WARNING severity"

  if ! start_mock_server 19877; then
    echo -e "${YELLOW}Skipping (no Python3)${NC}"
    return
  fi

  create_mock_response "WARNING" "Consider adding error handling"
  setup_config "http://127.0.0.1:19877"

  cd "$TEST_REPO_DIR"
  echo "const y = 2;" > test2.js
  git add test2.js

  local exit_code=0
  local output
  output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || exit_code=$?

  assert_exit_code "0" "$exit_code" "hook returns exit code 0 on WARNING"
  assert_contains "$output" "Commit allowed" "shows commit allowed"
  assert_contains "$output" "WARN" "shows WARNING severity"

  git reset HEAD test2.js --quiet
  rm -f test2.js
  stop_mock_server
}

test_hook_with_no_issues() {
  print_test "Hook allows commit when no issues"

  if ! start_mock_server 19878; then
    echo -e "${YELLOW}Skipping (no Python3)${NC}"
    return
  fi

  create_empty_response
  setup_config "http://127.0.0.1:19878"

  cd "$TEST_REPO_DIR"
  echo "const z = 3;" > test3.js
  git add test3.js

  local exit_code=0
  local output
  output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || exit_code=$?

  assert_exit_code "0" "$exit_code" "hook returns exit code 0 when no issues"
  assert_contains "$output" "No issues found" "shows no issues message"

  git reset HEAD test3.js --quiet
  rm -f test3.js
  stop_mock_server
}

test_hook_api_failure() {
  print_test "Hook blocks commit on API failure"

  # Use non-existent server
  setup_config "http://127.0.0.1:19999"

  cd "$TEST_REPO_DIR"
  echo "const w = 4;" > test4.js
  git add test4.js

  local exit_code=0
  local output
  output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || exit_code=$?

  # Should block commit on API failure (strict mode)
  assert_exit_code "1" "$exit_code" "hook blocks on API failure"

  git reset HEAD test4.js --quiet
  rm -f test4.js
}

test_hook_aireviewignore() {
  print_test "Hook respects .aireviewignore"

  if ! start_mock_server 19879; then
    echo -e "${YELLOW}Skipping (no Python3)${NC}"
    return
  fi

  create_mock_response "ERROR" "Should not see this"
  setup_config "http://127.0.0.1:19879"

  cd "$TEST_REPO_DIR"

  # Create .aireviewignore
  echo "*.log" > .aireviewignore
  git add .aireviewignore
  git commit -m "Add ignore file" --quiet

  # Create ignored file
  echo "log content" > debug.log
  git add debug.log

  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1)
  assert_contains "$output" "ignored files" "recognizes ignored files"

  git reset HEAD debug.log --quiet
  rm -f debug.log
  stop_mock_server
}

test_hook_sse_streaming() {
  print_test "Hook handles SSE streaming response"

  if ! start_sse_mock_server 19880; then
    echo -e "${YELLOW}Skipping (no Python3)${NC}"
    return
  fi

  create_sse_response "WARNING" "Consider adding error handling"
  setup_config "http://127.0.0.1:19880"

  cd "$TEST_REPO_DIR"
  echo "const sse = 1;" > test_sse.js
  git add test_sse.js

  local exit_code=0
  local output
  output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || exit_code=$?

  assert_exit_code "0" "$exit_code" "hook returns exit code 0 on streamed WARNING"
  assert_contains "$output" "Analyzing" "shows streaming progress"
  assert_contains "$output" "WARN" "shows streamed warning"

  git reset HEAD test_sse.js --quiet
  rm -f test_sse.js
  stop_mock_server
}

test_hook_sse_streaming_error() {
  print_test "Hook blocks on SSE streaming ERROR"

  if ! start_sse_mock_server 19881; then
    echo -e "${YELLOW}Skipping (no Python3)${NC}"
    return
  fi

  create_sse_response "ERROR" "Security vulnerability detected"
  setup_config "http://127.0.0.1:19881"

  cd "$TEST_REPO_DIR"
  echo "const sse_err = 1;" > test_sse_err.js
  git add test_sse_err.js

  local exit_code=0
  local output
  output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || exit_code=$?

  assert_exit_code "1" "$exit_code" "hook returns exit code 1 on streamed ERROR"
  assert_contains "$output" "ERROR" "shows streamed error"
  assert_contains "$output" "blocked" "shows blocked message"

  git reset HEAD test_sse_err.js --quiet
  rm -f test_sse_err.js
  stop_mock_server
}

# ============================================
# Run All Tests
# ============================================

run_all_tests() {
  echo -e "${BLUE}======================================${NC}"
  echo -e "${BLUE}   AI Review Hook Test Suite${NC}"
  echo -e "${BLUE}======================================${NC}"

  setup

  echo -e "\n${YELLOW}Running Hook Tests...${NC}\n"

  test_hook_syntax
  test_hook_missing_config
  test_hook_missing_url
  test_hook_missing_api_key
  test_hook_no_staged_changes
  test_hook_with_error_response
  test_hook_with_warning_response
  test_hook_with_no_issues
  test_hook_api_failure
  test_hook_aireviewignore
  test_hook_sse_streaming
  test_hook_sse_streaming_error

  # Print summary
  echo ""
  echo -e "${BLUE}======================================${NC}"
  echo -e "${BLUE}   Test Summary${NC}"
  echo -e "${BLUE}======================================${NC}"
  echo ""
  echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
  echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
  echo ""

  if [[ $TESTS_FAILED -gt 0 ]]; then
    echo -e "${RED}SOME TESTS FAILED${NC}"
    exit 1
  else
    echo -e "${GREEN}ALL TESTS PASSED${NC}"
    exit 0
  fi
}

run_all_tests
