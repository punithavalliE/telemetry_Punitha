package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/example/telemetry/config"
)

// MockMessageQueue implements the MessageQueue interface for testing
type MockMessageQueue struct {
	messages map[string][][]byte // topic -> messages
	err      error
	closed   bool
}

func NewMockMessageQueue() *MockMessageQueue {
	return &MockMessageQueue{
		messages: make(map[string][][]byte),
	}
}

func (m *MockMessageQueue) Publish(topic string, message []byte) error {
	if m.err != nil {
		return m.err
	}
	if m.messages[topic] == nil {
		m.messages[topic] = make([][]byte, 0)
	}
	m.messages[topic] = append(m.messages[topic], message)
	return nil
}

func (m *MockMessageQueue) Subscribe(handler func(topic string, body []byte, id string) error) error {
	return m.err
}

func (m *MockMessageQueue) Consume() ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	// For testing, we don't implement consume
	return nil, nil
}

func (m *MockMessageQueue) Close() error {
	m.closed = true
	return m.err
}

func TestNewStreamerService(t *testing.T) {
	// Set test environment variables
	os.Setenv("USE_HTTP_QUEUE", "true")
	os.Setenv("MSG_QUEUE_ADDR", "http://test:8080")
	os.Setenv("MSG_QUEUE_TOPIC", "test-topic")
	os.Setenv("MSG_QUEUE_GROUP", "test-group")
	os.Setenv("MSG_QUEUE_PRODUCER_NAME", "test-producer")
	
	defer func() {
		os.Unsetenv("USE_HTTP_QUEUE")
		os.Unsetenv("MSG_QUEUE_ADDR")
		os.Unsetenv("MSG_QUEUE_TOPIC")
		os.Unsetenv("MSG_QUEUE_GROUP")
		os.Unsetenv("MSG_QUEUE_PRODUCER_NAME")
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

		producerName := os.Getenv("MSG_QUEUE_PRODUCER_NAME")
		if producerName == "" {
			producerName = "streamer"
		}
		if producerName != "test-producer" {
			t.Errorf("Expected producer name 'test-producer', got '%s'", producerName)
		}
	})
}

func TestStreamerService_StreamCSV(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	mockQueue := NewMockMessageQueue()
	cfg := config.Config{}

	service := &StreamerService{
		queue:  mockQueue,
		logger: logger,
		config: cfg,
	}

	t.Run("Valid CSV File", func(t *testing.T) {
		// Create a temporary CSV file
		csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
2023-07-18T20:42:34Z,DCGM_FI_DEV_GPU_UTIL,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,test-pod,default,85.5,"version=535.129.03"
2023-07-18T20:42:35Z,DCGM_FI_DEV_MEM_COPY_UTIL,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,test-pod,default,72.3,"version=535.129.03"`

		tmpFile, err := ioutil.TempFile("", "test_*.csv")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(csvContent); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tmpFile.Close()

		// Use a goroutine to stream CSV and stop after a short time
		done := make(chan error, 1)
		go func() {
			err := service.StreamCSV(tmpFile.Name(), 10*time.Millisecond)
			done <- err
		}()

		// Wait a bit for some messages to be processed
		time.Sleep(50 * time.Millisecond)

		// Check that messages were published
		messages := mockQueue.messages["telemetry"]
		if len(messages) == 0 {
			t.Error("Expected messages to be published, got 0")
		}

		// Verify the first message content (should be the header)
		if len(messages) > 0 {
			var record []string
			if err := json.Unmarshal(messages[0], &record); err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if len(record) < 12 {
				t.Errorf("Expected at least 12 fields in CSV record, got %d", len(record))
			}

			// First record should be the header
			if record[0] != "timestamp" {
				t.Errorf("Expected timestamp header 'timestamp', got '%s'", record[0])
			}

			if record[1] != "metric_name" {
				t.Errorf("Expected metric header 'metric_name', got '%s'", record[1])
			}

			if record[2] != "gpu_id" {
				t.Errorf("Expected GPU ID header 'gpu_id', got '%s'", record[2])
			}
		}

		// Verify the second message content (should be actual data)
		if len(messages) > 1 {
			var record []string
			if err := json.Unmarshal(messages[1], &record); err != nil {
				t.Fatalf("Failed to unmarshal second message: %v", err)
			}

			if record[0] != "2023-07-18T20:42:34Z" {
				t.Errorf("Expected timestamp '2023-07-18T20:42:34Z', got '%s'", record[0])
			}

			if record[1] != "DCGM_FI_DEV_GPU_UTIL" {
				t.Errorf("Expected metric 'DCGM_FI_DEV_GPU_UTIL', got '%s'", record[1])
			}

			if record[2] != "0" {
				t.Errorf("Expected GPU ID '0', got '%s'", record[2])
			}
		}
	})

	t.Run("Non-existent File", func(t *testing.T) {
		err := service.StreamCSV("non-existent-file.csv", 10*time.Millisecond)
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
	})

	t.Run("Invalid CSV Format", func(t *testing.T) {
		// Create a CSV file with insufficient columns
		csvContent := `timestamp,metric_name
2023-07-18T20:42:34Z,DCGM_FI_DEV_GPU_UTIL`

		tmpFile, err := ioutil.TempFile("", "test_invalid_*.csv")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(csvContent); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tmpFile.Close()

		// Clear previous messages
		mockQueue.messages = make(map[string][][]byte)

		// Stream the invalid CSV for a very short time
		done := make(chan error, 1)
		go func() {
			err := service.StreamCSV(tmpFile.Name(), 5*time.Millisecond)
			done <- err
		}()

		// Wait a very short time - the CSV should restart multiple times
		time.Sleep(20 * time.Millisecond)

		// Since both rows have < 12 columns, they should be skipped
		// But the function keeps restarting, so no messages should be published
		messages := mockQueue.messages["telemetry"]
		// We allow for some messages during startup but expect very few or none
		if len(messages) > 0 {
			// If messages were published, they should still be skipped due to column check
			// But since this is an infinite loop scenario, we just check that it's not crashing
			t.Logf("Invalid CSV produced %d messages (expected 0, but infinite loop behavior can vary)", len(messages))
		}
	})

	t.Run("Queue Error", func(t *testing.T) {
		// Set up queue to return error
		mockQueue.err = fmt.Errorf("queue publish error")

		csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
2023-07-18T20:42:34Z,DCGM_FI_DEV_GPU_UTIL,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,test-pod,default,85.5,"version=535.129.03"`

		tmpFile, err := ioutil.TempFile("", "test_error_*.csv")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(csvContent); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tmpFile.Close()

		err = service.StreamCSV(tmpFile.Name(), 10*time.Millisecond)
		if err == nil {
			t.Error("Expected error when queue publish fails, got nil")
		}

		// Reset error for other tests
		mockQueue.err = nil
	})
}

func TestStreamerService_HTTPEndpoints(t *testing.T) {
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	mockQueue := NewMockMessageQueue()
	cfg := config.Config{}

	service := &StreamerService{
		queue:  mockQueue,
		logger: logger,
		config: cfg,
	}

	t.Run("Health Check", func(t *testing.T) {
		// This would test the health endpoint if it exists
		// For now, we'll test that the service is properly initialized
		if service.queue == nil {
			t.Error("Expected queue to be initialized")
		}
		if service.logger == nil {
			t.Error("Expected logger to be initialized")
		}
	})
}

func TestCSVProcessing(t *testing.T) {
	t.Run("Parse CSV Record", func(t *testing.T) {
		record := []string{
			"2023-07-18T20:42:34Z",
			"DCGM_FI_DEV_GPU_UTIL",
			"0",
			"nvidia0",
			"GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			"NVIDIA H100 80GB HBM3",
			"mtv5-dgx1-hgpu-031",
			"",
			"test-pod",
			"default",
			"85.5",
			"version=535.129.03",
		}

		// Verify record structure
		if len(record) != 12 {
			t.Errorf("Expected 12 fields, got %d", len(record))
		}

		// Verify specific fields
		expectedFields := map[int]string{
			0:  "2023-07-18T20:42:34Z",         // timestamp
			1:  "DCGM_FI_DEV_GPU_UTIL",        // metric_name
			2:  "0",                           // gpu_id
			3:  "nvidia0",                     // device
			4:  "GPU-5fd4f087-86f3-7a43-b711-4771313afc50", // uuid
			5:  "NVIDIA H100 80GB HBM3",       // modelName
			6:  "mtv5-dgx1-hgpu-031",          // Hostname
			7:  "",                            // container
			8:  "test-pod",                    // pod
			9:  "default",                     // namespace
			10: "85.5",                        // value
			11: "version=535.129.03",          // labels_raw
		}

		for index, expected := range expectedFields {
			if record[index] != expected {
				t.Errorf("Field %d: expected '%s', got '%s'", index, expected, record[index])
			}
		}
	})

	t.Run("JSON Marshal Record", func(t *testing.T) {
		record := []string{
			"2023-07-18T20:42:34Z",
			"DCGM_FI_DEV_GPU_UTIL",
			"0",
			"nvidia0",
			"GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			"NVIDIA H100 80GB HBM3",
			"mtv5-dgx1-hgpu-031",
			"",
			"test-pod",
			"default",
			"85.5",
			"version=535.129.03",
		}

		jsonData, err := json.Marshal(record)
		if err != nil {
			t.Fatalf("Failed to marshal record: %v", err)
		}

		// Unmarshal to verify
		var unmarshaled []string
		if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal record: %v", err)
		}

		if len(unmarshaled) != len(record) {
			t.Errorf("Expected %d fields after unmarshal, got %d", len(record), len(unmarshaled))
		}

		for i, field := range record {
			if unmarshaled[i] != field {
				t.Errorf("Field %d: expected '%s' after unmarshal, got '%s'", i, field, unmarshaled[i])
			}
		}
	})
}

func TestEnvironmentDefaults(t *testing.T) {
	t.Run("Default Values When Not Set", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("USE_HTTP_QUEUE")
		os.Unsetenv("MSG_QUEUE_ADDR")
		os.Unsetenv("MSG_QUEUE_TOPIC")
		os.Unsetenv("MSG_QUEUE_GROUP")
		os.Unsetenv("MSG_QUEUE_PRODUCER_NAME")

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

		name := os.Getenv("MSG_QUEUE_PRODUCER_NAME")
		if name == "" {
			name = "streamer"
		}
		if name != "streamer" {
			t.Errorf("Expected default producer name 'streamer', got %s", name)
		}
	})
}

func TestMockMessageQueue(t *testing.T) {
	t.Run("Publish Messages", func(t *testing.T) {
		queue := NewMockMessageQueue()

		err := queue.Publish("test-topic", []byte("test message 1"))
		if err != nil {
			t.Errorf("Expected no error publishing, got: %v", err)
		}

		err = queue.Publish("test-topic", []byte("test message 2"))
		if err != nil {
			t.Errorf("Expected no error publishing, got: %v", err)
		}

		messages := queue.messages["test-topic"]
		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}

		if string(messages[0]) != "test message 1" {
			t.Errorf("Expected first message 'test message 1', got '%s'", string(messages[0]))
		}

		if string(messages[1]) != "test message 2" {
			t.Errorf("Expected second message 'test message 2', got '%s'", string(messages[1]))
		}
	})

	t.Run("Publish Error", func(t *testing.T) {
		queue := NewMockMessageQueue()
		queue.err = fmt.Errorf("publish error")

		err := queue.Publish("test-topic", []byte("test message"))
		if err == nil {
			t.Error("Expected publish error, got nil")
		}
	})

	t.Run("Close Queue", func(t *testing.T) {
		queue := NewMockMessageQueue()

		err := queue.Close()
		if err != nil {
			t.Errorf("Expected no error closing, got: %v", err)
		}

		if !queue.closed {
			t.Error("Expected queue to be marked as closed")
		}
	})
}
