#!/usr/bin/env powershell
<#
.SYNOPSIS
    Deploy Telemetry Stack to Kubernetes Cluster

.DESCRIPTION
    This script deploys the complete telemetry monitoring stack to a fresh Kubernetes cluster.
    It handles Docker image building, Helm chart deployment, and service verification.

.PARAMETER SkipImageBuild
    Skip building Docker images (use if images already exist)

.PARAMETER Namespace
    Kubernetes namespace to deploy to (default: telemetry)

.PARAMETER ReleaseName
    Helm release name (default: telemetry-stack)

.PARAMETER KubeContext
    Kubernetes context to use (optional)

.EXAMPLE
    .\deploy-telemetry-stack.ps1
    Deploy with default settings

.EXAMPLE
    .\deploy-telemetry-stack.ps1 -SkipImageBuild -Namespace monitoring
    Deploy to monitoring namespace without building images
#>

param(
    [switch]$SkipImageBuild,
    [string]$Namespace = "telemetry",
    [string]$ReleaseName = "telemetry-stack",
    [string]$KubeContext = ""
)

# Script configuration
$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$HelmChartPath = Join-Path $ScriptDir "helm\telemetry-stack"

# Colors for output
function Write-Info { 
    param([string]$Message)
    Write-Host "🔵 INFO: $Message" -ForegroundColor Blue
}

function Write-Success { 
    param([string]$Message)
    Write-Host "✅ SUCCESS: $Message" -ForegroundColor Green
}

function Write-Warning { 
    param([string]$Message)
    Write-Host "⚠️  WARNING: $Message" -ForegroundColor Yellow
}

function Write-Error { 
    param([string]$Message)
    Write-Host "❌ ERROR: $Message" -ForegroundColor Red
}

function Write-Step { 
    param([string]$Message)
    Write-Host "`n🚀 STEP: $Message" -ForegroundColor Cyan
    Write-Host "=" * 80 -ForegroundColor Cyan
}

# Function to check if command exists
function Test-Command {
    param([string]$Command)
    return (Get-Command $Command -ErrorAction SilentlyContinue) -ne $null
}

# Function to wait for deployment readiness
function Wait-ForDeployment {
    param(
        [string]$DeploymentName,
        [string]$Namespace,
        [int]$TimeoutSeconds = 300
    )
    
    Write-Info "Waiting for deployment $DeploymentName to be ready..."
    $elapsed = 0
    $interval = 10
    
    while ($elapsed -lt $TimeoutSeconds) {
        try {
            $kubectl_args = @("get", "deployment", $DeploymentName, "-n", $Namespace, "-o", "jsonpath='{.status.readyReplicas}'")
            if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
            
            $ready = & kubectl @kubectl_args 2>$null
            $kubectl_args = @("get", "deployment", $DeploymentName, "-n", $Namespace, "-o", "jsonpath='{.spec.replicas}'")
            if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
            
            $desired = & kubectl @kubectl_args 2>$null
            
            if ($ready -eq $desired -and $ready -gt 0) {
                Write-Success "Deployment $DeploymentName is ready ($ready/$desired replicas)"
                return $true
            }
        }
        catch {
            Write-Warning "Error checking deployment status: $_"
        }
        
        Start-Sleep $interval
        $elapsed += $interval
        Write-Info "Waiting... ($elapsed/$TimeoutSeconds seconds elapsed)"
    }
    
    Write-Error "Timeout waiting for deployment $DeploymentName to be ready"
    return $false
}

# Function to wait for StatefulSet readiness
function Wait-ForStatefulSet {
    param(
        [string]$StatefulSetName,
        [string]$Namespace,
        [int]$TimeoutSeconds = 300
    )
    
    Write-Info "Waiting for StatefulSet $StatefulSetName to be ready..."
    $elapsed = 0
    $interval = 10
    
    while ($elapsed -lt $TimeoutSeconds) {
        try {
            $kubectl_args = @("get", "statefulset", $StatefulSetName, "-n", $Namespace, "-o", "jsonpath='{.status.readyReplicas}'")
            if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
            
            $ready = & kubectl @kubectl_args 2>$null
            $kubectl_args = @("get", "statefulset", $StatefulSetName, "-n", $Namespace, "-o", "jsonpath='{.spec.replicas}'")
            if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
            
            $desired = & kubectl @kubectl_args 2>$null
            
            if ($ready -eq $desired -and $ready -gt 0) {
                Write-Success "StatefulSet $StatefulSetName is ready ($ready/$desired replicas)"
                return $true
            }
        }
        catch {
            Write-Warning "Error checking StatefulSet status: $_"
        }
        
        Start-Sleep $interval
        $elapsed += $interval
        Write-Info "Waiting... ($elapsed/$TimeoutSeconds seconds elapsed)"
    }
    
    Write-Error "Timeout waiting for StatefulSet $StatefulSetName to be ready"
    return $false
}

# Main deployment function
function Deploy-TelemetryStack {
    Write-Step "Starting Telemetry Stack Deployment"
    
    # Check prerequisites
    Write-Step "Checking Prerequisites"
    
    $requiredCommands = @("kubectl", "helm", "docker")
    foreach ($cmd in $requiredCommands) {
        if (-not (Test-Command $cmd)) {
            Write-Error "$cmd is not installed or not in PATH"
            exit 1
        }
        Write-Success "$cmd is available"
    }
    
    # Check Kubernetes connectivity
    Write-Info "Testing Kubernetes connectivity..."
    try {
        $kubectl_args = @("cluster-info")
        if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
        
        $null = & kubectl @kubectl_args
        Write-Success "Kubernetes cluster is accessible"
    }
    catch {
        Write-Error "Cannot connect to Kubernetes cluster: $_"
        exit 1
    }
    
    # Check if Helm chart exists
    if (-not (Test-Path $HelmChartPath)) {
        Write-Error "Helm chart not found at: $HelmChartPath"
        exit 1
    }
    Write-Success "Helm chart found at: $HelmChartPath"
    
    # Create namespace
    Write-Step "Creating Namespace"
    try {
        $kubectl_args = @("create", "namespace", $Namespace, "--dry-run=client", "-o", "yaml")
        if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
        
        & kubectl @kubectl_args | kubectl apply -f -
        Write-Success "Namespace '$Namespace' is ready"
    }
    catch {
        Write-Warning "Namespace creation failed (may already exist): $_"
    }
    
    # Build Docker images (if not skipped)
    if (-not $SkipImageBuild) {
        Write-Step "Building Docker Images"
        
        $images = @(
            @{Name="api"; Tag="api"; Dockerfile="services/api/Dockerfile"},
            @{Name="collector"; Tag="collector"; Dockerfile="services/collector/Dockerfile"},
            @{Name="msg-queue"; Tag="msg-queue"; Dockerfile="services/msg_queue/Dockerfile"},
            @{Name="msg-queue-proxy"; Tag="msg-queue-proxy"; Dockerfile="services/msg_queue_proxy/Dockerfile"},
            @{Name="streamer"; Tag="streamer"; Dockerfile="services/streamer/Dockerfile"}
        )
        
        foreach ($image in $images) {
            Write-Info "Building $($image.Name) image..."
            try {
                & docker build -t $image.Tag -f $image.Dockerfile .
                Write-Success "Built image: $($image.Tag)"
            }
            catch {
                Write-Error "Failed to build image $($image.Name): $_"
                exit 1
            }
        }
        
        Write-Success "All Docker images built successfully"
    }
    else {
        Write-Info "Skipping Docker image build as requested"
    }
    
    # Deploy Helm chart
    Write-Step "Deploying Helm Chart"
    try {
        $helm_args = @(
            "upgrade", "--install", $ReleaseName, $HelmChartPath,
            "--namespace", $Namespace,
            "--create-namespace",
            "--wait",
            "--timeout", "600s"
        )
        
        if ($KubeContext) {
            $helm_args += @("--kube-context", $KubeContext)
        }
        
        Write-Info "Executing: helm $($helm_args -join ' ')"
        & helm @helm_args
        
        Write-Success "Helm chart deployed successfully"
    }
    catch {
        Write-Error "Helm deployment failed: $_"
        
        # Show helm status for debugging
        Write-Info "Checking Helm release status..."
        try {
            $status_args = @("status", $ReleaseName, "--namespace", $Namespace)
            if ($KubeContext) { $status_args += @("--kube-context", $KubeContext) }
            & helm @status_args
        }
        catch {
            Write-Warning "Could not retrieve Helm status: $_"
        }
        
        exit 1
    }
    
    # Wait for deployments to be ready
    Write-Step "Waiting for Services to be Ready"
    
    $deployments = @("api", "collector", "grafana", "prometheus", "msg-queue-proxy", "streamer")
    $statefulsets = @("influxdb", "msg-queue")
    
    # Wait for deployments
    foreach ($deployment in $deployments) {
        $fullName = "$ReleaseName-$deployment"
        if (-not (Wait-ForDeployment -DeploymentName $fullName -Namespace $Namespace)) {
            Write-Warning "Deployment $fullName is not ready, but continuing..."
        }
    }
    
    # Wait for StatefulSets
    foreach ($statefulset in $statefulsets) {
        $fullName = "$ReleaseName-$statefulset"
        if (-not (Wait-ForStatefulSet -StatefulSetName $fullName -Namespace $Namespace)) {
            Write-Warning "StatefulSet $fullName is not ready, but continuing..."
        }
    }
    
    # Display service information
    Write-Step "Service Information"
    
    Write-Info "Getting service endpoints..."
    try {
        $kubectl_args = @("get", "services", "-n", $Namespace)
        if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
        
        & kubectl @kubectl_args
        
        Write-Info "`nGetting pod status..."
        $kubectl_args = @("get", "pods", "-n", $Namespace)
        if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
        
        & kubectl @kubectl_args
    }
    catch {
        Write-Warning "Could not retrieve service information: $_"
    }
    
    # Port forwarding instructions
    Write-Step "Access Instructions"
    
    Write-Info "To access the services, use the following port-forward commands:"
    Write-Host ""
    Write-Host "# Grafana Dashboard (admin/admin123)" -ForegroundColor Yellow
    Write-Host "kubectl port-forward svc/$ReleaseName-grafana 3000:3000 -n $Namespace" -ForegroundColor White
    Write-Host "# Access at: http://localhost:3000" -ForegroundColor Gray
    Write-Host ""
    Write-Host "# Prometheus Metrics" -ForegroundColor Yellow  
    Write-Host "kubectl port-forward svc/$ReleaseName-prometheus 9090:9090 -n $Namespace" -ForegroundColor White
    Write-Host "# Access at: http://localhost:9090" -ForegroundColor Gray
    Write-Host ""
    Write-Host "# InfluxDB" -ForegroundColor Yellow
    Write-Host "kubectl port-forward svc/$ReleaseName-influxdb 8086:8086 -n $Namespace" -ForegroundColor White
    Write-Host "# Access at: http://localhost:8086 (admin/admin123)" -ForegroundColor Gray
    Write-Host ""
    Write-Host "# API Service" -ForegroundColor Yellow
    Write-Host "kubectl port-forward svc/$ReleaseName-api 8080:8080 -n $Namespace" -ForegroundColor White
    Write-Host "# Access at: http://localhost:8080" -ForegroundColor Gray
    Write-Host ""
    
    # API Testing examples
    Write-Info "API Testing Examples:"
    Write-Host ""
    Write-Host "# Health Check" -ForegroundColor Yellow
    Write-Host "curl http://localhost:8080/health" -ForegroundColor White
    Write-Host ""
    Write-Host "# Get Metrics" -ForegroundColor Yellow
    Write-Host "curl -H 'X-API-Key: telemetry-api-secret-2025' http://localhost:8080/metrics" -ForegroundColor White
    Write-Host ""
    
    Write-Success "Telemetry Stack deployment completed successfully!"
    Write-Info "Use 'kubectl get all -n $Namespace' to see all resources"
    Write-Info "Use 'helm status $ReleaseName -n $Namespace' to check Helm release status"
}

# Cleanup function
function Remove-TelemetryStack {
    Write-Step "Cleaning Up Telemetry Stack"
    
    try {
        $helm_args = @("uninstall", $ReleaseName, "--namespace", $Namespace)
        if ($KubeContext) { $helm_args += @("--kube-context", $KubeContext) }
        
        & helm @helm_args
        Write-Success "Helm release uninstalled"
        
        # Optionally remove namespace
        $response = Read-Host "Do you want to delete the namespace '$Namespace'? (y/N)"
        if ($response -eq 'y' -or $response -eq 'Y') {
            $kubectl_args = @("delete", "namespace", $Namespace, "--ignore-not-found=true")
            if ($KubeContext) { $kubectl_args += @("--context", $KubeContext) }
            
            & kubectl @kubectl_args
            Write-Success "Namespace '$Namespace' deleted"
        }
    }
    catch {
        Write-Error "Cleanup failed: $_"
        exit 1
    }
}

# Script execution
try {
    Write-Host "🔧 Telemetry Stack Kubernetes Deployment Script" -ForegroundColor Magenta
    Write-Host "===============================================" -ForegroundColor Magenta
    Write-Host ""
    
    if ($args -contains "--cleanup" -or $args -contains "--remove") {
        Remove-TelemetryStack
    }
    else {
        Deploy-TelemetryStack
    }
}
catch {
    Write-Error "Script execution failed: $_"
    Write-Info "For cleanup, run: .\deploy-telemetry-stack.ps1 --cleanup"
    exit 1
}