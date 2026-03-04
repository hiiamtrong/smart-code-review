#Requires -Modules @{ ModuleName = 'Pester'; ModuleVersion = '5.0' }
<#
.SYNOPSIS
    Tests for scripts/local/install.ps1 (Windows installer)

.DESCRIPTION
    Prerequisites:  pwsh 7+ and Pester 5+
      Install-Module Pester -Force -Scope CurrentUser
    Run:
      Invoke-Pester scripts/local/tests/install.Tests.ps1 -Output Detailed
#>

BeforeAll {
    $Script:ScriptPath = Join-Path (Split-Path $PSScriptRoot) 'install.ps1'
}

Describe 'install.ps1' -Skip:(-not $IsWindows) {

    BeforeEach {
        $Script:TestBinDir = Join-Path ([System.IO.Path]::GetTempPath()) "pester-bin-$(Get-Random)"
    }

    AfterEach {
        if ($Script:TestBinDir -and (Test-Path $Script:TestBinDir)) {
            Remove-Item -Recurse -Force $Script:TestBinDir -ErrorAction SilentlyContinue
        }
    }

    # ── Helper ───────────────────────────────────────────────────────────────
    # Runs install.ps1 in an isolated pwsh subprocess.
    # Mock functions are prepended so they shadow real cmdlets (PowerShell
    # resolves Functions before Cmdlets).  The original script is appended
    # verbatim, so no refactoring of install.ps1 is required.
    function Invoke-Installer {
        param([string]$ExtraPreamble = '')

        $mocks = @"
# ── Mock functions (shadow real cmdlets) ─────────────────────────────
function Invoke-RestMethod {
    param(`$Uri, `$Headers)
    [PSCustomObject]@{ tag_name = 'v9.9.9' }
}

function Invoke-WebRequest {
    param(`$Uri, `$OutFile, [switch]`$UseBasicParsing)
    Set-Content -Path `$OutFile -Value 'dummy-archive' -Force
}

function Expand-Archive {
    param(`$Path, `$DestinationPath, [switch]`$Force)
    New-Item -ItemType Directory -Path `$DestinationPath -Force | Out-Null
    Set-Content -Path (Join-Path `$DestinationPath 'ai-review.exe') -Value 'fake-binary' -Force
}

`$env:AI_REVIEW_BIN_DIR = '$($Script:TestBinDir)'
# ─────────────────────────────────────────────────────────────────────
"@

        $scriptBody = Get-Content -Path $Script:ScriptPath -Raw
        $tempScript = Join-Path ([System.IO.Path]::GetTempPath()) "install-test-$(Get-Random).ps1"
        $content    = $mocks, $ExtraPreamble, $scriptBody -join "`n"
        Set-Content -Path $tempScript -Value $content -Force

        try {
            $output = & pwsh -NoProfile -File $tempScript 2>&1 | Out-String
            return @{
                Output   = $output
                ExitCode = $LASTEXITCODE
            }
        } finally {
            Remove-Item $tempScript -Force -ErrorAction SilentlyContinue
        }
    }

    # ── Architecture detection ───────────────────────────────────────────────

    Context 'Architecture detection' {
        It 'detects x86_64 on 64-bit OS' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'windows/x86_64'
        }

        It 'detects ARM64 when PROCESSOR_ARCHITECTURE is ARM64' {
            $r = Invoke-Installer -ExtraPreamble '$env:PROCESSOR_ARCHITECTURE = "ARM64"'
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'windows/arm64'
        }
    }

    # ── Git prerequisite ─────────────────────────────────────────────────────

    Context 'Git prerequisite' {
        It 'succeeds when git is found' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'git is installed'
        }

        It 'exits with error when git is not in PATH' {
            $sysDir = [System.Environment]::GetFolderPath('System')
            $r = Invoke-Installer -ExtraPreamble "`$env:Path = '$sysDir'"
            $r.ExitCode | Should -Not -Be 0
            $r.Output   | Should -Match 'git is required'
        }
    }

    # ── Fetch latest tag ─────────────────────────────────────────────────────

    Context 'Fetch latest tag' {
        It 'reads tag_name from mocked API response' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'v9\.9\.9'
        }
    }

    # ── Binary installation ──────────────────────────────────────────────────

    Context 'Binary installation' {
        It 'installs ai-review.exe to BIN_DIR' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            Join-Path $Script:TestBinDir 'ai-review.exe' | Should -Exist
        }

        It 'prints Installation complete on success' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'Installation complete'
        }
    }

    # ── PATH configuration ───────────────────────────────────────────────────

    Context 'PATH configuration' {
        It 'adds BIN_DIR to User PATH' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'Added .+ to .* PATH'
        }
    }

    # ── Next steps output ────────────────────────────────────────────────────

    Context 'Next steps output' {
        It 'lists setup and install commands' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'ai-review setup'
            $r.Output   | Should -Match 'ai-review install'
        }
    }
}
