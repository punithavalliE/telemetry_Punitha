#!/usr/bin/env powershell
<#
.SYNOPSIS
    Quick one-liner deployment of Telemetry Stack

.DESCRIPTION
    Minimal script for rapid deployment with sensible defaults.
    For advanced options, use deploy-telemetry-stack.ps1
#>

Write-Host "🚀 Quick Telemetry Stack Deployment" -ForegroundColor Magenta
Write-Host "===================================" -ForegroundColor Magenta

# Check prerequisites
$commands = @("kubectl", "helm", "docker")
foreach ($cmd in $commands) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        Write-Host "❌ $cmd not found. Please install $cmd first." -ForegroundColor Red
        exit 1
    }
}

# Test cluster connectivity
try {
    kubectl cluster-info | Out-Null
    Write-Host "✅ Kubernetes cluster accessible" -ForegroundColor Green
} catch {
    Write-Host "❌ Cannot connect to Kubernetes cluster" -ForegroundColor Red
    exit 1
}

Write-Host "🔧 Building images and deploying..." -ForegroundColor Yellow

# Build images
$images = @("api", "collector", "msg-queue", "msg-queue-proxy", "streamer")
foreach ($img in $images) {
    Write-Host "Building $img..." -ForegroundColor Blue
    docker build -t $img -f "services/$($img.Replace('-', '_'))/Dockerfile" . | Out-Null
}

# Deploy with Helm
kubectl create namespace telemetry --dry-run=client -o yaml | kubectl apply -f - | Out-Null
helm upgrade --install telemetry-stack ./helm/telemetry-stack --namespace telemetry --wait --timeout 600s

Write-Host "`n✅ Deployment Complete!" -ForegroundColor Green
Write-Host "Access Grafana: kubectl port-forward svc/telemetry-stack-grafana 3000:3000 -n telemetry" -ForegroundColor Cyan
Write-Host "Then open: http://localhost:3000 (admin/admin123)" -ForegroundColor Gray