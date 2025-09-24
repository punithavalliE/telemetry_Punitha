# 🚀 One-Time Telemetry Stack Deployment Scripts

This repository provides ready-to-use scripts to deploy the complete telemetry monitoring stack on a fresh Kubernetes cluster in minutes.

## 📋 Quick Reference

| Script | Platform | Description |
|--------|----------|-------------|
| `quick-deploy.ps1` | Windows | Fastest deployment (one-liner style) |
| `quick-deploy.sh` | Linux/macOS | Fastest deployment (one-liner style) |
| `deploy-telemetry-stack.ps1` | Windows | Full-featured with options |
| `deploy-telemetry-stack.sh` | Linux/macOS | Full-featured with options |

## ⚡ Fastest Deployment (30 seconds)

### Windows
```powershell
.\quick-deploy.ps1
```

### Linux/macOS
```bash
chmod +x quick-deploy.sh
./quick-deploy.sh
```

## 🔧 Full Deployment with Options

### Windows PowerShell
```powershell
# Basic deployment
.\deploy-telemetry-stack.ps1

# Custom namespace
.\deploy-telemetry-stack.ps1 -Namespace "monitoring"

# Skip image building (if images exist)
.\deploy-telemetry-stack.ps1 -SkipImageBuild

# Cleanup
.\deploy-telemetry-stack.ps1 --cleanup
```

### Linux/macOS Bash
```bash
# Basic deployment
./deploy-telemetry-stack.sh

# Custom namespace
./deploy-telemetry-stack.sh --namespace monitoring

# Skip image building
./deploy-telemetry-stack.sh --skip-image-build

# Cleanup
./deploy-telemetry-stack.sh --cleanup
```

## 📊 What You Get

After deployment, you'll have a complete telemetry monitoring stack:

- **Grafana**: Dashboards at http://localhost:3000 (admin/admin123)
- **Prometheus**: Metrics at http://localhost:9090
- **InfluxDB**: Time-series DB at http://localhost:8086 (admin/admin123)
- **API Service**: REST API at http://localhost:8080
- **Message Queue**: Redis-based broker with load balancing
- **Data Collection**: Automated telemetry data streaming

## 🎯 Quick Access Commands

```bash
# Access Grafana
kubectl port-forward svc/telemetry-stack-grafana 3000:3000 -n telemetry

# Access Prometheus
kubectl port-forward svc/telemetry-stack-prometheus 9090:9090 -n telemetry

# Access InfluxDB
kubectl port-forward svc/telemetry-stack-influxdb 8086:8086 -n telemetry

# Access API
kubectl port-forward svc/telemetry-stack-api 8080:8080 -n telemetry

# Test API
curl -H "X-API-Key: telemetry-api-secret-2025" http://localhost:8080/health
```

## 📋 Prerequisites

1. **Kubernetes Cluster** (minikube, Docker Desktop, GKE, EKS, etc.)
2. **kubectl** configured and connected
3. **Helm 3+** installed
4. **Docker** running (for building images)

## 🏗️ Architecture

```
CSV Data → Streamer → Message Queue → Collector → InfluxDB
                          ↓              ↓         ↓
                    Message Queue    API Service  ← Prometheus
                       Proxy                      ↓
                                               Grafana
```

## 🔐 Default Credentials

- **Grafana**: admin / admin123
- **InfluxDB**: admin / admin123
- **API Key**: telemetry-api-secret-2025
- **Organization**: telemetryorg
- **Bucket**: telem_bucket

## 📚 Detailed Documentation

For comprehensive setup instructions, troubleshooting, and configuration options, see:
- **[DEPLOYMENT-GUIDE.md](DEPLOYMENT-GUIDE.md)** - Complete deployment guide
- **[helm/telemetry-stack/values.yaml](helm/telemetry-stack/values.yaml)** - Configuration options

## 🚨 Troubleshooting

### Quick Diagnostics
```bash
# Check all resources
kubectl get all -n telemetry

# Check pod logs
kubectl logs -f deployment/telemetry-stack-api -n telemetry

# Check Helm status
helm status telemetry-stack -n telemetry
```

### Common Issues
- **Images not building**: Ensure Docker is running
- **Pods not starting**: Check resource limits and storage classes
- **Can't access services**: Verify port-forwarding commands
- **Authentication fails**: Check API key in headers

## 🧹 Cleanup

```bash
# Remove everything
helm uninstall telemetry-stack -n telemetry
kubectl delete namespace telemetry
```

---

**Ready to deploy?** Just run the quick-deploy script for your platform! 🎉