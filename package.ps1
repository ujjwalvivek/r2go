# Produces a release zip: r2go.exe + notes.
param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

if (-not $Version) {
    $Version = Get-Date -Format "yyyyMMdd"
}

& .\build.ps1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$name = "r2go-$Version-windows-x64"
$dir = Join-Path $env:TEMP $name
if (Test-Path $dir) { Remove-Item -Recurse -Force $dir }
New-Item -ItemType Directory -Path $dir | Out-Null
Copy-Item .\r2go.exe $dir
Set-Content -Path (Join-Path $dir "README.txt") -Value @"
r2go - Cloudflare R2 file explorer ($Version)

Requires:
- Windows 10/11 64-bit
- (Optional, for the "Mount as drive" feature) WinFsp:
  winget install WinFsp.WinFsp

Run r2go.exe, then add an account (button in the sidebar):
Name, Cloudflare Account ID, R2 API Token Access Key ID and Secret.
"@

$zip = Join-Path $env:TEMP "$name.zip"
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path $dir -DestinationPath $zip
$hash = Get-FileHash $zip -Algorithm SHA256
Write-Host "Built: $zip"
Write-Host "SHA256: $($hash.Hash)"