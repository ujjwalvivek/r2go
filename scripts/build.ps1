$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

function Assert-LastExitCode([string]$Step) {
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $Step (exit code $LASTEXITCODE)" -ForegroundColor Red
        exit $LASTEXITCODE
    }
}

New-Item -ItemType Directory -Force -Path "dist" | Out-Null

# Embed the native Windows executable icon when MinGW's windres is available.
# Fyne already needs a C toolchain on Windows, and most MinGW installs include it.
$Syso = Join-Path $Root "cmd\r2go\r2go_windows.syso"
Remove-Item $Syso -ErrorAction SilentlyContinue
$Windres = Get-Command windres -ErrorAction SilentlyContinue
if ($null -ne $Windres) {
    Write-Host "==> windows icon"
    Push-Location (Join-Path $Root "assets")
    & $Windres.Source -i "r2go.rc" -o $Syso -O coff
    $Code = $LASTEXITCODE
    Pop-Location
    if ($Code -ne 0) {
        Write-Host "WARNING: native icon resource generation failed; runtime icon will still be used." -ForegroundColor Yellow
        Remove-Item $Syso -ErrorAction SilentlyContinue
    }
}

Write-Host "==> tests"
go test ./...
Assert-LastExitCode "tests"

Write-Host "==> vet"
go vet ./...
Assert-LastExitCode "vet"

Write-Host "==> build"
# windowsgui prevents the finished desktop app from opening a console window.
go build -v -trimpath -ldflags "-H=windowsgui -s -w" -o "dist/r2go.exe" ./cmd/r2go
Assert-LastExitCode "build"

Write-Host "Built: $Root/dist/r2go.exe" -ForegroundColor Green
