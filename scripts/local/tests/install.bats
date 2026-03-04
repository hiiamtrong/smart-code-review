#!/usr/bin/env bats
# Tests for scripts/local/install.sh
#
# Prerequisites (CI / Ubuntu):  sudo apt-get install -y bats
# Prerequisites (macOS):        brew install bats-core
# Run:                          bats scripts/local/tests/install.bats

INSTALL_SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/install.sh"

# ── Helpers ───────────────────────────────────────────────────────────────────

# Create an executable stub in STUBS_DIR.
# #!/bin/bash (absolute path) is used so stubs run correctly even when
# PATH="$STUBS_DIR" only — '#!/usr/bin/env bash' would need 'env' in PATH.
make_stub() {                       # make_stub <name> <body>
  local name="$1"; shift
  printf '#!/bin/bash\n%s\n' "$*" > "$STUBS_DIR/$name"
  chmod +x "$STUBS_DIR/$name"
}

setup() {
  STUBS_DIR="$(mktemp -d)"
  TEST_HOME="$(mktemp -d)"
  mkdir -p "$TEST_HOME/.local/bin"

  # ── Build a real tar.gz the real `tar` can extract ─────────────────────────
  printf '#!/bin/bash\necho fake-ai-review\n' > "$TEST_HOME/ai-review"
  chmod +x "$TEST_HOME/ai-review"
  tar -czf "$STUBS_DIR/fake.tar.gz" -C "$TEST_HOME" ai-review
  rm "$TEST_HOME/ai-review"         # will be re-created by the installer
  FAKE_ARCHIVE="$STUBS_DIR/fake.tar.gz"

  # ── PATH isolation: symlink required system utilities ──────────────────────
  # Use PATH=/usr/bin:/bin to resolve real binary paths (not shell aliases).
  for _cmd in grep sed head mktemp tar install mkdir rm cp; do
    _real="$(PATH=/usr/bin:/bin command -v "$_cmd" 2>/dev/null)"
    [[ -n "$_real" && -f "$_real" ]] && ln -sf "$_real" "$STUBS_DIR/$_cmd"
  done
  unset _cmd _real

  # ── Controlled stubs ──────────────────────────────────────────────────────

  # uname: Linux / x86_64 by default
  make_stub uname 'case "$1" in -s) echo "Linux";; -m) echo "x86_64";; esac'

  # curl handles two call patterns from install.sh:
  #   fetch_latest_tag  → curl -fsSL URL          → return GitHub API JSON
  #   download_binary   → curl -fsSL URL -o FILE   → copy fake archive to FILE
  make_stub curl \
'PREV="" OUTFILE=""
for arg; do
  [[ "$PREV" == "-o" ]] && OUTFILE="$arg"
  PREV="$arg"
done
if [[ -n "$OUTFILE" ]]; then
  cp "$FAKE_ARCHIVE" "$OUTFILE"
else
  printf '"'"'{"tag_name": "v9.9.9"}\n'"'"'
fi'

  # wget handles two call patterns:
  #   fetch_latest_tag  → wget -qO- URL      → return GitHub API JSON
  #   download_binary   → wget -qO FILE URL  → copy fake archive to FILE
  make_stub wget \
'PREV=""
for arg; do
  if [[ "$arg" == "-qO-" ]]; then
    printf '"'"'{"tag_name": "v9.9.9"}\n'"'"'
    exit 0
  fi
  if [[ "$PREV" == "-qO" ]]; then
    cp "$FAKE_ARCHIVE" "$arg"
    exit 0
  fi
  PREV="$arg"
done'

  # ── Build the base command array ───────────────────────────────────────────
  # Using /bin/bash (absolute) + env lets us pass PATH="$STUBS_DIR" while
  # still finding bash without requiring it to be in the stubs-only PATH.
  # $FAKE_ARCHIVE is expanded here so the value is baked into each element.
  INSTALLER=(
    env
    "PATH=$STUBS_DIR"
    "HOME=$TEST_HOME"
    "AI_REVIEW_BIN_DIR=$TEST_HOME/.local/bin"
    "FAKE_ARCHIVE=$FAKE_ARCHIVE"
    /bin/bash "$INSTALL_SCRIPT"
  )
}

teardown() {
  rm -rf "$STUBS_DIR" "$TEST_HOME"
}

# ── detect_platform ───────────────────────────────────────────────────────────

@test "detect_platform: Linux/x86_64" {
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" == *"linux/x86_64"* ]]
}

@test "detect_platform: Darwin/arm64" {
  make_stub uname 'case "$1" in -s) echo "Darwin";; -m) echo "arm64";; esac'
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" == *"darwin/arm64"* ]]
}

@test "detect_platform: Linux/aarch64 normalizes to arm64" {
  make_stub uname 'case "$1" in -s) echo "Linux";; -m) echo "aarch64";; esac'
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" == *"arm64"* ]]
}

@test "detect_platform: unsupported OS exits non-zero" {
  make_stub uname 'case "$1" in -s) echo "FreeBSD";; -m) echo "x86_64";; esac'
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -ne 0 ]
  [[ "$output" == *"Unsupported OS"* ]]
}

@test "detect_platform: unsupported arch exits non-zero" {
  make_stub uname 'case "$1" in -s) echo "Linux";; -m) echo "mips";; esac'
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -ne 0 ]
  [[ "$output" == *"Unsupported architecture"* ]]
}

# ── fetch_latest_tag ──────────────────────────────────────────────────────────

@test "fetch_latest_tag: reads tag via curl" {
  run "${INSTALLER[@]}"           # no --version → calls fetch_latest_tag
  [ "$status" -eq 0 ]
  [[ "$output" == *"v9.9.9"* ]]
}

@test "fetch_latest_tag: falls back to wget when curl absent" {
  rm -f "$STUBS_DIR/curl"         # removes curl from the stubs-only PATH
  run "${INSTALLER[@]}"
  [ "$status" -eq 0 ]
  [[ "$output" == *"v9.9.9"* ]]
}

@test "fetch_latest_tag: exits when neither curl nor wget available" {
  rm -f "$STUBS_DIR/curl" "$STUBS_DIR/wget"
  run "${INSTALLER[@]}"
  [ "$status" -ne 0 ]
  [[ "$output" == *"curl or wget is required"* ]]
}

@test "fetch_latest_tag: exits when API returns no tag_name" {
  make_stub curl 'printf "{}\n"'  # valid JSON but no tag_name field
  run "${INSTALLER[@]}"
  [ "$status" -ne 0 ]
  [[ "$output" == *"Could not determine"* ]]
}

# ── --version flag ────────────────────────────────────────────────────────────

@test "--version skips network fetch and uses the provided tag" {
  # The default curl returns "v9.9.9" for fetch calls — if fetch_latest_tag
  # were called it would set TAG=v9.9.9, overriding the flag. Testing that
  # the output contains v1.2.3 (not v9.9.9) proves fetch was skipped.
  run "${INSTALLER[@]}" --version v1.2.3
  [ "$status" -eq 0 ]
  [[ "$output" == *"v1.2.3"* ]]
  [[ "$output" != *"v9.9.9"* ]]
}

# ── EXIT trap regression (unbound variable) ───────────────────────────────────

@test "EXIT trap: no 'unbound variable' error on successful exit" {
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" != *"unbound variable"* ]]
}

@test "EXIT trap: no 'unbound variable' error when download fails mid-function" {
  # curl fails after mktemp -d has already run — tests that the EXIT trap
  # fires cleanly even when download_binary aborts early via set -e.
  make_stub curl 'exit 28'
  run "${INSTALLER[@]}" --version v9.9.9
  [[ "$output" != *"unbound variable"* ]]
}

# ── setup_path ────────────────────────────────────────────────────────────────

@test "setup_path: reports already-in-PATH when BIN_DIR is present" {
  local bin="$TEST_HOME/.local/bin"
  run env \
    "PATH=$STUBS_DIR:$bin" \
    "HOME=$TEST_HOME" \
    "AI_REVIEW_BIN_DIR=$bin" \
    "FAKE_ARCHIVE=$FAKE_ARCHIVE" \
    /bin/bash "$INSTALL_SCRIPT" --version v9.9.9
  [ "$status" -eq 0 ]
  [[ "$output" == *"already in PATH"* ]]
}

@test "setup_path: appends export to .zshrc for zsh users" {
  touch "$TEST_HOME/.zshrc"
  run env \
    "PATH=$STUBS_DIR" \
    "ZSH_VERSION=5.9" \
    "HOME=$TEST_HOME" \
    "AI_REVIEW_BIN_DIR=$TEST_HOME/.local/bin" \
    "FAKE_ARCHIVE=$FAKE_ARCHIVE" \
    /bin/bash "$INSTALL_SCRIPT" --version v9.9.9
  [ "$status" -eq 0 ]
  grep -q 'export PATH' "$TEST_HOME/.zshrc"
}

@test "setup_path: appends export to .bashrc when .bashrc exists" {
  touch "$TEST_HOME/.bashrc"
  run env \
    "PATH=$STUBS_DIR" \
    "HOME=$TEST_HOME" \
    "AI_REVIEW_BIN_DIR=$TEST_HOME/.local/bin" \
    "FAKE_ARCHIVE=$FAKE_ARCHIVE" \
    "SHELL=/bin/bash" \
    /bin/bash "$INSTALL_SCRIPT" --version v9.9.9
  [ "$status" -eq 0 ]
  grep -q 'export PATH' "$TEST_HOME/.bashrc"
}

@test "setup_path: does not add duplicate PATH entry to .zshrc" {
  touch "$TEST_HOME/.zshrc"
  printf '\n# Added by ai-review installer\nexport PATH="$HOME/.local/bin:$PATH"\n' \
    >> "$TEST_HOME/.zshrc"
  run env \
    "PATH=$STUBS_DIR" \
    "ZSH_VERSION=5.9" \
    "HOME=$TEST_HOME" \
    "AI_REVIEW_BIN_DIR=$TEST_HOME/.local/bin" \
    "FAKE_ARCHIVE=$FAKE_ARCHIVE" \
    /bin/bash "$INSTALL_SCRIPT" --version v9.9.9
  [ "$status" -eq 0 ]
  [ "$(grep -c 'ai-review installer' "$TEST_HOME/.zshrc")" -eq 1 ]
}

# ── installation result ───────────────────────────────────────────────────────

@test "binary is installed to BIN_DIR after successful run" {
  run "${INSTALLER[@]}" --version v9.9.9
  [ "$status" -eq 0 ]
  [ -f "$TEST_HOME/.local/bin/ai-review" ]
  [ -x "$TEST_HOME/.local/bin/ai-review" ]
}
