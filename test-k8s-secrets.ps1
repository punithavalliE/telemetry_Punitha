# Test Kubernetes Secrets for Telemetry API
# This script helps validate that secrets are properly configured

param(
    [string]$Namespace = "default",
    [string]$ReleaseName = "my-telemetry",
    [string]$ApiUrl = "http://localhost:8080"
)

Write-Host "=== Telemetry Kubernetes Secrets Test ===" -ForegroundColor Green
Write-Host "Namespace: $Namespace"
Write-Host "Release: $ReleaseName"
Write-Host "API URL: $ApiUrl"
Write-Host ""

# Calculate secret name
$secretName = "$ReleaseName-telemetry-stack-auth"

try {
    # Check if secret exists
    Write-Host "1. Checking if secret exists..." -ForegroundColor Yellow
    $secretExists = kubectl get secret $secretName -n $Namespace 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Secret '$secretName' exists" -ForegroundColor Green
    } else {
        Write-Host "❌ Secret '$secretName' not found" -ForegroundColor Red
        exit 1
    }

    # Get API key from secret
    Write-Host "2. Retrieving API key from secret..." -ForegroundColor Yellow
    $apiKeyBase64 = kubectl get secret $secretName -n $Namespace -o jsonpath='{.data.api-key}' 2>$null
    if ($apiKeyBase64) {
        $apiKey = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($apiKeyBase64))
        Write-Host "✅ API key retrieved (length: $($apiKey.Length) characters)" -ForegroundColor Green
    } else {
        Write-Host "❌ Failed to retrieve API key" -ForegroundColor Red
        exit 1
    }

    # Get service token from secret
    Write-Host "3. Retrieving service token from secret..." -ForegroundColor Yellow
    $serviceTokenBase64 = kubectl get secret $secretName -n $Namespace -o jsonpath='{.data.service-token}' 2>$null
    if ($serviceTokenBase64) {
        $serviceToken = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($serviceTokenBase64))
        Write-Host "✅ Service token retrieved (length: $($serviceToken.Length) characters)" -ForegroundColor Green
    } else {
        Write-Host "❌ Failed to retrieve service token" -ForegroundColor Red
        exit 1
    }

    # Test health endpoint (no auth required)
    Write-Host "4. Testing health endpoint (no auth)..." -ForegroundColor Yellow
    try {
        $healthResponse = Invoke-RestMethod -Uri "$ApiUrl/health" -Method GET -TimeoutSec 5
        Write-Host "✅ Health endpoint accessible: $healthResponse" -ForegroundColor Green
    } catch {
        Write-Host "❌ Health endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
    }

    # Test authenticated endpoint with API key
    Write-Host "5. Testing authenticated endpoint with API key..." -ForegroundColor Yellow
    try {
        $headers = @{ "X-API-Key" = $apiKey }
        $gpuResponse = Invoke-RestMethod -Uri "$ApiUrl/api/v1/gpus" -Method GET -Headers $headers -TimeoutSec 5
        Write-Host "✅ Authenticated request successful" -ForegroundColor Green
        Write-Host "   Response: $($gpuResponse | ConvertTo-Json -Depth 1 -Compress)"
    } catch {
        if ($_.Exception.Response.StatusCode -eq 401) {
            Write-Host "❌ Authentication failed - check API key" -ForegroundColor Red
        } else {
            Write-Host "❌ Request failed: $($_.Exception.Message)" -ForegroundColor Red
        }
    }

    # Test with wrong API key (should fail)
    Write-Host "6. Testing with wrong API key (should fail)..." -ForegroundColor Yellow
    try {
        $wrongHeaders = @{ "X-API-Key" = "wrong-key" }
        $response = Invoke-RestMethod -Uri "$ApiUrl/api/v1/gpus" -Method GET -Headers $wrongHeaders -TimeoutSec 5
        Write-Host "❌ Wrong API key was accepted (security issue!)" -ForegroundColor Red
    } catch {
        if ($_.Exception.Response.StatusCode -eq 401) {
            Write-Host "✅ Wrong API key correctly rejected" -ForegroundColor Green
        } else {
            Write-Host "⚠️  Unexpected error: $($_.Exception.Message)" -ForegroundColor Yellow
        }
    }

    Write-Host ""
    Write-Host "=== Summary ===" -ForegroundColor Green
    Write-Host "Secret Name: $secretName"
    Write-Host "API Key Length: $($apiKey.Length) characters"
    Write-Host "Service Token Length: $($serviceToken.Length) characters"
    Write-Host ""
    Write-Host "To use the API key:"
    Write-Host "curl -H 'X-API-Key: $apiKey' $ApiUrl/api/v1/gpus"

} catch {
    Write-Host "❌ Script failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✅ Kubernetes secrets test completed successfully!" -ForegroundColor Green