# AI Review installer for Windows — downloads the Go binary from GitHub Releases.
# Run: powershell -ExecutionPolicy Bypass -File install.ps1
#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$Repo       = "hiiamtrong/smart-code-review"
$BinaryName = "ai-review"
$BinDir     = if ($env:AI_REVIEW_BIN_DIR) { $env:AI_REVIEW_BIN_DIR } else { "$env:USERPROFILE\.local\bin" }

function Write-Success($msg) { Write-Host "[OK] $msg"    -ForegroundColor Green }
function Write-Info($msg)    { Write-Host "[INFO] $msg"  -ForegroundColor Cyan  }
function Write-Warn($msg)    { Write-Host "[WARN] $msg"  -ForegroundColor Yellow }
function Write-Err($msg)     { Write-Host "[ERROR] $msg" -ForegroundColor Red   }

Write-Host ""
Write-Host "AI Review Installer for Windows" -ForegroundColor Cyan
Write-Host "===================================="

# ── Detect architecture ───────────────────────────────────────────────────────
$arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else {
    Write-Err "32-bit Windows is not supported"; exit 1
}

# Check for ARM64
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $arch = "arm64" }

Write-Info "Detected platform: windows/$arch"

# ── Check git ─────────────────────────────────────────────────────────────────
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Err "git is required. Download from: https://git-scm.com/download/win"
    exit 1
}
Write-Success "git is installed"

# ── Fetch latest release tag ──────────────────────────────────────────────────
Write-Info "Checking latest release..."
$apiUrl  = "https://api.github.com/repos/$Repo/releases/latest"
$release = Invoke-RestMethod -Uri $apiUrl -Headers @{ "User-Agent" = "ai-review-installer" }
$tag     = $release.tag_name
Write-Info "Latest release: $tag"

# ── Download & extract ────────────────────────────────────────────────────────
$archive = "${BinaryName}_windows_${arch}.zip"
$dlUrl   = "https://github.com/$Repo/releases/download/$tag/$archive"

$tmpDir  = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmpDir | Out-Null

try {
    Write-Info "Downloading $archive..."
    $archivePath = "$tmpDir\$archive"
    Invoke-WebRequest -Uri $dlUrl -OutFile $archivePath -UseBasicParsing

    Write-Info "Extracting binary..."
    Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

    # Ensure destination directory exists.
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

    # Copy binary.
    $exeSrc = "$tmpDir\${BinaryName}.exe"
    if (-not (Test-Path $exeSrc)) {
        # Some goreleaser archives nest binaries in a subdirectory.
        $exeSrc = Get-ChildItem -Path $tmpDir -Filter "${BinaryName}.exe" -Recurse | Select-Object -First 1 -ExpandProperty FullName
    }
    Copy-Item -Path $exeSrc -Destination "$BinDir\${BinaryName}.exe" -Force
    Write-Success "Installed $BinaryName.exe to $BinDir"
} finally {
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
}

# ── Add to User PATH ──────────────────────────────────────────────────────────
Write-Info "Setting up PATH..."
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$BinDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$BinDir;$userPath", "User")
    Write-Success "Added $BinDir to User PATH"
} else {
    Write-Success "$BinDir is already in PATH"
}

# Make binary available in the current session immediately
if ($env:Path -notlike "*$BinDir*") {
    $env:Path = "$BinDir;$env:Path"
}

# ── Success ───────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "====================================" -ForegroundColor Green
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "====================================" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Restart your terminal (PowerShell, CMD, or Git Bash)"
Write-Host "  2. Run: ai-review setup       -- configure credentials"
Write-Host "  3. cd into any git repo"
Write-Host "  4. Run: ai-review install     -- install the pre-commit hook"
Write-Host ""
Write-Host "Commands:"
Write-Host "  ai-review setup      Configure credentials"
Write-Host "  ai-review install    Install hook in current repo"
Write-Host "  ai-review uninstall  Remove hook from current repo"
Write-Host "  ai-review status     Check installation status"
Write-Host "  ai-review update     Update to latest version"
Write-Host "  ai-review help       Show help"
Write-Host ""
