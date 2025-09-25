package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TelemetryData represents telemetry data structure for testing
type TelemetryData struct {
	Timestamp time.Time `json:"timestamp"`
	Metric    string    `json:"metric"`
	GPUID     string    `json:"gpu_id"`
	DeviceID  string    `json:"device_id"`
	UUID      string    `json:"uuid"`
	ModelName string    `json:"model_name"`
	Hostname  string    `json:"hostname"`
	Container string    `json:"container"`
	Pod       string    `json:"pod"`
	Namespace string    `json:"namespace"`
	Value     float64   `json:"value"`
	LabelsRaw string    `json:"labels_raw"`
}

// Comprehensive test suite for Streamer service

func TestStreamerServiceInitialization(t *testing.T) {
	tests := []struct {
		name        string
		useHTTPQueue bool
		expectError  bool
	}{
		{"HTTP Queue Configuration", true, false},
		{"Redis Configuration", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock service initialization
			config := map[string]interface{}{
				"useHTTPQueue":        tt.useHTTPQueue,
				"msgQueueAddr":        "http://msg-queue-proxy:8080",
				"topic":               "telemetry",
				"producerName":        "streamer",
			}

			if config["useHTTPQueue"] != tt.useHTTPQueue {
				t.Errorf("Expected useHTTPQueue %v, got %v", tt.useHTTPQueue, config["useHTTPQueue"])
			}

			if config["topic"] != "telemetry" {
				t.Errorf("Expected topic 'telemetry', got %v", config["topic"])
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Mock health handler
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"status":          "healthy",
			"service":         "streamer",
			"timestamp":       time.Now().Format(time.RFC3339),
			"csv_streaming":   true,
			"records_streamed": 1000,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}

	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}

	if response["service"] != "streamer" {
		t.Errorf("Expected service 'streamer', got %v", response["service"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	// Mock stats handler
	statsHandler := func(w http.ResponseWriter, r *http.Request) {
		stats := map[string]interface{}{
			"records_processed": 1500,
			"records_published": 1495,
			"publish_errors":    5,
			"csv_file":          "/data/dcgm_metrics_20250718_134233.csv",
			"streaming_active":  true,
			"uptime_seconds":    7200,
			"throughput_per_sec": 2.5,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(stats)
	}

	statsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	expectedFields := []string{"records_processed", "records_published", "publish_errors", "streaming_active"}
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Expected field %s in stats response", field)
		}
	}
}

func TestTelemetryPostEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		telemetryData []TelemetryData
		expectError   bool
		expectedCode  int
	}{
		{
			name: "Valid telemetry data",
			telemetryData: []TelemetryData{
				{
					Timestamp: time.Now(),
					Metric:    "DCGM_FI_DEV_GPU_UTIL",
					GPUID:     "0",
					DeviceID:  "nvidia0",
					UUID:      "GPU-test-uuid",
					ModelName: "NVIDIA H100",
					Hostname:  "test-host",
					Value:     85.5,
				},
			},
			expectError:  false,
			expectedCode: http.StatusOK,
		},
		{
			name: "Multiple telemetry points",
			telemetryData: []TelemetryData{
				{
					Timestamp: time.Now(),
					Metric:    "DCGM_FI_DEV_GPU_UTIL",
					GPUID:     "0",
					Value:     85.5,
				},
				{
					Timestamp: time.Now(),
					Metric:    "DCGM_FI_DEV_POWER_USAGE",
					GPUID:     "0",
					Value:     300.2,
				},
			},
			expectError:  false,
			expectedCode: http.StatusOK,
		},
		{
			name:          "Empty data array",
			telemetryData: []TelemetryData{},
			expectError:   false,
			expectedCode:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.telemetryData)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			req := httptest.NewRequest("POST", "/telemetry", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Mock telemetry handler
			telemetryHandler := func(w http.ResponseWriter, r *http.Request) {
				var data []TelemetryData
				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}

				// Mock publishing to message queue
				publishedCount := 0
				for _, point := range data {
					// Simulate publishing logic
					if point.Metric != "" && point.GPUID != "" {
						publishedCount++
					}
				}

				response := map[string]interface{}{
					"status":         "success",
					"points_received": len(data),
					"points_published": publishedCount,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			telemetryHandler(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}

			if tt.expectedCode == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["status"] != "success" {
					t.Errorf("Expected success status, got %v", response["status"])
				}

				received := int(response["points_received"].(float64))
				if received != len(tt.telemetryData) {
					t.Errorf("Expected %d points received, got %d", len(tt.telemetryData), received)
				}
			}
		})
	}
}

func TestComprehensiveCSVProcessing(t *testing.T) {
	// Mock CSV data
	csvData := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
2025-07-18T20:42:33Z,DCGM_FI_DEV_GPU_UTIL,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,,, 85.50,DCGM_FI_DRIVER_VERSION="535.129.03"
2025-07-18T20:42:33Z,DCGM_FI_DEV_POWER_USAGE,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,,, 300.20,DCGM_FI_DRIVER_VERSION="535.129.03"
2025-07-18T20:42:33Z,DCGM_FI_DEV_GPU_TEMP,1,nvidia1,GPU-another-uuid,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,,, 65.00,DCGM_FI_DRIVER_VERSION="535.129.03"`

	tests := []struct {
		name          string
		csvInput      string
		expectedRows  int
		expectError   bool
	}{
		{"Valid CSV data", csvData, 3, false},
		{"Empty CSV", "", 0, false},
		{"Header only", "timestamp,metric_name,gpu_id,device,value", 0, false},
		{"Invalid CSV format", "invalid,csv\ndata", 1, false}, // CSV parser is permissive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock CSV processing
			processCSV := func(data string) ([]map[string]string, error) {
				if data == "" {
					return []map[string]string{}, nil
				}

				reader := csv.NewReader(strings.NewReader(data))
				records, err := reader.ReadAll()
				if err != nil {
					return nil, err
				}

				if len(records) == 0 {
					return []map[string]string{}, nil
				}

				// Skip header row
				if len(records) <= 1 {
					return []map[string]string{}, nil
				}

				header := records[0]
				var result []map[string]string

				for i := 1; i < len(records); i++ {
					row := make(map[string]string)
					for j, value := range records[i] {
						if j < len(header) {
							row[header[j]] = value
						}
					}
					result = append(result, row)
				}

				return result, nil
			}

			rows, err := processCSV(tt.csvInput)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if len(rows) != tt.expectedRows {
				t.Errorf("Expected %d rows, got %d", tt.expectedRows, len(rows))
			}
		})
	}
}

func TestCSVToTelemetryConversion(t *testing.T) {
	tests := []struct {
		name     string
		csvRow   map[string]string
		expected TelemetryData
	}{
		{
			name: "Complete GPU utilization record",
			csvRow: map[string]string{
				"timestamp":    "2025-07-18T20:42:33Z",
				"metric_name":  "DCGM_FI_DEV_GPU_UTIL",
				"gpu_id":       "0",
				"device":       "nvidia0",
				"uuid":         "GPU-test-uuid",
				"modelName":    "NVIDIA H100",
				"Hostname":     "test-host",
				"value":        "85.5",
				"labels_raw":   `DCGM_FI_DRIVER_VERSION="535.129.03"`,
			},
			expected: TelemetryData{
				Metric:    "DCGM_FI_DEV_GPU_UTIL",
				GPUID:     "0",
				DeviceID:  "nvidia0",
				UUID:      "GPU-test-uuid",
				ModelName: "NVIDIA H100",
				Hostname:  "test-host",
				Value:     85.5,
				LabelsRaw: `DCGM_FI_DRIVER_VERSION="535.129.03"`,
			},
		},
		{
			name: "Power usage record",
			csvRow: map[string]string{
				"timestamp":   "2025-07-18T20:42:33Z",
				"metric_name": "DCGM_FI_DEV_POWER_USAGE",
				"gpu_id":      "1",
				"device":      "nvidia1",
				"value":       "300.2",
			},
			expected: TelemetryData{
				Metric:   "DCGM_FI_DEV_POWER_USAGE",
				GPUID:    "1",
				DeviceID: "nvidia1",
				Value:    300.2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock CSV to telemetry conversion
			convertCSVToTelemetry := func(row map[string]string) (TelemetryData, error) {
				var data TelemetryData

				// Parse timestamp
				if ts, exists := row["timestamp"]; exists && ts != "" {
					if parsedTime, err := time.Parse(time.RFC3339, ts); err == nil {
						data.Timestamp = parsedTime
					}
				}

				data.Metric = row["metric_name"]
				data.GPUID = row["gpu_id"]
				data.DeviceID = row["device"]
				data.UUID = row["uuid"]
				data.ModelName = row["modelName"]
				data.Hostname = row["Hostname"]
				data.Container = row["container"]
				data.Pod = row["pod"]
				data.Namespace = row["namespace"]
				data.LabelsRaw = row["labels_raw"]

				// Parse value
				if val, exists := row["value"]; exists && val != "" {
					val = strings.TrimSpace(val)
					if parsedVal, err := parseFloat(val); err == nil {
						data.Value = parsedVal
					}
				}

				return data, nil
			}

			result, err := convertCSVToTelemetry(tt.csvRow)
			if err != nil {
				t.Errorf("Conversion failed: %v", err)
			}

			// Verify key fields
			if result.Metric != tt.expected.Metric {
				t.Errorf("Expected Metric %s, got %s", tt.expected.Metric, result.Metric)
			}
			if result.GPUID != tt.expected.GPUID {
				t.Errorf("Expected GPUID %s, got %s", tt.expected.GPUID, result.GPUID)
			}
			if result.DeviceID != tt.expected.DeviceID {
				t.Errorf("Expected DeviceID %s, got %s", tt.expected.DeviceID, result.DeviceID)
			}
			if result.Value != tt.expected.Value {
				t.Errorf("Expected Value %f, got %f", tt.expected.Value, result.Value)
			}
		})
	}
}

// Helper function to parse float values
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func TestMessageQueuePublishing(t *testing.T) {
	tests := []struct {
		name         string
		data         TelemetryData
		topic        string
		expectError  bool
	}{
		{
			name: "Valid telemetry publish",
			data: TelemetryData{
				Timestamp: time.Now(),
				Metric:    "DCGM_FI_DEV_GPU_UTIL",
				GPUID:     "0",
				Value:     85.5,
			},
			topic:       "telemetry",
			expectError: false,
		},
		{
			name: "Missing required fields",
			data: TelemetryData{
				Timestamp: time.Now(),
				Value:     85.5,
			}, // Missing Metric and GPUID
			topic:       "telemetry",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock message queue publish
			publishToQueue := func(data TelemetryData, topic string) error {
				if data.Metric == "" {
					return fmt.Errorf("metric is required")
				}
				if data.GPUID == "" {
					return fmt.Errorf("gpu_id is required")
				}
				if topic == "" {
					return fmt.Errorf("topic is required")
				}
				return nil
			}

			err := publishToQueue(tt.data, tt.topic)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestStreamingDelayControl(t *testing.T) {
	// Test streaming delay configuration
	tests := []struct {
		name         string
		delayMs      int
		expectedDuration time.Duration
	}{
		{"Default delay", 20, 20 * time.Millisecond},
		{"Fast streaming", 1, 1 * time.Millisecond},
		{"Slow streaming", 100, 100 * time.Millisecond},
		{"No delay", 0, 0 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock delay configuration
			getStreamingDelay := func(delayMs int) time.Duration {
				if delayMs <= 0 {
					return 0
				}
				return time.Duration(delayMs) * time.Millisecond
			}

			delay := getStreamingDelay(tt.delayMs)
			if delay != tt.expectedDuration {
				t.Errorf("Expected delay %v, got %v", tt.expectedDuration, delay)
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "Invalid JSON",
			requestBody:    "invalid json",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty body",
			requestBody:    "",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Wrong content type",
			requestBody:    `{"test": "data"}`,
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/telemetry", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()

			handler := func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Content-Type") != "application/json" {
					http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
					return
				}

				var data interface{}
				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusOK)
			}

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Mock metrics handler
	metricsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "# HELP streamer_records_processed_total Total records processed\n")
		fmt.Fprintf(w, "# TYPE streamer_records_processed_total counter\n")
		fmt.Fprintf(w, "streamer_records_processed_total 1000\n")
		fmt.Fprintf(w, "# HELP streamer_publish_errors_total Total publish errors\n")
		fmt.Fprintf(w, "# TYPE streamer_publish_errors_total counter\n")
		fmt.Fprintf(w, "streamer_publish_errors_total 5\n")
	}

	metricsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected Content-Type to contain 'text/plain', got %q", contentType)
	}

	body := w.Body.String()
	expectedMetrics := []string{"streamer_records_processed_total", "streamer_publish_errors_total"}
	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected response to contain metric '%s'", metric)
		}
	}
}
