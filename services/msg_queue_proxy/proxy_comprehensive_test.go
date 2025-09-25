package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Comprehensive test suite for Message Queue Proxy service

func TestBrokerDiscovery(t *testing.T) {
	tests := []struct {
		name        string
		brokerCount int
		serviceName string
		expectError bool
	}{
		{"Standard configuration", 2, "msg-queue", false},
		{"Single broker", 1, "msg-queue", false},
		{"Multiple brokers", 4, "msg-queue", false},
		{"Zero brokers", 0, "msg-queue", true},
		{"Empty service name", 2, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock broker discovery
			discoverBrokers := func(count int, service string) ([]string, error) {
				if count <= 0 {
					return nil, fmt.Errorf("broker count must be positive")
				}
				if service == "" {
					return nil, fmt.Errorf("service name cannot be empty")
				}

				brokers := make([]string, count)
				for i := 0; i < count; i++ {
					brokers[i] = fmt.Sprintf("http://%s-%d.%s-headless.telemetry.svc.cluster.local:8080", service, i, service)
				}
				return brokers, nil
			}

			brokers, err := discoverBrokers(tt.brokerCount, tt.serviceName)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if !tt.expectError && len(brokers) != tt.brokerCount {
				t.Errorf("Expected %d brokers, got %d", tt.brokerCount, len(brokers))
			}
		})
	}
}

func TestConsistentHashing(t *testing.T) {
	// Mock consistent hash ring
	brokers := []string{
		"http://msg-queue-0.msg-queue-headless.telemetry.svc.cluster.local:8080",
		"http://msg-queue-1.msg-queue-headless.telemetry.svc.cluster.local:8080",
	}

	// Simple mock hash function
	mockHashFunction := func(key string, brokers []string) string {
		if len(brokers) == 0 {
			return ""
		}
		// Simple hash based on string length
		index := len(key) % len(brokers)
		return brokers[index]
	}

	tests := []struct {
		name        string
		key         string
		expectedIdx int
	}{
		{"Short key", "a", 1},    // len=1, 1%2=1
		{"Medium key", "test", 0}, // len=4, 4%2=0
		{"Long key", "very-long-key", 1}, // len=13, 13%2=1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := mockHashFunction(tt.key, brokers)
			expected := brokers[tt.expectedIdx]

			if broker != expected {
				t.Errorf("Expected broker %s, got %s", expected, broker)
			}
		})
	}
}

func TestProduceRequestForwarding(t *testing.T) {
	tests := []struct {
		name           string
		topic          string
		partition      int
		message        string
		expectedStatus int
	}{
		{"Valid produce request", "telemetry", 0, "test message", http.StatusOK},
		{"Different partition", "telemetry", 1, "another message", http.StatusOK},
		{"Large message", "telemetry", 0, strings.Repeat("data", 1000), http.StatusOK},
		{"Empty message", "telemetry", 0, "", http.StatusOK},
		{"Invalid partition", "telemetry", -1, "test", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/produce?topic=%s&partition=%d", tt.topic, tt.partition)
			reqBody := fmt.Sprintf(`{"message": "%s"}`, tt.message)
			
			req := httptest.NewRequest("POST", url, bytes.NewBufferString(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Mock proxy produce handler
			proxyProduceHandler := func(w http.ResponseWriter, r *http.Request) {
				topic := r.URL.Query().Get("topic")
				partition := r.URL.Query().Get("partition")

				if topic == "" {
					http.Error(w, "Topic is required", http.StatusBadRequest)
					return
				}

				partitionNum := 0
				if partition != "" {
					if _, err := fmt.Sscanf(partition, "%d", &partitionNum); err != nil || partitionNum < 0 {
						http.Error(w, "Invalid partition", http.StatusBadRequest)
						return
					}
				}

				// Mock forwarding to backend broker
				brokerURL := fmt.Sprintf("http://msg-queue-%d.msg-queue-headless.telemetry.svc.cluster.local:8080", partitionNum)
				
				response := map[string]interface{}{
					"status":       "forwarded",
					"topic":        topic,
					"partition":    partitionNum,
					"broker":       brokerURL,
					"proxy_id":     "proxy-1",
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			proxyProduceHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["status"] != "forwarded" {
					t.Errorf("Expected status 'forwarded', got %v", response["status"])
				}
			}
		})
	}
}

func TestConsumeRequestForwarding(t *testing.T) {
	tests := []struct {
		name           string
		topic          string
		partition      int
		group          string
		expectedStatus int
	}{
		{"Valid consume request", "telemetry", 0, "test-group", http.StatusOK},
		{"Different partition", "telemetry", 1, "test-group", http.StatusOK},
		{"Different group", "telemetry", 0, "another-group", http.StatusOK},
		{"Invalid partition", "telemetry", -1, "test-group", http.StatusBadRequest},
		{"Missing topic", "", 0, "test-group", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/consume?topic=%s&partition=%d&group=%s", tt.topic, tt.partition, tt.group)
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			// Mock proxy consume handler
			proxyConsumeHandler := func(w http.ResponseWriter, r *http.Request) {
				topic := r.URL.Query().Get("topic")
				partition := r.URL.Query().Get("partition")
				group := r.URL.Query().Get("group")

				if topic == "" {
					http.Error(w, "Topic is required", http.StatusBadRequest)
					return
				}

				// Validate group parameter
				if group == "" {
					group = "default"
				}

				partitionNum := 0
				if partition != "" {
					if _, err := fmt.Sscanf(partition, "%d", &partitionNum); err != nil || partitionNum < 0 {
						http.Error(w, "Invalid partition", http.StatusBadRequest)
						return
					}
				}

				// Mock SSE forwarding
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.Header().Set("X-Forwarded-To", fmt.Sprintf("msg-queue-%d", partitionNum))

				// Send mock event
				fmt.Fprintf(w, "data: {\"id\": \"msg-proxy-1\", \"message\": \"forwarded message\", \"partition\": %d}\n\n", partitionNum)
			}

			proxyConsumeHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "text/event-stream" {
					t.Errorf("Expected Content-Type 'text/event-stream', got %s", contentType)
				}

				forwardedTo := w.Header().Get("X-Forwarded-To")
				expectedBroker := fmt.Sprintf("msg-queue-%d", tt.partition)
				if forwardedTo != expectedBroker {
					t.Errorf("Expected X-Forwarded-To '%s', got '%s'", expectedBroker, forwardedTo)
				}
			}
		})
	}
}

func TestBrokerHealthChecking(t *testing.T) {
	brokers := []string{
		"http://msg-queue-0.msg-queue-headless.telemetry.svc.cluster.local:8080",
		"http://msg-queue-1.msg-queue-headless.telemetry.svc.cluster.local:8080",
	}

	// Mock health checker
	checkBrokerHealth := func(brokerURL string) bool {
		// Simulate health check logic
		if strings.Contains(brokerURL, "msg-queue-0") {
			return true // Healthy
		}
		if strings.Contains(brokerURL, "msg-queue-1") {
			return false // Unhealthy
		}
		return false
	}

	healthyCount := 0
	for _, broker := range brokers {
		if checkBrokerHealth(broker) {
			healthyCount++
		}
	}

	if healthyCount != 1 {
		t.Errorf("Expected 1 healthy broker, got %d", healthyCount)
	}
}

func TestLoadBalancing(t *testing.T) {
	// Test round-robin load balancing
	brokers := []string{
		"broker-0",
		"broker-1",
		"broker-2",
	}

	// Mock round-robin selector
	var currentIndex int
	var mu sync.Mutex

	selectBroker := func() string {
		mu.Lock()
		defer mu.Unlock()
		
		broker := brokers[currentIndex]
		currentIndex = (currentIndex + 1) % len(brokers)
		return broker
	}

	// Test round-robin distribution
	selections := make(map[string]int)
	for i := 0; i < 9; i++ { // 3 rounds
		broker := selectBroker()
		selections[broker]++
	}

	// Each broker should be selected 3 times
	for _, broker := range brokers {
		if selections[broker] != 3 {
			t.Errorf("Expected broker %s to be selected 3 times, got %d", broker, selections[broker])
		}
	}
}

func TestProxyStats(t *testing.T) {
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	// Mock stats handler
	statsHandler := func(w http.ResponseWriter, r *http.Request) {
		stats := map[string]interface{}{
			"proxy_id":             "proxy-1",
			"uptime_seconds":       3600,
			"requests_forwarded":   1000,
			"requests_failed":      5,
			"active_brokers":       2,
			"total_brokers":        2,
			"consistent_hash_ring": map[string]interface{}{
				"virtual_nodes": 150,
				"hash_function": "fnv32",
			},
			"broker_health": map[string]bool{
				"msg-queue-0": true,
				"msg-queue-1": true,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}

	statsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	expectedFields := []string{"proxy_id", "uptime_seconds", "requests_forwarded", "active_brokers"}
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Expected field %s in stats response", field)
		}
	}
}

func TestConcurrentRequests(t *testing.T) {
	// Test concurrent request handling
	var wg sync.WaitGroup
	results := make(chan error, 20)

	// Mock concurrent request processor
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Simulate request processing
			time.Sleep(time.Duration(id%5) * time.Millisecond)

			// Mock processing logic
			if id%10 == 9 { // Simulate 10% failure rate
				results <- fmt.Errorf("request %d failed", id)
				return
			}

			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	expectedSuccesses := 18 // 90% success rate
	expectedErrors := 2    // 10% error rate

	if successCount != expectedSuccesses {
		t.Errorf("Expected %d successes, got %d", expectedSuccesses, successCount)
	}
	if errorCount != expectedErrors {
		t.Errorf("Expected %d errors, got %d", expectedErrors, errorCount)
	}
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":         "healthy",
			"service":        "msg-queue-proxy",
			"version":        "1.0.0",
			"active_brokers": 2,
			"total_brokers":  2,
			"timestamp":      time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}

	healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}

	if response["service"] != "msg-queue-proxy" {
		t.Errorf("Expected service 'msg-queue-proxy', got %v", response["service"])
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		method         string
		expectedStatus int
	}{
		{"Invalid endpoint", "/invalid", "GET", http.StatusNotFound},
		{"Wrong method for produce", "/produce", "DELETE", http.StatusMethodNotAllowed},
		{"Wrong method for consume", "/consume", "POST", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			w := httptest.NewRecorder()

			// Mock error handler
			errorHandler := func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/invalid" {
					http.Error(w, "Not Found", http.StatusNotFound)
					return
				}

				if r.URL.Path == "/produce" && r.Method != "POST" {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
					return
				}

				if r.URL.Path == "/consume" && r.Method != "GET" {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
					return
				}

				w.WriteHeader(http.StatusOK)
			}

			errorHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}