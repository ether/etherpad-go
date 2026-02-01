# Generate Swagger Documentation
# This script generates the OpenAPI/Swagger documentation from Go annotations

Write-Host "Generating Swagger documentation..." -ForegroundColor Green

# Check if swag is installed
$swagPath = Get-Command swag -ErrorAction SilentlyContinue
if (-not $swagPath) {
    Write-Host "swag CLI not found. Installing..." -ForegroundColor Yellow
    go install github.com/swaggo/swag/cmd/swag@latest
}

# Change to project root
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptDir
Set-Location $projectRoot

Write-Host "Working directory: $projectRoot" -ForegroundColor Cyan

# Generate Swagger
swag init --parseDependency --parseInternal -g main.go -o docs

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "Swagger documentation generated successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Generated files:" -ForegroundColor Cyan
    Write-Host "  - docs/docs.go      (Go embedding for binary)"
    Write-Host "  - docs/swagger.json (OpenAPI 2.0 JSON)"
    Write-Host "  - docs/swagger.yaml (OpenAPI 2.0 YAML)"
    Write-Host ""
    Write-Host "The documentation is automatically embedded in the binary!" -ForegroundColor Green
    Write-Host "Swagger UI available at: http://localhost:9001/swagger/index.html" -ForegroundColor Cyan
} else {
    Write-Host "Error generating Swagger documentation!" -ForegroundColor Red
    exit 1
}
