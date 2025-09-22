#!/bin/bash

# API Endpoint and Secret Configuration
API_BASE_URL="http://localhost:8080"
API_SECRET="your-secret-key-here"

echo "=== Telemetry API Client Example ==="
echo "Base URL: $API_BASE_URL"
echo "Authentication: Using API key"
echo ""

# Health check (no auth required)
echo "1. Health Check (no auth):"
curl -s "$API_BASE_URL/health"
echo -e "\n"

# Test authenticated endpoints
echo "2. Get GPUs (with API key):"
curl -s -H "X-API-Key: $API_SECRET" "$API_BASE_URL/api/v1/gpus" | jq '.'
echo ""

echo "3. Get recent telemetry (with Bearer token):"
curl -s -H "Authorization: Bearer $API_SECRET" "$API_BASE_URL/gpus" | jq '.'
echo ""

echo "4. Get hosts (with API key):"
curl -s -H "X-API-Key: $API_SECRET" "$API_BASE_URL/api/v1/hosts" | jq '.'
echo ""

echo "5. Test unauthorized access (no key):"
curl -s "$API_BASE_URL/api/v1/gpus"
echo -e "\n"

echo "=== Authentication Methods ==="
echo "Method 1: X-API-Key header"
echo "  curl -H 'X-API-Key: your-secret' $API_BASE_URL/api/v1/gpus"
echo ""
echo "Method 2: Authorization Bearer header"
echo "  curl -H 'Authorization: Bearer your-secret' $API_BASE_URL/gpus"
echo ""