package mocks

import (
	"time"
	"fmt"

	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// MockInfluxWriter is a mock implementation of InfluxDB writer for testing
type MockInfluxWriter struct {
	points     []*write.Point
	queryData  []interface{}
	err        error
	closed     bool
	writeDelay time.Duration
}

// NewMockInfluxWriter creates a new mock InfluxDB writer
func NewMockInfluxWriter() *MockInfluxWriter {
	return &MockInfluxWriter{
		points:    make([]*write.Point, 0),
		queryData: make([]interface{}, 0),
	}
}

// SetError sets the error that the mock should return
func (m *MockInfluxWriter) SetError(err error) {
	m.err = err
}

// SetQueryData sets the data that queries should return
func (m *MockInfluxWriter) SetQueryData(data []interface{}) {
	m.queryData = data
}

// SetWriteDelay sets a delay for write operations to simulate latency
func (m *MockInfluxWriter) SetWriteDelay(delay time.Duration) {
	m.writeDelay = delay
}

// WritePoints implements the InfluxWriter interface
func (m *MockInfluxWriter) WritePoints(points []*write.Point) error {
	if m.writeDelay > 0 {
		time.Sleep(m.writeDelay)
	}
	
	if m.err != nil {
		return m.err
	}
	
	m.points = append(m.points, points...)
	return nil
}

// Close implements the InfluxWriter interface
func (m *MockInfluxWriter) Close() {
	m.closed = true
}

// QueryRecentTelemetry implements query functionality for testing
func (m *MockInfluxWriter) QueryRecentTelemetry(limit int) ([]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	if len(m.queryData) > limit {
		return m.queryData[:limit], nil
	}
	return m.queryData, nil
}

// QueryTelemetryByGPUID implements GPU-specific query functionality
func (m *MockInfluxWriter) QueryTelemetryByGPUID(gpuID string, startTime, endTime *time.Time, limit int) ([]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	// Filter by GPU ID for testing
	var filtered []interface{}
	for _, data := range m.queryData {
		// This would need to be implemented based on your actual data structure
		filtered = append(filtered, data)
	}
	
	if len(filtered) > limit {
		return filtered[:limit], nil
	}
	return filtered, nil
}

// GetWrittenPoints returns the points that were written (for testing)
func (m *MockInfluxWriter) GetWrittenPoints() []*write.Point {
	return m.points
}

// GetWriteCount returns the number of write operations
func (m *MockInfluxWriter) GetWriteCount() int {
	return len(m.points)
}

// Reset clears all data and resets the mock
func (m *MockInfluxWriter) Reset() {
	m.points = make([]*write.Point, 0)
	m.queryData = make([]interface{}, 0)
	m.err = nil
	m.closed = false
	m.writeDelay = 0
}

// IsClosed returns whether the client was closed
func (m *MockInfluxWriter) IsClosed() bool {
	return m.closed
}

// MockMessageQueue is a mock implementation of message queue for testing
type MockMessageQueue struct {
	messages    map[string][][]byte // topic -> messages
	consumed    [][]byte
	err         error
	closed      bool
	publishFunc func(topic string, message []byte) error
	consumeFunc func() ([]byte, error)
}

// NewMockMessageQueue creates a new mock message queue
func NewMockMessageQueue() *MockMessageQueue {
	return &MockMessageQueue{
		messages: make(map[string][][]byte),
		consumed: make([][]byte, 0),
	}
}

// SetError sets the error that the mock should return
func (m *MockMessageQueue) SetError(err error) {
	m.err = err
}

// SetPublishFunc sets a custom publish function for testing
func (m *MockMessageQueue) SetPublishFunc(fn func(topic string, message []byte) error) {
	m.publishFunc = fn
}

// SetConsumeFunc sets a custom consume function for testing
func (m *MockMessageQueue) SetConsumeFunc(fn func() ([]byte, error)) {
	m.consumeFunc = fn
}

// Publish implements the message queue publish interface
func (m *MockMessageQueue) Publish(topic string, message []byte) error {
	if m.publishFunc != nil {
		return m.publishFunc(topic, message)
	}
	
	if m.err != nil {
		return m.err
	}
	
	if m.messages[topic] == nil {
		m.messages[topic] = make([][]byte, 0)
	}
	m.messages[topic] = append(m.messages[topic], message)
	return nil
}

// Produce is an alias for Publish to match different interfaces
func (m *MockMessageQueue) Produce(message string) error {
	return m.Publish("default", []byte(message))
}

// Consume implements the message queue consume interface
func (m *MockMessageQueue) Consume() (string, error) {
	if m.consumeFunc != nil {
		data, err := m.consumeFunc()
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	
	if m.err != nil {
		return "", m.err
	}
	
	// Return messages from all topics
	for topic, messages := range m.messages {
		if len(messages) > 0 {
			msg := messages[0]
			m.messages[topic] = messages[1:]
			m.consumed = append(m.consumed, msg)
			return string(msg), nil
		}
	}
	
	// No messages available
	time.Sleep(10 * time.Millisecond) // Simulate blocking
	return "", nil
}

// Subscribe implements subscription functionality
func (m *MockMessageQueue) Subscribe(topic, group, name string) error {
	return m.err
}

// Close implements the close interface
func (m *MockMessageQueue) Close() error {
	m.closed = true
	return m.err
}

// GetMessages returns messages for a specific topic (for testing)
func (m *MockMessageQueue) GetMessages(topic string) [][]byte {
	return m.messages[topic]
}

// GetConsumedMessages returns all consumed messages (for testing)
func (m *MockMessageQueue) GetConsumedMessages() [][]byte {
	return m.consumed
}

// GetMessageCount returns the total number of messages across all topics
func (m *MockMessageQueue) GetMessageCount() int {
	count := 0
	for _, messages := range m.messages {
		count += len(messages)
	}
	return count
}

// AddMessage adds a message directly to the queue (for testing)
func (m *MockMessageQueue) AddMessage(topic string, message []byte) {
	if m.messages[topic] == nil {
		m.messages[topic] = make([][]byte, 0)
	}
	m.messages[topic] = append(m.messages[topic], message)
}

// Reset clears all data and resets the mock
func (m *MockMessageQueue) Reset() {
	m.messages = make(map[string][][]byte)
	m.consumed = make([][]byte, 0)
	m.err = nil
	m.closed = false
	m.publishFunc = nil
	m.consumeFunc = nil
}

// IsClosed returns whether the queue was closed
func (m *MockMessageQueue) IsClosed() bool {
	return m.closed
}

// SimulatePublishError creates a function that returns an error after n successful publishes
func (m *MockMessageQueue) SimulatePublishError(afterCount int, err error) {
	count := 0
	m.SetPublishFunc(func(topic string, message []byte) error {
		count++
		if count > afterCount {
			return err
		}
		
		if m.messages[topic] == nil {
			m.messages[topic] = make([][]byte, 0)
		}
		m.messages[topic] = append(m.messages[topic], message)
		return nil
	})
}

// SimulateConsumeError creates a function that returns an error after n successful consumes
func (m *MockMessageQueue) SimulateConsumeError(afterCount int, err error) {
	count := 0
	m.SetConsumeFunc(func() ([]byte, error) {
		count++
		if count > afterCount {
			return nil, err
		}
		
		// Return messages from all topics
		for topic, messages := range m.messages {
			if len(messages) > 0 {
				msg := messages[0]
				m.messages[topic] = messages[1:]
				m.consumed = append(m.consumed, msg)
				return msg, nil
			}
		}
		
		return nil, fmt.Errorf("no messages available")
	})
}
