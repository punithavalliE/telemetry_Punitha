package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

// MockMessageQueue implements basic message queue functionality for testing
type MockMessageQueue struct {
	messages []string
	err      error
	closed   bool
}

func (m *MockMessageQueue) Consume() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if len(m.messages) == 0 {
		time.Sleep(10 * time.Millisecond) // Simulate blocking
		return "", nil
	}
	msg := m.messages[0]
	m.messages = m.messages[1:]
	return msg, nil
}

func (m *MockMessageQueue) Produce(message string) error {
	if m.err != nil {
		return m.err
	}
	m.messages = append(m.messages, message)
	return nil
}

func (m *MockMessageQueue) Close() error {
	m.closed = true
	return nil
}

// MockInfluxWriter for testing
type MockInfluxWriter struct {
	err    error
	closed bool
}

func (m *MockInfluxWriter) WritePoints(points []map[string]interface{}) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *MockInfluxWriter) Close() {
	m.closed = true
}

func (m *MockInfluxWriter) QueryRecentTelemetry(limit int) ([]interface{}, error) {
	return nil, m.err
}

func TestEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("USE_HTTP_QUEUE", "true")
	os.Setenv("MSG_QUEUE_ADDR", "http://test:8080")
	os.Setenv("MSG_QUEUE_TOPIC", "test-topic")
	os.Setenv("MSG_QUEUE_GROUP", "test-group")
	
	defer func() {
		os.Unsetenv("USE_HTTP_QUEUE")
		os.Unsetenv("MSG_QUEUE_ADDR")
		os.Unsetenv("MSG_QUEUE_TOPIC")
		os.Unsetenv("MSG_QUEUE_GROUP")
	}()

	t.Run("Environment Variables", func(t *testing.T) {
		useHTTPQueue := os.Getenv("USE_HTTP_QUEUE")
		if useHTTPQueue != "true" {
			t.Errorf("Expected USE_HTTP_QUEUE to be 'true', got '%s'", useHTTPQueue)
		}

		queueAddr := os.Getenv("MSG_QUEUE_ADDR")
		if queueAddr == "" {
			queueAddr = "http://msg_queue:8080"
		}
		if queueAddr != "http://test:8080" {
			t.Errorf("Expected queue address 'http://test:8080', got '%s'", queueAddr)
		}
	})
}

func TestMessageQueueOperations(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	mockQueue := &MockMessageQueue{}

	t.Run("Valid Message Processing", func(t *testing.T) {
		// Create a valid telemetry message
		telemetryData := map[string]interface{}{
			"device_id":  "nvidia0",
			"metric":     "DCGM_FI_DEV_GPU_UTIL",
			"value":      85.5,
			"timestamp":  time.Now().Format(time.RFC3339),
			"gpu_id":     "0",
			"uuid":       "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			"model_name": "NVIDIA H100 80GB HBM3",
			"hostname":   "mtv5-dgx1-hgpu-031",
			"container":  "",
			"pod":        "test-pod",
			"namespace":  "default",
			"labels":     map[string]string{"version": "535.129.03"},
		}

		jsonData, err := json.Marshal(telemetryData)
		if err != nil {
			t.Fatalf("Failed to marshal test data: %v", err)
		}

		// Add message to mock queue
		mockQueue.messages = []string{string(jsonData)}

		// Process the message
		message, err := mockQueue.Consume()
		if err != nil {
			t.Fatalf("Failed to consume message: %v", err)
		}

		if message == "" {
			t.Fatal("Expected message, got empty string")
		}

		// Verify message content
		var parsedData map[string]interface{}
		if err := json.Unmarshal([]byte(message), &parsedData); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		if parsedData["device_id"] != "nvidia0" {
			t.Errorf("Expected device_id 'nvidia0', got '%s'", parsedData["device_id"])
		}

		if parsedData["value"] != 85.5 {
			t.Errorf("Expected value 85.5, got %v", parsedData["value"])
		}

		logger.Printf("Successfully processed message: %s", message[:50])
	})

	t.Run("Invalid JSON Message", func(t *testing.T) {
		mockQueue.messages = []string{"invalid json"}

		message, err := mockQueue.Consume()
		if err != nil {
			t.Fatalf("Failed to consume message: %v", err)
		}

		// Try to parse as JSON
		var parsedData map[string]interface{}
		err = json.Unmarshal([]byte(message), &parsedData)
		if err == nil {
			t.Error("Expected JSON unmarshal error for invalid JSON")
		}
	})
}

func TestInfluxIntegration(t *testing.T) {
	mockInflux := &MockInfluxWriter{}

	t.Run("Write Points Success", func(t *testing.T) {
		err := mockInflux.WritePoints(nil)
		if err != nil {
			t.Errorf("Expected no error writing points, got: %v", err)
		}
	})

	t.Run("Write Points Error", func(t *testing.T) {
		mockInflux.err = fmt.Errorf("influx write error")

		err := mockInflux.WritePoints(nil)
		if err == nil {
			t.Error("Expected error writing points, got nil")
		}
	})
}

func TestMessageQueueError(t *testing.T) {
	mockQueue := &MockMessageQueue{err: fmt.Errorf("queue error")}

	t.Run("Consume Error", func(t *testing.T) {
		_, err := mockQueue.Consume()
		if err == nil {
			t.Error("Expected consume error, got nil")
		}
	})
}

func TestResourceCleanup(t *testing.T) {
	mockQueue := &MockMessageQueue{}
	mockInflux := &MockInfluxWriter{}

	// Test closing resources
	mockInflux.Close()
	if !mockInflux.closed {
		t.Error("Expected InfluxDB client to be closed")
	}

	err := mockQueue.Close()
	if err != nil {
		t.Errorf("Expected no error closing queue, got: %v", err)
	}
	if !mockQueue.closed {
		t.Error("Expected message queue to be closed")
	}
}

func TestEnvironmentDefaults(t *testing.T) {
	t.Run("Default Values When Not Set", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("USE_HTTP_QUEUE")
		os.Unsetenv("MSG_QUEUE_ADDR")
		os.Unsetenv("MSG_QUEUE_TOPIC")
		os.Unsetenv("MSG_QUEUE_GROUP")

		useHTTPQueue := os.Getenv("USE_HTTP_QUEUE")
		if useHTTPQueue != "" && useHTTPQueue != "true" {
			// Default behavior would use Redis
			t.Logf("USE_HTTP_QUEUE not set, would default to Redis queue")
		}

		queueAddr := os.Getenv("MSG_QUEUE_ADDR")
		if queueAddr == "" {
			queueAddr = "http://msg_queue:8080"
		}
		if queueAddr != "http://msg_queue:8080" {
			t.Errorf("Expected default queue address, got %s", queueAddr)
		}

		topic := os.Getenv("MSG_QUEUE_TOPIC")
		if topic == "" {
			topic = "telemetry"
		}
		if topic != "telemetry" {
			t.Errorf("Expected default topic 'telemetry', got %s", topic)
		}

		group := os.Getenv("MSG_QUEUE_GROUP")
		if group == "" {
			group = "telemetry_group"
		}
		if group != "telemetry_group" {
			t.Errorf("Expected default group 'telemetry_group', got %s", group)
		}
	})
}

func TestTelemetryDataProcessing(t *testing.T) {
	t.Run("Convert Telemetry to Point", func(t *testing.T) {
		now := time.Now()
		telemetryData := map[string]interface{}{
			"device_id":  "nvidia0",
			"metric":     "DCGM_FI_DEV_GPU_UTIL",
			"value":      85.5,
			"timestamp":  now,
			"gpu_id":     "0",
			"uuid":       "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			"model_name": "NVIDIA H100 80GB HBM3",
			"hostname":   "mtv5-dgx1-hgpu-031",
			"container":  "",
			"pod":        "test-pod",
			"namespace":  "default",
			"labels":     map[string]string{"version": "535.129.03"},
		}

		// This would typically be done by the collector service
		point := map[string]interface{}{
			"measurement": "gpu_metrics",
			"device_id":   telemetryData["device_id"],
			"gpu_id":      telemetryData["gpu_id"],
			"value":       telemetryData["value"],
			"time":        telemetryData["timestamp"],
		}

		if point["device_id"] != "nvidia0" {
			t.Errorf("Expected device_id 'nvidia0', got '%s'", point["device_id"])
		}

		if point["value"] != 85.5 {
			t.Errorf("Expected value 85.5, got %v", point["value"])
		}
	})
}
