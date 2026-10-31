#requires -Version 5.1
<#
.SYNOPSIS
    Installs the aria2c download engine that Conduit runs.

.DESCRIPTION
    Downloads the pinned aria2 release archive, extracts aria2c.exe, checks its
    SHA-256, and copies it to the destination. A copy that already has the
    expected hash is left in place, so upgrades work offline. Exits 0 on
    success and 1 on any failure. Each step is also written to
    %TEMP%\conduit-aria2-install.log, because the installer does not show this
    script's output.

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
$LogPath = Join-Path ([System.IO.Path]::GetTempPath()) 'conduit-aria2-install.log'

function Write-Log([string]$Message) {
    $line = '{0} {1}' -f (Get-Date -Format 's'), $Message
    Write-Host $line
    Add-Content -LiteralPath $LogPath -Value $line -Encoding utf8 -ErrorAction SilentlyContinue
}

function Get-FileSha256([string]$Path) {
    return ([BitConverter]::ToString([System.Security.Cryptography.SHA256]::Create().ComputeHash([System.IO.File]::ReadAllBytes($Path))) -replace '-', '').ToLowerInvariant()
}

Set-Content -LiteralPath $LogPath -Value '' -Encoding utf8 -ErrorAction SilentlyContinue
Write-Log "aria2 install to $Destination from $Url"
Write-Log "PowerShell $($PSVersionTable.PSVersion) $($PSVersionTable.PSEdition), PSModulePath=$env:PSModulePath"

# A setup started from PowerShell 7 inherits its PSModulePath, which lists
# PowerShell 7's own modules first. Windows PowerShell then autoloads the
# PowerShell 7 Microsoft.PowerShell.Utility, which it cannot use, and loses the
# script functions (Get-FileHash, New-Guid, ...) of its own copy. Drop the
# PowerShell 7 folders and put this PowerShell's modules first.
if ($PSVersionTable.PSEdition -ne 'Core') {
    $kept = @($env:PSModulePath -split ';' | Where-Object { $_ -and $_ -notmatch '\\PowerShell\\' })
    $env:PSModulePath = ((@(Join-Path $PSHOME 'Modules') + $kept) | Select-Object -Unique) -join ';'
}

$status = 1
$work = $null
try {
    if ((Test-Path -LiteralPath $Destination) -and ((Get-FileSha256 $Destination) -eq $expected)) {
        Write-Log 'aria2c.exe is already installed and verified.'
        $status = 0
    } else {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $work = Join-Path ([System.IO.Path]::GetTempPath()) ('conduit-aria2-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $work | Out-Null

        $zip = Join-Path $work 'aria2.zip'
        # A dropped connection should not fail the install, so try three times.
        for ($attempt = 1; $attempt -le 3; $attempt++) {
            try {
                Invoke-WebRequest -Uri $Url -OutFile $zip -UseBasicParsing
                break
            } catch {
                Write-Log "download attempt $attempt failed: $($_.Exception.Message)"
                if ($attempt -eq 3) { throw }
                Start-Sleep -Seconds (5 * $attempt)
            }
        }
        Write-Log 'downloaded the archive'

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
        Write-Log "installed $Destination"
        $status = 0
    }
} catch {
    Write-Log "aria2 install failed: $($_.Exception.Message)"
} finally {
    if ($work) {
        Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
    }
}
exit $status
