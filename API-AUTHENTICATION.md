# API Authentication

The Telemetry API uses a simple shared secret authentication mechanism for protecting REST API endpoints.

## Authentication Methods

### Method 1: X-API-Key Header
```bash
curl -H "X-API-Key: your-secret-key" http://localhost:8080/api/v1/gpus
```

### Method 2: Authorization Bearer Header
```bash
curl -H "Authorization: Bearer your-secret-key" http://localhost:8080/gpus
```

## Public Endpoints (No Authentication Required)

- `GET /health` - Health check
- `GET /swagger/` - API documentation

## Protected Endpoints (Authentication Required)

- `GET /gpus` - Recent telemetry data (legacy)
- `GET /api/v1/gpus` - List available GPUs
- `GET /api/v1/gpus/{id}/telemetry` - GPU telemetry data
- `GET /api/v1/hosts` - List available hosts
- `GET /api/v1/namespaces` - List available namespaces

## Configuration

### Environment Variables

Set the API secret using the `API_KEY` environment variable:

```bash
export API_KEY="your-secret-key-here"
```

### Kubernetes Deployment

#### Option 1: Using Kubernetes Secrets (Recommended)

The Helm chart automatically creates and manages Kubernetes secrets:

```yaml
# values.yaml
secrets:
  enabled: true
  apiKey: "your-secure-api-key"
  serviceToken: "your-secure-service-token"
```

#### Option 2: Direct Environment Variables

Or set directly in Helm values (less secure):

```yaml
# values.yaml
env:
- name: API_KEY
  value: "your-secret-key-here"
```

**⚠️ Important: Use Kubernetes secrets for production deployments!**

See [KUBERNETES-SECRETS.md](./KUBERNETES-SECRETS.md) for detailed secret management instructions.

### Default Secret

If no `API_KEY` environment variable is set, the system uses a default secret:
`default-secret-key-change-in-production`

**⚠️ Important: Always change the default secret in production!**

## Client Examples

### PowerShell
```powershell
$headers = @{ "X-API-Key" = "your-secret-key" }
$response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/gpus" -Headers $headers
```

### curl
```bash
curl -H "X-API-Key: your-secret-key" http://localhost:8080/api/v1/gpus
```

### Python
```python
import requests

headers = {"X-API-Key": "your-secret-key"}
response = requests.get("http://localhost:8080/api/v1/gpus", headers=headers)
data = response.json()
```

## Error Responses

### 401 Unauthorized
Returned when:
- No API key is provided
- Invalid API key is provided

```json
{
  "error": "Unauthorized: Invalid API key"
}
```

## Security Features

- **Constant-time comparison**: Prevents timing attacks
- **Flexible header support**: Both `X-API-Key` and `Authorization: Bearer` formats
- **Selective protection**: Health and documentation endpoints remain public
- **Environment-based configuration**: Easy to change secrets without code changes

## Service-to-Service Authentication

Internal services (like collector, streamer) use a separate `SERVICE_TOKEN` for authenticating with the message queue service. This is automatically handled by the application.