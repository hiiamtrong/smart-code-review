#!/usr/bin/env bash
set -e

# Source platform abstraction layer if available
_DETECT_LANG_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$_DETECT_LANG_SCRIPT_DIR/lib/platform.sh" ]]; then
  source "$_DETECT_LANG_SCRIPT_DIR/lib/platform.sh"
elif [[ -f "$HOME/.config/ai-review/hooks/platform.sh" ]]; then
  source "$HOME/.config/ai-review/hooks/platform.sh"
fi

# Install reviewdog
mkdir -p $HOME/bin
curl -sfL https://raw.githubusercontent.com/reviewdog/reviewdog/master/install.sh | sh -s -- -b $HOME/bin
export PATH="$HOME/bin:$PATH"

echo "Detecting project language..."

if [ -f "package.json" ]; then
  if grep -q '"typescript"' package.json; then
    LANG="typescript"
  else
    LANG="javascript"
  fi
elif [ -f "requirements.txt" ] || [ -f "pyproject.toml" ]; then
  LANG="python"
elif [ -f "pom.xml" ] || [ -f "build.gradle" ]; then
  LANG="java"
elif [ -f "go.mod" ]; then
  LANG="go"
elif ls *.csproj 1> /dev/null 2>&1; then
  LANG="dotnet"
else
  LANG="unknown"
fi

echo "Detected language: $LANG"

# Export environment variables for the AI review script
export DETECTED_LANGUAGE="$LANG"

# Run AI review script (it will handle both language-specific linting and AI review)
bash $(dirname "$0")/ai-review.sh
