#!/usr/bin/env bash
# Test suite for ai-review CLI
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
CLI="$SCRIPT_DIR/../ai-review"
INSTALL_SCRIPT="$SCRIPT_DIR/../install.sh"
HOOK_SCRIPT="$SCRIPT_DIR/../pre-commit.sh"

# Test directories
TEST_DIR=$(mktemp -d)
TEST_CONFIG_DIR="$TEST_DIR/.config/ai-review"
TEST_BIN_DIR="$TEST_DIR/.local/bin"
TEST_REPO_DIR="$TEST_DIR/test-repo"

# Override HOME for testing
export HOME="$TEST_DIR"

# Cleanup function
cleanup() {
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT

# Test helper functions
print_test() {
  echo -e "${BLUE}TEST:${NC} $1"
}

pass() {
  echo -e "${GREEN}✓ PASS${NC}: $1"
  TESTS_PASSED=$((TESTS_PASSED + 1))
}

fail() {
  echo -e "${RED}✗ FAIL${NC}: $1"
  TESTS_FAILED=$((TESTS_FAILED + 1))
}

assert_equals() {
  local expected="$1"
  local actual="$2"
  local message="$3"
  if [[ "$expected" == "$actual" ]]; then
    pass "$message"
  else
    fail "$message (expected: '$expected', got: '$actual')"
  fi
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

assert_file_exists() {
  local file="$1"
  local message="$2"
  if [[ -f "$file" ]]; then
    pass "$message"
  else
    fail "$message (file not found: $file)"
  fi
}

assert_file_not_exists() {
  local file="$1"
  local message="$2"
  if [[ ! -f "$file" ]]; then
    pass "$message"
  else
    fail "$message (file should not exist: $file)"
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
# Setup
# ============================================

setup() {
  echo -e "\n${YELLOW}Setting up test environment...${NC}"

  # Create directories
  mkdir -p "$TEST_CONFIG_DIR/hooks"
  mkdir -p "$TEST_BIN_DIR"

  # Copy scripts
  cp "$CLI" "$TEST_BIN_DIR/ai-review"
  cp "$HOOK_SCRIPT" "$TEST_CONFIG_DIR/hooks/pre-commit.sh"
  chmod +x "$TEST_BIN_DIR/ai-review"
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

  echo -e "${GREEN}✓ Test environment ready${NC}"
  echo ""
}

# ============================================
# CLI Tests
# ============================================

test_cli_help() {
  print_test "CLI help command"

  local output=$("$TEST_BIN_DIR/ai-review" help 2>&1)
  assert_contains "$output" "AI Review CLI" "help shows title"
  assert_contains "$output" "install" "help shows install command"
  assert_contains "$output" "uninstall" "help shows uninstall command"
  assert_contains "$output" "config" "help shows config command"
  assert_contains "$output" "status" "help shows status command"
  assert_contains "$output" "update" "help shows update command"
}

test_cli_version() {
  print_test "CLI version command"

  local output=$("$TEST_BIN_DIR/ai-review" --version 2>&1)
  assert_contains "$output" "ai-review version" "version shows version info"
}

test_cli_unknown_command() {
  print_test "CLI unknown command"

  local output=$("$TEST_BIN_DIR/ai-review" foobar 2>&1) || true
  assert_contains "$output" "Unknown command" "shows error for unknown command"
}

test_cli_status_no_config() {
  print_test "CLI status without config"

  # Ensure no config exists
  rm -f "$TEST_CONFIG_DIR/config"

  local output=$("$TEST_BIN_DIR/ai-review" status 2>&1)
  assert_contains "$output" "Config not found" "status shows config not found"
}

test_cli_config_create_and_show() {
  print_test "CLI config create and show"

  # Create config
  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_URL="https://test.example.com/api"
AI_GATEWAY_API_KEY="test-api-key-12345"
AI_MODEL="test-model"
AI_PROVIDER="test-provider"
EOF

  local output=$("$TEST_BIN_DIR/ai-review" config show 2>&1)
  assert_contains "$output" "test.example.com" "config shows gateway URL"
  assert_contains "$output" "test...2345" "config masks API key"
  assert_contains "$output" "test-model" "config shows model"
}

test_cli_config_set() {
  print_test "CLI config set"

  # Ensure config exists
  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_URL="https://old.example.com"
AI_MODEL="old-model"
EOF

  "$TEST_BIN_DIR/ai-review" config set AI_MODEL "new-model" 2>&1
  local output=$(cat "$TEST_CONFIG_DIR/config")
  assert_contains "$output" "new-model" "config set updates value"
}

test_cli_install_not_in_repo() {
  print_test "CLI install outside git repo"

  cd "$TEST_DIR"  # Not a git repo
  local output=$("$TEST_BIN_DIR/ai-review" install 2>&1) || true
  assert_contains "$output" "Not a git repository" "shows error outside git repo"
}

test_cli_install_no_config() {
  print_test "CLI install without config"

  cd "$TEST_REPO_DIR"
  rm -f "$TEST_CONFIG_DIR/config"

  local output=$("$TEST_BIN_DIR/ai-review" install 2>&1) || true
  assert_contains "$output" "Configuration not found" "shows error when no config"
}

test_cli_install_success() {
  print_test "CLI install hook"

  cd "$TEST_REPO_DIR"

  # Create config
  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_URL="https://test.example.com/api"
AI_GATEWAY_API_KEY="test-key"
EOF

  local output=$("$TEST_BIN_DIR/ai-review" install 2>&1)
  assert_contains "$output" "hook installed" "install shows success"
  assert_file_exists "$TEST_REPO_DIR/.git/hooks/pre-commit" "hook file created"

  # Check hook content
  local hook_content=$(cat "$TEST_REPO_DIR/.git/hooks/pre-commit")
  assert_contains "$hook_content" "AI-REVIEW-HOOK" "hook contains marker"
}

test_cli_status_with_hook() {
  print_test "CLI status with hook installed"

  cd "$TEST_REPO_DIR"
  local output=$("$TEST_BIN_DIR/ai-review" status 2>&1)
  assert_contains "$output" "AI Review hook is installed" "status shows hook installed"
}

test_cli_install_already_installed() {
  print_test "CLI install when already installed"

  cd "$TEST_REPO_DIR"
  local output=$(echo "n" | "$TEST_BIN_DIR/ai-review" install 2>&1) || true
  assert_contains "$output" "already installed" "shows already installed warning"
}

test_cli_uninstall_success() {
  print_test "CLI uninstall hook"

  cd "$TEST_REPO_DIR"
  local output=$("$TEST_BIN_DIR/ai-review" uninstall 2>&1)
  assert_contains "$output" "hook removed" "uninstall shows success"
  assert_file_not_exists "$TEST_REPO_DIR/.git/hooks/pre-commit" "hook file removed"
}

test_cli_uninstall_not_installed() {
  print_test "CLI uninstall when not installed"

  cd "$TEST_REPO_DIR"
  local output=$("$TEST_BIN_DIR/ai-review" uninstall 2>&1)
  assert_contains "$output" "No pre-commit hook found" "shows not installed"
}

# ============================================
# Script Syntax Tests
# ============================================

test_script_syntax() {
  print_test "Script syntax validation"

  bash -n "$CLI" 2>&1
  assert_exit_code "0" "$?" "ai-review CLI syntax OK"

  bash -n "$INSTALL_SCRIPT" 2>&1
  assert_exit_code "0" "$?" "install.sh syntax OK"

  bash -n "$HOOK_SCRIPT" 2>&1
  assert_exit_code "0" "$?" "pre-commit.sh syntax OK"
}

# ============================================
# Hook Tests
# ============================================

test_hook_no_config() {
  print_test "Hook without config"

  rm -f "$TEST_CONFIG_DIR/config"
  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1) || true
  assert_contains "$output" "Configuration not found" "hook shows config error"
}

test_hook_no_staged_changes() {
  print_test "Hook with no staged changes"

  # Create config
  cat > "$TEST_CONFIG_DIR/config" << EOF
AI_GATEWAY_URL="https://test.example.com/api"
AI_GATEWAY_API_KEY="test-key"
EOF

  cd "$TEST_REPO_DIR"
  local output=$("$TEST_CONFIG_DIR/hooks/pre-commit.sh" 2>&1)
  assert_contains "$output" "No staged changes" "hook skips when no changes"
}

# ============================================
# Run All Tests
# ============================================

run_all_tests() {
  echo -e "${BLUE}======================================${NC}"
  echo -e "${BLUE}   AI Review CLI Test Suite${NC}"
  echo -e "${BLUE}======================================${NC}"

  setup

  echo -e "\n${YELLOW}Running CLI Tests...${NC}\n"

  test_script_syntax
  test_cli_help
  test_cli_version
  test_cli_unknown_command
  test_cli_status_no_config
  test_cli_config_create_and_show
  test_cli_config_set
  test_cli_install_not_in_repo
  test_cli_install_no_config
  test_cli_install_success
  test_cli_status_with_hook
  test_cli_install_already_installed
  test_cli_uninstall_success
  test_cli_uninstall_not_installed

  echo -e "\n${YELLOW}Running Hook Tests...${NC}\n"

  test_hook_no_config
  test_hook_no_staged_changes

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
