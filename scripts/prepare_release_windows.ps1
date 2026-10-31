#requires -Version 5.1
<#
.SYNOPSIS
    Stage the Windows release files in dist\release\.

.DESCRIPTION
    Runs scripts\build_windows.ps1 and copies only the upload set into dist\release\.

    The release workflow (.github\workflows\release.yml) runs this script on
    a tag, so a local run is a dry run of exactly what CI publishes. Publish
    from CI, not by uploading these files by hand; see docs\RELEASING.md.

    The version comes from daemon\version\VERSION. Never edit it here.

.PARAMETER Version
    Optional. When given (without a leading v), the script fails unless it
    matches daemon\version\VERSION. CI passes the tag here as a cross-check.

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare_release_windows.ps1
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [string]$Version = ''
)

$ErrorActionPreference = 'Stop'

function Fail([string]$Message) {
    throw $Message
}

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $false)][string[]]$Arguments = @()
    )
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath failed with exit code $LASTEXITCODE"
    }
}

$Root = Split-Path -Parent $PSScriptRoot
$DistDir = Join-Path $Root 'dist'
$ReleaseDir = Join-Path $DistDir 'release'
$VersionFile = Join-Path $Root 'daemon\version\VERSION'

$failed = $false
try {
    $current = (Get-Content -LiteralPath $VersionFile -Raw).Trim()
    $Version = $Version.Trim()
    if ($Version -and $Version -ne $current) {
        Fail "Requested version $Version does not match daemon\version\VERSION ($current). Bump with: cd daemon; go run ./tools/release bump $Version"
    }
    $Version = $current

    if (Test-Path -LiteralPath $ReleaseDir) {
        Remove-Item -LiteralPath $ReleaseDir -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $ReleaseDir | Out-Null

    Write-Host ''
    Write-Host "==> Building Windows artifacts for $Version" -ForegroundColor Cyan
    Invoke-Native -FilePath 'powershell' -Arguments @(
        '-NoProfile',
        '-ExecutionPolicy', 'Bypass',
        '-File', (Join-Path $PSScriptRoot 'build_windows.ps1')
    )

    Write-Host ''
    Write-Host '==> Staging dist\release' -ForegroundColor Cyan
    $built = @(
        'Conduit-windows-amd64-standalone.zip',
        'Conduit-windows-amd64-setup.exe',
        'conduit-extension.zip'
    )
    foreach ($name in $built) {
        $source = Join-Path $DistDir $name
        if (-not (Test-Path -LiteralPath $source)) {
            Fail "Build did not produce $source"
        }
        Copy-Item -LiteralPath $source -Destination (Join-Path $ReleaseDir $name) -Force
    }

    Write-Host ''
    Write-Host "Release folder for ${Version}:" -ForegroundColor Green
    Get-ChildItem -LiteralPath $ReleaseDir -File | Sort-Object Name | ForEach-Object {
        Write-Host ("  {0,-55} {1,8:N2} MB" -f $_.Name, ($_.Length / 1MB))
    }
    Write-Host ''
    Write-Host "  $($ReleaseDir)"
    Write-Host ''
} catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    # In GitHub Actions, also surface the reason as an annotation on the run.
    if ($env:GITHUB_ACTIONS) { Write-Host "::error file=$($MyInvocation.ScriptName)::$($_.Exception.Message)" }
    $failed = $true
}

if ($failed) {
    exit 1
}
