#!/usr/bin/env bash
# AI Review installer — downloads the Go binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash
set -euo pipefail

REPO="hiiamtrong/smart-code-review"
BIN_DIR="${AI_REVIEW_BIN_DIR:-$HOME/.local/bin}"
BINARY_NAME="ai-review"

# ── Colors ────────────────────────────────────────────────────────────────────
if [ -t 1 ]; then
  GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; RED='\033[0;31m'; BOLD='\033[1m'; NC='\033[0m'
else
  GREEN=''; YELLOW=''; BLUE=''; RED=''; BOLD=''; NC=''
fi
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1" >&2; }

echo ""
echo -e "${BOLD}AI Review Installer${NC}"
echo "========================================"

# ── OS / arch detection ───────────────────────────────────────────────────────
detect_platform() {
  local uos; uos="$(uname -s)"
  local uarch; uarch="$(uname -m)"

  case "$uos" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux"  ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *)
      log_error "Unsupported OS: $uos"
      exit 1
      ;;
  esac

  case "$uarch" in
    x86_64|amd64) ARCH="x86_64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
      log_error "Unsupported architecture: $uarch"
      exit 1
      ;;
  esac

  log_success "Detected platform: ${OS}/${ARCH}"
}

# ── Resolve latest tag ────────────────────────────────────────────────────────
fetch_latest_tag() {
  local api_url="https://api.github.com/repos/${REPO}/releases/latest"
  if command -v curl &>/dev/null; then
    TAG="$(curl -fsSL "$api_url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  elif command -v wget &>/dev/null; then
    TAG="$(wget -qO- "$api_url" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  else
    log_error "curl or wget is required"
    exit 1
  fi

  if [ -z "$TAG" ]; then
    log_error "Could not determine latest release tag"
    exit 1
  fi
  log_info "Latest release: $TAG"
}

# ── Download & extract ────────────────────────────────────────────────────────
download_binary() {
  local ext
  if [ "$OS" = "windows" ]; then ext="zip"; else ext="tar.gz"; fi

  local archive="${BINARY_NAME}_${OS}_${ARCH}.${ext}"
  local url="https://github.com/${REPO}/releases/download/${TAG}/${archive}"

  log_info "Downloading $archive..."

  local tmp_dir; tmp_dir="$(mktemp -d)"
  trap 'rm -rf "${tmp_dir:-}"' EXIT

  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$tmp_dir/$archive"
  else
    wget -qO "$tmp_dir/$archive" "$url"
  fi

  log_info "Extracting binary..."
  if [ "$ext" = "tar.gz" ]; then
    tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" "${BINARY_NAME}"
  else
    unzip -q "$tmp_dir/$archive" "${BINARY_NAME}.exe" -d "$tmp_dir"
    mv "$tmp_dir/${BINARY_NAME}.exe" "$tmp_dir/${BINARY_NAME}"
  fi

  mkdir -p "$BIN_DIR"
  install -m 755 "$tmp_dir/${BINARY_NAME}" "$BIN_DIR/${BINARY_NAME}"
  log_success "Installed $BINARY_NAME to $BIN_DIR/$BINARY_NAME"
}

# ── PATH setup ────────────────────────────────────────────────────────────────
setup_path() {
  case ":$PATH:" in
    *":$BIN_DIR:"*) log_success "$BIN_DIR already in PATH"; return ;;
  esac

  local shell_config=""
  if [ -n "${ZSH_VERSION:-}" ] || [ "${SHELL:-}" = "$(command -v zsh)" ]; then
    shell_config="$HOME/.zshrc"
  elif [ -f "$HOME/.bashrc" ]; then
    shell_config="$HOME/.bashrc"
  elif [ -f "$HOME/.bash_profile" ]; then
    shell_config="$HOME/.bash_profile"
  fi

  if [ -n "$shell_config" ]; then
    if ! grep -q "export PATH=\"\$HOME/.local/bin:\$PATH\"" "$shell_config" 2>/dev/null; then
      printf '\n# Added by ai-review installer\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$shell_config"
      log_success "Added $BIN_DIR to PATH in $shell_config"
    else
      log_success "PATH entry already present in $shell_config"
    fi
  else
    log_warn "Could not detect shell config — add this to your shell profile manually:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  fi
}

# ── Success message ───────────────────────────────────────────────────────────
print_next_steps() {
  echo ""
  echo "========================================"
  echo -e "${GREEN}Installation complete!${NC}"
  echo "========================================"
  echo ""
  echo "Next steps:"
  echo "  1. Restart your terminal (or run: source ~/.zshrc / ~/.bashrc)"
  echo "  2. Run: ai-review setup       — configure credentials"
  echo "  3. cd into any git repo"
  echo "  4. Run: ai-review install     — install the pre-commit hook"
  echo ""
  echo "Commands:"
  echo "  ai-review setup      Configure credentials"
  echo "  ai-review install    Install hook in current repo"
  echo "  ai-review uninstall  Remove hook from current repo"
  echo "  ai-review status     Check installation status"
  echo "  ai-review update     Update to latest version"
  echo "  ai-review help       Show help"
  echo ""
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  # Parse optional --version flag (e.g. --version v1.2.3)
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version) TAG="$2"; shift 2 ;;
      *) shift ;;
    esac
  done

  detect_platform
  if [ -z "${TAG:-}" ]; then
    fetch_latest_tag
  else
    log_info "Using specified version: $TAG"
  fi
  download_binary
  setup_path
  print_next_steps
}

main "$@"
