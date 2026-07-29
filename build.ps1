# Build script for r2go on Windows.
# Locates WinFsp FUSE headers for cgofuse automatically and builds the app.
param(
    [switch]$Run
)

$ErrorActionPreference = "Stop"

function Find-WinFspHeaders {
    $candidates = @(
        "$env:ProgramFiles(x86)\WinFsp\inc\fuse",
        "$env:ProgramFiles\WinFsp\inc\fuse"
    )
    if ($env:WINFSF_FUSE) { $candidates = @($env:WINFSF_FUSE) + $candidates }
    foreach ($c in $candidates) {
        if (Test-Path (Join-Path $c "fuse.h")) { return $c }
    }
    return $null
}

$headers = Find-WinFspHeaders
if (-not $headers) {
    Write-Warning "WinFsp FUSE headers not found. cgofuse (mount feature) needs them."
    Write-Warning "Install WinFsp (with Developer files) or extract the MSI, then set the"
    Write-Warning "WINFSF_FUSE env var to the 'inc\fuse' folder."
    Write-Warning "Extract without installing:  msiexec /a winfsp.msi /qn TARGETDIR=C:\winfsp"
    Write-Warning "Then: \$env:WINFSF_FUSE = 'C:\winfsp\DYNAMIC\inc\fuse'"
    exit 1
}
Write-Host "Using WinFsp headers from: $headers"
$env:CPATH = $headers

go build -o r2go.exe ./cmd/r2go
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($Run) {
    .\r2go.exe
}