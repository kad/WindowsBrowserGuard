# Build script for Windows Browser Guard
# Builds the main executable

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Windows Browser Guard - Build Script" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Clean old builds
Write-Host "[1/2] Cleaning old builds..." -ForegroundColor Yellow
Remove-Item -Path "WindowsBrowserGuard.exe" -ErrorAction SilentlyContinue
Remove-Item -Path "cmd\WindowsBrowserGuard\WindowsBrowserGuard.exe" -ErrorAction SilentlyContinue

# Build main application
Write-Host "[2/2] Building WindowsBrowserGuard..." -ForegroundColor Yellow
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
Write-Host "Dry-run mode:" -ForegroundColor Yellow
Write-Host "  • Runs without requiring Administrator privileges" -ForegroundColor Gray
Write-Host "  • Detects all extension policies" -ForegroundColor Gray
Write-Host "  • Watches for registry changes in real-time" -ForegroundColor Gray
Write-Host "  • Shows planned operations without executing them" -ForegroundColor Gray
Write-Host "  • Perfect for testing and validation" -ForegroundColor Gray
Write-Host ""
