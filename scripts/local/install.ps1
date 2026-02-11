# AI Review Installer for Windows
# Run: powershell -ExecutionPolicy Bypass -File install.ps1

$ErrorActionPreference = "Stop"

# Colors
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Success($message) {
    Write-Host "[OK] $message" -ForegroundColor Green
}

function Write-Info($message) {
    Write-Host "[INFO] $message" -ForegroundColor Cyan
}

function Write-Warning($message) {
    Write-Host "[WARN] $message" -ForegroundColor Yellow
}

function Write-Error($message) {
    Write-Host "[ERROR] $message" -ForegroundColor Red
}

# Paths
$ConfigDir = "$env:USERPROFILE\.config\ai-review"
$HooksDir = "$ConfigDir\hooks"
$BinDir = "$env:USERPROFILE\.local\bin"

Write-Host ""
Write-Host "AI Review Installer for Windows" -ForegroundColor Blue
Write-Host "===================================" -ForegroundColor Blue
Write-Host ""

# Check for Git Bash
function Test-GitBash {
    $gitPath = Get-Command git -ErrorAction SilentlyContinue
    if (-not $gitPath) {
        Write-Error "Git is not installed. Please install Git for Windows first."
        Write-Host "Download from: https://git-scm.com/download/win"
        exit 1
    }

    # Check for Git Bash
    $gitDir = Split-Path (Split-Path $gitPath.Source)
    $bashPath = Join-Path $gitDir "bin\bash.exe"

    if (Test-Path $bashPath) {
        Write-Success "Git Bash found at $bashPath"
        return $bashPath
    }

    # Alternative location
    $bashPath = "C:\Program Files\Git\bin\bash.exe"
    if (Test-Path $bashPath) {
        Write-Success "Git Bash found at $bashPath"
        return $bashPath
    }

    Write-Error "Git Bash not found. Please install Git for Windows with Bash."
    exit 1
}

# Check for required tools
function Test-Dependencies {
    Write-Host ""
    Write-Info "Checking dependencies..."

    # Git
    if (Get-Command git -ErrorAction SilentlyContinue) {
        Write-Success "git is installed"
    } else {
        Write-Error "git is required"
        exit 1
    }

    # curl (comes with Windows 10+)
    if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
        Write-Success "curl is installed"
    } else {
        Write-Warning "curl not found, will use Invoke-WebRequest"
    }

    # jq - install if missing
    if (-not (Get-Command jq -ErrorAction SilentlyContinue)) {
        Write-Warning "jq not found, installing..."

        # Try winget first
        if (Get-Command winget -ErrorAction SilentlyContinue) {
            winget install jqlang.jq --silent
        }
        # Try chocolatey
        elseif (Get-Command choco -ErrorAction SilentlyContinue) {
            choco install jq -y
        }
        # Try scoop
        elseif (Get-Command scoop -ErrorAction SilentlyContinue) {
            scoop install jq
        }
        # Manual download
        else {
            Write-Info "Downloading jq manually..."
            $jqUrl = "https://github.com/jqlang/jq/releases/latest/download/jq-win64.exe"
            $jqPath = "$BinDir\jq.exe"
            New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
            Invoke-WebRequest -Uri $jqUrl -OutFile $jqPath
            Write-Success "jq installed to $jqPath"
        }
    } else {
        Write-Success "jq is installed"
    }
}

# Create directories
function New-Directories {
    Write-Host ""
    Write-Info "Creating directories..."

    New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
    New-Item -ItemType Directory -Force -Path $HooksDir | Out-Null
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

    Write-Success "Created $ConfigDir"
    Write-Success "Created $BinDir"
}

# Install scripts
function Install-Scripts {
    Write-Host ""
    Write-Info "Installing scripts..."

    $ScriptDir = Split-Path -Parent $MyInvocation.ScriptName

    # Check if running from local source
    if (Test-Path "$ScriptDir\ai-review") {
        Copy-Item "$ScriptDir\ai-review" "$BinDir\ai-review" -Force
        Copy-Item "$ScriptDir\pre-commit.sh" "$HooksDir\pre-commit.sh" -Force
        Copy-Item "$ScriptDir\enable-local-sonarqube.sh" "$HooksDir\enable-local-sonarqube.sh" -Force
        
        # Copy SonarQube scripts from parent scripts directory
        $ParentScriptDir = Split-Path -Parent $ScriptDir
        if (Test-Path "$ParentScriptDir\sonarqube-review.sh") {
            Copy-Item "$ParentScriptDir\sonarqube-review.sh" "$HooksDir\sonarqube-review.sh" -Force
        }
        if (Test-Path "$ParentScriptDir\showlinenum.awk") {
            Copy-Item "$ParentScriptDir\showlinenum.awk" "$HooksDir\showlinenum.awk" -Force
        }
    } else {
        # Download from remote
        $RepoUrl = "https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main"
        Invoke-WebRequest -Uri "$RepoUrl/scripts/local/ai-review" -OutFile "$BinDir\ai-review"
        Invoke-WebRequest -Uri "$RepoUrl/scripts/local/pre-commit.sh" -OutFile "$HooksDir\pre-commit.sh"
        Invoke-WebRequest -Uri "$RepoUrl/scripts/local/enable-local-sonarqube.sh" -OutFile "$HooksDir\enable-local-sonarqube.sh"
        
        # Download SonarQube scripts (optional, may not exist in older versions)
        try {
            Invoke-WebRequest -Uri "$RepoUrl/scripts/sonarqube-review.sh" -OutFile "$HooksDir\sonarqube-review.sh" -ErrorAction SilentlyContinue
        } catch {
            Write-Warning "sonarqube-review.sh not available from remote"
        }
        try {
            Invoke-WebRequest -Uri "$RepoUrl/scripts/showlinenum.awk" -OutFile "$HooksDir\showlinenum.awk" -ErrorAction SilentlyContinue
        } catch {
            Write-Warning "showlinenum.awk not available from remote"
        }
    }

    # Create Windows wrapper batch file
    $wrapperContent = @"
@echo off
"%USERPROFILE%\.local\bin\bash.exe" "%USERPROFILE%\.local\bin\ai-review" %*
"@

    # Find bash path for wrapper
    $bashPath = Test-GitBash

    $wrapperContent = @"
@echo off
"$bashPath" "%USERPROFILE%\.local\bin\ai-review" %*
"@

    Set-Content -Path "$BinDir\ai-review.cmd" -Value $wrapperContent

    # Create Windows wrapper for enable-local-sonarqube script
    $sonarWrapperContent = @"
@echo off
"$bashPath" "%USERPROFILE%\.config\ai-review\hooks\enable-local-sonarqube.sh" %*
"@
    Set-Content -Path "$BinDir\enable-local-sonarqube.cmd" -Value $sonarWrapperContent

    Write-Success "Installed ai-review CLI"
    Write-Success "Installed hook template"
    Write-Success "Installed SonarQube integration scripts"
}

# Configure credentials
function Set-Configuration {
    Write-Host ""
    Write-Host "Configuration" -ForegroundColor Blue
    Write-Host "Please provide your AI Gateway credentials:"
    Write-Host ""

    # AI Gateway URL
    do {
        $AI_GATEWAY_URL = Read-Host "Enter AI Gateway URL"
        if ([string]::IsNullOrWhiteSpace($AI_GATEWAY_URL)) {
            Write-Warning "URL is required"
        }
    } while ([string]::IsNullOrWhiteSpace($AI_GATEWAY_URL))

    # API Key (masked input)
    do {
        $secureKey = Read-Host "Enter AI Gateway API Key" -AsSecureString
        $AI_GATEWAY_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureKey))
        if ([string]::IsNullOrWhiteSpace($AI_GATEWAY_API_KEY)) {
            Write-Warning "API Key is required"
        }
    } while ([string]::IsNullOrWhiteSpace($AI_GATEWAY_API_KEY))

    # AI Model (optional)
    $AI_MODEL = Read-Host "Enter AI Model [gemini-2.0-flash]"
    if ([string]::IsNullOrWhiteSpace($AI_MODEL)) {
        $AI_MODEL = "gemini-2.0-flash"
    }

    # AI Provider (optional)
    $AI_PROVIDER = Read-Host "Enter AI Provider [google]"
    if ([string]::IsNullOrWhiteSpace($AI_PROVIDER)) {
        $AI_PROVIDER = "google"
    }

    # Save config
    $configContent = @"
# AI Review Configuration
# Generated by installer on $(Get-Date)

AI_GATEWAY_URL="$AI_GATEWAY_URL"
AI_GATEWAY_API_KEY="$AI_GATEWAY_API_KEY"
AI_MODEL="$AI_MODEL"
AI_PROVIDER="$AI_PROVIDER"
"@

    Set-Content -Path "$ConfigDir\config" -Value $configContent

    Write-Host ""
    Write-Success "Configuration saved"
}

# Setup PATH
function Set-Path {
    Write-Host ""
    Write-Info "Setting up PATH..."

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")

    if ($currentPath -notlike "*$BinDir*") {
        $newPath = "$BinDir;$currentPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Success "Added $BinDir to PATH"
        Write-Warning "Please restart your terminal for PATH changes to take effect"
    } else {
        Write-Success "$BinDir is already in PATH"
    }
}

# Print success
function Show-Success {
    Write-Host ""
    Write-Host "===================================" -ForegroundColor Green
    Write-Host "Installation complete!" -ForegroundColor Green
    Write-Host "===================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "  1. Restart your terminal (PowerShell, CMD, or Git Bash)"
    Write-Host "  2. Navigate to any git repository"
    Write-Host "  3. Run: ai-review install"
    Write-Host ""
    Write-Host "Available commands:"
    Write-Host "  ai-review install              - Install hook in current repo"
    Write-Host "  ai-review uninstall            - Remove hook from current repo"
    Write-Host "  ai-review config               - View/edit configuration"
    Write-Host "  ai-review status               - Check installation status"
    Write-Host "  ai-review update               - Update to latest version"
    Write-Host "  ai-review help                 - Show help"
    Write-Host "  enable-local-sonarqube         - Enable/disable SonarQube locally"
    Write-Host ""
}

# Main
function Main {
    $bashPath = Test-GitBash
    Test-Dependencies
    New-Directories
    Install-Scripts
    Set-Configuration
    Set-Path
    Show-Success
}

Main
