#requires -Version 5.1
<#
.SYNOPSIS
    Fetch the pinned aria2c.exe for the Windows integration tests.

.DESCRIPTION
    Conduit does not ship aria2c. The Windows installer downloads the pinned
    build, and Linux uses the aria2 package from apt. This script fills
    .cache\aria2\windows\ with the pinned copy for local Windows test runs,
    verifies its SHA-256, and prints the path. Set CONDUIT_ARIA2C to that path
    to run the engine tests.
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Get-Sha256([string]$Path) {
    return ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash([System.IO.File]::ReadAllBytes($Path))) -replace '-', '').ToLowerInvariant()
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
$url = $settings['ARIA2_WIN_URL']
$expected = $settings['ARIA2_WIN_SHA256'].ToLowerInvariant()
$name = 'aria2c.exe'

$OutDir = Join-Path $Root '.cache\aria2\windows'
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
