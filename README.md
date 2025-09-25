# Telemetry Stack - Comprehensive Documentation

# Prerequisites

[![Go](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.25+-green.svg)](https://kubernetes.io)
[![Docker](https://img.shields.io/badge/Docker-20.10+-blue.svg)](https://docker.com)
[![Helm](https://img.shields.io/badge/Helm-3.0+-yellow.svg)](https://helm.sh)

# Deploy the stack
./quick-deploy.sh

# Clean up (calls cleanup.sh)
./quick-deploy.sh cleanup

# Force cleanup (calls cleanup.sh --force)
./quick-deploy.sh cleanup --force

# Clean up and redeploy
./quick-deploy.sh redeploy

# Architecture Overview

A cloud-native, microservices-based telemetry system designed for high-performance collection, processing, and storage of GPU monitoring data. Built on Kubernetes with Go-based services, providing scalable, fault-tolerant telemetry data pipeline with real-time streaming capabilities.

## 🎯 Table of Contents

- [System Overview](#system-overview)
- [Architecture](#architecture)
- [Service Components](#service-components)
- [Quick Start](#quick-start)
- [Deployment Guide](#deployment-guide)
- [Configuration](#configuration)
- [API Documentation](#api-documentation)
- [Authentication & Security](#authentication--security)
- [Observability & Monitoring](#observability--monitoring)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

---

## 🏗️ System Overview

### Core Philosophy
- **Microservices Architecture**: Decoupled services for scalability and maintainability
- **Cloud-Native Design**: Kubernetes-native deployment with containerized services
- **Event-Driven Processing**: Asynchronous message-based communication
- **Observability-First**: Built-in monitoring, metrics, and health checks
- **Fault Tolerance**: Resilient design with graceful degradation

### Key Features
- ✅ **High Performance**: Memory-first architecture with optimized disk I/O
- ✅ **Horizontal Scalability**: All services designed for horizontal scaling
- ✅ **Data Durability**: Persistent storage with intelligent fallback mechanisms
- ✅ **Real-time Processing**: Server-Sent Events (SSE) for live data streaming
- ✅ **Comprehensive Monitoring**: Prometheus metrics and Grafana dashboards
- ✅ **Multi-layer Security**: API key and bearer token authentication
- ✅ **Production Ready**: Battle-tested patterns and configurations

---

## 🏛️ Architecture

### System Flow
```
GPU Metrics → CSV Files → Streamer Service → Message Queue Proxy → 
Broker Partitions → Collector Service → InfluxDB → API Service → 
Grafana Dashboards
```

### Component Diagram
```
┌─────────────┐    ┌──────────────────┐    ┌────────────────┐
│  Streamers  │────│  Smart Proxy     │────│  Msg Brokers   │
│             │    │                  │    │                │
│ N Instances │    │ - Consistent     │    │ 2 Instances    │
│             │    │   Hashing        │    │ (StatefulSet)  │
└─────────────┘    │ - Health Checks  │    └────────────────┘
                   │ - Request        │            
┌─────────────┐    │   Routing        │
│ Collectors  │────│                  │
│             │    │ Load Balanced    │
│ N Instances │    │ (2+ Replicas)    │
│             │    └──────────────────┘
└─────────────┘
		|
		|
┌────────────────┐
│   InfluxDB     │
│                │
│   Database     │
└────────────────┘
 
```

### Key Design Patterns
1. **Producer-Consumer Pipeline**: Decoupled message processing
2. **Partition-Based Load Distribution**: Consistent hashing for even load
3. **Memory-First with Disk Fallback**: Optimal performance with durability
4. **Retry with Circuit Breaker**: Exponential backoff and failure isolation

---

## 🔧 Service Components

### 1. Streamer Service
**Purpose**: Real-time telemetry data ingestion and streaming

**Key Features**:
- CSV Input-format support (CSV based data ingestion)
- Server-Sent Events (SSE) for real-time data streaming
- Resilient publishing with retry logic (3 attempts for CSV with exponential backoff)
- Graceful error handling prevents service crashes
- Load balancing across multiple broker partitions
- Prometheus metrics for monitoring production rates

**Configuration**:
```yaml
env:
- name: USE_HTTP_QUEUE
  value: "true"
- name: MSG_QUEUE_ADDR
  value: "http://msg-queue-proxy:8080"
- name: MSG_QUEUE_TOPIC
  value: "telemetry"
```

### 2. Message Queue Broker (msg-queue)
**Purpose**: High-performance message broker with persistent storage

**Key Features**:
- **Configurable Queue Capacity**: Environment-driven sizing (QUEUE_SIZE, default 2000)
- **Non-Blocking Operations**: Prevents HTTP handler blocking when queues are full
- **Partition-Based Architecture**: Multiple partitions per topic for parallel processing
- **Intelligent Persistence**: Disk storage only as fallback when in-memory queue is full
- **Visibility Timeout**: Automatic message requeuing (30-second timeout)
- **Dynamic Partition Creation**: On-demand partition creation for load balancing
- Prometheus metrics for monitoring production and consumption rates

**Storage Structure**:
```
./data/
├── telemetry/
│   ├── partition-0.log
│   ├── partition-1.log
│   └── partition-2.log
└── events/
    ├── partition-0.log
    └── partition-1.log
```

**API Endpoints**:
```bash
# Produce Message
POST /produce?topic=<topic>&partition=<partition>

# Consume Messages (SSE)
GET /consume?topic=<topic>&partition=<partition>&group=<group>

# Acknowledge Message
POST /ack?topic=<topic>&partition=<partition>&group=<group>

# Get Topics
GET /topics
```

### 3. Message Queue Proxy (msg-queue-proxy)
**Purpose**: Smart routing layer with consistent hashing for scalable message distribution

**Key Features**:
- **Consistent Hashing**: Minimal rebalancing (~25% vs 83% with modulo hashing)
- **Virtual Nodes**: 150 virtual nodes per broker for even distribution
- **Health Monitoring**: Continuous health checks on all brokers
- **Failover Support**: Automatic routing to healthy brokers
- **Connection Pooling**: Efficient HTTP client with connection reuse

**Configuration**:
```yaml
env:
- name: BROKER_SERVICE
  value: "msg-queue"
- name: BROKER_COUNT
  value: "2"
- name: VIRTUAL_NODES
  value: "150"
- name: MAX_PARTITIONS
  value: "2"
```

### 4. Collector Service
**Purpose**: Message consumption and data transformation for storage

**Key Features**:
- Multi-partition consumption with parallel processing
- InfluxDB integration for time-series data writing
- Configurable processing for different deployment scenarios
- Comprehensive retry logic with exponential backoff
- Prometheus metrics for monitoring consumption rates

### 5. API Service
**Purpose**: RESTful API for telemetry data access and management

**Key Features**:
- Swagger documentation with auto-generated API docs
- Multi-layer authentication (API key and bearer token)
- InfluxDB query interface for efficient time-series data retrieval
- Rate limiting protection against API abuse
- CORS support for web clients

**Protected Endpoints**:
```bash
GET /api/v1/gpus              # List available GPUs
GET /api/v1/gpus/{id}/telemetry  # GPU telemetry data
```

---

## 🚀 Quick Start

### Prerequisites
- Kubernetes cluster (1.25+)
- Docker (20.10+)
- Helm (3.0+)
- Go (1.21+) for development

### 1. Clone Repository
```bash
git clone https://github.com/punithavalliE/telemetry_Punitha.git
cd telemetry_Punitha
```

### 2. Build Docker Images
```bash
# Build all service images
docker build --no-cache -t api:latest -f services/api/Dockerfile .
docker build -t collector:latest -f services/collector/Dockerfile .
docker build -t streamer:latest -f services/streamer/Dockerfile .
docker build -t msg-queue:latest -f services/msg_queue/Dockerfile .
docker build -t msg-queue-proxy:latest -f services/msg_queue_proxy/Dockerfile .
```

### 3. Deploy with Helm
```bash
# Install the complete stack
helm install telemetry ./helm/telemetry-stack --wait

# Check deployment status
kubectl get pods -l release=telemetry
```

### 4. Access Services
```bash
# API Service
kubectl port-forward svc/api-service 8080:8080
# API_KEY: "telemetry-api-secret-2025"

# Grafana Dashboard
kubectl port-forward svc/grafana-service 3000:3000
# Login: admin / admin123

# Prometheus Metrics
kubectl port-forward svc/prometheus-service 9090:9090

# InfluxDB
kubectl port-forward svc/influxdb 8086:8086
# Login: admin / admin123

```

---

## 📦 Deployment Guide

### Kubernetes StatefulSet Deployment (Recommended)

**Benefits of StatefulSet vs Deployment**:
| Feature | StatefulSet | Deployment |
|---------|-------------|------------|
| **Unique Broker Index** | ✅ Auto-assigned (0,1,2) | ❌ All same (0,0,0) |
| **Persistent Storage** | ✅ Per-replica PVC | ❌ Shared PVC conflict |
| **Partition Distribution** | ✅ Proper load balancing | ❌ Uneven distribution |
| **Stable Pod Names** | ✅ Predictable ordering | ❌ Random pod names |

**Configuration**:
```yaml
# values.yaml
msgQueue:
  useStatefulSet: true
  replicaCount: 2
  persistence:
    enabled: true
    size: 5Gi
  env:
    queueSize: "2000"
```

**Deployment Commands**:
```bash
# Deploy with StatefulSet
helm upgrade --install telemetry ./helm/telemetry-stack

# Check StatefulSet status
kubectl get statefulset msg-queue

# Verify broker indices
kubectl logs msg-queue-0 | grep "Starting broker"
kubectl logs msg-queue-1 | grep "Starting broker"
```

### Environment-Specific Deployments

#### Development Environment
```yaml
# dev-values.yaml
global:
  imagePullPolicy: Never

msgQueue:
  replicaCount: 1
  persistence:
    size: 1Gi

influxdb:
  persistence:
    size: 1Gi
```

#### Production Environment
```yaml
# prod-values.yaml
msgQueue:
  replicaCount: 3
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 2000m
      memory: 4Gi
  persistence:
    size: 10Gi

influxdb:
  persistence:
    size: 50Gi
```

---

## ⚙️ Configuration

### Environment Variables

#### Message Queue Configuration
```yaml
QUEUE_SIZE: "2000"                    # Queue capacity per partition
VISIBILITY_TIMEOUT: "30s"            # Message visibility timeout
PARTITIONS_PER_TOPIC: "4"           # Number of partitions per topic
BROKER_COUNT: "3"                   # Number of broker instances
```

#### Database Configuration
```yaml
INFLUXDB_URL: "http://influxdb:8086"
INFLUXDB_TOKEN: "supersecrettoken"
INFLUXDB_ORG: "telemetryorg"
INFLUXDB_BUCKET: "telem_bucket"
```

#### Security Configuration
```yaml
API_KEY: "telemetry-api-secret-2025"
SERVICE_TOKEN: "internal-service-token-2025"
```

### Helm Values Configuration

**Complete Example**:
```yaml
# values.yaml
global:
  imagePullPolicy: Never

security:
  apiKey: "telemetry-api-secret-2025"
  serviceToken: "internal-service-token-2025"

secrets:
  enabled: true
  apiKey: "secure-api-key"
  serviceToken: "secure-service-token"

msgQueue:
  useStatefulSet: true
  replicaCount: 2
  persistence:
    enabled: true
    size: 5Gi
  env:
    queueSize: "2000"

influxdb:
  enabled: true
  persistence:
    enabled: true
    size: 10Gi
  env:
    adminUser: "admin"
    adminPassword: "admin123"
    org: "telemetryorg"
    bucket: "telem_bucket"

prometheus:
  enabled: true
  persistence:
    size: 5Gi

grafana:
  enabled: true
  adminPassword: "admin123"
```

---

## 📚 API Documentation

### Authentication
The API supports two authentication methods:

#### Method 1: X-API-Key Header
```bash
curl -H "X-API-Key: telemetry-api-secret-2025" \
     http://localhost:8080/api/v1/gpus
```

#### Method 2: Authorization Bearer Header
```bash
curl -H "Authorization: Bearer telemetry-api-secret-2025" \
     http://localhost:8080/api/v1/gpus
```

### Public Endpoints (No Authentication)
- `GET /health` - Health check
- `GET /swagger/` - API documentation
- `GET /metrics` - Prometheus metrics

### Protected Endpoints (Authentication Required)
- `GET /api/v1/gpus` - List available GPUs
- `GET /api/v1/gpus/{id}/telemetry` - GPU telemetry data
- `GET /api/v1/hosts` - List available hosts
- `GET /api/v1/namespaces` - List available namespaces
- `POST /telemetry` - Submit telemetry data

### Example API Calls

#### Submit Telemetry Data
```bash
curl -X POST http://localhost:8080/telemetry \
  -H "Content-Type: application/json" \
  -H "X-API-Key: telemetry-api-secret-2025" \
  -d '{
    "device_id": "gpu-001",
    "metric": "temperature",
    "value": 75.5,
    "time": "2025-09-25T10:30:00Z",
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

#### Query GPU Data
```bash
curl -H "X-API-Key: telemetry-api-secret-2025" \
     "http://localhost:8080/api/v1/gpus/gpu-001/telemetry?start=2025-09-25T00:00:00Z&end=2025-09-25T23:59:59Z"
```

---

## 🔐 Authentication & Security

### Kubernetes Secrets Management

#### Option 1: Helm-Managed Secrets (Recommended)
```yaml
# values.yaml
secrets:
  enabled: true
  apiKey: "your-secure-api-key"
  serviceToken: "your-secure-service-token"
```

#### Option 2: Use Existing Secret
```yaml
# values.yaml
secrets:
  enabled: false
  existingSecret: "my-existing-secret"
```

#### Option 3: Manual Secret Creation
```bash
kubectl create secret generic telemetry-auth-secret \
  --from-literal=api-key="your-secure-api-key" \
  --from-literal=service-token="your-secure-service-token"
```

### Secret Rotation
```bash
# Update Helm values with new secrets
helm upgrade telemetry ./helm/telemetry-stack -f updated-values.yaml

# Restart pods to pick up new secrets
kubectl rollout restart deployment/api-service
kubectl rollout restart statefulset/msg-queue
```

### Security Best Practices
- ✅ Use Kubernetes secrets for production deployments
- ✅ Rotate secrets regularly
- ✅ Use strong, randomly generated API keys
- ✅ Enable RBAC for pod-to-pod communication
- ✅ Consider service mesh for mTLS

---

## 📊 Observability & Monitoring

### Prometheus Integration

**Custom Metrics Available**:
- `http_requests_total` - HTTP request count by endpoint and status
- `telemetry_data_points_total` - Total telemetry data points processed
- `message_queue_size` - Current queue size per partition
- `message_processing_duration_seconds` - Message processing latency
- `broker_health_status` - Broker health status (1=healthy, 0=unhealthy)
- `messages_consumed_total` - total messages consumed by collectors
- `messages_produced_total` - total messages produced by streamers

**Example Queries**:
```promql
# Request rate per service
rate(http_requests_total[5m])

#Message processing Speed
message_processing_duration_seconds_bucket

# Processing latency 95th percentile
histogram_quantile(0.95, rate(message_processing_duration_seconds_bucket[5m]))
```

### Grafana Dashboards

**Pre-configured Dashboards**:
1. **Telemetry Services Overview**: System-wide metrics and health
2. **Message Queue Performance**: Queue sizes, throughput, and latency
3. **API Service Metrics**: Request rates, response times, and errors
4. **Infrastructure Monitoring**: CPU, memory, and storage metrics

### Health Checks

**Kubernetes Probes Configuration**:
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 2
```

### Quick Monitoring Setup

#### Access Monitoring Interfaces
```bash
# Prometheus
kubectl port-forward svc/prometheus-service 9090:9090
# Open: http://localhost:9090

# Grafana  
kubectl port-forward svc/grafana-service 3000:3000
# Open: http://localhost:3000 (admin/admin123)

# View raw metrics
kubectl port-forward svc/api-service 8080:8080
curl http://localhost:8080/metrics
```

---

## 🧪 Testing

### Test Suite Overview

**Test Coverage Areas**:
- **API Service**: HTTP handlers, InfluxDB integration, authentication
- **Collector Service**: Message queue consumption, data processing
- **Message Queue**: Partition management, persistence, HTTP API
- **Streamer Service**: Message publishing, CSV processing
- **Test Utilities**: Mocks, sample data, test helpers

### Running Tests

#### Run All Tests
```bash
go test ./...
```

#### Run Tests with Coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

#### Run Specific Service Tests
```bash
# Individual service tests
go test ./services/api
go test ./services/collector
go test ./services/msg_queue
go test ./services/streamer

# With verbose output
go test -v ./services/...

# With race detection
go test -race ./...
```

#### PowerShell Test Script
```powershell
# Windows users can use the provided script
.\run_tests.ps1
```

### Test Utilities

**Mock Implementations**:
- `MockInfluxWriter`: Simulates InfluxDB operations
- `MockMessageQueue`: Configurable message queue behavior
- `MockLogger`: Silent logger for testing

**Sample Data**:
- Pre-defined telemetry data samples
- CSV test files
- JSON test payloads

### Integration Testing

**End-to-End Test Flow**:
```bash
# 1. Deploy test environment
helm install test-telemetry ./helm/telemetry-stack -f test-values.yaml

# 2. Run integration tests
go test ./tests/integration/...

# 3. Cleanup
helm uninstall test-telemetry
```

---

## 🔧 Troubleshooting

### Common Issues

#### 1. Pod Startup Issues
```bash
# Check pod status
kubectl get pods -l app=msg-queue

# View pod logs
kubectl logs -f msg-queue-0

# Describe pod for events
kubectl describe pod msg-queue-0
```

#### 2. Message Queue Connection Issues
```bash
# Test broker connectivity
kubectl exec -it msg-queue-0 -- curl localhost:8080/topics

# Check proxy health
kubectl port-forward svc/msg-queue-proxy 8080:8080
curl http://localhost:8080/health
```

#### 3. Storage Issues
```bash
# Check persistent volumes
kubectl get pv,pvc

# Verify storage usage
kubectl exec -it msg-queue-0 -- df -h /data
```

#### 4. Authentication Problems
```bash
# Verify secrets exist
kubectl get secrets

# Check secret contents
kubectl get secret telemetry-auth-secret -o yaml

# Test API authentication
curl -H "X-API-Key: $(kubectl get secret telemetry-auth-secret -o jsonpath='{.data.api-key}' | base64 -d)" \
     http://localhost:8080/health
```

### Performance Tuning

#### Queue Size Optimization
```yaml
# For high-throughput scenarios
msgQueue:
  env:
    queueSize: "5000"
    visibilityTimeout: "120s"
```

#### Resource Allocation
```yaml
# Production resource settings
resources:
  requests:
    cpu: 1000m
    memory: 2Gi
  limits:
    cpu: 2000m
    memory: 4Gi
```

#### Database Performance
```yaml
# InfluxDB optimization
influxdb:
  env:
    # Increase batch size for better throughput
    batchSize: "10000"
    # Adjust retention policy
    retentionDuration: "720h"  # 30 days
```

### Debugging Commands

#### Service Health Checks
```bash
# Check all service health
for service in api-service collector-service streamer-service msg-queue-proxy; do
  echo "=== $service ==="
  kubectl port-forward svc/$service 8080:8080 &
  sleep 2
  curl -s http://localhost:8080/health || echo "Health check failed"
  pkill -f "port-forward svc/$service"
done
```

#### Message Flow Verification
```bash
# 1. Check producer (streamer)
kubectl logs -f deployment/streamer-service | grep "Published message"

# 2. Check proxy routing
kubectl logs -f deployment/msg-queue-proxy | grep "Routing to broker"

# 3. Check broker queues
kubectl exec msg-queue-0 -- curl -s localhost:8080/topics | jq

# 4. Check consumer (collector)
kubectl logs -f deployment/collector-service | grep "Processed message"
```

---

## 🤝 Contributing

### Development Setup

#### Prerequisites
- Go 1.21+
- Docker 20.10+
- Kubernetes cluster (minikube, kind, or full cluster)
- Helm 3.0+

#### Local Development
```bash
# Clone repository
git clone https://github.com/punithavalliE/telemetry_Punitha.git
cd telemetry_Punitha

# Install dependencies
go mod download

# Run tests
go test ./...

# Build services locally
make build-all
```

#### Code Quality
```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Security scan
gosec ./...

# Generate documentation
swag init -g services/api/main.go
```

### Contribution Guidelines

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

### Code Standards
- Follow Go conventions and best practices
- Write comprehensive unit tests (minimum 80% coverage)  
- Update documentation for new features
- Use meaningful commit messages
- Include integration tests for API changes

---

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 📞 Support

- **Documentation**: Check this README and individual service docs
- **Issues**: GitHub Issues for bug reports and feature requests  
- **Discussions**: GitHub Discussions for questions and community support

---

**Last Updated**: September 25, 2025  
**Version**: 2.0.0  
**Maintainer**: Telemetry Team