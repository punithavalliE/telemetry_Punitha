# Message Queue StatefulSet Deployment Guide

## What this StatefulSet provides:

### ✅ Proper Broker Partitioning
- **msg-queue-0**: BROKER_INDEX=0
- **msg-queue-1**: BROKER_INDEX=1  
- **msg-queue-2**: BROKER_INDEX=2

### ✅ Persistent Storage Per Broker
- Each pod gets its own PVC: `msg-queue-storage-msg-queue-0`, etc.
- Data survives pod restarts and rescheduling

### ✅ Stable Network Identity
- Predictable pod names: `msg-queue-0`, `msg-queue-1`, `msg-queue-2`
- Headless service for direct pod access: `msg-queue-0.msg-queue-headless.default.svc.cluster.local`

### ✅ Load Balancer Service
- External access via LoadBalancer service
- Traffic distributed across all replicas

## Deployment Commands:

```bash
# Deploy with StatefulSet (default configuration)
cd C:\Users\peaswaran\telemetry_Punitha
helm upgrade --install telemetry ./helm/telemetry-stack

# Check StatefulSet status
kubectl get statefulset msg-queue

# Check pods and their broker indices
kubectl get pods -l app=msg-queue
kubectl logs msg-queue-0 | grep "Starting broker"
kubectl logs msg-queue-1 | grep "Starting broker"  
kubectl logs msg-queue-2 | grep "Starting broker"

# Check persistent volumes
kubectl get pvc

# Test the load balancer
kubectl get service msg-queue
EXTERNAL_IP=$(kubectl get service msg-queue -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl http://${EXTERNAL_IP}:8080/topics
```

## Switch Between Deployment Types:

### Use StatefulSet (Recommended for production):
```yaml
msgQueue:
  useStatefulSet: true
  replicaCount: 3
  persistence:
    enabled: true
```

### Use Regular Deployment (For development):
```yaml
msgQueue:
  useStatefulSet: false
  replicaCount: 1  # Must be 1 for persistence
  persistence:
    enabled: true
```

## Benefits of StatefulSet vs Deployment:

| Feature | StatefulSet | Deployment |
|---------|-------------|------------|
| **Unique Broker Index** | ✅ Auto-assigned (0,1,2) | ❌ All same (0,0,0) |
| **Persistent Storage** | ✅ Per-replica PVC | ❌ Shared PVC conflict |
| **Partition Distribution** | ✅ Proper load balancing | ❌ Uneven distribution |
| **Stable Pod Names** | ✅ Predictable ordering | ❌ Random pod names |
| **Rolling Updates** | ✅ Ordered updates | ✅ Parallel updates |

## Troubleshooting:

```bash
# Check broker assignments
for i in 0 1 2; do
  echo "=== Broker $i ==="
  kubectl exec msg-queue-$i -- env | grep BROKER
done

# Test individual brokers
kubectl exec msg-queue-0 -- curl localhost:8080/topics
kubectl exec msg-queue-1 -- curl localhost:8080/topics  
kubectl exec msg-queue-2 -- curl localhost:8080/topics

# Check partition ownership
kubectl exec msg-queue-0 -- curl localhost:8080/topics | jq
```