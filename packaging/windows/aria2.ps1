#requires -Version 5.1
<#
.SYNOPSIS
    Installs the aria2c download engine that Conduit runs.

.DESCRIPTION
    Downloads the pinned aria2 release archive, extracts aria2c.exe, checks its
    SHA-256, and copies it to the destination. A copy that already has the
    expected hash is left in place, so upgrades work offline. Exits 0 on
    success and 1 on any failure.

.PARAMETER Url
    The aria2 release archive to download.

.PARAMETER Sha256
    The expected SHA-256 of aria2c.exe.

.PARAMETER Destination
    Where to put aria2c.exe.
#>
param(
    [Parameter(Mandatory = $true)][string]$Url,
    [Parameter(Mandatory = $true)][string]$Sha256,
    [Parameter(Mandatory = $true)][string]$Destination
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$expected = $Sha256.ToLowerInvariant()

function Get-FileSha256([string]$Path) {
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

$status = 1
$work = $null
try {
    if ((Test-Path -LiteralPath $Destination) -and ((Get-FileSha256 $Destination) -eq $expected)) {
        Write-Host 'aria2c.exe is already installed and verified.'
        $status = 0
    } else {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $work = Join-Path ([System.IO.Path]::GetTempPath()) ('conduit-aria2-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $work | Out-Null

        $zip = Join-Path $work 'aria2.zip'
        Invoke-WebRequest -Uri $Url -OutFile $zip -UseBasicParsing

        $extract = Join-Path $work 'extracted'
        Expand-Archive -LiteralPath $zip -DestinationPath $extract -Force
        $found = Get-ChildItem -LiteralPath $extract -Recurse -File -Filter 'aria2c.exe' | Select-Object -First 1
        if (-not $found) {
            throw 'the downloaded archive does not contain aria2c.exe'
        }
        if ((Get-FileSha256 $found.FullName) -ne $expected) {
            throw 'aria2c.exe failed its SHA-256 check'
        }

        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
        Copy-Item -LiteralPath $found.FullName -Destination $Destination -Force
        Unblock-File -LiteralPath $Destination -ErrorAction SilentlyContinue
        Write-Host "Installed $Destination"
        $status = 0
    }
} catch {
    Write-Host "aria2 install failed: $($_.Exception.Message)"
} finally {
    if ($work) {
        Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
    }
}
exit $status
