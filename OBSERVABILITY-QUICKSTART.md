# Quick Start: Testing Observability

This guide helps you quickly test the observability setup with Prometheus and Grafana.

## Build and Test Locally

### 1. Build Docker Images
```powershell
# Build all service images
docker build -t telemetry-api:latest -f services/api/Dockerfile .
docker build -t telemetry-collector:latest -f services/collector/Dockerfile .
docker build -t telemetry-streamer:latest -f services/streamer/Dockerfile .
docker build -t telemetry-msg-queue:latest -f services/msg_queue/Dockerfile .
```

### 2. Deploy with Helm
```powershell
# Install the complete stack with observability
helm install telemetry ./helm/telemetry-stack --wait

# Check deployment status
kubectl get pods -l release=telemetry
```

### 3. Access Monitoring Interfaces

#### Prometheus
```powershell
# Port forward Prometheus
kubectl port-forward svc/telemetry-telemetry-stack-prometheus-service 9090:9090

# Open browser: http://localhost:9090
# Check Status -> Targets to see all services
```

#### Grafana
```powershell
# Port forward Grafana
kubectl port-forward svc/telemetry-telemetry-stack-grafana-service 3000:3000

# Open browser: http://localhost:3000
# Login: admin / grafana123
# Navigate to Dashboards -> Telemetry Services Overview
```

### 4. Generate Test Data

#### Send test telemetry data
```powershell
# Port forward API service
kubectl port-forward svc/api-service 8080:8080

# Send test data (in another terminal)
curl -X POST http://localhost:8080/telemetry \
  -H "Content-Type: application/json" \
  -H "X-API-Key: telemetry-api-secret-2025" \
  -d '{
    "device_id": "gpu-001",
    "metric": "temperature",
    "value": 75.5,
    "time": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "gpu_id": "0",
    "uuid": "12345678-1234-1234-1234-123456789012",
    "model_name": "NVIDIA RTX 4090",
    "hostname": "worker-node-1",
    "container": "gpu-workload",
    "pod": "gpu-pod-1",
    "namespace": "default",
    "labels_raw": "app=ml-training"
  }'
```

#### Stream CSV data
```powershell
# Port forward streamer service
kubectl port-forward svc/streamer-service 8080:8080

# Upload CSV file (if you have one)
curl -X POST http://localhost:8080/telemetry \
  -H "Content-Type: application/json" \
  -d @sample-telemetry.json
```

### 5. View Metrics

#### Raw Metrics
```powershell
# API service metrics
curl http://localhost:8080/metrics

# Check specific metrics
curl http://localhost:8080/metrics | grep "http_requests_total"
curl http://localhost:8080/metrics | grep "telemetry_data_points_total"
```

#### Prometheus Queries
Open Prometheus UI (http://localhost:9090) and try these queries:

```promql
# Request rate per service
rate(http_requests_total[5m])

# Service health status
service_health

# Message processing rate
rate(messages_consumed_total[5m])

# Database operation latency
histogram_quantile(0.95, rate(database_operation_duration_seconds_bucket[5m]))

# Telemetry data points rate
rate(telemetry_data_points_total[5m])
```

### 6. Grafana Dashboard Features

In the "Telemetry Services Overview" dashboard, you should see:

1. **HTTP Request Rate**: Real-time graphs showing API calls
2. **Service Health**: Green indicators for healthy services
3. **Message Queue Throughput**: Message flow between services
4. **Request Duration**: Latency percentiles
5. **Telemetry Data Points**: Rate of data processing
6. **Database Operations**: InfluxDB performance metrics

## Troubleshooting

### Services Not Appearing in Prometheus
```powershell
# Check if pods have correct annotations
kubectl describe pod <pod-name> | grep prometheus

# Check if metrics endpoints are accessible
kubectl port-forward <pod-name> 8080:8080
curl http://localhost:8080/metrics
```

### Grafana Dashboard Issues
```powershell
# Check if Prometheus datasource is configured
kubectl exec -it <grafana-pod> -- curl http://telemetry-telemetry-stack-prometheus-service:9090/api/v1/status/config

# Restart Grafana if needed
kubectl delete pod <grafana-pod>
```

### No Metrics Data
```powershell
# Check service logs
kubectl logs <service-pod>

# Verify network connectivity
kubectl exec -it <prometheus-pod> -- nslookup api-service
```

## Sample Test Scripts

### PowerShell Test Script
```powershell
# test-observability.ps1
$API_ENDPOINT = "http://localhost:8080"
$PROMETHEUS_ENDPOINT = "http://localhost:9090"
$GRAFANA_ENDPOINT = "http://localhost:3000"

# Test API health
Write-Host "Testing API health..."
Invoke-RestMethod -Uri "$API_ENDPOINT/health"

# Send test telemetry data
Write-Host "Sending test data..."
for ($i = 1; $i -le 10; $i++) {
    $body = @{
        device_id = "gpu-$i"
        metric = "temperature"
        value = 70 + $i
        time = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        gpu_id = "$i"
        uuid = "12345678-1234-1234-1234-12345678901$i"
        model_name = "NVIDIA RTX 4090"
        hostname = "test-node"
        container = "test-container"
        pod = "test-pod"
        namespace = "default"
        labels_raw = "test=true"
    } | ConvertTo-Json
    
    Invoke-RestMethod -Uri "$API_ENDPOINT/telemetry" `
        -Method POST `
        -Headers @{"X-API-Key" = "telemetry-api-secret-2025"; "Content-Type" = "application/json"} `
        -Body $body
}

Write-Host "Test data sent. Check Grafana dashboard for metrics."
```

### Expected Results

After running the tests, you should see:

1. **Prometheus Targets**: All services showing as "UP"
2. **Grafana Dashboard**: Graphs showing activity
3. **Metrics Increasing**: Request counts, data points, etc.
4. **Service Health**: All services showing healthy (value = 1)

## Cleanup

```powershell
# Remove the deployment
helm uninstall telemetry

# Clean up any persistent volumes (if needed)
kubectl delete pvc -l release=telemetry
```

This completes the observability testing setup. Your telemetry services now have comprehensive monitoring with Prometheus and Grafana!