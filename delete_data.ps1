# PowerShell script to delete data from InfluxDB telem_bucket
param(
    [Parameter(Mandatory=$true, Position=0)]
    [ValidateSet("all", "telemetry", "device")]
    [string]$Action,
    
    [Parameter(Position=1)]
    [string]$DeviceID = ""
)

# Set environment variables for InfluxDB connection
$env:INFLUXDB_URL = "http://localhost:8086"
$env:INFLUXDB_TOKEN = "supersecrettoken"
$env:INFLUXDB_ORG = "telemetryorg"
$env:INFLUXDB_BUCKET = "telem_bucket"

Write-Host "🗑️  InfluxDB Data Deletion Utility" -ForegroundColor Cyan
Write-Host "=================================" -ForegroundColor Cyan
Write-Host ""

# Check if we need to port-forward to InfluxDB (if running in Kubernetes)
$portForwardRunning = Get-Process -Name "kubectl" -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -eq "kubectl" }
if (-not $portForwardRunning) {
    Write-Host "⚠️  No kubectl port-forward detected. If running in Kubernetes, you may need to run:" -ForegroundColor Yellow
    Write-Host "   kubectl port-forward service/influxdb 8086:8086" -ForegroundColor Yellow
    Write-Host ""
}

# Build and run the delete command
$currentDir = Get-Location
Set-Location "$PSScriptRoot\.."

try {
    switch ($Action) {
        "all" {
            Write-Host "⚠️  WARNING: This will delete ALL data from the telem_bucket!" -ForegroundColor Red
            $confirm = Read-Host "Are you sure you want to continue? (y/N)"
            if ($confirm -ne "y" -and $confirm -ne "Y") {
                Write-Host "❌ Operation cancelled." -ForegroundColor Yellow
                exit 1
            }
            Write-Host "🗑️  Deleting all data..." -ForegroundColor Red
            go run cmd/delete_data/main.go all
        }
        "telemetry" {
            Write-Host "🗑️  Deleting telemetry measurement data..." -ForegroundColor Yellow
            go run cmd/delete_data/main.go telemetry
        }
        "device" {
            if ([string]::IsNullOrEmpty($DeviceID)) {
                Write-Host "❌ Device ID is required for device deletion." -ForegroundColor Red
                Write-Host "Usage: .\delete_data.ps1 device <DeviceID>" -ForegroundColor Yellow
                exit 1
            }
            Write-Host "🗑️  Deleting data for device: $DeviceID..." -ForegroundColor Yellow
            go run cmd/delete_data/main.go device $DeviceID
        }
    }
} catch {
    Write-Host "❌ Error occurred: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    Set-Location $currentDir
}

Write-Host ""
Write-Host "✨ Done!" -ForegroundColor Green