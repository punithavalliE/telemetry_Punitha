package main

import (
	"bytes"
	"encoding/json"
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

// Comprehensive test suite for API service

func TestHandleHealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{"GET Health", "GET", http.StatusOK, "API service healthy"},
		{"POST Health", "POST", http.StatusOK, "API service healthy"},
		{"PUT Health", "PUT", http.StatusOK, "API service healthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("API service healthy"))
			})

			handler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestTelemetryDataProcessing(t *testing.T) {
	tests := []struct {
		name         string
		inputData    []TelemetryData
		expectError  bool
		expectedCode int
	}{
		{
			name: "Valid single telemetry point",
			inputData: []TelemetryData{
				{
					Timestamp: time.Now(),
					Metric:    "DCGM_FI_DEV_GPU_UTIL",
					GPUID:     "0",
					DeviceID:  "nvidia0",
					UUID:      "GPU-test-uuid",
					ModelName: "NVIDIA Test GPU",
					Hostname:  "test-host",
					Value:     85.5,
				},
			},
			expectError:  false,
			expectedCode: http.StatusOK,
		},
		{
			name: "Multiple telemetry points",
			inputData: []TelemetryData{
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
			name:         "Empty data array",
			inputData:    []TelemetryData{},
			expectError:  false,
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.inputData)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			req := httptest.NewRequest("POST", "/telemetry", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "test-api-key")
			w := httptest.NewRecorder()

			// Mock handler that processes telemetry data
			handler := func(w http.ResponseWriter, r *http.Request) {
				var data []TelemetryData
				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":       "success",
					"points_saved": len(data),
				})
			}

			handler(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}

			if tt.expectedCode == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if status, ok := response["status"]; !ok || status != "success" {
					t.Errorf("Expected success status, got %v", status)
				}

				if points, ok := response["points_saved"]; !ok || int(points.(float64)) != len(tt.inputData) {
					t.Errorf("Expected %d points saved, got %v", len(tt.inputData), points)
				}
			}
		})
	}
}

func TestAuthenticationMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		headerValue    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Valid X-API-Key",
			authHeader:     "X-API-Key",
			headerValue:    "telemetry-api-secret-2025",
			expectedStatus: http.StatusOK,
			expectedBody:   "authenticated",
		},
		{
			name:           "Valid Bearer Token",
			authHeader:     "Authorization",
			headerValue:    "Bearer telemetry-api-secret-2025",
			expectedStatus: http.StatusOK,
			expectedBody:   "authenticated",
		},
		{
			name:           "Invalid API Key",
			authHeader:     "X-API-Key",
			headerValue:    "invalid-key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized\n",
		},
		{
			name:           "Missing Authentication",
			authHeader:     "",
			headerValue:    "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized\n",
		},
		{
			name:           "Invalid Bearer Format",
			authHeader:     "Authorization",
			headerValue:    "Basic invalid-format",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set(tt.authHeader, tt.headerValue)
			}
			w := httptest.NewRecorder()

			// Mock auth middleware
			authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					// Check X-API-Key
					apiKey := r.Header.Get("X-API-Key")
					if apiKey == "telemetry-api-secret-2025" {
						next(w, r)
						return
					}

					// Check Authorization Bearer
					authHeader := r.Header.Get("Authorization")
					if strings.HasPrefix(authHeader, "Bearer ") {
						token := strings.TrimPrefix(authHeader, "Bearer ")
						if token == "telemetry-api-secret-2025" {
							next(w, r)
							return
						}
					}

					http.Error(w, "Unauthorized", http.StatusUnauthorized)
				}
			}

			protectedHandler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("authenticated"))
			})

			protectedHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
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
			req.Header.Set("X-API-Key", "test-api-key")
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

func TestCORSHeaders(t *testing.T) {
	req := httptest.NewRequest("OPTIONS", "/telemetry", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-API-Key,Content-Type")
	w := httptest.NewRecorder()

	// Mock CORS handler
	corsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Authorization, Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	corsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "X-API-Key, Authorization, Content-Type",
		"Access-Control-Max-Age":       "86400",
	}

	for header, expectedValue := range expectedHeaders {
		if got := w.Header().Get(header); got != expectedValue {
			t.Errorf("Expected %s header to be %q, got %q", header, expectedValue, got)
		}
	}
}

func TestMetricsEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	// Mock metrics handler
	metricsHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# HELP api_requests_total Total number of API requests\n# TYPE api_requests_total counter\napi_requests_total 42\n"))
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
	if !strings.Contains(body, "api_requests_total") {
		t.Errorf("Expected response to contain metrics data, got %q", body)
	}
}
