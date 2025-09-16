#!/usr/bin/env bash
set -e

# Install reviewdog
mkdir -p $HOME/bin
curl -sfL https://raw.githubusercontent.com/reviewdog/reviewdog/master/install.sh | sh -s -- -b $HOME/bin
export PATH="$HOME/bin:$PATH"

echo "🔎 Detecting project language..."

if [ -f "package.json" ]; then
  LANG="node"
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

echo "➡️ Detected language: $LANG"

bash $(dirname "$0")/ai-review.sh $LANG
