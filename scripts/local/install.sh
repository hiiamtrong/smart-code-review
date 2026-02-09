#!/usr/bin/env bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Installation paths
CONFIG_DIR="$HOME/.config/ai-review"
BIN_DIR="$HOME/.local/bin"
HOOKS_DIR="$CONFIG_DIR/hooks"

log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

echo ""
echo -e "${BOLD}AI Review Installer${NC}"
echo "========================================"

# Check if running interactively
is_interactive() {
  [[ -t 0 ]]
}

# Detect OS
detect_os() {
  case "$(uname -s)" in
    Darwin*)
      OS="macos"
      ;;
    Linux*)
      if grep -qi microsoft /proc/version 2>/dev/null; then
        OS="wsl"
      else
        OS="linux"
      fi
      ;;
    *)
      log_error "Unsupported operating system"
      exit 1
      ;;
  esac
  log_success "Detected OS: $OS"
}

# Install dependencies
install_dependencies() {
  echo ""
  log_info "Checking dependencies..."

  # Check git
  if ! command -v git &> /dev/null; then
    log_error "git is required but not installed"
    exit 1
  fi
  log_success "git is installed"

  # Check curl
  if ! command -v curl &> /dev/null; then
    log_error "curl is required but not installed"
    exit 1
  fi
  log_success "curl is installed"

  # Check/install jq
  if ! command -v jq &> /dev/null; then
    log_warn "jq not found, installing..."
    case "$OS" in
      macos)
        if ! command -v brew &> /dev/null; then
          log_warn "Homebrew not found, installing..."
          /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        fi
        brew install jq
        ;;
      linux|wsl)
        sudo apt-get update && sudo apt-get install -y jq
        ;;
    esac
    log_success "jq installed"
  else
    log_success "jq is installed"
  fi
}

# Create directories
create_directories() {
  echo ""
  log_info "Creating directories..."

  mkdir -p "$CONFIG_DIR"
  mkdir -p "$BIN_DIR"
  mkdir -p "$HOOKS_DIR"

  log_success "Created $CONFIG_DIR"
  log_success "Created $BIN_DIR"
  log_success "Created $HOOKS_DIR"
}

# Get script directory (works for both curl|bash and direct execution)
get_script_source() {
  if [[ -n "$BASH_SOURCE" && -f "$(dirname "${BASH_SOURCE[0]}")/ai-review" ]]; then
    # Direct execution - files are alongside install.sh
    SCRIPT_SOURCE="local"
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  else
    # Remote execution via curl|bash - download from repo
    SCRIPT_SOURCE="remote"
    REPO_URL="https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main"
  fi
}

# Install CLI and hook scripts
install_scripts() {
  echo ""
  log_info "Installing scripts..."

  get_script_source

  if [[ "$SCRIPT_SOURCE" == "local" ]]; then
    # Copy from local source
    cp "$SCRIPT_DIR/ai-review" "$BIN_DIR/ai-review"
    cp "$SCRIPT_DIR/pre-commit.sh" "$HOOKS_DIR/pre-commit.sh"
    cp "$SCRIPT_DIR/enable-local-sonarqube.sh" "$HOOKS_DIR/enable-local-sonarqube.sh"
    # Copy SonarQube scripts from parent scripts directory
    local SONAR_REVIEW_SRC="$(dirname "$SCRIPT_DIR")/sonarqube-review.sh"
    local SHOWLINENUM_SRC="$(dirname "$SCRIPT_DIR")/showlinenum.awk"
    if [[ -f "$SONAR_REVIEW_SRC" ]]; then
      cp "$SONAR_REVIEW_SRC" "$HOOKS_DIR/sonarqube-review.sh"
    fi
    if [[ -f "$SHOWLINENUM_SRC" ]]; then
      cp "$SHOWLINENUM_SRC" "$HOOKS_DIR/showlinenum.awk"
    fi
  else
    # Download from remote
    curl -sSL "$REPO_URL/scripts/local/ai-review" -o "$BIN_DIR/ai-review"
    curl -sSL "$REPO_URL/scripts/local/pre-commit.sh" -o "$HOOKS_DIR/pre-commit.sh"
    curl -sSL "$REPO_URL/scripts/local/enable-local-sonarqube.sh" -o "$HOOKS_DIR/enable-local-sonarqube.sh"
    curl -sSL "$REPO_URL/scripts/sonarqube-review.sh" -o "$HOOKS_DIR/sonarqube-review.sh" 2>/dev/null || true
    curl -sSL "$REPO_URL/scripts/showlinenum.awk" -o "$HOOKS_DIR/showlinenum.awk" 2>/dev/null || true
  fi

  chmod +x "$BIN_DIR/ai-review"
  chmod +x "$HOOKS_DIR/pre-commit.sh"
  chmod +x "$HOOKS_DIR/enable-local-sonarqube.sh"
  [[ -f "$HOOKS_DIR/sonarqube-review.sh" ]] && chmod +x "$HOOKS_DIR/sonarqube-review.sh"
  [[ -f "$HOOKS_DIR/showlinenum.awk" ]] && chmod +x "$HOOKS_DIR/showlinenum.awk"

  log_success "Installed ai-review CLI to $BIN_DIR/ai-review"
  log_success "Installed hook template to $HOOKS_DIR/pre-commit.sh"
  log_success "Installed SonarQube integration scripts"
}

# Interactive configuration
configure_credentials() {
  # Skip if not interactive (e.g., curl | bash)
  if ! is_interactive; then
    echo ""
    log_warn "Non-interactive mode detected"
    log_info "Run 'ai-review setup' to configure credentials"
    return
  fi

  echo ""
  log_info "Configuration"
  echo "Please provide your AI Gateway credentials:"
  echo ""

  # AI Gateway URL
  read -p "Enter AI Gateway URL: " AI_GATEWAY_URL
  while [[ -z "$AI_GATEWAY_URL" ]]; do
    log_error "URL is required"
    read -p "Enter AI Gateway URL: " AI_GATEWAY_URL
  done

  # AI Gateway API Key
  read -sp "Enter AI Gateway API Key: " AI_GATEWAY_API_KEY
  echo ""
  while [[ -z "$AI_GATEWAY_API_KEY" ]]; do
    log_error "API Key is required"
    read -sp "Enter AI Gateway API Key: " AI_GATEWAY_API_KEY
    echo ""
  done

  # AI Model (optional)
  read -p "Enter AI Model [gemini-2.0-flash]: " AI_MODEL
  AI_MODEL="${AI_MODEL:-gemini-2.0-flash}"

  # AI Provider (optional)
  read -p "Enter AI Provider [google]: " AI_PROVIDER
  AI_PROVIDER="${AI_PROVIDER:-google}"

  # Save config
  cat > "$CONFIG_DIR/config" << EOF
# AI Review Configuration
# Generated by installer on $(date)

AI_GATEWAY_URL="$AI_GATEWAY_URL"
AI_GATEWAY_API_KEY="$AI_GATEWAY_API_KEY"
AI_MODEL="$AI_MODEL"
AI_PROVIDER="$AI_PROVIDER"
EOF

  chmod 600 "$CONFIG_DIR/config"
  log_success "Configuration saved to $CONFIG_DIR/config"
}

# Add to PATH
setup_path() {
  echo ""
  log_info "Setting up PATH..."

  # Check if already in PATH
  if echo "$PATH" | grep -q "$BIN_DIR"; then
    log_success "$BIN_DIR is already in PATH"
    return
  fi

  # Detect shell config file
  SHELL_CONFIG=""
  if [[ -n "$ZSH_VERSION" ]] || [[ "$SHELL" == *"zsh"* ]]; then
    SHELL_CONFIG="$HOME/.zshrc"
  elif [[ -n "$BASH_VERSION" ]] || [[ "$SHELL" == *"bash"* ]]; then
    if [[ -f "$HOME/.bashrc" ]]; then
      SHELL_CONFIG="$HOME/.bashrc"
    elif [[ -f "$HOME/.bash_profile" ]]; then
      SHELL_CONFIG="$HOME/.bash_profile"
    fi
  fi

  if [[ -n "$SHELL_CONFIG" ]]; then
    # Check if export already exists
    if ! grep -q "export PATH=\"\$HOME/.local/bin:\$PATH\"" "$SHELL_CONFIG" 2>/dev/null; then
      echo "" >> "$SHELL_CONFIG"
      echo "# Added by ai-review installer" >> "$SHELL_CONFIG"
      echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_CONFIG"
      log_success "Added $BIN_DIR to PATH in $SHELL_CONFIG"
    else
      log_success "PATH export already exists in $SHELL_CONFIG"
    fi
  else
    log_warn "Could not detect shell config file"
    echo "Please add the following to your shell config:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
}

# Print success message
print_success_message() {
  echo ""
  echo "========================================"
  echo -e "${GREEN}Installation complete!${NC}"
  echo "========================================"
  echo ""
  echo "Next steps:"
  if ! is_interactive; then
    echo "  1. Run: ai-review setup"
    echo "  2. Restart your terminal or run: source ~/.zshrc (or ~/.bashrc)"
  else
    echo "  1. Restart your terminal or run: source ~/.zshrc (or ~/.bashrc)"
  fi
  echo "  2. Navigate to any git repository"
  echo "  3. Run: ai-review install"
  echo ""
  echo "Available commands:"
  echo "  ai-review setup      - Configure credentials"
  echo "  ai-review install    - Install hook in current repo"
  echo "  ai-review uninstall  - Remove hook from current repo"
  echo "  ai-review config     - View/edit configuration"
  echo "  ai-review status     - Check installation status"
  echo "  ai-review update     - Update to latest version"
  echo "  ai-review help       - Show help"
  echo ""
}

# Main installation flow
main() {
  detect_os
  install_dependencies
  create_directories
  install_scripts
  configure_credentials
  setup_path
  print_success_message
}

main
