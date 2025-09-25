#!/bin/bash
set -e

# Telemetry Stack Cleanup Script
echo "🧹 Telemetry Stack Cleanup"
echo "=========================="
echo "This script will:"
echo "  • Stop all port-forwarding processes"
echo "  • Uninstall Helm releases"
echo "  • Remove Docker images"
echo "  • Clean up Kubernetes resources"
echo ""

# Function to kill port-forward processes
cleanup_port_forwards() {
    echo "🔌 Stopping port-forward processes..."
    
    # Kill kubectl port-forward processes
    if pgrep -f "kubectl port-forward" > /dev/null; then
        echo "   Stopping kubectl port-forward processes..."
        pkill -f "kubectl port-forward" || true
        sleep 2
    else
        echo "   No active port-forward processes found"
    fi
    
    echo "   ✅ Port-forward cleanup complete"
}

# Function to uninstall Helm releases
cleanup_helm() {
    echo "📦 Cleaning up Helm releases..."
    
    # Check if telemetry release exists
    if helm list | grep -q "telemetry"; then
        echo "   Uninstalling telemetry Helm release..."
        helm uninstall telemetry || true
        echo "   ✅ Helm release uninstalled"
    else
        echo "   No telemetry Helm release found"
    fi
    
    # Check if telemetry-stack release exists (alternative name)
    if helm list | grep -q "telemetry-stack"; then
        echo "   Uninstalling telemetry-stack Helm release..."
        helm uninstall telemetry-stack || true
        echo "   ✅ Helm release uninstalled"
    fi
}

# Function to cleanup Kubernetes resources
cleanup_kubernetes() {
    echo "☸️  Cleaning up Kubernetes resources..."
    
    # Delete persistent volume claims
    if kubectl get pvc | grep -q "telemetry\|msg-queue\|influxdb\|grafana\|prometheus"; then
        echo "   Deleting persistent volume claims..."
        kubectl delete pvc --all --timeout=60s || true
    fi
    
    # Delete any remaining pods
    if kubectl get pods | grep -q "telemetry\|msg-queue\|influxdb\|grafana\|prometheus\|api\|collector\|streamer"; then
        echo "   Force deleting remaining pods..."
        kubectl delete pods --all --grace-period=0 --force || true
    fi
    
    # Delete configmaps and secrets
    echo "   Cleaning up ConfigMaps and Secrets..."
    kubectl delete configmap --all || true
    kubectl delete secret --all || true
    
    echo "   ✅ Kubernetes cleanup complete"
}

# Function to cleanup Docker images
cleanup_docker() {
    echo "🐳 Cleaning up Docker images..."
    
    # List of telemetry images to remove
    IMAGES=("api" "collector" "msg-queue" "msg-queue-proxy" "streamer" "telemetry-api" "telemetry-collector" "telemetry-streamer" "telemetry-msg-queue" "telemetry-msg-queue-proxy" "influxdb" "")
    
    for img in "${IMAGES[@]}"; do
        if docker images | grep -q "^$img"; then
            echo "   Removing Docker image: $img"
            docker rmi "$img" --force || true
        fi
    done
    
    # Clean up dangling images
    echo "   Removing dangling Docker images..."
    docker image prune -f || true
    
    echo "   ✅ Docker cleanup complete"
}

# Function to verify cleanup
verify_cleanup() {
    echo "🔍 Verifying cleanup..."
    
    # Check for remaining pods
    if kubectl get pods 2>/dev/null | grep -q "telemetry\|msg-queue\|influxdb\|grafana\|prometheus\|api\|collector\|streamer"; then
        echo "   ⚠️  Some pods are still running:"
        kubectl get pods | grep "telemetry\|msg-queue\|influxdb\|grafana\|prometheus\|api\|collector\|streamer" || true
    else
        echo "   ✅ No telemetry pods found"
    fi
    
    # Check for remaining services
    if kubectl get svc 2>/dev/null | grep -q "telemetry\|msg-queue\|influxdb\|grafana\|prometheus\|api\|collector\|streamer"; then
        echo "   ⚠️  Some services are still running:"
        kubectl get svc | grep "telemetry\|msg-queue\|influxdb\|grafana\|prometheus\|api\|collector\|streamer" || true
    else
        echo "   ✅ No telemetry services found"
    fi
    
    # Check for port-forward processes
    if pgrep -f "kubectl port-forward" > /dev/null; then
        echo "   ⚠️  Some port-forward processes are still running"
    else
        echo "   ✅ No port-forward processes found"
    fi
}

# Main cleanup function
main() {
    echo "⚠️  WARNING: This will completely remove the telemetry stack!"
    echo "Are you sure you want to continue? (y/N)"
    
    # Auto-confirm if --force flag is provided
    if [[ "$1" == "--force" || "$1" == "-f" ]]; then
        echo "Force flag detected, proceeding with cleanup..."
    else
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            echo "Cleanup cancelled."
            exit 0
        fi
    fi
    
    echo ""
    echo "🚀 Starting cleanup process..."
    echo ""
    
    cleanup_port_forwards
    echo ""
    
    cleanup_helm
    echo ""
    
    cleanup_kubernetes
    echo ""
    
    cleanup_docker
    echo ""
    
    verify_cleanup
    echo ""
    
    echo "✅ Cleanup complete!"
    echo ""
    echo "📝 Next steps:"
    echo "   • Run ./quick-deploy.sh to redeploy"
    echo "   • Check cluster status: kubectl cluster-info"
    echo "   • Verify no resources remain: kubectl get all"
    echo ""
}

# Show help
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -f, --force    Skip confirmation prompt"
    echo "  -h, --help     Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0             Interactive cleanup with confirmation"
    echo "  $0 --force     Automatic cleanup without confirmation"
    echo ""
}

# Parse command line arguments
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac