# Build script for Windows Browser Guard
# Builds the main executable

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Windows Browser Guard - Build Script" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Clean old builds
Write-Host "[1/4] Cleaning old builds..." -ForegroundColor Yellow
Remove-Item -Path "WindowsBrowserGuard.exe" -ErrorAction SilentlyContinue
Remove-Item -Path "cmd\WindowsBrowserGuard\WindowsBrowserGuard.exe" -ErrorAction SilentlyContinue

# Vet
Write-Host "[2/4] Running go vet..." -ForegroundColor Yellow
go vet ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ go vet failed!" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ go vet passed" -ForegroundColor Green

# Lint
Write-Host "[3/4] Running golangci-lint..." -ForegroundColor Yellow
$lintCmd = Get-Command golangci-lint -ErrorAction SilentlyContinue
if ($lintCmd) {
    golangci-lint run ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "✗ golangci-lint failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "  ✓ golangci-lint passed" -ForegroundColor Green
} else {
    Write-Host "  ⚠ golangci-lint not found — skipping (install: https://golangci-lint.run/welcome/install/)" -ForegroundColor Yellow
}

# Build main application
Write-Host "[4/4] Building WindowsBrowserGuard..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o WindowsBrowserGuard.exe ./cmd/WindowsBrowserGuard
if ($LASTEXITCODE -ne 0) {
    Write-Host "✗ Build failed!" -ForegroundColor Red
    exit 1
}
Write-Host "  ✓ WindowsBrowserGuard.exe" -ForegroundColor Green

# Summary
Write-Host ""
Write-Host "Build complete!" -ForegroundColor Cyan
Write-Host ""

$size = [math]::Round((Get-Item "WindowsBrowserGuard.exe").Length / 1MB, 2)
Write-Host "Executable created:" -ForegroundColor Cyan
Write-Host "  WindowsBrowserGuard.exe - $size MB" -ForegroundColor White

Write-Host ""
Write-Host "Usage:" -ForegroundColor Cyan
Write-Host "  .\WindowsBrowserGuard.exe           (requires admin - makes changes)" -ForegroundColor White
Write-Host "  .\WindowsBrowserGuard.exe --dry-run (no admin - read-only mode)" -ForegroundColor White
Write-Host ""
