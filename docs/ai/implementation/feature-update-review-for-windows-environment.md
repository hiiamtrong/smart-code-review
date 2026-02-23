---
phase: implementation
title: Implementation - Update Review for Windows Environment
description: Technical implementation notes for Windows compatibility
---

# Implementation - Update Review for Windows Environment

## Code Structure

### New File
- `scripts/lib/platform.sh` - Cross-platform utility library

### Modified Files
- `scripts/local/install.sh` - Windows OS detection + jq installation
- `scripts/sonarqube-review.sh` - Windows scanner binary + unzip fallback
- `scripts/local/pre-commit.sh` - mktemp, CRLF, process substitution fixes
- `scripts/ai-review.sh` - mktemp + CRLF fixes
- `scripts/detect-language.sh` - Source platform.sh
- `scripts/filter-ignored-files.sh` - CRLF stripping
- `scripts/local/enable-local-sonarqube.sh` - sed -i wrapper
- `scripts/local/install.ps1` - Copy platform.sh during install
- `SETUP_GUIDE.md` - Windows documentation
- `README.md` - Windows support info

## Implementation Notes

### Platform Detection Pattern
```bash
# Source platform library (resolves path relative to script location)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../lib/platform.sh" 2>/dev/null || source "${HOOKS_DIR}/platform.sh" 2>/dev/null || true
```

### CRLF Handling Pattern
```bash
# Before: breaks on Windows
while IFS= read -r line; do ...

# After: strips carriage returns
while IFS= read -r line; do
  line="${line%$'\r'}"  # Strip CR
  ...
```

### mktemp Replacement Pattern
```bash
# Before: may fail on Windows
local temp_file=$(mktemp)

# After: uses platform utility
local temp_file=$(safe_mktemp)
```

### Process Substitution Fallback
```bash
# Before: unreliable on Windows Git Bash
while read -r line; do ... done < <(curl -s "$URL")

# After: temp file approach
local temp_file=$(safe_mktemp)
curl -s "$URL" > "$temp_file"
while read -r line; do ... done < "$temp_file"
rm -f "$temp_file"
```

## Error Handling

- `check_required_tools()` validates at startup and exits with clear install instructions
- `safe_mktemp()` falls back to manual temp file creation if mktemp is unavailable
- Color output degrades to plain text on unsupported terminals
- Scanner download detects OS correctly; clear error if architecture not supported

## Security Notes

- No changes to authentication or API key handling
- No changes to data flow or external service communication
- Temp files cleaned up via trap handlers (verified for MINGW signal handling)
