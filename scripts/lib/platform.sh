#!/usr/bin/env bash
# Platform abstraction layer for cross-platform compatibility
# Source this file at the top of any script that needs OS-specific behavior

# Guard against multiple sourcing
if [[ -n "$_PLATFORM_SH_LOADED" ]]; then
  return 0 2>/dev/null || true
fi
_PLATFORM_SH_LOADED=1

# ============================================
# Platform Detection
# ============================================

# Detect the current platform
# Sets PLATFORM to: macos, linux, wsl, windows
detect_platform() {
  if [[ -n "$PLATFORM" ]]; then
    return 0
  fi

  case "$(uname -s)" in
    Darwin*)
      PLATFORM="macos"
      ;;
    Linux*)
      if grep -qi microsoft /proc/version 2>/dev/null; then
        PLATFORM="wsl"
      else
        PLATFORM="linux"
      fi
      ;;
    MINGW*|MSYS*|CYGWIN*)
      PLATFORM="windows"
      ;;
    *)
      PLATFORM="unknown"
      ;;
  esac

  export PLATFORM
}

# Check if running on Windows (Git Bash / MSYS / Cygwin)
is_windows() {
  detect_platform
  [[ "$PLATFORM" == "windows" ]]
}

# ============================================
# Path Utilities
# ============================================

# Get the platform-appropriate config directory
get_config_dir() {
  # On all platforms (including Windows Git Bash), $HOME/.config works
  echo "$HOME/.config/ai-review"
}

# Get platform-appropriate temp directory
get_temp_dir() {
  if is_windows; then
    # On Windows Git Bash, $TEMP or $TMP is set; fall back to /tmp
    echo "${TEMP:-${TMP:-/tmp}}"
  else
    echo "${TMPDIR:-/tmp}"
  fi
}

# Normalize path: convert backslashes to forward slashes
normalize_path() {
  local path="$1"
  echo "$path" | tr '\\' '/'
}

# ============================================
# Temp File Utilities
# ============================================

# Cross-platform mktemp wrapper
# Falls back to manual temp file creation if mktemp is unavailable
safe_mktemp() {
  local prefix="${1:-platform}"
  if command -v mktemp &>/dev/null; then
    mktemp 2>/dev/null || mktemp -t "$prefix" 2>/dev/null
  else
    # Fallback for environments where mktemp is missing
    local tmp_dir
    tmp_dir="$(get_temp_dir)"
    local tmp_file="${tmp_dir}/${prefix}-$$-${RANDOM}"
    : > "$tmp_file"
    echo "$tmp_file"
  fi
}

# ============================================
# sed Compatibility
# ============================================

# Cross-platform in-place sed
# macOS sed requires -i '' while GNU sed uses -i (or -i.bak)
safe_sed_inplace() {
  local expression="$1"
  local file="$2"

  if [[ "$(uname -s)" == Darwin* ]]; then
    sed -i '' "$expression" "$file"
  else
    sed -i "$expression" "$file"
  fi
}

# ============================================
# Color Support
# ============================================

# Check if the terminal supports ANSI colors
# Sets NO_COLOR=1 if colors should be disabled
check_color_support() {
  if [[ -n "$NO_COLOR" ]]; then
    # User explicitly disabled colors
    return
  fi

  # Check if stdout is a terminal
  if [[ ! -t 1 ]]; then
    NO_COLOR=1
    return
  fi

  if is_windows; then
    # Windows 10+ supports ANSI in most terminals
    # ConEmu, Windows Terminal, Git Bash all support ANSI
    # Only old cmd.exe has issues, but Git Bash handles it
    # We keep colors enabled by default on Windows Git Bash
    :
  fi

  # Check TERM variable
  case "${TERM:-}" in
    dumb|"")
      NO_COLOR=1
      ;;
  esac
}

# Apply color settings - call after defining color variables
# Usage: define your RED, GREEN, etc. variables, then call apply_color_settings
apply_color_settings() {
  check_color_support
  if [[ -n "$NO_COLOR" ]]; then
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    CYAN=''
    BOLD=''
    NC=''
  fi
}

# ============================================
# CRLF Handling
# ============================================

# Strip carriage returns from input (pipe-friendly)
# Usage: some_command | strip_cr
strip_cr() {
  tr -d '\r'
}

# Strip trailing CR from a variable value
# Usage: value=$(strip_cr_var "$value")
strip_cr_var() {
  printf '%s' "${1%$'\r'}"
}

# ============================================
# Tool Validation
# ============================================

# Check that required tools are available
# Usage: check_required_tools git curl jq
check_required_tools() {
  local missing=()
  for tool in "$@"; do
    if ! command -v "$tool" &>/dev/null; then
      missing+=("$tool")
    fi
  done

  if [[ ${#missing[@]} -gt 0 ]]; then
    echo "[ERROR] Missing required tools: ${missing[*]}" >&2
    if is_windows; then
      echo "  Install via Git Bash, or use a package manager:" >&2
      echo "    winget install <package>" >&2
      echo "    choco install <package>" >&2
      echo "    scoop install <package>" >&2
    fi
    return 1
  fi
  return 0
}

# ============================================
# Unzip Compatibility
# ============================================

# Cross-platform unzip
# Falls back to PowerShell Expand-Archive on Windows if unzip is unavailable
safe_unzip() {
  local zip_file="$1"
  local dest_dir="${2:-.}"

  if command -v unzip &>/dev/null; then
    unzip -q "$zip_file" -d "$dest_dir"
  elif is_windows; then
    # Use PowerShell Expand-Archive as fallback
    local win_zip
    local win_dest
    win_zip=$(cygpath -w "$zip_file" 2>/dev/null || echo "$zip_file")
    win_dest=$(cygpath -w "$dest_dir" 2>/dev/null || echo "$dest_dir")
    powershell.exe -NoProfile -Command "Expand-Archive -Path '$win_zip' -DestinationPath '$win_dest' -Force" 2>/dev/null
  else
    echo "[ERROR] unzip is not installed and no fallback available" >&2
    return 1
  fi
}

# ============================================
# Auto-detect platform on source
# ============================================
detect_platform
