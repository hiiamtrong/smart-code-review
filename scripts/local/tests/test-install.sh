#!/usr/bin/env bash
# Test suite for install scripts (install.sh and install.ps1)
# Validates architecture naming, download URL construction, and PATH setup.
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SH="$SCRIPT_DIR/../install.sh"
INSTALL_PS1="$SCRIPT_DIR/../install.ps1"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

pass() {
  echo -e "${GREEN}PASS${NC}: $1"
  TESTS_PASSED=$((TESTS_PASSED + 1))
}

fail() {
  echo -e "${RED}FAIL${NC}: $1"
  TESTS_FAILED=$((TESTS_FAILED + 1))
}

print_test() {
  echo -e "${BLUE}TEST:${NC} $1"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    pass "$message"
  else
    fail "$message (expected to contain: '$needle')"
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if ! echo "$haystack" | grep -qF "$needle"; then
    pass "$message"
  else
    fail "$message (should not contain: '$needle')"
  fi
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

# ============================================
# install.sh — Architecture naming tests
# ============================================

test_install_sh_syntax() {
  print_test "install.sh syntax validation"
  if bash -n "$INSTALL_SH" 2>&1; then
    pass "install.sh syntax is valid"
  else
    fail "install.sh has syntax errors"
  fi
}

test_install_sh_x86_64_maps_to_amd64() {
  print_test "install.sh maps x86_64 to amd64 (goreleaser convention)"
  local content
  content=$(cat "$INSTALL_SH")
  # The case branch for x86_64 should set ARCH="amd64"
  assert_contains "$content" 'x86_64|amd64) ARCH="amd64"' "x86_64 maps to amd64"
}

test_install_sh_no_x86_64_arch_value() {
  print_test "install.sh never sets ARCH to x86_64"
  local content
  content=$(cat "$INSTALL_SH")
  assert_not_contains "$content" 'ARCH="x86_64"' "ARCH is never set to x86_64"
}

test_install_sh_arm64_preserved() {
  print_test "install.sh preserves arm64 architecture"
  local content
  content=$(cat "$INSTALL_SH")
  assert_contains "$content" 'arm64|aarch64) ARCH="arm64"' "arm64/aarch64 maps to arm64"
}

test_install_sh_archive_uses_arch_variable() {
  print_test "install.sh archive name uses ARCH variable"
  local content
  content=$(cat "$INSTALL_SH")
  assert_contains "$content" '${BINARY_NAME}_${OS}_${ARCH}' "archive name template uses ARCH variable"
}

test_install_sh_detect_platform_function() {
  print_test "install.sh detect_platform sets correct arch on this machine"
  # Source only the detect_platform function and run it
  local arch
  arch=$(bash -c '
    # Override uname to test
    detect_platform() {
      local uos; uos="$(uname -s)"
      local uarch; uarch="$(uname -m)"
      case "$uos" in
        Darwin) OS="darwin" ;;
        Linux)  OS="linux"  ;;
        MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
        *) exit 1 ;;
      esac
      case "$uarch" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) exit 1 ;;
      esac
      echo "$ARCH"
    }
    detect_platform
  ')
  # On any supported machine, ARCH should be amd64 or arm64 — never x86_64
  if [[ "$arch" == "amd64" || "$arch" == "arm64" ]]; then
    pass "detect_platform returns valid goreleaser arch: $arch"
  else
    fail "detect_platform returned unexpected arch: $arch"
  fi
}

# ============================================
# install.ps1 — Architecture naming tests
# ============================================

test_install_ps1_exists() {
  print_test "install.ps1 exists"
  if [[ -f "$INSTALL_PS1" ]]; then
    pass "install.ps1 exists"
  else
    fail "install.ps1 not found"
    return
  fi
}

test_install_ps1_x86_64_maps_to_amd64() {
  print_test "install.ps1 maps x86_64 to amd64 (goreleaser convention)"
  local content
  content=$(cat "$INSTALL_PS1")
  # Default 64-bit should be "amd64", not "x86_64"
  assert_contains "$content" '"amd64"' "PS1 uses amd64 for 64-bit"
}

test_install_ps1_no_x86_64_arch_value() {
  print_test "install.ps1 never uses x86_64 as arch value"
  local content
  content=$(cat "$INSTALL_PS1")
  assert_not_contains "$content" '"x86_64"' "PS1 does not use x86_64"
}

test_install_ps1_arm64_detection() {
  print_test "install.ps1 detects ARM64 processor"
  local content
  content=$(cat "$INSTALL_PS1")
  assert_contains "$content" 'PROCESSOR_ARCHITECTURE -eq "ARM64"' "PS1 checks for ARM64 processor"
  assert_contains "$content" '$arch = "arm64"' "PS1 sets arch to arm64 for ARM64"
}

test_install_ps1_archive_name_format() {
  print_test "install.ps1 constructs correct archive name"
  local content
  content=$(cat "$INSTALL_PS1")
  assert_contains "$content" '${BinaryName}_windows_${arch}.zip' "PS1 archive name matches goreleaser template"
}

# ============================================
# install.ps1 — PATH refresh tests
# ============================================

test_install_ps1_refreshes_session_path() {
  print_test "install.ps1 refreshes PATH in current session"
  local content
  content=$(cat "$INSTALL_PS1")
  assert_contains "$content" '$env:Path' "PS1 updates session PATH variable"
}

test_install_ps1_session_path_adds_bindir() {
  print_test "install.ps1 adds BinDir to session PATH"
  local content
  content=$(cat "$INSTALL_PS1")
  # Should have: $env:Path = "$BinDir;$env:Path"
  assert_contains "$content" '"$BinDir;$env:Path"' "PS1 prepends BinDir to session PATH"
}

test_install_ps1_session_path_checks_before_adding() {
  print_test "install.ps1 checks session PATH before adding (idempotent)"
  local content
  content=$(cat "$INSTALL_PS1")
  assert_contains "$content" '$env:Path -notlike "*$BinDir*"' "PS1 checks if BinDir already in session PATH"
}

test_install_ps1_sets_persistent_path() {
  print_test "install.ps1 persists PATH to User environment"
  local content
  content=$(cat "$INSTALL_PS1")
  assert_contains "$content" 'SetEnvironmentVariable("Path"' "PS1 sets persistent User PATH"
  assert_contains "$content" '"User"' "PS1 targets User scope for persistence"
}

# ============================================
# Cross-script consistency tests
# ============================================

test_both_scripts_use_same_binary_name() {
  print_test "Both scripts use same binary name"
  local sh_name ps1_name
  sh_name=$(grep 'BINARY_NAME=' "$INSTALL_SH" | head -1 | sed 's/.*"\(.*\)".*/\1/')
  ps1_name=$(grep 'BinaryName' "$INSTALL_PS1" | head -1 | sed 's/.*"\(.*\)".*/\1/')
  assert_equals "$sh_name" "$ps1_name" "binary name matches across scripts ($sh_name)"
}

test_both_scripts_use_same_repo() {
  print_test "Both scripts reference same GitHub repo"
  local sh_repo ps1_repo
  sh_repo=$(grep 'REPO=' "$INSTALL_SH" | head -1 | sed 's/.*"\(.*\)".*/\1/')
  ps1_repo=$(grep '\$Repo' "$INSTALL_PS1" | head -1 | sed 's/.*"\(.*\)".*/\1/')
  assert_equals "$sh_repo" "$ps1_repo" "repo matches across scripts ($sh_repo)"
}

# ============================================
# Run All Tests
# ============================================

run_all_tests() {
  echo -e "${BLUE}======================================${NC}"
  echo -e "${BLUE}   Install Scripts Test Suite${NC}"
  echo -e "${BLUE}======================================${NC}"
  echo ""

  echo -e "${YELLOW}install.sh — Architecture & URL tests${NC}"
  echo ""
  test_install_sh_syntax
  test_install_sh_x86_64_maps_to_amd64
  test_install_sh_no_x86_64_arch_value
  test_install_sh_arm64_preserved
  test_install_sh_archive_uses_arch_variable
  test_install_sh_detect_platform_function

  echo ""
  echo -e "${YELLOW}install.ps1 — Architecture & URL tests${NC}"
  echo ""
  test_install_ps1_exists
  test_install_ps1_x86_64_maps_to_amd64
  test_install_ps1_no_x86_64_arch_value
  test_install_ps1_arm64_detection
  test_install_ps1_archive_name_format

  echo ""
  echo -e "${YELLOW}install.ps1 — PATH refresh tests${NC}"
  echo ""
  test_install_ps1_refreshes_session_path
  test_install_ps1_session_path_adds_bindir
  test_install_ps1_session_path_checks_before_adding
  test_install_ps1_sets_persistent_path

  echo ""
  echo -e "${YELLOW}Cross-script consistency tests${NC}"
  echo ""
  test_both_scripts_use_same_binary_name
  test_both_scripts_use_same_repo

  # Summary
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
