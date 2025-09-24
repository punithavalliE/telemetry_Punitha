#!/bin/bash
set -e

# Quick one-liner deployment of Telemetry Stack
echo "🚀 Quick Telemetry Stack Deployment"
echo "==================================="

# Check prerequisites
for cmd in kubectl helm docker; do
    if ! command -v $cmd >/dev/null; then
        echo "❌ $cmd not found. Please install $cmd first."
        exit 1
    fi
done

# Test cluster connectivity
if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "❌ Cannot connect to Kubernetes cluster"
    exit 1
fi
echo "✅ Kubernetes cluster accessible"

echo "🔧 Building images and deploying..."

# Build images
for img in api collector msg-queue msg-queue-proxy streamer; do
    echo "Building $img..."
    docker build -t $img -f services/${img//-/_}/Dockerfile . #>/dev/null
done

# Deploy with Helm
kubectl create namespace telemetry --dry-run=client -o yaml | kubectl apply -f - #>/dev/null
helm upgrade --install telemetry-stack ./helm/telemetry-stack --namespace telemetry --wait --timeout 600s

echo ""
echo "✅ Deployment Complete!"
echo "Access Grafana: kubectl port-forward svc/telemetry-stack-grafana 3000:3000 -n telemetry"
echo "Then open: http://localhost:3000 (admin/admin123)"