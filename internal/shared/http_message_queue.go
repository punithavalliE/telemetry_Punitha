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

	// Round-robin partition assignment for publishing
	maxPartitions  int
	publishCounter uint64
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
	// Get max partitions from environment, default to 2
	maxPartitions := 2
	if envPartitions := os.Getenv("MAX_PARTITIONS"); envPartitions != "" {
		if parsed, err := strconv.Atoi(envPartitions); err == nil && parsed > 0 {
			maxPartitions = parsed
		}
	}

	return &HTTPMessageQueue{
		baseURL:        baseURL,
		client:         &http.Client{Timeout: 120 * time.Second}, // Increased timeout for better resilience
		topic:          topic,
		group:          group,
		name:           name,
		maxPartitions:  maxPartitions,
		publishCounter: 0,
	}, nil
}

// calculatePublishPartition returns the next partition for publishing in round-robin fashion
func (h *HTTPMessageQueue) calculatePublishPartition(topic string) int {
	// Atomic increment for thread safety
	current := atomic.AddUint64(&h.publishCounter, 1)
	return int((current - 1) % uint64(h.maxPartitions))
}

// Publish sends a message to the queue with retry logic
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

	// Retry logic for publish
	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := time.Duration(attempt) * baseDelay
			fmt.Printf("[%s] Retrying publish to partition %d after %v (attempt %d/%d)\n", h.name, partition, delay, attempt+1, maxRetries)
			time.Sleep(delay)
		}

		resp, err := h.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to publish message after %d attempts: %w", maxRetries, err)
			}
			fmt.Printf("[%s] Publish attempt %d failed: %v\n", h.name, attempt+1, err)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil // Success!
		}

		body, _ := io.ReadAll(resp.Body)
		if attempt == maxRetries-1 {
			return fmt.Errorf("publish failed after %d attempts with status %d: %s", maxRetries, resp.StatusCode, string(body))
		}
		fmt.Printf("[%s] Publish attempt %d failed with status %d: %s\n", h.name, attempt+1, resp.StatusCode, string(body))
	}

	return fmt.Errorf("publish failed after %d attempts", maxRetries)
}

// Subscribe starts consuming messages from the queue (consumes from all partitions)
func (h *HTTPMessageQueue) Subscribe(handler func(string, []byte, string) error) error {
	// Start consumer goroutines for all partitions
	errChan := make(chan error, h.maxPartitions)

	for partition := 0; partition < h.maxPartitions; partition++ {
		partition := partition // capture loop variable
		go func() {
			fmt.Printf("[%s] Starting consumer for partition %d\n", h.name, partition)
			h.consumeFromPartition(partition, handler, errChan)
		}()
	}

	// Wait for any consumer to report an error (this blocks indefinitely)
	return <-errChan
}

// consumeFromPartition handles consumption from a specific partition
func (h *HTTPMessageQueue) consumeFromPartition(partition int, handler func(string, []byte, string) error, errChan chan error) {
	url := fmt.Sprintf("%s/consume?topic=%s&partition=%d&group=%s", h.baseURL, h.topic, partition, h.group)

	// Create context for cancellation
	ctx := context.Background()

	for {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		resp, err := h.client.Do(req)
		if err != nil {
			// Check if it's a timeout error
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
				fmt.Printf("[%s] Consume timeout from partition %d, retrying in 5s: %v\n", h.name, partition, err)
				time.Sleep(5 * time.Second)
			} else {
				fmt.Printf("[%s] Failed to start consuming from partition %d: %v\n", h.name, partition, err)
				time.Sleep(time.Second)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			
			// Use longer delay for server errors
			delay := time.Second
			if resp.StatusCode >= 500 {
				delay = 5 * time.Second
				fmt.Printf("[%s] Server error from partition %d (status %d), retrying in %v: %s\n", h.name, partition, resp.StatusCode, delay, string(body))
			} else {
				fmt.Printf("[%s] Consume failed from partition %d with status %d: %s\n", h.name, partition, resp.StatusCode, string(body))
			}
			
			time.Sleep(delay)
			continue
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
			// Check if it's a timeout/connection error
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "EOF") {
				fmt.Printf("[%s] Connection lost from partition %d, reconnecting in 5s: %v\n", h.name, partition, err)
				time.Sleep(5 * time.Second)
			} else {
				fmt.Printf("[%s] Scanner error from partition %d: %v\n", h.name, partition, err)
				time.Sleep(time.Second)
			}
		} else {
			// Normal disconnect, wait briefly before reconnecting
			fmt.Printf("[%s] Connection closed from partition %d, reconnecting...\n", h.name, partition)
			time.Sleep(time.Second)
		}
	}
}

// ackMessage acknowledges a processed message with retry logic
func (h *HTTPMessageQueue) ackMessage(topic string, partition int, messageID string) error {
	url := fmt.Sprintf("%s/ack?topic=%s&partition=%d&group=%s", h.baseURL, topic, partition, h.group)

	reqBody := map[string]string{
		"id": messageID,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal ack request: %w", err)
	}

	// Retry ACK a few times
	maxRetries := 2
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("[%s] Retrying ACK for message %s (attempt %d/%d)\n", h.name, messageID, attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		resp, err := h.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to ack message after %d attempts: %w", maxRetries, err)
			}
			fmt.Printf("[%s] ACK attempt %d failed: %v\n", h.name, attempt+1, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil // Success!
		}

		body, _ := io.ReadAll(resp.Body)
		if attempt == maxRetries-1 {
			return fmt.Errorf("ack failed after %d attempts with status %d: %s", maxRetries, resp.StatusCode, string(body))
		}
		fmt.Printf("[%s] ACK attempt %d failed with status %d: %s\n", h.name, attempt+1, resp.StatusCode, string(body))
	}

	return fmt.Errorf("ack failed after %d attempts", maxRetries)
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
