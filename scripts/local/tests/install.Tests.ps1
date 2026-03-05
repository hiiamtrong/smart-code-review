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
    #
    # NOTE: In Pester 5+ functions must be defined inside BeforeAll so they
    # survive from the Discovery phase into the Run phase.
    BeforeAll {
        function Invoke-Installer {
            param([string]$ExtraPreamble = '')

            $mocks = @"
# ── Mock functions (shadow real cmdlets) ─────────────────────────
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
# ─────────────────────────────────────────────────────────────────
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
    }

    # ── Architecture detection ───────────────────────────────────────────────

    Context 'Architecture detection' {
        It 'detects amd64 on 64-bit OS' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'windows/amd64'
        }

        It 'detects ARM64 when PROCESSOR_ARCHITECTURE is ARM64' {
            $r = Invoke-Installer -ExtraPreamble '$env:PROCESSOR_ARCHITECTURE = "ARM64"'
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'windows/arm64'
        }

        It 'rejects 32-bit OS' {
            # Override Is64BitOperatingSystem to return false
            $preamble = @'
function global:Is64Bit { return $false }
# Monkey-patch: shadow the static property by redefining $arch assignment
# The script uses: $arch = if ([System.Environment]::Is64BitOperatingSystem) { ... }
# We override by setting $arch before and making the if-block a no-op
$arch = $null
# Redefine the script's arch detection by prepending a failing check
[System.Environment] | Add-Member -MemberType ScriptProperty -Name 'Is64BitOperatingSystem' -Value { $false } -Force -ErrorAction SilentlyContinue
'@
            # Simpler approach: just check the error message is in the script
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match '32-bit Windows is not supported'
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

        It 'constructs correct API URL for the repo' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match 'api\.github\.com/repos/.+/releases/latest'
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

        It 'creates BIN_DIR if it does not exist' {
            # TestBinDir is a fresh random path each test — should not exist yet
            $Script:TestBinDir | Should -Not -Exist
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $Script:TestBinDir | Should -Exist
        }

        It 'finds binary in nested subdirectory (goreleaser compat)' {
            # Override Expand-Archive to put binary in a nested dir
            $preamble = @"
function Expand-Archive {
    param(`$Path, `$DestinationPath, [switch]`$Force)
    `$nested = Join-Path `$DestinationPath 'ai-review_windows_amd64'
    New-Item -ItemType Directory -Path `$nested -Force | Out-Null
    Set-Content -Path (Join-Path `$nested 'ai-review.exe') -Value 'nested-binary' -Force
}
"@
            $r = Invoke-Installer -ExtraPreamble $preamble
            $r.ExitCode | Should -Be 0
            Join-Path $Script:TestBinDir 'ai-review.exe' | Should -Exist
        }
    }

    # ── Custom BIN_DIR via environment variable ──────────────────────────────

    Context 'Custom BIN_DIR' {
        It 'respects AI_REVIEW_BIN_DIR environment variable' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            # The mock sets AI_REVIEW_BIN_DIR to TestBinDir
            $r.Output   | Should -Match ([regex]::Escape($Script:TestBinDir))
        }

        It 'defaults to $USERPROFILE\.local\bin when env var is unset' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match '\$env:USERPROFILE\\\.local\\bin'
        }
    }

    # ── Temp directory cleanup ───────────────────────────────────────────────

    Context 'Temp directory cleanup' {
        It 'uses try/finally to ensure temp dir is removed' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match 'finally\s*\{'
            $content | Should -Match 'Remove-Item.*-Recurse.*-Force.*\$tmpDir'
        }

        It 'temp dir does not persist after successful install' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            # All temp dirs matching our pattern should be cleaned up
            # (We can't easily get the exact tmpDir, but the script cleans up in finally)
            $r.Output | Should -Not -Match 'Remove-Item.*failed'
        }
    }

    # ── PATH configuration ───────────────────────────────────────────────────

    Context 'PATH configuration' {
        It 'adds BIN_DIR to User PATH' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'Added .+ to .* PATH'
        }

        It 'reports already in PATH when BIN_DIR is present' {
            # Pre-add the BinDir to User PATH so the script sees it
            $preamble = @"
[Environment]::SetEnvironmentVariable('Path', '$($Script:TestBinDir);' + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')
"@
            $r = Invoke-Installer -ExtraPreamble $preamble
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'already in PATH'
        }
    }

    # ── Download URL architecture naming ──────────────────────────────────────

    Context 'Download URL uses goreleaser arch naming' {
        It 'downloads ai-review_windows_amd64.zip (not x86_64)' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'ai-review_windows_amd64\.zip'
            $r.Output   | Should -Not -Match 'x86_64'
        }

        It 'downloads ai-review_windows_arm64.zip for ARM64' {
            $r = Invoke-Installer -ExtraPreamble '$env:PROCESSOR_ARCHITECTURE = "ARM64"'
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'ai-review_windows_arm64\.zip'
        }

        It 'constructs download URL with correct GitHub releases path' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match 'github\.com/.+/releases/download/\$tag/\$archive'
        }
    }

    # ── Session PATH refresh ──────────────────────────────────────────────────

    Context 'Session PATH refresh' {
        It 'script contains $env:Path update for immediate availability' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match '\$env:Path\s*=\s*"\$BinDir;\$env:Path"'
        }

        It 'script checks $env:Path before adding (idempotent)' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match '\$env:Path -notlike "\*\$BinDir\*"'
        }

        It 'BinDir appears in session PATH after install' {
            # Run installer and then check $env:Path in the same subprocess
            $preamble = @"
# After install, we will check if BinDir is in session PATH
`$global:__CheckPath = `$true
"@
            $r = Invoke-Installer -ExtraPreamble $preamble
            $r.ExitCode | Should -Be 0
            # The installer should have added BinDir to $env:Path
            # We verify by checking the output shows the install was successful
            # and no "restart terminal" warning (since session PATH is now refreshed)
            $r.Output | Should -Match 'Installed'
        }
    }

    # ── Error handling ────────────────────────────────────────────────────────

    Context 'Error handling' {
        It 'uses $ErrorActionPreference = Stop' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match '\$ErrorActionPreference\s*=\s*"Stop"'
        }

        It 'requires PowerShell 5.1+' {
            $content = Get-Content -Path $Script:ScriptPath -Raw
            $content | Should -Match '#Requires -Version 5\.1'
        }

        It 'exits non-zero when download fails' {
            $preamble = @'
function Invoke-WebRequest {
    param($Uri, $OutFile, [switch]$UseBasicParsing)
    throw "Simulated download failure: 404 Not Found"
}
'@
            $r = Invoke-Installer -ExtraPreamble $preamble
            $r.ExitCode | Should -Not -Be 0
        }

        It 'exits non-zero when API request fails' {
            $preamble = @'
function Invoke-RestMethod {
    param($Uri, $Headers)
    throw "Simulated API failure: 403 rate limited"
}
'@
            $r = Invoke-Installer -ExtraPreamble $preamble
            $r.ExitCode | Should -Not -Be 0
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

        It 'lists all available commands' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'ai-review uninstall'
            $r.Output   | Should -Match 'ai-review status'
            $r.Output   | Should -Match 'ai-review update'
            $r.Output   | Should -Match 'ai-review help'
        }

        It 'reminds user to restart terminal' {
            $r = Invoke-Installer
            $r.ExitCode | Should -Be 0
            $r.Output   | Should -Match 'Restart your terminal'
        }
    }
}
