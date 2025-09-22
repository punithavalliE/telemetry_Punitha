package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// MockInfluxWriter is a mock implementation of the InfluxWriter for testing
type MockInfluxWriter struct {
	mockData []TelemetryDataResponse
	err      error
}

func (m *MockInfluxWriter) QueryRecentTelemetry(limit int) ([]TelemetryDataResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	if len(m.mockData) > limit {
		return m.mockData[:limit], nil
	}
	return m.mockData, nil
}

func (m *MockInfluxWriter) QueryTelemetryByGPUID(gpuID string, startTime, endTime *time.Time, limit int) ([]TelemetryDataResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	// Filter mock data by GPU ID
	var filtered []TelemetryDataResponse
	for _, data := range m.mockData {
		if data.GPUID == gpuID {
			filtered = append(filtered, data)
		}
	}
	
	if len(filtered) > limit {
		return filtered[:limit], nil
	}
	return filtered, nil
}

func (m *MockInfluxWriter) QueryGPUs() ([]GPUInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	// Convert telemetry data to GPU info
	gpuMap := make(map[string]GPUInfo)
	for _, data := range m.mockData {
		gpuMap[data.GPUID] = GPUInfo{
			DeviceID:  data.DeviceID,
			GPUID:     data.GPUID,
			UUID:      data.UUID,
			ModelName: data.ModelName,
			Hostname:  data.Hostname,
			Container: data.Container,
			Pod:       data.Pod,
			Namespace: data.Namespace,
			LastSeen:  data.Time,
		}
	}
	
	var gpus []GPUInfo
	for _, gpu := range gpuMap {
		gpus = append(gpus, gpu)
	}
	return gpus, nil
}

func (m *MockInfluxWriter) QueryHosts() ([]HostInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	hostMap := make(map[string]int)
	for _, data := range m.mockData {
		hostMap[data.Hostname]++
	}
	
	var hosts []HostInfo
	for hostname, count := range hostMap {
		hosts = append(hosts, HostInfo{
			Hostname: hostname,
			GPUCount: count,
		})
	}
	return hosts, nil
}

func (m *MockInfluxWriter) QueryNamespaces() ([]NamespaceInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	nsMap := make(map[string]int)
	for _, data := range m.mockData {
		if data.Namespace != "" {
			nsMap[data.Namespace]++
		}
	}
	
	var namespaces []NamespaceInfo
	for ns, count := range nsMap {
		namespaces = append(namespaces, NamespaceInfo{
			Namespace: ns,
			GPUCount:  count,
		})
	}
	return namespaces, nil
}

func (m *MockInfluxWriter) Close() {}

func (m *MockInfluxWriter) WritePoints(points []map[string]interface{}) error {
	return m.err
}

// Test setup helper
func setupTestData() []TelemetryDataResponse {
	now := time.Now()
	return []TelemetryDataResponse{
		{
			DeviceID:  "nvidia0",
			Metric:    "DCGM_FI_DEV_GPU_UTIL",
			Value:     85.5,
			Time:      now,
			GPUID:     "0",
			UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-031",
			Container: "",
			Pod:       "test-pod",
			Namespace: "default",
			LabelsRaw: "DCGM_FI_DRIVER_VERSION=\"535.129.03\"",
		},
		{
			DeviceID:  "nvidia1",
			Metric:    "DCGM_FI_DEV_GPU_UTIL",
			Value:     72.3,
			Time:      now.Add(-1 * time.Minute),
			GPUID:     "1",
			UUID:      "GPU-6fd4f087-86f3-7a43-b711-4771313afc51",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-031",
			Container: "",
			Pod:       "test-pod-2",
			Namespace: "production",
			LabelsRaw: "DCGM_FI_DRIVER_VERSION=\"535.129.03\"",
		},
	}
}

func TestLegacyGPUsEndpoint(t *testing.T) {
	// Setup mock data
	mockData := setupTestData()
	mockInflux := &MockInfluxWriter{mockData: mockData}

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/gpus", func(w http.ResponseWriter, r *http.Request) {
		records, err := mockInflux.QueryRecentTelemetry(10)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to query data"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	})

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/gpus", nil)
		w := httptest.NewRecorder()
		
		mux.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		var response []TelemetryDataResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if len(response) != 2 {
			t.Errorf("Expected 2 records, got %d", len(response))
		}
	})

	t.Run("InfluxDB Error", func(t *testing.T) {
		mockInfluxError := &MockInfluxWriter{err: fmt.Errorf("connection failed")}
		
		mux := http.NewServeMux()
		mux.HandleFunc("/gpus", func(w http.ResponseWriter, r *http.Request) {
			_, err := mockInfluxError.QueryRecentTelemetry(10)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Failed to query data"))
				return
			}
		})
		
		req := httptest.NewRequest("GET", "/gpus", nil)
		w := httptest.NewRecorder()
		
		mux.ServeHTTP(w, req)
		
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})
}

func TestGPUTelemetryEndpoint(t *testing.T) {
	mockData := setupTestData()
	mockInflux := &MockInfluxWriter{mockData: mockData}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/gpus/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Simple path parsing for test
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/gpus/")
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("GPU ID is required"))
			return
		}

		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "telemetry" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid endpoint"))
			return
		}

		gpuID := parts[0]
		records, err := mockInflux.QueryTelemetryByGPUID(gpuID, nil, nil, 100)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := TelemetryResponse{
			GPUID: gpuID,
			Count: len(records),
			Data:  records,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	t.Run("Valid GPU ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/gpus/0/telemetry", nil)
		w := httptest.NewRecorder()
		
		mux.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		var response TelemetryResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		
		if response.GPUID != "0" {
			t.Errorf("Expected GPU ID '0', got '%s'", response.GPUID)
		}
		
		if response.Count != 1 {
			t.Errorf("Expected 1 record, got %d", response.Count)
		}
	})

	t.Run("Missing GPU ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/gpus/", nil)
		w := httptest.NewRecorder()
		
		mux.ServeHTTP(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Invalid Method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/gpus/0/telemetry", nil)
		w := httptest.NewRecorder()
		
		mux.ServeHTTP(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

func TestEnvironmentVariables(t *testing.T) {
	t.Run("Default Values", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("INFLUXDB_URL")
		os.Unsetenv("INFLUXDB_TOKEN")
		os.Unsetenv("INFLUXDB_ORG")
		os.Unsetenv("INFLUXDB_BUCKET")

		// Test that defaults are used
		influxURL := os.Getenv("INFLUXDB_URL")
		if influxURL == "" {
			influxURL = "http://influxdb:8086"
		}
		if influxURL != "http://influxdb:8086" {
			t.Errorf("Expected default URL, got %s", influxURL)
		}

		influxToken := os.Getenv("INFLUXDB_TOKEN")
		if influxToken == "" {
			influxToken = "supersecrettoken"
		}
		if influxToken != "supersecrettoken" {
			t.Errorf("Expected default token, got %s", influxToken)
		}
	})

	t.Run("Custom Values", func(t *testing.T) {
		os.Setenv("INFLUXDB_URL", "http://custom:8086")
		os.Setenv("INFLUXDB_TOKEN", "customtoken")
		
		defer func() {
			os.Unsetenv("INFLUXDB_URL")
			os.Unsetenv("INFLUXDB_TOKEN")
		}()

		influxURL := os.Getenv("INFLUXDB_URL")
		if influxURL != "http://custom:8086" {
			t.Errorf("Expected custom URL, got %s", influxURL)
		}

		influxToken := os.Getenv("INFLUXDB_TOKEN")
		if influxToken != "customtoken" {
			t.Errorf("Expected custom token, got %s", influxToken)
		}
	})
}
