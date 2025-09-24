package shared

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// HTTPMessageQueue implements a client for the msg_queue service
type HTTPMessageQueue struct {
	baseURL string
	client  *http.Client
	topic   string
	group   string
	name    string

	// Round-robin partition assignment - separate counters for publish/subscribe
	maxPartitions    int
	publishCounter   uint64
	subscribeCounter uint64
}

// Message represents a message from the queue
type QueueMessage struct {
	ID        string    `json:"id"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
	Topic     string    `json:"topic"`
	Partition int       `json:"partition"`
}

// NewHTTPMessageQueue creates a new HTTP message queue client
func NewHTTPMessageQueue(baseURL, topic, group, name string) (*HTTPMessageQueue, error) {
	// Get max partitions from environment, default to 4
	maxPartitions := 4
	if envPartitions := os.Getenv("MAX_PARTITIONS"); envPartitions != "" {
		if parsed, err := strconv.Atoi(envPartitions); err == nil && parsed > 0 {
			maxPartitions = parsed
		}
	}

	return &HTTPMessageQueue{
		baseURL:          baseURL,
		client:           &http.Client{Timeout: 60 * time.Second},
		topic:            topic,
		group:            group,
		name:             name,
		maxPartitions:    maxPartitions,
		publishCounter:   0,
		subscribeCounter: 0,
	}, nil
}

// calculatePublishPartition returns the next partition for publishing in round-robin fashion
func (h *HTTPMessageQueue) calculatePublishPartition(topic string) int {
	// Atomic increment for thread safety
	current := atomic.AddUint64(&h.publishCounter, 1)
	return int((current - 1) % uint64(h.maxPartitions))
}

// calculateSubscribePartition returns the next partition for subscribing in round-robin fashion
func (h *HTTPMessageQueue) calculateSubscribePartition(topic string) int {
	// Atomic increment for thread safety
	current := atomic.AddUint64(&h.subscribeCounter, 1)
	return int((current - 1) % uint64(h.maxPartitions))
}

// Publish sends a message to the queue
func (h *HTTPMessageQueue) Publish(topic string, payload []byte) error {
	// Calculate partition using separate publish counter (client-side partition assignment)
	partition := h.calculatePublishPartition(topic)

	// Log partition assignment for visibility
	fmt.Printf("[%s] Publishing to topic=%s, partition=%d (publish round-robin assignment)\n", h.name, topic, partition)

	// Send partition explicitly to proxy - no key needed
	url := fmt.Sprintf("%s/produce?topic=%s&partition=%d", h.baseURL, topic, partition)

	// Create request body with payload
	reqBody := map[string]string{
		"payload": string(payload),
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := h.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("publish failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Subscribe starts consuming messages from the queue
func (h *HTTPMessageQueue) Subscribe(handler func(string, []byte, string) error) error {
	// Calculate partition using separate subscribe counter
	partition := h.calculateSubscribePartition(h.topic)
	url := fmt.Sprintf("%s/consume?topic=%s&partition=%d&group=%s", h.baseURL, h.topic, partition, h.group)

	// Log which partition this consumer is using
	fmt.Printf("[%s] Consumer subscribing to topic=%s, partition=%d (subscribe round-robin assignment)\n", h.name, h.topic, partition)

	// Create context for cancellation
	ctx := context.Background()

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := h.client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to start consuming: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("consume failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Parse Server-Sent Events
		scanner := bufio.NewScanner(resp.Body)
		var messageID string
		var messageData string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "id: ") {
				messageID = strings.TrimPrefix(line, "id: ")
			} else if strings.HasPrefix(line, "data: ") {
				messageData = strings.TrimPrefix(line, "data: ")
			} else if line == "" && messageID != "" && messageData != "" {
				// End of message, parse and handle
				var msg QueueMessage
				if err := json.Unmarshal([]byte(messageData), &msg); err != nil {
					fmt.Printf("Failed to decode message: %v\n", err)
					messageID = ""
					messageData = ""
					continue
				}

				// Process the message
				if err := handler(msg.Topic, []byte(msg.Payload), msg.ID); err != nil {
					// Log error but continue processing
					fmt.Printf("Message handler error: %v\n", err)
				} else {
					// Acknowledge the message only if handler succeeded
					if err := h.ackMessage(msg.Topic, msg.Partition, msg.ID); err != nil {
						fmt.Printf("Failed to ack message %s: %v\n", msg.ID, err)
					}
				}

				// Reset for next message
				messageID = ""
				messageData = ""
			}
		}

		resp.Body.Close()

		if err := scanner.Err(); err != nil {
			fmt.Printf("Scanner error: %v\n", err)
		}

		// Wait a bit before reconnecting
		time.Sleep(time.Second)
	}
}

// ackMessage acknowledges a processed message
func (h *HTTPMessageQueue) ackMessage(topic string, partition int, messageID string) error {
	url := fmt.Sprintf("%s/ack?topic=%s&partition=%d&group=%s", h.baseURL, topic, partition, h.group)

	reqBody := map[string]string{
		"id": messageID,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal ack request: %w", err)
	}

	resp, err := h.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to ack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ack failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the HTTP client (no-op for HTTP client)
func (h *HTTPMessageQueue) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// GetTopics returns available topics (for compatibility)
func (h *HTTPMessageQueue) GetTopics() (map[string][]int, error) {
	url := fmt.Sprintf("%s/topics", h.baseURL)

	resp, err := h.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get topics failed with status %d: %s", resp.StatusCode, string(body))
	}

	var topics map[string][]int
	if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
		return nil, fmt.Errorf("failed to decode topics response: %w", err)
	}

	return topics, nil
}
