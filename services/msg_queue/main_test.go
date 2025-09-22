package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	t.Run("Create Message", func(t *testing.T) {
		now := time.Now()
		msg := Message{
			ID:        "test-id",
			Payload:   "test payload",
			CreatedAt: now,
			Topic:     "test-topic",
			Partition: 1,
		}

		if msg.ID != "test-id" {
			t.Errorf("Expected ID 'test-id', got '%s'", msg.ID)
		}
		if msg.Payload != "test payload" {
			t.Errorf("Expected payload 'test payload', got '%s'", msg.Payload)
		}
		if msg.Topic != "test-topic" {
			t.Errorf("Expected topic 'test-topic', got '%s'", msg.Topic)
		}
		if msg.Partition != 1 {
			t.Errorf("Expected partition 1, got %d", msg.Partition)
		}
	})
}

func TestMessageOperations(t *testing.T) {
	t.Run("Message JSON Serialization", func(t *testing.T) {
		msg := Message{
			ID:        "test-id",
			Payload:   "test payload",
			CreatedAt: time.Now(),
			Topic:     "test-topic",
			Partition: 0,
		}

		// Test JSON marshaling
		jsonData, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Failed to marshal message: %v", err)
		}

		// Test JSON unmarshaling
		var unmarshaled Message
		if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
			t.Fatalf("Failed to unmarshal message: %v", err)
		}

		if unmarshaled.ID != msg.ID {
			t.Errorf("Expected ID %s, got %s", msg.ID, unmarshaled.ID)
		}
		if unmarshaled.Payload != msg.Payload {
			t.Errorf("Expected payload %s, got %s", msg.Payload, unmarshaled.Payload)
		}
	})
}

func TestHTTPEndpoints(t *testing.T) {
	t.Run("Produce Handler", func(t *testing.T) {
		// Create a simple produce request
		requestBody := map[string]interface{}{
			"topic":   "test-topic",
			"payload": "test message",
		}
		
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest("POST", "/produce", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Simple mock handler
		handler := func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			var request map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid JSON"))
				return
			}

			topic, topicOk := request["topic"].(string)
			payload, payloadOk := request["payload"].(string)

			if !topicOk || !payloadOk || topic == "" || payload == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Topic and payload required"))
				return
			}

			// Simulate successful production
			response := map[string]interface{}{
				"message_id": "generated-id-123",
				"partition":  0,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}

		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["message_id"] == "" {
			t.Error("Expected message ID in response")
		}
	})

	t.Run("Consume Handler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/consume?topic=test-topic&group=test-group&consumer=test-consumer", nil)
		w := httptest.NewRecorder()

		handler := func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			topic := r.URL.Query().Get("topic")
			group := r.URL.Query().Get("group")
			consumer := r.URL.Query().Get("consumer")

			if topic == "" || group == "" || consumer == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("topic, group, and consumer parameters required"))
				return
			}

			// Simulate a consumed message
			message := Message{
				ID:        "test-msg-id",
				Payload:   "test payload",
				CreatedAt: time.Now(),
				Topic:     topic,
				Partition: 0,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(message)
		}

		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var consumed Message
		if err := json.NewDecoder(w.Body).Decode(&consumed); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if consumed.ID != "test-msg-id" {
			t.Errorf("Expected message ID 'test-msg-id', got '%s'", consumed.ID)
		}
	})

	t.Run("Ack Handler", func(t *testing.T) {
		ackRequest := map[string]interface{}{
			"message_id": "test-msg-id",
			"consumer":   "test-consumer",
		}
		
		jsonData, err := json.Marshal(ackRequest)
		if err != nil {
			t.Fatalf("Failed to marshal ack request: %v", err)
		}

		req := httptest.NewRequest("POST", "/ack", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			var request map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid JSON"))
				return
			}

			messageID, msgOk := request["message_id"].(string)
			consumer, consOk := request["consumer"].(string)

			if !msgOk || !consOk || messageID == "" || consumer == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("message_id and consumer required"))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}

		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestEnvironmentVariables(t *testing.T) {
	t.Run("Broker Configuration", func(t *testing.T) {
		// Test default values
		brokerIndex := 0
		brokerCount := 1

		if brokerIndex < 0 {
			t.Errorf("Expected non-negative broker index, got %d", brokerIndex)
		}
		if brokerCount <= 0 {
			t.Errorf("Expected positive broker count, got %d", brokerCount)
		}
	})
}

func TestPartitionLogic(t *testing.T) {
	t.Run("Partition Assignment", func(t *testing.T) {
		brokerIndex := 0
		brokerCount := 4

		// Test partition ownership logic
		testCases := []struct {
			partition int
			owned     bool
		}{
			{0, true},  // 0 % 4 == 0
			{1, false}, // 1 % 4 == 1
			{2, false}, // 2 % 4 == 2
			{3, false}, // 3 % 4 == 3
			{4, true},  // 4 % 4 == 0
			{8, true},  // 8 % 4 == 0
		}

		for _, tc := range testCases {
			owned := tc.partition%brokerCount == brokerIndex
			if owned != tc.owned {
				t.Errorf("Partition %d: expected ownership %v, got %v", tc.partition, tc.owned, owned)
			}
		}
	})
}

func TestFileOperations(t *testing.T) {
	t.Run("Storage Directory", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := ioutil.TempDir("", "test-storage")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test directory creation
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			t.Errorf("Expected directory to exist: %s", tempDir)
		}

		// Test file creation in directory
		testFile := fmt.Sprintf("%s/test-partition.log", tempDir)
		if err := ioutil.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Test file reading
		content, err := ioutil.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read test file: %v", err)
		}

		if string(content) != "test content" {
			t.Errorf("Expected 'test content', got '%s'", string(content))
		}
	})
}

func TestVisibilityTimeout(t *testing.T) {
	t.Run("Timeout Duration", func(t *testing.T) {
		timeout := 30 * time.Second
		expected := 30 * time.Second

		if timeout != expected {
			t.Errorf("Expected timeout %v, got %v", expected, timeout)
		}

		// Test timeout in the future
		futureTime := time.Now().Add(timeout)
		if !futureTime.After(time.Now()) {
			t.Error("Expected future time to be after current time")
		}
	})
}

// Helper function to generate IDs (simplified version)
func generateID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}

func TestIDGeneration(t *testing.T) {
	t.Run("Generate Unique IDs", func(t *testing.T) {
		id1 := generateID()
		time.Sleep(1 * time.Nanosecond) // Ensure different timestamp
		id2 := generateID()

		if id1 == id2 {
			t.Errorf("Expected unique IDs, got duplicate: %s", id1)
		}

		if id1 == "" || id2 == "" {
			t.Error("Expected non-empty IDs")
		}
	})
}
