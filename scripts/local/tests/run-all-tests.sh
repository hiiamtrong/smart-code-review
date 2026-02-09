#!/usr/bin/env bash
# Run all AI Review tests
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     AI Review Test Suite Runner        ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""

TOTAL_PASSED=0
TOTAL_FAILED=0

run_test_suite() {
  local name="$1"
  local script="$2"

  echo -e "${YELLOW}Running $name...${NC}"
  echo ""

  if bash "$script"; then
    echo -e "${GREEN}$name completed successfully${NC}"
    return 0
  else
    echo -e "${RED}$name had failures${NC}"
    return 1
  fi
}

# Run CLI tests
echo ""
if run_test_suite "CLI Tests" "$SCRIPT_DIR/test-cli.sh"; then
  TOTAL_PASSED=$((TOTAL_PASSED + 1))
else
  TOTAL_FAILED=$((TOTAL_FAILED + 1))
fi

echo ""
echo "────────────────────────────────────────"
echo ""

# Run Hook tests
if run_test_suite "Hook Tests" "$SCRIPT_DIR/test-hook.sh"; then
  TOTAL_PASSED=$((TOTAL_PASSED + 1))
else
  TOTAL_FAILED=$((TOTAL_FAILED + 1))
fi

# Final summary
echo ""
echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          Final Summary                 ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "Test suites passed: ${GREEN}$TOTAL_PASSED${NC}"
echo -e "Test suites failed: ${RED}$TOTAL_FAILED${NC}"
echo ""

if [[ $TOTAL_FAILED -gt 0 ]]; then
  echo -e "${RED}SOME TEST SUITES FAILED${NC}"
  exit 1
else
  echo -e "${GREEN}ALL TEST SUITES PASSED${NC}"
  exit 0
fi
