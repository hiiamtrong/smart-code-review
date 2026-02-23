#!/usr/bin/env bash
# Script to enable/disable SonarQube in local pre-commit hooks

set -e

# Source platform abstraction layer if available
_SONAR_ENABLE_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$_SONAR_ENABLE_SCRIPT_DIR/../lib/platform.sh" ]]; then
  source "$_SONAR_ENABLE_SCRIPT_DIR/../lib/platform.sh"
elif [[ -f "$_SONAR_ENABLE_SCRIPT_DIR/platform.sh" ]]; then
  source "$_SONAR_ENABLE_SCRIPT_DIR/platform.sh"
elif [[ -f "$HOME/.config/ai-review/hooks/platform.sh" ]]; then
  source "$HOME/.config/ai-review/hooks/platform.sh"
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m'

# Apply color settings if platform.sh is loaded
if type apply_color_settings &>/dev/null; then
  apply_color_settings
fi

CONFIG_DIR="$HOME/.config/ai-review"
CONFIG_FILE="$CONFIG_DIR/config"

echo -e "${BOLD}SonarQube Local Configuration${NC}"
echo ""

# Check if config exists
if [[ ! -f "$CONFIG_FILE" ]]; then
  echo -e "${RED}Error: AI Review not configured${NC}"
  echo "Run 'ai-review setup' first"
  exit 1
fi

# Load current config
source "$CONFIG_FILE"

# Show current status
echo "Current Configuration:"
echo "  SonarQube Local: ${ENABLE_SONARQUBE_LOCAL:-disabled}"
if [[ "$ENABLE_SONARQUBE_LOCAL" == "true" ]]; then
  echo "  Host: ${SONAR_HOST_URL:-not set}"
  echo "  Project: ${SONAR_PROJECT_KEY:-not set}"
fi
echo ""

# Performance warning
cat << EOF
${YELLOW}Performance Warning${NC}

Running SonarQube in pre-commit hooks will:
  • Increase commit time from ~5s to ~30-60s
  • Require a running SonarQube server
  • May frustrate developers with slow commits

${GREEN}Recommended Approach:${NC}
  • Pre-commit: AI only (fast)
  • CI/CD: SonarQube + AI (thorough)

EOF

# Ask what to do
echo "What would you like to do?"
echo "  1) Enable SonarQube locally (slow commits)"
echo "  2) Disable SonarQube locally (fast commits, recommended)"
echo "  3) Show current configuration"
echo "  4) Cancel"
echo ""
read -p "Choose option [1-4]: " choice

case "$choice" in
  1)
    echo ""
    echo -e "${BOLD}Enabling SonarQube for local commits${NC}"
    echo ""
    
    # Get SonarQube URL
    if [[ -n "$SONAR_HOST_URL" ]]; then
      read -p "SonarQube URL [$SONAR_HOST_URL]: " new_url
      SONAR_HOST_URL="${new_url:-$SONAR_HOST_URL}"
    else
      read -p "SonarQube URL (e.g., http://localhost:9000): " SONAR_HOST_URL
      while [[ -z "$SONAR_HOST_URL" ]]; do
        echo -e "${RED}URL is required${NC}"
        read -p "SonarQube URL: " SONAR_HOST_URL
      done
    fi
    
    # Get token
    if [[ -n "$SONAR_TOKEN" ]]; then
      masked="${SONAR_TOKEN:0:4}****${SONAR_TOKEN: -4}"
      read -sp "SonarQube Token [$masked]: " new_token
      echo ""
      SONAR_TOKEN="${new_token:-$SONAR_TOKEN}"
    else
      read -sp "SonarQube Token: " SONAR_TOKEN
      echo ""
      while [[ -z "$SONAR_TOKEN" ]]; do
        echo -e "${RED}Token is required${NC}"
        read -sp "SonarQube Token: " SONAR_TOKEN
        echo ""
      done
    fi
    
    # Get project key
    if [[ -n "$SONAR_PROJECT_KEY" ]]; then
      read -p "Project Key [$SONAR_PROJECT_KEY]: " new_key
      SONAR_PROJECT_KEY="${new_key:-$SONAR_PROJECT_KEY}"
    else
      read -p "Project Key (optional, auto-generated if empty): " SONAR_PROJECT_KEY
    fi
    
    # Update config
    if grep -q "^ENABLE_SONARQUBE_LOCAL=" "$CONFIG_FILE"; then
      if type safe_sed_inplace &>/dev/null; then
        safe_sed_inplace 's/^ENABLE_SONARQUBE_LOCAL=.*/ENABLE_SONARQUBE_LOCAL="true"/' "$CONFIG_FILE"
      else
        sed -i.bak 's/^ENABLE_SONARQUBE_LOCAL=.*/ENABLE_SONARQUBE_LOCAL="true"/' "$CONFIG_FILE"
        rm -f "$CONFIG_FILE.bak"
      fi
    else
      echo 'ENABLE_SONARQUBE_LOCAL="true"' >> "$CONFIG_FILE"
    fi
    
    if grep -q "^SONAR_HOST_URL=" "$CONFIG_FILE"; then
      if type safe_sed_inplace &>/dev/null; then
        safe_sed_inplace "s|^SONAR_HOST_URL=.*|SONAR_HOST_URL=\"$SONAR_HOST_URL\"|" "$CONFIG_FILE"
      else
        sed -i.bak "s|^SONAR_HOST_URL=.*|SONAR_HOST_URL=\"$SONAR_HOST_URL\"|" "$CONFIG_FILE"
        rm -f "$CONFIG_FILE.bak"
      fi
    else
      echo "SONAR_HOST_URL=\"$SONAR_HOST_URL\"" >> "$CONFIG_FILE"
    fi
    
    if grep -q "^SONAR_TOKEN=" "$CONFIG_FILE"; then
      if type safe_sed_inplace &>/dev/null; then
        safe_sed_inplace "s|^SONAR_TOKEN=.*|SONAR_TOKEN=\"$SONAR_TOKEN\"|" "$CONFIG_FILE"
      else
        sed -i.bak "s|^SONAR_TOKEN=.*|SONAR_TOKEN=\"$SONAR_TOKEN\"|" "$CONFIG_FILE"
        rm -f "$CONFIG_FILE.bak"
      fi
    else
      echo "SONAR_TOKEN=\"$SONAR_TOKEN\"" >> "$CONFIG_FILE"
    fi
    
    if [[ -n "$SONAR_PROJECT_KEY" ]]; then
      if grep -q "^SONAR_PROJECT_KEY=" "$CONFIG_FILE"; then
        if type safe_sed_inplace &>/dev/null; then
          safe_sed_inplace "s|^SONAR_PROJECT_KEY=.*|SONAR_PROJECT_KEY=\"$SONAR_PROJECT_KEY\"|" "$CONFIG_FILE"
        else
          sed -i.bak "s|^SONAR_PROJECT_KEY=.*|SONAR_PROJECT_KEY=\"$SONAR_PROJECT_KEY\"|" "$CONFIG_FILE"
          rm -f "$CONFIG_FILE.bak"
        fi
      else
        echo "SONAR_PROJECT_KEY=\"$SONAR_PROJECT_KEY\"" >> "$CONFIG_FILE"
      fi
    fi
    
    rm -f "$CONFIG_FILE.bak"
    
    echo ""
    echo -e "${GREEN}SonarQube enabled for local commits${NC}"
    echo ""
    echo "Expected commit time: 30-60 seconds"
    echo ""
    echo "To disable, run: $0 and choose option 2"
    ;;
    
  2)
    echo ""
    echo -e "${BOLD}Disabling SonarQube for local commits${NC}"
    
    # Update config
    if grep -q "^ENABLE_SONARQUBE_LOCAL=" "$CONFIG_FILE"; then
      if type safe_sed_inplace &>/dev/null; then
        safe_sed_inplace 's/^ENABLE_SONARQUBE_LOCAL=.*/ENABLE_SONARQUBE_LOCAL="false"/' "$CONFIG_FILE"
      else
        sed -i.bak 's/^ENABLE_SONARQUBE_LOCAL=.*/ENABLE_SONARQUBE_LOCAL="false"/' "$CONFIG_FILE"
        rm -f "$CONFIG_FILE.bak"
      fi
    else
      echo 'ENABLE_SONARQUBE_LOCAL="false"' >> "$CONFIG_FILE"
    fi
    
    rm -f "$CONFIG_FILE.bak"
    
    echo ""
    echo -e "${GREEN}SonarQube disabled for local commits${NC}"
    echo ""
    echo "AI review will still run (fast mode)"
    echo "Expected commit time: 2-10 seconds"
    echo ""
    echo "SonarQube will still run in CI/CD if configured"
    ;;
    
  3)
    echo ""
    echo -e "${BOLD}Current Configuration:${NC}"
    echo ""
    cat "$CONFIG_FILE" | grep -v "API_KEY\|TOKEN" | sed 's/^/  /'
    echo ""
    echo "API keys are hidden for security"
    ;;
    
  *)
    echo ""
    echo "Cancelled"
    exit 0
    ;;
esac

echo ""
echo -e "${BLUE}Tip:${NC} For best results:"
echo "  • Local commits: AI only (fast)"
echo "  • GitHub PR: SonarQube + AI (thorough)"
echo ""
echo "Read more: LOCAL_SONARQUBE_SETUP.md"

