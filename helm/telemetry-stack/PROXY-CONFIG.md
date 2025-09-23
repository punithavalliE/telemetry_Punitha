# Message Queue Proxy Helm Configuration

This document describes the Helm configuration for the msg-queue-proxy component.

## Configuration Options

### Basic Configuration

```yaml
msgQueueProxy:
  enabled: true                    # Enable/disable the proxy
  name: msg-queue-proxy           # Service name
  replicaCount: 2                 # Number of proxy replicas for HA
```

### Image Configuration

```yaml
msgQueueProxy:
  image:
    repository: msg-queue-proxy   # Container image repository
    tag: latest                   # Image tag
```

### Service Configuration

```yaml
msgQueueProxy:
  service:
    type: ClusterIP              # Service type (ClusterIP, LoadBalancer, NodePort)
    port: 8080                   # Service port
    external:
      enabled: false             # Enable external LoadBalancer
      type: LoadBalancer        # External service type
```

### Environment Configuration

```yaml
msgQueueProxy:
  env:
    port: "8080"                 # Proxy listening port
    brokerService: "msg-queue"   # Broker service name
    brokerCount: "3"             # Number of broker instances
    virtualNodes: "150"          # Virtual nodes per broker in hash ring
    maxPartitions: "12"          # Maximum partitions per topic
    healthIntervalSeconds: "30"  # Health check interval
```

### Resource Configuration

```yaml
msgQueueProxy:
  resources:
    requests:
      memory: "64Mi"             # Memory request
      cpu: "50m"                 # CPU request
    limits:
      memory: "128Mi"            # Memory limit
      cpu: "100m"                # CPU limit
```

### Health Check Configuration

```yaml
msgQueueProxy:
  healthCheck:
    path: "/health"                        # Health check endpoint
    initialDelaySeconds: 30                # Initial delay for liveness probe
    periodSeconds: 10                      # Liveness probe period
    readinessInitialDelaySeconds: 5        # Initial delay for readiness probe
    readinessPeriodSeconds: 5              # Readiness probe period
```

### Monitoring Configuration

```yaml
msgQueueProxy:
  monitoring:
    enabled: true                # Enable Prometheus monitoring
    metricsPath: "/metrics"      # Prometheus metrics endpoint
    statsPath: "/stats"          # Human-readable stats endpoint
```

## Service Endpoints

Once deployed, the proxy provides the following endpoints:

- **`/produce`** - Message production (routes to appropriate broker)
- **`/consume`** - Message consumption (routes to appropriate broker)
- **`/ack`** - Message acknowledgment (routes to appropriate broker)
- **`/topics`** - List available topics
- **`/health`** - Proxy health status
- **`/status`** - Detailed proxy status and configuration
- **`/stats`** - Human-readable statistics
- **`/metrics`** - Prometheus metrics

## Service Discovery

The proxy automatically discovers broker instances using:

```
{brokerService}-{index}.{brokerService}.{namespace}.svc.cluster.local
```

For example:
- `msg-queue-0.msg-queue.default.svc.cluster.local`
- `msg-queue-1.msg-queue.default.svc.cluster.local`
- `msg-queue-2.msg-queue.default.svc.cluster.local`

## Client Configuration

Update client services to use the proxy instead of direct broker access:

```yaml
# Before (direct broker access)
msgQueueAddr: "http://msg-queue:8080"

# After (via proxy)
msgQueueAddr: "http://msg-queue-proxy:8080"
```

## Monitoring Integration

The proxy includes full Prometheus integration:

### ServiceMonitor

A ServiceMonitor is automatically created when monitoring is enabled, allowing Prometheus to scrape metrics.

### Key Metrics

- `proxy_requests_total` - Total requests by type and status
- `proxy_request_duration_seconds` - Request latency histograms
- `proxy_broker_requests_total` - Per-broker request distribution
- `proxy_broker_health` - Real-time broker health status
- `proxy_health_checks_total` - Health check operation counters

### Grafana Dashboard

The proxy metrics can be visualized in Grafana dashboards showing:

- Request rate and latency
- Success/error rates
- Broker health status
- Load distribution across brokers
- Hash ring statistics

## High Availability

The proxy is configured for high availability:

- **Multiple Replicas**: 2+ proxy instances for redundancy
- **Health Checks**: Continuous broker health monitoring
- **Failover**: Automatic routing to healthy brokers
- **Load Distribution**: Even request distribution via consistent hashing

## Scaling Considerations

When scaling brokers:

1. Update `brokerCount` in proxy configuration
2. The consistent hash algorithm minimizes partition rebalancing
3. Only ~25% of partitions move when adding/removing brokers
4. No downtime required for broker scaling

## Troubleshooting

### Check Proxy Status

```bash
kubectl port-forward svc/msg-queue-proxy 8080:8080
curl http://localhost:8080/status
```

### View Metrics

```bash
curl http://localhost:8080/metrics
curl http://localhost:8080/stats
```

### Check Logs

```bash
kubectl logs -f deployment/msg-queue-proxy
```