$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$setup = Join-Path $env:TEMP ("MicrosoftEdgeWebview2Setup-conduit-" + [System.Guid]::NewGuid().ToString("N") + ".exe")
try {
    Invoke-WebRequest -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile $setup
    $proc = Start-Process -FilePath $setup -ArgumentList "/silent","/install" -Wait -PassThru
    exit $proc.ExitCode
}
finally {
    Remove-Item -LiteralPath $setup -Force -ErrorAction SilentlyContinue
}
