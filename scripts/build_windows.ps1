#requires -Version 5.1
<#
.SYNOPSIS
    Quick Windows build for Conduit: native app, license bundle, extension zip, and NSIS installer.

.DESCRIPTION
    Requires Go (the module pins 1.27.2; GOTOOLCHAIN=auto can fetch it), the
    Wails v2 CLI, and NSIS 3. Zip files are built with .NET so Windows
    Explorer can open them. Do not use tar -a on '.' — that stores ./
    prefixes and Explorer shows the archive as empty.

    Outputs:
      dist\Conduit-windows-amd64-standalone.exe
      dist\Conduit-windows-amd64-standalone.zip
      dist\conduit-extension.zip
      dist\Conduit-windows-amd64-setup.exe
#>
[CmdletBinding()]
param()

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
$DesktopDir = Join-Path $Root 'desktop'
$ExtensionDir = Join-Path $Root 'extension'
$DistDir = Join-Path $Root 'dist'
$NsiScript = Join-Path $Root 'packaging\windows\conduit.nsi'

$StandaloneOut = Join-Path $DistDir 'Conduit-windows-amd64-standalone.exe'
$StandaloneZip = Join-Path $DistDir 'Conduit-windows-amd64-standalone.zip'
$ExtensionOut = Join-Path $DistDir 'conduit-extension.zip'
$InstallerOut = Join-Path $DistDir 'Conduit-windows-amd64-setup.exe'

function Copy-LicenseBundle {
    param([Parameter(Mandatory = $true)][string]$Destination)
    foreach ($name in @('LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'SOURCE_OFFER.md')) {
        Copy-Item -LiteralPath (Join-Path $Root $name) -Destination $Destination -Force
    }
    Copy-Item -LiteralPath (Join-Path $Root 'licenses') -Destination $Destination -Recurse -Force
}

function New-ReleaseZip {
    param(
        [Parameter(Mandatory = $true)][string]$SourceDirectory,
        [Parameter(Mandatory = $true)][string]$DestinationZip
    )
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    if (Test-Path -LiteralPath $DestinationZip) {
        Remove-Item -LiteralPath $DestinationZip -Force
    }
    $sourceRoot = (Resolve-Path -LiteralPath $SourceDirectory).Path.TrimEnd('\')
    $archive = [System.IO.Compression.ZipFile]::Open($DestinationZip, 'Create')
    try {
        Get-ChildItem -LiteralPath $sourceRoot -Recurse -File | ForEach-Object {
            $relative = $_.FullName.Substring($sourceRoot.Length + 1).Replace('\', '/')
            [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
                $archive,
                $_.FullName,
                $relative,
                [System.IO.Compression.CompressionLevel]::Optimal
            )
        }
    } finally {
        $archive.Dispose()
    }
    if (-not (Test-Path -LiteralPath $DestinationZip)) {
        Fail "The zip was not created: $DestinationZip"
    }
}

function Assert-ZipUsable {
    param(
        [Parameter(Mandatory = $true)][string]$ZipPath,
        [Parameter(Mandatory = $true)][string[]]$RequiredEntries
    )
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        $names = @($archive.Entries | ForEach-Object { $_.FullName.Replace('\', '/') })
        if ($names.Count -eq 0) {
            Fail "The zip is empty: $ZipPath"
        }
        foreach ($name in $names) {
            if ($name.StartsWith('./')) {
                Fail "Zip member '$name' uses a ./ prefix; Windows Explorer shows this archive as empty: $ZipPath"
            }
        }
        foreach ($required in $RequiredEntries) {
            if ($names -notcontains $required) {
                Fail "Zip $ZipPath is missing required member $required"
            }
        }
    } finally {
        $archive.Dispose()
    }
}

Push-Location $Root
$failed = $false
try {
    if (-not (Test-Path (Join-Path $DesktopDir 'wails.json'))) {
        Fail "This script must live inside the Conduit repository (desktop/wails.json was not found)."
    }

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Fail "go was not found on PATH. Install Go 1.27.2+ and reopen the terminal."
    }

    $wailsCommand = Get-Command wails -ErrorAction SilentlyContinue
    if ($wailsCommand) {
        $Wails = $wailsCommand.Source
    } else {
        $candidate = Join-Path $env:USERPROFILE 'go\bin\wails.exe'
        if (Test-Path $candidate) {
            $Wails = $candidate
        } else {
            Fail "wails was not found. Run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0"
        }
    }

    $makensisCommand = Get-Command makensis -ErrorAction SilentlyContinue
    if ($makensisCommand) {
        $Makensis = $makensisCommand.Source
    } else {
        $Makensis = $null
        foreach ($candidate in @(
                'C:\Program Files (x86)\NSIS\makensis.exe',
                'C:\Program Files\NSIS\makensis.exe'
            )) {
            if (Test-Path $candidate) {
                $Makensis = $candidate
                break
            }
        }
        if (-not $Makensis) {
            Fail "makensis was not found. Install NSIS 3 from https://nsis.sourceforge.io/."
        }
    }

    New-Item -ItemType Directory -Force $DistDir | Out-Null
    Remove-Item -LiteralPath $StandaloneOut, $StandaloneZip, $ExtensionOut, $InstallerOut -Force -ErrorAction SilentlyContinue

    # daemon/version/VERSION is the only place the version is written. The
    # release tool fails here if wails.json or the extension manifest drifted.
    Write-Host ''
    Write-Host '==> Checking version fields' -ForegroundColor Cyan
    Push-Location (Join-Path $Root 'daemon')
    try {
        Invoke-Native -FilePath 'go' -Arguments @('run', './tools/release', 'check')
        $Version = (& go run ./tools/release print).Trim()
        $VersionQuad = (& go run ./tools/release print -quad).Trim()
    } finally {
        Pop-Location
    }
    if (-not $Version -or -not $VersionQuad) {
        Fail 'Could not read daemon/version/VERSION.'
    }
    # CI sets GITHUB_SHA; a local checkout asks git. Builds outside git stay "dev".
    $Commit = $env:GITHUB_SHA
    if (-not $Commit -and (Get-Command git -ErrorAction SilentlyContinue)) {
        $Commit = (& git -C $Root rev-parse HEAD 2>$null)
    }
    if (-not $Commit) {
        $Commit = 'dev'
    }
    $Commit = "$Commit".Trim()
    $LdFlags = "-X conduit/daemon/version.Commit=$Commit"
    Write-Host "    Conduit $Version ($Commit)"

    Write-Host ''
    Write-Host '==> Building native Conduit app' -ForegroundColor Cyan
    Push-Location $DesktopDir
    try {
        Invoke-Native -FilePath $Wails -Arguments @(
            'build',
            '-skipbindings',
            '-skipembedcreate',
            '-platform', 'windows/amd64',
            '-ldflags', $LdFlags
        )
    } finally {
        Pop-Location
    }

    $builtApp = Join-Path $DesktopDir 'build\bin\Conduit.exe'
    if (-not (Test-Path $builtApp)) {
        Fail "Wails reported success but did not produce $builtApp"
    }
    Copy-Item -LiteralPath $builtApp -Destination $StandaloneOut -Force

    Write-Host ''
    Write-Host '==> Packaging standalone app with license bundle' -ForegroundColor Cyan
    $stage = Join-Path ([System.IO.Path]::GetTempPath()) ("conduit-standalone-" + [guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Force -Path $stage | Out-Null
        Copy-Item -LiteralPath $StandaloneOut -Destination (Join-Path $stage 'Conduit.exe') -Force
        Copy-LicenseBundle -Destination $stage
        New-ReleaseZip -SourceDirectory $stage -DestinationZip $StandaloneZip
        Assert-ZipUsable -ZipPath $StandaloneZip -RequiredEntries @('Conduit.exe', 'LICENSE', 'NOTICE')
    } finally {
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host ''
    Write-Host '==> Packaging browser extension' -ForegroundColor Cyan
    $extStage = Join-Path ([System.IO.Path]::GetTempPath()) ("conduit-extension-" + [guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Force -Path $extStage | Out-Null
        Get-ChildItem -LiteralPath $ExtensionDir -Force | Where-Object {
            $_.Name -notin @('test', '.DS_Store', 'Thumbs.db')
        } | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $extStage $_.Name) -Recurse -Force
        }
        New-ReleaseZip -SourceDirectory $extStage -DestinationZip $ExtensionOut
        Assert-ZipUsable -ZipPath $ExtensionOut -RequiredEntries @('manifest.json', 'background.js', 'content.js')
    } finally {
        Remove-Item -LiteralPath $extStage -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host ''
    Write-Host '==> Building NSIS installer' -ForegroundColor Cyan
    # The installer downloads the aria2 engine, so it needs the pinned URL and hash.
    $pins = @{}
    foreach ($line in Get-Content -LiteralPath (Join-Path $Root 'scripts\aria2_binaries.env')) {
        if ($line -match '^\s*([A-Z0-9_]+)\s*=\s*"?([^"]*)"?\s*$') {
            $pins[$matches[1]] = $matches[2]
        }
    }
    foreach ($key in @('ARIA2_WIN_URL', 'ARIA2_WIN_SHA256')) {
        if (-not $pins.ContainsKey($key) -or [string]::IsNullOrWhiteSpace($pins[$key])) {
            Fail "Missing $key in scripts\aria2_binaries.env"
        }
    }
    Invoke-Native -FilePath $Makensis -Arguments @(
        "/DVERSION=$Version",
        "/DVERSION_QUAD=$VersionQuad",
        "/DARIA2_WIN_URL=$($pins['ARIA2_WIN_URL'])",
        "/DARIA2_WIN_SHA256=$($pins['ARIA2_WIN_SHA256'])",
        $NsiScript
    )
    if (-not (Test-Path $InstallerOut)) {
        Fail "NSIS reported success but did not produce $InstallerOut"
    }

    Write-Host ''
    Write-Host 'Build complete:' -ForegroundColor Green
    foreach ($file in @($StandaloneOut, $StandaloneZip, $ExtensionOut, $InstallerOut)) {
        $item = Get-Item -LiteralPath $file
        Write-Host ("  {0,-55} {1,8:N2} MB" -f $item.FullName, ($item.Length / 1MB))
    }
    Write-Host ''
} catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    # In GitHub Actions, also surface the reason as an annotation on the run.
    if ($env:GITHUB_ACTIONS) { Write-Host "::error file=$($MyInvocation.ScriptName)::$($_.Exception.Message)" }
    $failed = $true
} finally {
    Pop-Location
}

if ($failed) {
    exit 1
}
