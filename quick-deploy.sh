#!/bin/bash
set -e

# Quick deployment script for Telemetry Stack
# This script builds Docker images and deploys all services to Kubernetes

# Function to show help
show_help() {
    echo "🚀 Quick Telemetry Stack Deployment"
    echo "==================================="
    echo ""
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  deploy         Deploy the telemetry stack (default)"
    echo "  cleanup        Clean up the telemetry stack"
    echo "  redeploy       Clean up and redeploy"
    echo ""
    echo "Options:"
    echo "  -f, --force    Skip confirmation prompts"
    echo "  -h, --help     Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0             Deploy the stack"
    echo "  $0 deploy      Deploy the stack"
    echo "  $0 cleanup     Clean up everything"
    echo "  $0 redeploy    Clean up and redeploy"
    echo "  $0 cleanup -f  Force cleanup without confirmation"
    echo ""
}

# Function to cleanup telemetry stack - calls dedicated cleanup script
cleanup_telemetry() {
    local force_flag="$1"
    
    # Check if cleanup script exists
    if [[ ! -f "./cleanup.sh" ]]; then
        echo "❌ Error: cleanup.sh script not found in current directory"
        echo "💡 Make sure cleanup.sh is in the same directory as quick-deploy.sh"
        exit 1
    fi
    
    # Make cleanup script executable and call it
    chmod +x ./cleanup.sh
    
    if [[ "$force_flag" == "--force" || "$force_flag" == "-f" ]]; then
        echo "🧹 Running cleanup with force flag..."
        ./cleanup.sh --force
    else
        echo "🧹 Running cleanup script..."
        ./cleanup.sh
    fi
}

# Function to deploy telemetry stack
deploy_telemetry() {
    echo "🚀 Quick Telemetry Stack Deployment"
    echo "==================================="
    echo "This script will:"
    echo "  • Build all Docker images"
    echo "  • Deploy to Kubernetes using Helm"
    echo "  • Provide port-forwarding commands for service access"
    echo ""

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
    echo "🔄 Setting up port forwarding..."
    echo ""
    
    # Start port forwarding in background for immediate access
    echo "Starting port forwards..."
    
    # Kill any existing port forwards
    pkill -f "kubectl port-forward" 2>/dev/null || true
    sleep 2
    
    # Start key services port forwarding in background
    nohup kubectl port-forward -n telemetry svc/grafana-service 3000:3000 > /dev/null 2>&1 &
    nohup kubectl port-forward -n telemetry svc/prometheus-service 9090:9090 > /dev/null 2>&1 &
    nohup kubectl port-forward -n telemetry svc/api-nodeport 8080:8080 > /dev/null 2>&1 &
    nohup kubectl port-forward -n telemetry svc/influxdb 8086:8086 > /dev/null 2>&1 &
    
    # Wait a moment for port forwards to establish
    sleep 3

    echo ""
    echo "✅ Deployment Complete!"
    echo ""
    echo "🌐 ACTIVE PORT FORWARDS & SERVICE ACCESS"
    echo "========================================"
    echo ""
    echo "🎉 The following services are now accessible:"
    echo ""
    echo "📊 MONITORING & OBSERVABILITY:"
    echo "   ✅ Grafana Dashboard: http://localhost:3000 (admin/admin123)"
    echo "   ✅ Prometheus Metrics: http://localhost:9090"
    echo "   ✅ InfluxDB: http://localhost:8086 (admin/admin123)"
    echo "   ✅ Telemetry API: http://localhost:8080/swagger/ (API Key: telemetry-api-secret-2025)"
    echo ""
    echo "🔄 Additional Port-Forwarding Commands:"
    echo "   Grafana Dashboard:"
    echo "   kubectl port-forward -n telemetry svc/grafana-service 3000:3000"
    echo "   → http://localhost:3000 (admin/admin123)"
    echo ""
    echo "   Prometheus Metrics:"
    echo "   kubectl port-forward -n telemetry svc/prometheus-service 9090:9090"
    echo "   → http://localhost:9090"
    echo ""
    echo "💾 DATABASE:"
    echo "   InfluxDB:"
    echo "   kubectl port-forward -n telemetry svc/influxdb 8086:8086"
    echo "   → http://localhost:8086 (admin/admin123)"
    echo ""
    echo "🔌 API SERVICES:"
    echo "   Telemetry API:"
    echo "   kubectl port-forward -n telemetry svc/api-nodeport 8080:8080"
    echo "   → http://localhost:8080/swagger/ (API Key: telemetry-api-secret-2025)"
    echo ""
    echo "   Streamer Service:"
    echo "   kubectl port-forward -n telemetry svc/streamer-service 8081:8080"
    echo "   → http://localhost:8081/health"
    echo ""
    echo "   Collector Service:"
    echo "   kubectl port-forward -n telemetry svc/collector-service 8082:8080"
    echo "   → http://localhost:8082/health"
    echo ""
    echo "⚙️  MESSAGE QUEUE:"
    echo "   Message Queue Proxy:"
    echo "   kubectl port-forward -n telemetry svc/msg-queue-proxy-service 8083:8080"
    echo "   → http://localhost:8083/health"
    echo ""
    echo "   Message Queue Broker:"
    echo "   kubectl port-forward -n telemetry svc/msg-queue 8084:8080"
    echo "   → http://localhost:8084/topics"
    echo ""
    echo "🚀 QUICK START (Services Already Running):"
    echo "   1. Open Grafana: http://localhost:3000 (admin/admin123)"
    echo "   2. Open Prometheus: http://localhost:9090"
    echo "   3. Open API Docs: http://localhost:8080/swagger/"
    echo "   4. Test API: curl -H \"X-API-Key: telemetry-api-secret-2025\" http://localhost:8080/health"
    echo ""
    echo "📝 USEFUL COMMANDS:"
    echo "   Check pod status: kubectl get pods -n telemetry"
    echo "   View service logs: kubectl logs -f deployment/<service-name> -n telemetry"
    echo "   Scale services: kubectl scale deployment <service-name> --replicas=<count> -n telemetry"
    echo "   Stop port forwards: pkill -f 'kubectl port-forward'"
    echo ""
    echo "🔍 METRICS ENDPOINTS (after port-forwarding):"
    echo "   API metrics: http://localhost:8080/metrics"
    echo "   Streamer metrics: http://localhost:8081/metrics"
    echo "   Collector metrics: http://localhost:8082/metrics"
    echo "   Proxy metrics: http://localhost:8083/metrics"
    echo ""
    echo "🔧 TROUBLESHOOTING:"
    echo "   • If pods are not ready: kubectl describe pod <pod-name>"
    echo "   • Check service logs: kubectl logs -f deployment/<service-name>"
    echo "   • Port already in use: pkill -f 'port-forward' (kill existing forwards)"
    echo "   • Reset deployment: helm uninstall telemetry-stack"
    echo "   • Verify cluster: kubectl cluster-info"
    echo ""
    echo "📚 DOCUMENTATION:"
    echo "   • Full documentation: README.md"
    echo "   • API docs: http://localhost:8080/swagger/ (after port-forwarding)"
    echo "   • Architecture: Telemetry_Stack_Architecture_Document.md"
    echo ""
    echo "Happy monitoring! 🎉"
}

# Main script execution
main() {
    case "${1:-deploy}" in
        "deploy")
            deploy_telemetry
            ;;
        "cleanup")
            cleanup_telemetry "${2:-}"
            ;;
        "redeploy")
            echo "🔄 Redeploying telemetry stack..."
            cleanup_telemetry "${2:-}"
            sleep 2
            deploy_telemetry
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            echo "❌ Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# Execute main function with all arguments
main "$@"