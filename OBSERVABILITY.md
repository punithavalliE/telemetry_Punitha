# Observability with Prometheus and Grafana

This document describes the observability setup for the telemetry services using Prometheus for metrics collection and Grafana for visualization and dashboards.

## Overview

The telemetry stack now includes comprehensive monitoring capabilities:

- **Prometheus**: Collects metrics from all services via HTTP endpoints
- **Grafana**: Provides dashboards and visualization for metrics analysis
- **Service Discovery**: Automatically discovers and monitors new service instances
- **Custom Metrics**: Business-specific metrics for telemetry data processing

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ API Service │    │ Collector   │    │ Streamer    │    │ Msg Queue   │
│             │    │ Service     │    │ Service     │    │ Service     │
│ :8080       │    │ :8081       │    │ :8080       │    │ :8080       │
│ /metrics    │    │ /metrics    │    │ /metrics    │    │ /metrics    │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │                   │
       └───────────────────┼───────────────────┼───────────────────┘
                           │                   │
                    ┌─────────────┐    ┌─────────────┐
                    │ Prometheus  │    │ Grafana     │
                    │ :9090       │────│ :3000       │
                    └─────────────┘    └─────────────┘
```

## Metrics Exposed

### HTTP Metrics
- `http_requests_total`: Total number of HTTP requests by service, method, endpoint, and status
- `http_request_duration_seconds`: Duration of HTTP requests in seconds

### Message Queue Metrics
- `messages_produced_total`: Total messages produced to the queue by service and topic
- `messages_consumed_total`: Total messages consumed from the queue by service and topic
- `message_processing_duration_seconds`: Duration of message processing in seconds

### Database Metrics
- `database_operations_total`: Total database operations by service, operation, and status
- `database_operation_duration_seconds`: Duration of database operations in seconds

### Service Health Metrics
- `service_health`: Health status of services (1 = healthy, 0 = unhealthy)
- `active_connections`: Number of active connections per service

### Telemetry-Specific Metrics
- `telemetry_data_points_total`: Total telemetry data points processed by service and type

## Grafana Dashboards

### Telemetry Services Overview
The main dashboard provides a comprehensive view of the entire telemetry stack:

1. **HTTP Request Rate**: Real-time request rate across all services
2. **Service Health Status**: Current health status of all services
3. **Message Queue Throughput**: Message production and consumption rates
4. **HTTP Request Duration**: 95th and 50th percentile request latencies
5. **Telemetry Data Points Rate**: Rate of telemetry data processing
6. **Database Operations**: Database operation latencies and error rates

## Deployment

### Prerequisites
1. Kubernetes cluster
2. Helm 3.x
3. Persistent storage (optional but recommended)

### Installation

1. **Deploy the full stack with observability**:
```bash
helm install telemetry ./helm/telemetry-stack
```

2. **Deploy with custom values**:
```bash
helm install telemetry ./helm/telemetry-stack -f custom-values.yaml
```

3. **Enable only observability components**:
```bash
helm install telemetry ./helm/telemetry-stack \
  --set prometheus.enabled=true \
  --set grafana.enabled=true \
  --set api.enabled=false \
  --set collector.enabled=false \
  --set streamer.enabled=false \
  --set msgQueue.enabled=false
```

### Configuration

#### Prometheus Configuration
```yaml
prometheus:
  enabled: true
  persistence:
    enabled: true
    size: 2Gi
  retention: 15d
  scrapeInterval: 15s
```

#### Grafana Configuration
```yaml
grafana:
  enabled: true
  adminPassword: "your-secure-password"
  persistence:
    enabled: true
    size: 1Gi
  service:
    type: NodePort  # For external access
```

## Access

### Prometheus Web UI
```bash
# Port forward to access Prometheus
kubectl port-forward svc/telemetry-prometheus-service 9090:9090

# Access at: http://localhost:9090
```

### Grafana Dashboards
```bash
# Port forward to access Grafana
kubectl port-forward svc/telemetry-grafana-service 3000:3000

# Access at: http://localhost:3000
# Default credentials: admin / grafana123
```

### Service Metrics Endpoints
```bash
# API Service metrics
kubectl port-forward svc/api-service 8080:8080
curl http://localhost:8080/metrics

# Collector Service metrics
kubectl port-forward svc/collector-service 8081:8081
curl http://localhost:8081/metrics

# Streamer Service metrics
kubectl port-forward svc/streamer-service 8080:8080
curl http://localhost:8080/metrics

# Message Queue Service metrics
kubectl port-forward svc/msg-queue-service 8080:8080
curl http://localhost:8080/metrics
```

## Monitoring Checklist

### Service Health
- [ ] All services show `service_health = 1`
- [ ] HTTP request success rates > 95%
- [ ] Database operation error rates < 1%

### Performance
- [ ] HTTP request 95th percentile < 500ms
- [ ] Message processing latency < 100ms
- [ ] Database operation latency < 50ms

### Throughput
- [ ] Message production/consumption rates are balanced
- [ ] Telemetry data points are being processed
- [ ] No significant message queue backlog

## Alerting (Future Enhancement)

### Recommended Alerts
1. **Service Down**: `service_health == 0`
2. **High Error Rate**: `rate(http_requests_total{status=~"5.."}[5m]) > 0.05`
3. **High Latency**: `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5`
4. **Database Errors**: `rate(database_operations_total{status="error"}[5m]) > 0.01`
5. **Message Queue Lag**: `messages_produced_total - messages_consumed_total > 1000`

## Troubleshooting

### Common Issues

1. **Metrics not appearing in Prometheus**:
   - Check if services are running: `kubectl get pods`
   - Verify metrics endpoints: `kubectl port-forward <pod> 8080:8080 && curl localhost:8080/metrics`
   - Check Prometheus configuration: `kubectl logs <prometheus-pod>`

2. **Grafana dashboards not loading**:
   - Verify Prometheus datasource connectivity
   - Check Grafana logs: `kubectl logs <grafana-pod>`
   - Ensure dashboard ConfigMap is mounted

3. **Service discovery not working**:
   - Verify pod annotations include `prometheus.io/scrape: "true"`
   - Check RBAC permissions for Prometheus service account
   - Review Prometheus scrape configuration

### Debug Commands
```bash
# Check all pods status
kubectl get pods -l release=telemetry

# View Prometheus configuration
kubectl get configmap telemetry-prometheus-config -o yaml

# Check service endpoints
kubectl get endpoints

# View Prometheus targets
# Access Prometheus UI -> Status -> Targets
```

## Security Considerations

1. **Network Policies**: Restrict access to metrics endpoints
2. **RBAC**: Limit Prometheus service account permissions
3. **Authentication**: Use strong passwords for Grafana
4. **TLS**: Enable HTTPS for production deployments
5. **Secrets Management**: Store sensitive configuration in Kubernetes secrets

## Performance Considerations

1. **Retention Policy**: Adjust Prometheus retention based on storage capacity
2. **Scrape Intervals**: Balance between data granularity and resource usage
3. **Storage**: Use persistent storage for production deployments
4. **Resource Limits**: Set appropriate CPU and memory limits

## Next Steps

1. **Custom Dashboards**: Create service-specific dashboards
2. **Alerting**: Implement AlertManager for proactive monitoring
3. **Log Integration**: Add centralized logging with ELK stack
4. **Distributed Tracing**: Integrate with Jaeger or Zipkin
5. **SLIs/SLOs**: Define service level indicators and objectives