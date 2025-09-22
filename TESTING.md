# Telemetry Unit Tests

This directory contains comprehensive unit tests for all services in the telemetry system.

## Overview

The test suite includes:

### 1. API Service Tests (`services/api/`)
- **main_test.go**: Tests for HTTP handlers, endpoints, and InfluxDB integration
- **models_test.go**: Tests for data models and JSON serialization

### 2. Collector Service Tests (`services/collector/`)
- **main_test.go**: Tests for message queue integration and telemetry processing

### 3. Message Queue Service Tests (`services/msg_queue/`)
- **main_test.go**: Tests for partition management, message persistence, and HTTP API

### 4. Streamer Service Tests (`services/streamer/`)
- **main_test.go**: Tests for message publishing and queue integration
- **stream_csv_test.go**: Tests for CSV processing functionality

### 5. Test Utilities (`internal/testutils/`)
- **testutils.go**: Common test utilities and helpers
- **mocks/mocks.go**: Mock implementations for external dependencies
- **files.go**: File and directory testing utilities

## Running Tests

### Run All Tests
```bash
# From the root directory
go test ./...
```

### Run Tests with Coverage
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Run Specific Service Tests
```bash
# API service tests
go test ./services/api

# Collector service tests
go test ./services/collector

# Message queue tests
go test ./services/msg_queue

# Streamer service tests
go test ./services/streamer
```

### Run Tests with Verbose Output
```bash
go test -v ./...
```

### Run Tests with Race Detection
```bash
go test -race ./...
```

## Test Features

### Mock Implementations
- **MockInfluxWriter**: Simulates InfluxDB operations for testing
- **MockMessageQueue**: Simulates message queue operations with configurable behavior
- **MockLogger**: Silent logger for testing

### Test Utilities
- **Sample Data Generation**: Pre-defined test data for telemetry, CSV, and JSON
- **Environment Setup**: Automated test environment configuration
- **File Operations**: Temporary file and directory management
- **Assertion Helpers**: Common test assertions and error checking

### Test Coverage Areas

#### API Service
- HTTP endpoint handlers
- Request/response validation
- Error handling
- InfluxDB integration
- JSON serialization/deserialization
- Environment variable configuration

#### Collector Service
- Message queue consumption
- Telemetry data processing
- InfluxDB point writing
- Error handling and recovery
- Environment configuration

#### Message Queue Service
- Message production and consumption
- Partition management
- Message persistence
- HTTP API endpoints
- Visibility timeout handling
- Acknowledgment processing

#### Streamer Service
- CSV file processing
- Message publishing
- Data validation
- Error handling
- Queue integration

## Test Data

### Sample Telemetry Data
```go
TestTelemetryData{
    DeviceID:  "nvidia0",
    Metric:    "DCGM_FI_DEV_GPU_UTIL", 
    Value:     85.5,
    GPUID:     "0",
    UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
    ModelName: "NVIDIA H100 80GB HBM3",
    Hostname:  "mtv5-dgx1-hgpu-031",
    // ... additional fields
}
```

### Sample CSV Format
```csv
timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
2023-07-18T20:42:34Z,DCGM_FI_DEV_GPU_UTIL,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,test-pod,default,85.5,"version=535.129.03"
```

## Configuration

Tests use environment variables for configuration. The test utilities automatically set up and clean up test environments:

```go
// Automatically configured in tests
INFLUXDB_URL=http://test-influxdb:8086
INFLUXDB_TOKEN=test-token
INFLUXDB_ORG=test-org
INFLUXDB_BUCKET=test-bucket
USE_HTTP_QUEUE=true
MSG_QUEUE_ADDR=http://test-queue:8080
```

## CI/CD Integration

The tests are designed to run in CI/CD environments:

```bash
# GitHub Actions example
- name: Run Tests
  run: |
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

- name: Upload Coverage
  uses: actions/upload-artifact@v2
  with:
    name: coverage-report
    path: coverage.html
```

## Best Practices

1. **Isolation**: Each test is isolated and doesn't depend on external services
2. **Cleanup**: Automatic cleanup of temporary files and directories
3. **Mocking**: All external dependencies are mocked for reliable testing
4. **Coverage**: Comprehensive test coverage across all code paths
5. **Performance**: Tests run quickly without external network dependencies

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Tests use mock services, no real ports needed
2. **File Permissions**: Temporary files are created with appropriate permissions
3. **Environment Variables**: Test utilities handle all environment setup
4. **Race Conditions**: Tests include race detection capabilities

### Debug Mode
Enable verbose output for debugging:
```bash
go test -v -race ./services/api
```

## Contributing

When adding new tests:

1. Follow the existing test patterns
2. Use the provided test utilities and mocks
3. Ensure proper cleanup in test teardown
4. Add both positive and negative test cases
5. Update this documentation as needed
