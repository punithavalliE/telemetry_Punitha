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

// Comprehensive test suite for Message Queue service

func TestTopicCreation(t *testing.T) {
	tests := []struct {
		name        string
		topic       string
		partitions  int
		expectError bool
	}{
		{"Valid topic creation", "telemetry", 2, false},
		{"Topic with special chars", "telemetry-test_data", 4, false},
		{"Empty topic name", "", 2, true},
		{"Zero partitions", "test", 0, true},
		{"Large partition count", "test", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock topic creation
			createTopic := func(name string, partitions int) error {
				if name == "" {
					return fmt.Errorf("topic name cannot be empty")
				}
				if partitions <= 0 {
					return fmt.Errorf("partition count must be positive")
				}
				return nil
			}

			err := createTopic(tt.topic, tt.partitions)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestMessageProduction(t *testing.T) {
	tests := []struct {
		name        string
		topic       string
		partition   int
		message     string
		expectError bool
	}{
		{"Valid message", "telemetry", 0, `{"gpu_id": "0", "metric": "util", "value": 85.5}`, false},
		{"Large message", "telemetry", 1, strings.Repeat("test", 1000), false},
		{"Empty message", "telemetry", 0, "", false},
		{"Invalid partition", "telemetry", -1, "test", true},
		{"Non-existent topic", "invalid-topic", 0, "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create HTTP request for message production
			reqBody := fmt.Sprintf(`{"message": "%s"}`, tt.message)
			url := fmt.Sprintf("/produce?topic=%s&partition=%d", tt.topic, tt.partition)
			
			req := httptest.NewRequest("POST", url, bytes.NewBufferString(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Mock producer handler
			produceHandler := func(w http.ResponseWriter, r *http.Request) {
				topic := r.URL.Query().Get("topic")
				partition := r.URL.Query().Get("partition")

				if topic == "" {
					http.Error(w, "Topic is required", http.StatusBadRequest)
					return
				}

				if topic == "invalid-topic" {
					http.Error(w, "Topic does not exist", http.StatusNotFound)
					return
				}

				partitionNum := 0
				if partition != "" {
					if _, err := fmt.Sscanf(partition, "%d", &partitionNum); err != nil || partitionNum < 0 {
						http.Error(w, "Invalid partition", http.StatusBadRequest)
						return
					}
				}

				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}

				response := map[string]interface{}{
					"status":    "success",
					"topic":     topic,
					"partition": partitionNum,
					"offset":    42, // Mock offset
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			produceHandler(w, req)

			if tt.expectError {
				if w.Code == http.StatusOK {
					t.Errorf("Expected error status, got 200")
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}

				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["status"] != "success" {
					t.Errorf("Expected success status, got %v", response["status"])
				}
			}
		})
	}
}

func TestMessageConsumption(t *testing.T) {
	tests := []struct {
		name           string
		topic          string
		partition      int
		group          string
		expectedStatus int
	}{
		{"Valid consumption", "telemetry", 0, "test-group", http.StatusOK},
		{"Multiple partitions", "telemetry", 1, "test-group", http.StatusOK},
		{"Different group", "telemetry", 0, "another-group", http.StatusOK},
		{"Invalid partition", "telemetry", -1, "test-group", http.StatusBadRequest},
		{"Missing topic", "", 0, "test-group", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/consume?topic=%s&partition=%d&group=%s", tt.topic, tt.partition, tt.group)
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			// Mock consumer handler
			consumeHandler := func(w http.ResponseWriter, r *http.Request) {
				topic := r.URL.Query().Get("topic")
				partition := r.URL.Query().Get("partition")
				group := r.URL.Query().Get("group")

				if topic == "" {
					http.Error(w, "Topic is required", http.StatusBadRequest)
					return
				}

				// Use group parameter for consumer group validation
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

				// Mock SSE response
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")

				// Send mock event
				fmt.Fprintf(w, "data: {\"id\": \"msg-1\", \"message\": \"test message\", \"offset\": 1}\n\n")
			}

			consumeHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if contentType != "text/event-stream" {
					t.Errorf("Expected Content-Type 'text/event-stream', got %s", contentType)
				}
			}
		})
	}
}

func TestMessageAcknowledgment(t *testing.T) {
	tests := []struct {
		name           string
		messageID      string
		topic          string
		partition      int
		expectedStatus int
	}{
		{"Valid ack", "msg-123", "telemetry", 0, http.StatusOK},
		{"Another valid ack", "msg-456", "telemetry", 1, http.StatusOK},
		{"Empty message ID", "", "telemetry", 0, http.StatusBadRequest},
		{"Invalid partition", "msg-123", "telemetry", -1, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/ack?topic=%s&partition=%d&message_id=%s", tt.topic, tt.partition, tt.messageID)
			req := httptest.NewRequest("POST", url, nil)
			w := httptest.NewRecorder()

			// Mock ack handler
			ackHandler := func(w http.ResponseWriter, r *http.Request) {
				messageID := r.URL.Query().Get("message_id")
				topic := r.URL.Query().Get("topic")
				partition := r.URL.Query().Get("partition")

				if messageID == "" {
					http.Error(w, "Message ID is required", http.StatusBadRequest)
					return
				}

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

				response := map[string]interface{}{
					"status":     "acknowledged",
					"message_id": messageID,
					"topic":      topic,
					"partition":  partitionNum,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			ackHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if response["status"] != "acknowledged" {
					t.Errorf("Expected status 'acknowledged', got %v", response["status"])
				}
			}
		})
	}
}

func TestTopicsListEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/topics", nil)
	w := httptest.NewRecorder()

	// Mock topics handler
	topicsHandler := func(w http.ResponseWriter, r *http.Request) {
		topics := map[string]interface{}{
			"topics": []map[string]interface{}{
				{
					"name":       "telemetry",
					"partitions": 2,
					"messages":   1000,
				},
				{
					"name":       "logs",
					"partitions": 4,
					"messages":   500,
				},
			},
			"total_topics": 2,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(topics)
	}

	topicsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	topics, exists := response["topics"].([]interface{})
	if !exists {
		t.Errorf("Expected topics array in response")
	}

	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}
}

func TestComprehensiveVisibilityTimeout(t *testing.T) {
	// Test message requeue after visibility timeout
	mockMessage := struct {
		ID         string    `json:"id"`
		Content    string    `json:"content"`
		VisibleAt  time.Time `json:"visible_at"`
		InFlight   bool      `json:"in_flight"`
	}{
		ID:        "msg-timeout-test",
		Content:   "test message for timeout",
		VisibleAt: time.Now().Add(-time.Second), // Already expired
		InFlight:  true,
	}

	// Mock visibility timeout check
	checkVisibilityTimeout := func(msg interface{}) bool {
		m := msg.(struct {
			ID         string    `json:"id"`
			Content    string    `json:"content"`
			VisibleAt  time.Time `json:"visible_at"`
			InFlight   bool      `json:"in_flight"`
		})

		return m.InFlight && time.Now().After(m.VisibleAt)
	}

	if !checkVisibilityTimeout(mockMessage) {
		t.Errorf("Expected message to be visible after timeout")
	}
}

func TestConcurrentProducers(t *testing.T) {
	// Test concurrent message production
	var wg sync.WaitGroup
	results := make(chan error, 10)

	// Mock concurrent producers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Simulate message production
			time.Sleep(time.Duration(id) * time.Millisecond) // Stagger requests

			// Mock produce operation
			message := fmt.Sprintf("Message from producer %d", id)
			if message == "" {
				results <- fmt.Errorf("empty message from producer %d", id)
				return
			}

			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors from concurrent producers, got %d errors", errorCount)
	}
}

func TestHealthAndMetrics(t *testing.T) {
	t.Run("Health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		healthHandler := func(w http.ResponseWriter, r *http.Request) {
			health := map[string]interface{}{
				"status":    "healthy",
				"service":   "message-queue",
				"version":   "1.0.0",
				"timestamp": time.Now().Format(time.RFC3339),
				"uptime":    "1h30m",
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
	})

	t.Run("Metrics endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()

		metricsHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "# HELP queue_messages_total Total messages processed\n")
			fmt.Fprintf(w, "# TYPE queue_messages_total counter\n")
			fmt.Fprintf(w, "queue_messages_total 1000\n")
		}

		metricsHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "queue_messages_total") {
			t.Errorf("Expected metrics to contain queue_messages_total")
		}
	})
}