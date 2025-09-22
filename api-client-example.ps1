# PowerShell API Client Example

# API Endpoint and Secret Configuration
$API_BASE_URL = "http://localhost:8080"
$API_SECRET = "your-secret-key-here"

Write-Host "=== Telemetry API Client Example ===" -ForegroundColor Green
Write-Host "Base URL: $API_BASE_URL"
Write-Host "Authentication: Using API key"
Write-Host ""

# Health check (no auth required)
Write-Host "1. Health Check (no auth):" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$API_BASE_URL/health" -Method GET
    Write-Host $response
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test authenticated endpoints with X-API-Key header
Write-Host "2. Get GPUs (with X-API-Key header):" -ForegroundColor Yellow
try {
    $headers = @{ "X-API-Key" = $API_SECRET }
    $response = Invoke-RestMethod -Uri "$API_BASE_URL/api/v1/gpus" -Method GET -Headers $headers
    $response | ConvertTo-Json -Depth 3
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test authenticated endpoints with Authorization Bearer header
Write-Host "3. Get recent telemetry (with Authorization Bearer):" -ForegroundColor Yellow
try {
    $headers = @{ "Authorization" = "Bearer $API_SECRET" }
    $response = Invoke-RestMethod -Uri "$API_BASE_URL/gpus" -Method GET -Headers $headers
    $response | ConvertTo-Json -Depth 3
} catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test unauthorized access
Write-Host "4. Test unauthorized access (no key):" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$API_BASE_URL/api/v1/gpus" -Method GET
    $response | ConvertTo-Json
} catch {
    Write-Host "Expected error - Unauthorized: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "=== Authentication Methods ===" -ForegroundColor Green
Write-Host "Method 1: X-API-Key header"
Write-Host "  `$headers = @{ 'X-API-Key' = 'your-secret' }"
Write-Host "  Invoke-RestMethod -Uri '$API_BASE_URL/api/v1/gpus' -Headers `$headers"
Write-Host ""
Write-Host "Method 2: Authorization Bearer header"
Write-Host "  `$headers = @{ 'Authorization' = 'Bearer your-secret' }"
Write-Host "  Invoke-RestMethod -Uri '$API_BASE_URL/gpus' -Headers `$headers"
Write-Host ""