#!/bin/bash
set -euo pipefail

# Telemetry Stack Kubernetes Deployment Script
# This script deploys the complete telemetry monitoring stack to a fresh Kubernetes cluster.

# Default configuration
NAMESPACE="${NAMESPACE:-telemetry}"
RELEASE_NAME="${RELEASE_NAME:-telemetry-stack}"
KUBE_CONTEXT="${KUBE_CONTEXT:-}"
SKIP_IMAGE_BUILD="${SKIP_IMAGE_BUILD:-false}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELM_CHART_PATH="$SCRIPT_DIR/helm/telemetry-stack"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}🔵 INFO: $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ SUCCESS: $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  WARNING: $1${NC}"
}

log_error() {
    echo -e "${RED}❌ ERROR: $1${NC}"
}

log_step() {
    echo -e "\n${CYAN}🚀 STEP: $1${NC}"
    echo -e "${CYAN}$(printf '=%.0s' {1..80})${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to wait for deployment readiness
wait_for_deployment() {
    local deployment_name="$1"
    local namespace="$2"
    local timeout_seconds="${3:-300}"
    
    log_info "Waiting for deployment $deployment_name to be ready..."
    local elapsed=0
    local interval=10
    
    while [ $elapsed -lt $timeout_seconds ]; do
        local kubectl_cmd="kubectl"
        if [ -n "$KUBE_CONTEXT" ]; then
            kubectl_cmd="kubectl --context $KUBE_CONTEXT"
        fi
        
        local ready=$($kubectl_cmd get deployment "$deployment_name" -n "$namespace" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local desired=$($kubectl_cmd get deployment "$deployment_name" -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
        
        if [ "$ready" = "$desired" ] && [ "$ready" -gt 0 ]; then
            log_success "Deployment $deployment_name is ready ($ready/$desired replicas)"
            return 0
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
        log_info "Waiting... ($elapsed/$timeout_seconds seconds elapsed)"
    done
    
    log_error "Timeout waiting for deployment $deployment_name to be ready"
    return 1
}

# Function to wait for StatefulSet readiness
wait_for_statefulset() {
    local statefulset_name="$1"
    local namespace="$2"
    local timeout_seconds="${3:-300}"
    
    log_info "Waiting for StatefulSet $statefulset_name to be ready..."
    local elapsed=0
    local interval=10
    
    while [ $elapsed -lt $timeout_seconds ]; do
        local kubectl_cmd="kubectl"
        if [ -n "$KUBE_CONTEXT" ]; then
            kubectl_cmd="kubectl --context $KUBE_CONTEXT"
        fi
        
        local ready=$($kubectl_cmd get statefulset "$statefulset_name" -n "$namespace" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local desired=$($kubectl_cmd get statefulset "$statefulset_name" -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
        
        if [ "$ready" = "$desired" ] && [ "$ready" -gt 0 ]; then
            log_success "StatefulSet $statefulset_name is ready ($ready/$desired replicas)"
            return 0
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
        log_info "Waiting... ($elapsed/$timeout_seconds seconds elapsed)"
    done
    
    log_error "Timeout waiting for StatefulSet $statefulset_name to be ready"
    return 1
}

# Main deployment function
deploy_telemetry_stack() {
    log_step "Starting Telemetry Stack Deployment"
    
    # Check prerequisites
    log_step "Checking Prerequisites"
    
    local required_commands=("kubectl" "helm" "docker")
    for cmd in "${required_commands[@]}"; do
        if ! command_exists "$cmd"; then
            log_error "$cmd is not installed or not in PATH"
            exit 1
        fi
        log_success "$cmd is available"
    done
    
    # Check Kubernetes connectivity
    log_info "Testing Kubernetes connectivity..."
    local kubectl_cmd="kubectl cluster-info"
    if [ -n "$KUBE_CONTEXT" ]; then
        kubectl_cmd="kubectl --context $KUBE_CONTEXT cluster-info"
    fi
    
    if ! $kubectl_cmd >/dev/null 2>&1; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    log_success "Kubernetes cluster is accessible"
    
    # Check if Helm chart exists
    if [ ! -d "$HELM_CHART_PATH" ]; then
        log_error "Helm chart not found at: $HELM_CHART_PATH"
        exit 1
    fi
    log_success "Helm chart found at: $HELM_CHART_PATH"
    
    # Create namespace
    log_step "Creating Namespace"
    local kubectl_cmd="kubectl"
    if [ -n "$KUBE_CONTEXT" ]; then
        kubectl_cmd="kubectl --context $KUBE_CONTEXT"
    fi
    
    $kubectl_cmd create namespace "$NAMESPACE" --dry-run=client -o yaml | $kubectl_cmd apply -f - || true
    log_success "Namespace '$NAMESPACE' is ready"
    
    # Build Docker images (if not skipped)
    if [ "$SKIP_IMAGE_BUILD" != "true" ]; then
        log_step "Building Docker Images"
        
        declare -A images=(
            ["api"]="services/api/Dockerfile"
            ["collector"]="services/collector/Dockerfile"
            ["msg-queue"]="services/msg_queue/Dockerfile"
            ["msg-queue-proxy"]="services/msg_queue_proxy/Dockerfile"
            ["streamer"]="services/streamer/Dockerfile"
        )
        
        for image_name in "${!images[@]}"; do
            local dockerfile="${images[$image_name]}"
            log_info "Building $image_name image..."
            
            if ! docker build -t "$image_name" -f "$dockerfile" .; then
                log_error "Failed to build image $image_name"
                exit 1
            fi
            log_success "Built image: $image_name"
        done
        
        log_success "All Docker images built successfully"
    else
        log_info "Skipping Docker image build as requested"
    fi
    
    # Deploy Helm chart
    log_step "Deploying Helm Chart"
    local helm_cmd="helm upgrade --install $RELEASE_NAME $HELM_CHART_PATH --namespace $NAMESPACE --create-namespace --wait --timeout 600s"
    
    if [ -n "$KUBE_CONTEXT" ]; then
        helm_cmd="helm --kube-context $KUBE_CONTEXT upgrade --install $RELEASE_NAME $HELM_CHART_PATH --namespace $NAMESPACE --create-namespace --wait --timeout 600s"
    fi
    
    log_info "Executing: $helm_cmd"
    if ! eval "$helm_cmd"; then
        log_error "Helm deployment failed"
        
        # Show helm status for debugging
        log_info "Checking Helm release status..."
        local status_cmd="helm status $RELEASE_NAME --namespace $NAMESPACE"
        if [ -n "$KUBE_CONTEXT" ]; then
            status_cmd="helm --kube-context $KUBE_CONTEXT status $RELEASE_NAME --namespace $NAMESPACE"
        fi
        eval "$status_cmd" || true
        
        exit 1
    fi
    
    log_success "Helm chart deployed successfully"
    
    # Wait for deployments to be ready
    log_step "Waiting for Services to be Ready"
    
    local deployments=("api" "collector" "grafana" "prometheus" "msg-queue-proxy" "streamer")
    local statefulsets=("influxdb" "msg-queue")
    
    # Wait for deployments
    for deployment in "${deployments[@]}"; do
        local full_name="$RELEASE_NAME-$deployment"
        wait_for_deployment "$full_name" "$NAMESPACE" || log_warning "Deployment $full_name is not ready, but continuing..."
    done
    
    # Wait for StatefulSets
    for statefulset in "${statefulsets[@]}"; do
        local full_name="$RELEASE_NAME-$statefulset"
        wait_for_statefulset "$full_name" "$NAMESPACE" || log_warning "StatefulSet $full_name is not ready, but continuing..."
    done
    
    # Display service information
    log_step "Service Information"
    
    log_info "Getting service endpoints..."
    local kubectl_cmd="kubectl"
    if [ -n "$KUBE_CONTEXT" ]; then
        kubectl_cmd="kubectl --context $KUBE_CONTEXT"
    fi
    
    $kubectl_cmd get services -n "$NAMESPACE" || true
    
    log_info "Getting pod status..."
    $kubectl_cmd get pods -n "$NAMESPACE" || true
    
    # Port forwarding instructions
    log_step "Access Instructions"
    
    log_info "To access the services, use the following port-forward commands:"
    echo ""
    echo -e "${YELLOW}# Grafana Dashboard (admin/admin123)${NC}"
    echo -e "${NC}kubectl port-forward svc/$RELEASE_NAME-grafana 3000:3000 -n $NAMESPACE${NC}"
    echo -e "${RED}# Access at: http://localhost:3000${NC}"
    echo ""
    echo -e "${YELLOW}# Prometheus Metrics${NC}"
    echo -e "${NC}kubectl port-forward svc/$RELEASE_NAME-prometheus 9090:9090 -n $NAMESPACE${NC}"
    echo -e "${RED}# Access at: http://localhost:9090${NC}"
    echo ""
    echo -e "${YELLOW}# InfluxDB${NC}"
    echo -e "${NC}kubectl port-forward svc/$RELEASE_NAME-influxdb 8086:8086 -n $NAMESPACE${NC}"
    echo -e "${RED}# Access at: http://localhost:8086 (admin/admin123)${NC}"
    echo ""
    echo -e "${YELLOW}# API Service${NC}"
    echo -e "${NC}kubectl port-forward svc/$RELEASE_NAME-api 8080:8080 -n $NAMESPACE${NC}"
    echo -e "${RED}# Access at: http://localhost:8080${NC}"
    echo ""
    
    # API Testing examples
    log_info "API Testing Examples:"
    echo ""
    echo -e "${YELLOW}# Health Check${NC}"
    echo -e "${NC}curl http://localhost:8080/health${NC}"
    echo ""
    echo -e "${YELLOW}# Get Metrics${NC}"
    echo -e "${NC}curl -H 'X-API-Key: telemetry-api-secret-2025' http://localhost:8080/metrics${NC}"
    echo ""
    
    log_success "Telemetry Stack deployment completed successfully!"
    log_info "Use 'kubectl get all -n $NAMESPACE' to see all resources"
    log_info "Use 'helm status $RELEASE_NAME -n $NAMESPACE' to check Helm release status"
}

# Cleanup function
cleanup_telemetry_stack() {
    log_step "Cleaning Up Telemetry Stack"
    
    local helm_cmd="helm uninstall $RELEASE_NAME --namespace $NAMESPACE"
    if [ -n "$KUBE_CONTEXT" ]; then
        helm_cmd="helm --kube-context $KUBE_CONTEXT uninstall $RELEASE_NAME --namespace $NAMESPACE"
    fi
    
    if eval "$helm_cmd"; then
        log_success "Helm release uninstalled"
        
        # Optionally remove namespace
        echo -n "Do you want to delete the namespace '$NAMESPACE'? (y/N): "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            local kubectl_cmd="kubectl delete namespace $NAMESPACE --ignore-not-found=true"
            if [ -n "$KUBE_CONTEXT" ]; then
                kubectl_cmd="kubectl --context $KUBE_CONTEXT delete namespace $NAMESPACE --ignore-not-found=true"
            fi
            
            eval "$kubectl_cmd"
            log_success "Namespace '$NAMESPACE' deleted"
        fi
    else
        log_error "Cleanup failed"
        exit 1
    fi
}

# Help function
show_help() {
    echo -e "${MAGENTA}🔧 Telemetry Stack Kubernetes Deployment Script${NC}"
    echo -e "${MAGENTA}===============================================${NC}"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --namespace NAME         Kubernetes namespace (default: telemetry)"
    echo "  --release-name NAME      Helm release name (default: telemetry-stack)"
    echo "  --kube-context CONTEXT   Kubernetes context to use"
    echo "  --skip-image-build       Skip building Docker images"
    echo "  --cleanup               Remove the deployment"
    echo "  --help                  Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  NAMESPACE               Set default namespace"
    echo "  RELEASE_NAME            Set default release name"
    echo "  KUBE_CONTEXT            Set default kube context"
    echo "  SKIP_IMAGE_BUILD        Set to 'true' to skip image building"
    echo ""
    echo "Examples:"
    echo "  $0                                    # Deploy with defaults"
    echo "  $0 --namespace monitoring             # Deploy to monitoring namespace"
    echo "  $0 --skip-image-build                 # Deploy without building images"
    echo "  $0 --cleanup                          # Remove deployment"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --release-name)
            RELEASE_NAME="$2"
            shift 2
            ;;
        --kube-context)
            KUBE_CONTEXT="$2"
            shift 2
            ;;
        --skip-image-build)
            SKIP_IMAGE_BUILD="true"
            shift
            ;;
        --cleanup)
            cleanup_telemetry_stack
            exit 0
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Main execution
main() {
    echo -e "${MAGENTA}🔧 Telemetry Stack Kubernetes Deployment Script${NC}"
    echo -e "${MAGENTA}===============================================${NC}"
    echo ""
    
    deploy_telemetry_stack
}

# Run main function
main "$@"