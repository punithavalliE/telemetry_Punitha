# Kubernetes Secrets Management

This document describes how to manage API keys and service tokens using Kubernetes secrets in the telemetry stack.

## Overview

The telemetry stack uses Kubernetes secrets to securely store:
- **API Key**: For external REST API authentication
- **Service Token**: For internal service-to-service communication

## Secret Structure

The Helm chart creates a secret named `<release-name>-telemetry-stack-auth` with the following keys:
- `api-key`: Base64 encoded API key for REST API access
- `service-token`: Base64 encoded token for internal service communication

## Configuration Options

### Option 1: Helm-Managed Secrets (Default)

The Helm chart automatically creates and manages secrets for you:

```yaml
# values.yaml
secrets:
  enabled: true
  apiKey: "your-secure-api-key-here"
  serviceToken: "your-secure-service-token-here"
```

### Option 2: Use Existing Secret

If you have an existing secret, you can reference it:

```yaml
# values.yaml
secrets:
  enabled: false  # Don't create new secret
  existingSecret: "my-existing-secret"
```

Your existing secret must have keys `api-key` and `service-token`.

## Manual Secret Management

### Create Secret Manually

```bash
# Create secret with kubectl
kubectl create secret generic telemetry-auth-secret \
  --from-literal=api-key="your-secure-api-key" \
  --from-literal=service-token="your-secure-service-token" \
  --namespace=default

# Or using a YAML file
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: telemetry-auth-secret
  namespace: default
type: Opaque
data:
  api-key: $(echo -n "your-secure-api-key" | base64)
  service-token: $(echo -n "your-secure-service-token" | base64)
EOF
```

### View Secret Contents

```bash
# List secrets
kubectl get secrets

# View secret details (shows keys but not values)
kubectl describe secret <release-name>-telemetry-stack-auth

# Decode secret values (use carefully)
kubectl get secret <release-name>-telemetry-stack-auth -o jsonpath='{.data.api-key}' | base64 -d
kubectl get secret <release-name>-telemetry-stack-auth -o jsonpath='{.data.service-token}' | base64 -d
```

## Secret Rotation

### Method 1: Update Helm Values and Upgrade

1. Update your values.yaml with new secrets:
```yaml
secrets:
  apiKey: "new-secure-api-key"
  serviceToken: "new-secure-service-token"
```

2. Upgrade the Helm release:
```bash
helm upgrade my-telemetry ./helm/telemetry-stack -f values.yaml
```

3. Restart pods to pick up new secrets:
```bash
kubectl rollout restart deployment/api
kubectl rollout restart deployment/collector
kubectl rollout restart deployment/streamer
kubectl rollout restart statefulset/msg-queue
```

### Method 2: Update Secret Directly

1. Update the secret:
```bash
kubectl patch secret <release-name>-telemetry-stack-auth \
  -p='{"data":{"api-key":"'$(echo -n "new-api-key" | base64)'"}}'

kubectl patch secret <release-name>-telemetry-stack-auth \
  -p='{"data":{"service-token":"'$(echo -n "new-service-token" | base64)'"}}'
```

2. Restart deployments:
```bash
kubectl rollout restart deployment/api
kubectl rollout restart deployment/collector
kubectl rollout restart deployment/streamer
kubectl rollout restart statefulset/msg-queue
```

## Security Best Practices

### 1. Use Strong, Random Keys

Generate cryptographically secure random keys:

```bash
# Generate 32-byte random API key
openssl rand -hex 32

# Generate UUID-based tokens
uuidgen

# Use password generators
pwgen -s 32 1
```

### 2. Limit Secret Access

Use RBAC to restrict who can view secrets:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secret-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["telemetry-auth-secret"]
  verbs: ["get", "list"]
```

### 3. Enable Secret Encryption at Rest

Ensure your Kubernetes cluster has encryption at rest enabled for etcd.

### 4. Regular Rotation

Implement a secret rotation schedule:
- API keys: Rotate monthly
- Service tokens: Rotate quarterly
- Keep track of rotation in your change management system

### 5. Monitoring and Auditing

Monitor secret access:

```bash
# Check audit logs for secret access
kubectl logs -n kube-system -l component=kube-apiserver | grep "secrets.*telemetry"
```

## Troubleshooting

### Secret Not Found Error

```bash
# Check if secret exists
kubectl get secret <release-name>-telemetry-stack-auth

# Check if it's in the correct namespace
kubectl get secret --all-namespaces | grep telemetry
```

### Authentication Failures

1. Verify secret values are correct:
```bash
# Check API key
kubectl get secret <secret-name> -o jsonpath='{.data.api-key}' | base64 -d

# Test with curl
curl -H "X-API-Key: $(kubectl get secret <secret-name> -o jsonpath='{.data.api-key}' | base64 -d)" \
  http://localhost:8080/api/v1/gpus
```

2. Check pod environment variables:
```bash
kubectl exec -it <pod-name> -- env | grep -E "(API_KEY|SERVICE_TOKEN)"
```

### Secret Update Not Reflected

Kubernetes doesn't automatically restart pods when secrets change. You must manually restart:

```bash
kubectl rollout restart deployment/api
```

## Example Commands

### Deploy with Custom Secrets

```bash
# Create values file with your secrets
cat > my-secrets.yaml <<EOF
secrets:
  enabled: true
  apiKey: "$(openssl rand -hex 32)"
  serviceToken: "$(uuidgen)"
EOF

# Deploy
helm install my-telemetry ./helm/telemetry-stack -f my-secrets.yaml
```

### Complete Secret Rotation

```bash
#!/bin/bash
NEW_API_KEY=$(openssl rand -hex 32)
NEW_SERVICE_TOKEN=$(uuidgen)

echo "Rotating secrets..."
echo "New API Key: $NEW_API_KEY"
echo "New Service Token: $NEW_SERVICE_TOKEN"

# Update secret
kubectl patch secret my-telemetry-telemetry-stack-auth \
  -p="{\"data\":{\"api-key\":\"$(echo -n $NEW_API_KEY | base64)\",\"service-token\":\"$(echo -n $NEW_SERVICE_TOKEN | base64)\"}}"

# Restart all services
kubectl rollout restart deployment/api
kubectl rollout restart deployment/collector
kubectl rollout restart deployment/streamer
kubectl rollout restart statefulset/msg-queue

echo "Secret rotation complete!"
```

## References

- [Kubernetes Secrets Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)
- [Helm Secrets Management](https://helm.sh/docs/chart_best_practices/secrets/)
- [RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)