# Telemetry Stack Deployment Guide

This guide provides one-time scripts to deploy the complete telemetry monitoring stack on a fresh Kubernetes cluster.

## Quick Start

### Prerequisites

1. **Kubernetes Cluster**: Ensure you have a running Kubernetes cluster and `kubectl` configured
2. **Docker**: Docker must be installed and running for building images
3. **Helm**: Helm v3+ must be installed

### Windows (PowerShell)

```powershell
# Deploy with default settings
.\deploy-telemetry-stack.ps1

# Deploy to custom namespace
.\deploy-telemetry-stack.ps1 -Namespace "monitoring"

# Skip image building (if images already exist)
.\deploy-telemetry-stack.ps1 -SkipImageBuild

# Use specific Kubernetes context
.\deploy-telemetry-stack.ps1 -KubeContext "my-cluster"

# Clean up deployment
.\deploy-telemetry-stack.ps1 --cleanup
```

### Linux/macOS (Bash)

```bash
# Make script executable
chmod +x deploy-telemetry-stack.sh

# Deploy with default settings
./deploy-telemetry-stack.sh

# Deploy to custom namespace
./deploy-telemetry-stack.sh --namespace monitoring

# Skip image building
./deploy-telemetry-stack.sh --skip-image-build

# Use specific Kubernetes context
./deploy-telemetry-stack.sh --kube-context my-cluster

# Clean up deployment
./deploy-telemetry-stack.sh --cleanup
```

## What Gets Deployed

The deployment script creates the following components:

### Core Services
- **API Service**: REST API for telemetry data access
- **Collector**: Collects and processes telemetry data
- **Message Queue**: Redis-based message broker with StatefulSet
- **Message Queue Proxy**: Load balancer for message queue
- **Streamer**: Streams CSV data to the system

### Monitoring Stack
- **InfluxDB**: Time-series database for metrics storage
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Visualization and dashboards

### Configuration
- **Namespace**: `telemetry` (customizable)
- **Release Name**: `telemetry-stack` (customizable)
- **Storage**: Persistent volumes for data persistence
- **Security**: API keys and service tokens configured

## Access the Services

After deployment, use these commands to access the services:

### Grafana Dashboard
```bash
kubectl port-forward svc/telemetry-stack-grafana 3000:3000 -n telemetry
```
- URL: http://localhost:3000
- Username: `admin`
- Password: `admin123`

### Prometheus Metrics
```bash
kubectl port-forward svc/telemetry-stack-prometheus 9090:9090 -n telemetry
```
- URL: http://localhost:9090

### InfluxDB
```bash
kubectl port-forward svc/telemetry-stack-influxdb 8086:8086 -n telemetry
```
- URL: http://localhost:8086
- Username: `admin`
- Password: `admin123`
- Organization: `telemetryorg`
- Bucket: `telem_bucket`

### API Service
```bash
kubectl port-forward svc/telemetry-stack-api 8080:8080 -n telemetry
```
- URL: http://localhost:8080
- API Key: `telemetry-api-secret-2025`

## Testing the Deployment

### Health Check
```bash
curl http://localhost:8080/health
```

### Get Metrics (with authentication)
```bash
curl -H "X-API-Key: telemetry-api-secret-2025" http://localhost:8080/metrics
```

### Check All Resources
```bash
kubectl get all -n telemetry
```

### Check Helm Release Status
```bash
helm status telemetry-stack -n telemetry
```

## Troubleshooting

### Check Pod Status
```bash
kubectl get pods -n telemetry
kubectl describe pod <pod-name> -n telemetry
```

### Check Logs
```bash
kubectl logs <pod-name> -n telemetry
kubectl logs -f deployment/telemetry-stack-api -n telemetry
```

### Check Services
```bash
kubectl get svc -n telemetry
kubectl describe svc <service-name> -n telemetry
```

### Check Persistent Volumes
```bash
kubectl get pv
kubectl get pvc -n telemetry
```

## Configuration

### Default Values

The deployment uses these default configurations from `helm/telemetry-stack/values.yaml`:

- **API Key**: `telemetry-api-secret-2025`
- **Service Token**: `internal-service-token-2025`
- **InfluxDB Admin**: `admin` / `admin123`
- **Grafana Admin**: `admin` / `admin123`
- **Message Queue Replicas**: 2
- **Prometheus Retention**: 15 days

### Custom Configuration

To customize the deployment, you can:

1. **Modify values.yaml**: Edit `helm/telemetry-stack/values.yaml`
2. **Pass Helm values**: Add `--set key=value` to the Helm command
3. **Use custom values file**: Create a custom values file and use `--values custom-values.yaml`

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Streamer      │───▶│ Message Queue   │───▶│   Collector     │
└─────────────────┘    │   (StatefulSet) │    └─────────────────┘
                       └─────────────────┘             │
                                │                      ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Service   │◀───│ Message Queue   │    │   InfluxDB      │
└─────────────────┘    │     Proxy       │    │ (Time Series)   │
                       └─────────────────┘    └─────────────────┘
                                                        │
┌─────────────────┐    ┌─────────────────┐             │
│   Grafana       │◀───│   Prometheus    │◀────────────┘
│ (Dashboards)    │    │   (Metrics)     │
└─────────────────┘    └─────────────────┘
```

## Security

### API Authentication
- REST API uses `X-API-Key` header or `Authorization: Bearer` token
- Default API key: `telemetry-api-secret-2025`

### Service-to-Service Communication
- Internal services use service token: `internal-service-token-2025`
- Kubernetes secrets store authentication credentials

### Network Policies
- Services communicate within the cluster using ClusterIP
- External access requires port-forwarding or ingress configuration

## Scaling

### Message Queue Scaling
```bash
# Scale message queue replicas
kubectl scale statefulset telemetry-stack-msg-queue --replicas=3 -n telemetry

# Update broker count in proxy
kubectl patch deployment telemetry-stack-msg-queue-proxy \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"msg-queue-proxy","env":[{"name":"BROKER_COUNT","value":"3"}]}]}}}}' \
  -n telemetry
```

### API Service Scaling
```bash
kubectl scale deployment telemetry-stack-api --replicas=3 -n telemetry
```

## Cleanup

### Complete Removal
```bash
# Windows
.\deploy-telemetry-stack.ps1 --cleanup

# Linux/macOS
./deploy-telemetry-stack.sh --cleanup
```

### Manual Cleanup
```bash
# Remove Helm release
helm uninstall telemetry-stack -n telemetry

# Remove namespace (optional)
kubectl delete namespace telemetry

# Remove persistent volumes (if needed)
kubectl delete pv --selector="app.kubernetes.io/instance=telemetry-stack"
```

## Support

For issues or questions:
1. Check pod logs: `kubectl logs <pod-name> -n telemetry`
2. Verify connectivity: `kubectl get all -n telemetry`
3. Check Helm status: `helm status telemetry-stack -n telemetry`
4. Review the troubleshooting section above