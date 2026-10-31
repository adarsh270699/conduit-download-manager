#requires -Version 5.1
<#
.SYNOPSIS
    Smoke-test the Windows installer: silent install, launch, upgrade in
    place, and silent uninstall.

.DESCRIPTION
    CI runs this after scripts\build_windows.ps1 on every push and before
    every release. It installs into the current user's profile exactly like
    a real install, so it refuses to run when Conduit is already installed
    unless -Force is given. Do not run it on a machine whose Conduit install
    you want to keep.

    Checks:
      1. (with -PreviousInstaller) install the previous release and launch it,
         so the upgrade below starts from a real older install.
      2. Silent install of the new build: files, Start menu shortcut,
         uninstall registry entry, and DisplayVersion equal to VERSION.
      3. Conduit.exe --version reports VERSION.
      4. Conduit starts in the tray and /api/health reports VERSION.
      5. A second silent install updates in place and keeps config.json.
      6. Silent uninstall removes the program and registry entry and keeps
         the settings (the silent default).

.PARAMETER Installer
    The setup executable to test. Defaults to dist\Conduit-windows-amd64-setup.exe.

.PARAMETER PreviousInstaller
    Optional older setup executable for an upgrade test.

.PARAMETER SkipLaunch
    Skip the launch and health check (for machines without WebView2).

.PARAMETER Force
    Run even though Conduit is already installed for this user.
#>
[CmdletBinding()]
param(
    [string]$Installer = '',
    [string]$PreviousInstaller = '',
    [switch]$SkipLaunch,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

# The daemon falls back to an aria2c on PATH. Take those folders out of PATH and
# drop CONDUIT_ARIA2C, so the engine checks below test the installed engine.
Remove-Item -LiteralPath Env:CONDUIT_ARIA2C -ErrorAction SilentlyContinue
$env:PATH = (($env:PATH -split ';') | Where-Object { $_.Trim('"') -and -not (Test-Path -LiteralPath (Join-Path $_.Trim('"') 'aria2c.exe') -ErrorAction SilentlyContinue) }) -join ';'

$Root = Split-Path -Parent $PSScriptRoot
if (-not $Installer) {
    $Installer = Join-Path $Root 'dist\Conduit-windows-amd64-setup.exe'
}
$Installer = (Resolve-Path -LiteralPath $Installer).Path
$Expected = (Get-Content -LiteralPath (Join-Path $Root 'daemon\version\VERSION') -Raw).Trim()
$UninstallKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit'
$ConfigFile = Join-Path $env:APPDATA 'conduit\config.json'
$HealthUrl = 'http://127.0.0.1:18800/api/health'
$Aria2Log = Join-Path ([System.IO.Path]::GetTempPath()) 'conduit-aria2-install.log'

$script:failures = 0
function Pass([string]$Message) { Write-Host "  ok   $Message" -ForegroundColor Green }
function Fail([string]$Message) {
    Write-Host "  FAIL $Message" -ForegroundColor Red
    # In GitHub Actions each failed check also becomes an annotation on the run.
    if ($env:GITHUB_ACTIONS) { Write-Host "::error title=Installer smoke test::$Message" }
    $script:failures++
}
function Check([bool]$Condition, [string]$Message) { if ($Condition) { Pass $Message } else { Fail $Message } }
function Step([string]$Message) { Write-Host ''; Write-Host "==> $Message" -ForegroundColor Cyan }

function Invoke-Setup([string]$Path, [string[]]$Arguments) {
    $p = Start-Process -FilePath $Path -ArgumentList $Arguments -Wait -PassThru
    return $p.ExitCode
}

function Get-InstallDir {
    $entry = Get-ItemProperty -Path $UninstallKey -ErrorAction SilentlyContinue
    if ($entry) { return $entry.InstallLocation }
    return $null
}

function Get-VersionOutput([string]$Exe) {
    $out = [System.IO.Path]::GetTempFileName()
    try {
        # Conduit.exe is a GUI-subsystem program; redirecting stdout to a
        # file is how its --version text is captured.
        $p = Start-Process -FilePath $Exe -ArgumentList '--version' -Wait -PassThru -NoNewWindow -RedirectStandardOutput $out
        return @{ Code = $p.ExitCode; Text = (Get-Content -LiteralPath $out -Raw) }
    } finally {
        Remove-Item -LiteralPath $out -Force -ErrorAction SilentlyContinue
    }
}

function Wait-Health([int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            return Invoke-RestMethod -Uri $HealthUrl -TimeoutSec 2
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    return $null
}

function Start-AndCheck([string]$Dir, [string]$Version, [switch]$OldRelease, [string]$Engine = 'ready') {
    $exe = Join-Path $Dir 'Conduit.exe'
    $proc = Start-Process -FilePath $exe -ArgumentList '--background' -PassThru
    try {
        $health = Wait-Health 60
        Check ($null -ne $health) "Conduit $Version answered $HealthUrl"
        if ($health) {
            Check ($health.app -eq 'conduit') "health reports app=conduit"
            Check ($health.version -eq $Version) "health reports version $Version (got $($health.version))"
            Check ($health.engine -eq $Engine) "download engine is $Engine (got $($health.engine))"
        }
    } finally {
        & $exe --shutdown | Out-Null
        if (-not $proc.WaitForExit(20000)) {
            $proc.Refresh()
            $why = "Conduit $Version did not exit after --shutdown; killing it (window handle: $($proc.MainWindowHandle), responding: $($proc.Responding))"
            if ($OldRelease) {
                # Releases before 1.2.1 ignore quit requests when the window has
                # not started. They cannot be fixed; the installer falls back to
                # taskkill, which the upgrade below exercises.
                Write-Host "  warn $why" -ForegroundColor Yellow
                if ($env:GITHUB_ACTIONS) { Write-Host "::warning title=Installer smoke test::$why" }
            } else {
                Fail $why
            }
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        }
    }
}

if ((Get-InstallDir) -and -not $Force) {
    Write-Host 'Conduit is already installed for this user. This test would replace and uninstall it.' -ForegroundColor Yellow
    Write-Host 'Run it in CI or a throwaway VM, or pass -Force.' -ForegroundColor Yellow
    exit 2
}

Write-Host "Testing $Installer (expected version $Expected)"

if ($PreviousInstaller) {
    Step "Install previous release $PreviousInstaller"
    $code = Invoke-Setup $PreviousInstaller @('/S')
    Check ($code -eq 0) "previous installer exited 0 (got $code)"
    $previousVersion = (Get-ItemProperty -Path $UninstallKey -ErrorAction SilentlyContinue).DisplayVersion
    if (-not $SkipLaunch -and (Get-InstallDir)) {
        Start-AndCheck (Get-InstallDir) $previousVersion -OldRelease
    }
}

Step 'Silent install'
Remove-Item -LiteralPath $Aria2Log -Force -ErrorAction SilentlyContinue
$code = Invoke-Setup $Installer @('/S')
Check ($code -eq 0) "installer exited 0 (got $code)"
$dir = Get-InstallDir
Check ([bool]$dir) "uninstall registry entry exists"
if (-not $dir) {
    Write-Host "Cannot continue without an install location." -ForegroundColor Red
    exit 1
}
$entry = Get-ItemProperty -Path $UninstallKey
Check ($entry.DisplayVersion -eq $Expected) "DisplayVersion is $Expected (got $($entry.DisplayVersion))"
foreach ($rel in @('Conduit.exe', 'uninstall.exe', 'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'SOURCE_OFFER.md', 'licenses', 'extension\manifest.json', 'extension\background.js', 'engine\aria2c.exe')) {
    Check (Test-Path -LiteralPath (Join-Path $dir $rel)) "installed $rel"
}
$engine = Join-Path $dir 'engine\aria2c.exe'
if (Test-Path -LiteralPath $engine) {
    $engineVersion = "$(& $engine --version | Select-Object -First 1)"
    Check ($engineVersion -like 'aria2 version *') "installed aria2c runs (got '$engineVersion')"
} else {
    Write-Host "aria2 install log ($Aria2Log):" -ForegroundColor Yellow
    if (Test-Path -LiteralPath $Aria2Log) {
        Get-Content -LiteralPath $Aria2Log | ForEach-Object { Write-Host "  $_" }
    } else {
        Write-Host '  no log: aria2.ps1 did not run' -ForegroundColor Yellow
    }
}
Check (-not (Test-Path -LiteralPath (Join-Path $dir 'extension\test'))) "extension tests are not installed"
Check (Test-Path -LiteralPath (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Conduit\Conduit.lnk')) "Start menu shortcut exists"

$setupInfo = (Get-Item -LiteralPath $Installer).VersionInfo
Check ($setupInfo.ProductVersion -eq $Expected) "setup.exe ProductVersion is $Expected (got $($setupInfo.ProductVersion))"

Step 'Conduit.exe --version'
$v = Get-VersionOutput (Join-Path $dir 'Conduit.exe')
Check ($v.Code -eq 0) "--version exited 0 (got $($v.Code))"
Check ("$($v.Text)".Trim().StartsWith("Conduit $Expected")) "--version prints 'Conduit $Expected' (got '$("$($v.Text)".Trim())')"

if (-not $SkipLaunch) {
    Step 'Launch and health check'
    Start-AndCheck $dir $Expected
}

Step 'Update in place (second silent install)'
$configBefore = if (Test-Path -LiteralPath $ConfigFile) { Get-Content -LiteralPath $ConfigFile -Raw } else { $null }
$code = Invoke-Setup $Installer @('/S')
Check ($code -eq 0) "update-in-place exited 0 (got $code)"
Check ((Get-InstallDir) -eq $dir) "install location unchanged"
if ($null -ne $configBefore) {
    Check ((Test-Path -LiteralPath $ConfigFile) -and ((Get-Content -LiteralPath $ConfigFile -Raw) -eq $configBefore)) "config.json kept across the update"
}

Step 'Silent uninstall'
# _?= runs the uninstaller in place so Start-Process can wait for it.
$code = Invoke-Setup (Join-Path $dir 'uninstall.exe') @('/S', "_?=$dir")
Check ($code -eq 0) "uninstaller exited 0 (got $code)"
Check (-not (Test-Path -Path $UninstallKey)) "uninstall registry entry removed"
Check (-not (Test-Path -LiteralPath (Join-Path $dir 'Conduit.exe'))) "Conduit.exe removed"
Check (-not (Test-Path -LiteralPath (Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Conduit'))) "Start menu folder removed"
if ($null -ne $configBefore) {
    Check (Test-Path -LiteralPath $ConfigFile) "silent uninstall keeps settings"
}
Remove-Item -LiteralPath $dir -Recurse -Force -ErrorAction SilentlyContinue


Step 'Install without the engine download (/NOARIA2)'
$code = Invoke-Setup $Installer @('/S', '/NOARIA2')
Check ($code -eq 0) "installer with /NOARIA2 exited 0 (got $code)"
$offlineDir = Get-InstallDir
Check ([bool]$offlineDir) "install location found after /NOARIA2"
if ($offlineDir) {
    Check (-not (Test-Path -LiteralPath (Join-Path $offlineDir 'engine\aria2c.exe'))) "/NOARIA2 skips the engine download"
    if (-not $SkipLaunch) {
        Start-AndCheck $offlineDir $Expected -Engine 'missing'
    }
    $code = Invoke-Setup (Join-Path $offlineDir 'uninstall.exe') @('/S', "_?=$offlineDir")
    Check ($code -eq 0) "uninstall after /NOARIA2 install exited 0 (got $code)"
    Remove-Item -LiteralPath $offlineDir -Recurse -Force -ErrorAction SilentlyContinue
}

# Runs last, so a failure in the /NOARIA2 section also fails the script.
Write-Host ''
if ($script:failures -gt 0) {
    Write-Host "$($script:failures) installer check(s) failed." -ForegroundColor Red
    exit 1
}
Write-Host 'All installer checks passed.' -ForegroundColor Green
