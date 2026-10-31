#requires -Version 5.1
<#
.SYNOPSIS
    Fetch the pinned aria2c binary for development and tests.

.DESCRIPTION
    Conduit does not ship aria2c. The installer downloads it on Windows, and
    Linux installs get it from apt. Tests and local runs use a copy in
    .cache\aria2\<platform>\, which this script fills and verifies. It prints the
    path. Set CONDUIT_ARIA2C to that path to run the engine tests.

.PARAMETER Platform
    Windows (default) or Linux.
#>
[CmdletBinding()]
param(
    [ValidateSet('Windows', 'Linux')]
    [string]$Platform = 'Windows'
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Get-Sha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

$Root = Split-Path -Parent $PSScriptRoot
$EnvFile = Join-Path $PSScriptRoot 'aria2_binaries.env'
if (-not (Test-Path -LiteralPath $EnvFile)) {
    throw "Missing $EnvFile"
}

$settings = @{}
foreach ($line in Get-Content -LiteralPath $EnvFile) {
    if ($line -match '^\s*([A-Z0-9_]+)\s*=\s*"?([^"]*)"?\s*$') {
        $settings[$matches[1]] = $matches[2]
    }
}

if ($Platform -eq 'Windows') {
    $url = $settings['ARIA2_WIN_URL']
    $expected = $settings['ARIA2_WIN_SHA256'].ToLowerInvariant()
    $name = 'aria2c.exe'
} else {
    $url = $settings['ARIA2_LINUX_URL']
    $expected = $settings['ARIA2_LINUX_SHA256'].ToLowerInvariant()
    $name = 'aria2c'
}

$OutDir = Join-Path $Root (Join-Path '.cache\aria2' $Platform.ToLowerInvariant())
$OutPath = Join-Path $OutDir $name
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

if ((Test-Path -LiteralPath $OutPath) -and ((Get-Sha256 $OutPath) -eq $expected)) {
    Write-Host "Already present and verified: $OutPath"
    Write-Output $OutPath
    exit 0
}

$work = Join-Path ([System.IO.Path]::GetTempPath()) ('conduit-aria2-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
try {
    $archive = Join-Path $work 'download.zip'
    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $archive -UseBasicParsing

    $extract = Join-Path $work 'extracted'
    Expand-Archive -LiteralPath $archive -DestinationPath $extract -Force
    $found = Get-ChildItem -LiteralPath $extract -Recurse -File | Where-Object { $_.Name -eq $name } | Select-Object -First 1
    if (-not $found) {
        throw "The downloaded archive does not contain $name."
    }
    if ((Get-Sha256 $found.FullName) -ne $expected) {
        throw "$name failed its SHA-256 check."
    }

    Copy-Item -LiteralPath $found.FullName -Destination $OutPath -Force
    Write-Output $OutPath
} finally {
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}
