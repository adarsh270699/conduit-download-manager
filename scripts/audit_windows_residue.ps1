param(
    [ValidateSet("clean", "keep", "purge")]
    [string]$Expect = "clean",
    [string]$ConfigDir = (Join-Path $env:APPDATA "conduit"),
    [string]$LocalDataDir = (Join-Path $env:LOCALAPPDATA "conduit"),
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "Programs\Conduit")
)

$ErrorActionPreference = "Stop"
$failures = New-Object System.Collections.Generic.List[string]

function Assert-Missing([string]$Path) {
    if (Test-Path -LiteralPath $Path) {
        $failures.Add("unexpected path exists: $Path")
    }
}

function Assert-Exists([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) {
        $failures.Add("expected path is missing: $Path")
    }
}

function Assert-NoProcessPath([string]$ImageName, [string]$Root) {
    $normalizedRoot = [System.IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
    $found = Get-CimInstance Win32_Process -Filter "Name='$ImageName'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -and [System.IO.Path]::GetFullPath($_.ExecutablePath).StartsWith($normalizedRoot, [System.StringComparison]::OrdinalIgnoreCase) }
    foreach ($match in $found) {
        $failures.Add("process still running from managed path: $($match.ExecutablePath) (PID $($match.ProcessId))")
    }
}

function Assert-RegistryValueMissing([string]$Key, [string]$Name) {
    $value = Get-ItemProperty -LiteralPath $Key -Name $Name -ErrorAction SilentlyContinue
    if ($null -ne $value) {
        $failures.Add("unexpected registry value: $Key\$Name")
    }
}

$legacyWebView = Join-Path $env:APPDATA "Conduit.exe"
$legacyWebViewCleanup = Join-Path $env:APPDATA "Conduit.exe.cleanup"
$configFile = Join-Path $ConfigDir "config.json"
$sessionFile = Join-Path $ConfigDir "aria2.session"
$engineFile = Join-Path $ConfigDir "engine.json"
$engineDir = Join-Path $ConfigDir "bin"
$lockFile = Join-Path $ConfigDir "instance.lock"
$uninstallKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit"
$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$startupRunKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run"
$startupRun32Key = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run32"

switch ($Expect) {
    "keep" {
        Assert-Exists $configFile
        Assert-Exists $sessionFile
        Assert-Missing $engineFile
        Assert-Missing $engineDir
        Assert-Missing $lockFile
        Assert-Missing (Join-Path $LocalDataDir "webview")
        Assert-Missing $legacyWebView
        Assert-Missing $legacyWebViewCleanup
        Assert-Missing $InstallDir
    }
    "purge" {
        Assert-Missing $ConfigDir
        Assert-Missing $LocalDataDir
        Assert-Missing $legacyWebView
        Assert-Missing $legacyWebViewCleanup
        Assert-Missing $InstallDir
    }
    "clean" {
        Assert-Missing $ConfigDir
        Assert-Missing $LocalDataDir
        Assert-Missing $legacyWebView
        Assert-Missing $legacyWebViewCleanup
        Assert-Missing $InstallDir
    }
}

Assert-RegistryValueMissing $runKey "Conduit"
Assert-RegistryValueMissing $startupRunKey "Conduit"
Assert-RegistryValueMissing $startupRun32Key "Conduit"
if (Test-Path -LiteralPath $uninstallKey) {
    $failures.Add("unexpected registry key: $uninstallKey")
}

Assert-NoProcessPath "aria2c.exe" (Join-Path $ConfigDir "bin")
Assert-NoProcessPath "aria2c.exe" (Join-Path $InstallDir "engine")
Assert-NoProcessPath "Conduit.exe" $InstallDir

if ($failures.Count -gt 0) {
    Write-Host "Residue audit FAILED ($Expect):" -ForegroundColor Red
    foreach ($failure in $failures) {
        Write-Host "  - $failure" -ForegroundColor Red
    }
    exit 1
}

Write-Host "Residue audit PASSED ($Expect)." -ForegroundColor Green
