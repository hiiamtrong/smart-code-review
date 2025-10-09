#!/usr/bin/env bash
# Filter out files matching patterns in .aireviewignore from git diff

set -e

DIFF_INPUT="$1"
IGNORE_FILE="${2:-.aireviewignore}"

# If no ignore file exists, output the original diff
if [[ ! -f "$IGNORE_FILE" ]]; then
  echo "$DIFF_INPUT"
  exit 0
fi

# Read ignore patterns from file (skip empty lines and comments)
IGNORE_PATTERNS=()
while IFS= read -r line || [[ -n "$line" ]]; do
  # Skip empty lines and comments
  if [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]]; then
    continue
  fi
  # Trim whitespace
  line=$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
  if [[ -n "$line" ]]; then
    IGNORE_PATTERNS+=("$line")
  fi
done < "$IGNORE_FILE"

# If no patterns found, output original diff
if [[ ${#IGNORE_PATTERNS[@]} -eq 0 ]]; then
  echo "$DIFF_INPUT"
  exit 0
fi

echo "🔍 Found ${#IGNORE_PATTERNS[@]} ignore patterns in $IGNORE_FILE" >&2

# Parse diff and filter out ignored files
OUTPUT_DIFF=""
CURRENT_FILE=""
CURRENT_BLOCK=""
IN_FILE_BLOCK=false

while IFS= read -r line; do
  # Check if this is a file header (diff --git or +++ line)
  if [[ "$line" =~ ^diff\ --git\ a/(.+)\ b/(.+)$ ]] || [[ "$line" =~ ^\+\+\+\ b/(.+)$ ]]; then
    # Save previous block if it wasn't ignored
    if [[ -n "$CURRENT_FILE" && "$IN_FILE_BLOCK" == true ]]; then
      SHOULD_IGNORE=false
      for pattern in "${IGNORE_PATTERNS[@]}"; do
        # Convert gitignore-style pattern to grep pattern
        # Support wildcards: *.ext, dir/*, **/file, etc.
        grep_pattern=$(echo "$pattern" | sed 's/\./\\./g' | sed 's/\*/.*/g')
        if echo "$CURRENT_FILE" | grep -qE "^${grep_pattern}$" || echo "$CURRENT_FILE" | grep -qE "${grep_pattern}"; then
          SHOULD_IGNORE=true
          echo "⏭️  Ignoring: $CURRENT_FILE (matches: $pattern)" >&2
          break
        fi
      done
      
      if [[ "$SHOULD_IGNORE" == false ]]; then
        OUTPUT_DIFF="${OUTPUT_DIFF}${CURRENT_BLOCK}"
      fi
    fi
    
    # Start new file block
    if [[ "$line" =~ ^diff\ --git\ a/(.+)\ b/(.+)$ ]]; then
      CURRENT_FILE="${BASH_REMATCH[2]}"
    elif [[ "$line" =~ ^\+\+\+\ b/(.+)$ ]]; then
      CURRENT_FILE="${BASH_REMATCH[1]}"
    fi
    CURRENT_BLOCK="$line\n"
    IN_FILE_BLOCK=true
  else
    # Accumulate lines for current file block
    if [[ "$IN_FILE_BLOCK" == true ]]; then
      CURRENT_BLOCK="${CURRENT_BLOCK}${line}\n"
    else
      # Lines before first file (e.g., index lines)
      OUTPUT_DIFF="${OUTPUT_DIFF}${line}\n"
    fi
  fi
done <<< "$DIFF_INPUT"

# Don't forget the last block
if [[ -n "$CURRENT_FILE" && "$IN_FILE_BLOCK" == true ]]; then
  SHOULD_IGNORE=false
  for pattern in "${IGNORE_PATTERNS[@]}"; do
    grep_pattern=$(echo "$pattern" | sed 's/\./\\./g' | sed 's/\*/.*/g')
    if echo "$CURRENT_FILE" | grep -qE "^${grep_pattern}$" || echo "$CURRENT_FILE" | grep -qE "${grep_pattern}"; then
      SHOULD_IGNORE=true
      echo "⏭️  Ignoring: $CURRENT_FILE (matches: $pattern)" >&2
      break
    fi
  done
  
  if [[ "$SHOULD_IGNORE" == false ]]; then
    OUTPUT_DIFF="${OUTPUT_DIFF}${CURRENT_BLOCK}"
  fi
fi

# Output filtered diff
echo -e "$OUTPUT_DIFF"
