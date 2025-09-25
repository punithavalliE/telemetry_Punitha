// main.go
//
// Simple scalable message queue broker in Go.
// Features:
// - Topics and fixed number of partitions per topic.
// - Dynamic partition creation: partitions are created on-demand when first accessed
//   (you can run multiple broker instances for load balancing).
// - HTTP API for producing messages, consuming (SSE), ack-ing messages.
// - In-memory queue with append-only file persistence per partition.
// - Visibility timeout for in-flight messages and automatic requeue on timeout.

package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/telemetry/internal/metrics"
)

const (
	defaultVisibilityTimeout = 30 * time.Second
	storageDir               = "./data"
)

// Message is the unit of transfer.
type Message struct {
	ID        string    `json:"id"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
	Topic     string    `json:"topic"`
	Partition int       `json:"partition"`
	// attempt meta (not serialized)
}

// pending holds in-flight message meta for ack/timeouts.
type pending struct {
	msg      Message
	deadline time.Time
	group    string
}

// Partition holds the queue and persistence for a single partition.
type Partition struct {
	topic     string
	index     int
	queue     chan Message // main queue
	pendingMu sync.Mutex
	pending   map[string]pending // messageID -> pending
	file      *os.File
	fileMu    sync.Mutex
	visTO     time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
}

func newPartition(topic string, index int, visTO time.Duration) (*Partition, error) {
	dir := filepath.Join(storageDir, topic)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	fpath := filepath.Join(dir, fmt.Sprintf("partition-%d.log", index))
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Partition{
		topic:   topic,
		index:   index,
		queue:   make(chan Message, 2000),
		pending: make(map[string]pending),
		file:    f,
		visTO:   visTO,
		ctx:     ctx,
		cancel:  cancel,
	}
	// load persisted messages into queue asynchronously to avoid blocking
	// Commenting out file loading to test timeout issues
	// go func() {
	// 	if err := p.loadFromFile(); err != nil {
	// 		log.Printf("partition %s-%d: failed to load from file: %v", topic, index, err)
	// 	} else {
	// 		log.Printf("partition %s-%d: successfully loaded messages from file", topic, index)
	// 	}
	// }()
	// start monitor for timeouts
	go p.monitorPending()
	return p, nil
}

func (p *Partition) Close() {
	p.cancel()
	p.file.Close()
	close(p.queue)
}

func (p *Partition) persist(m Message) error {
	p.fileMu.Lock()
	defer p.fileMu.Unlock()
	b, _ := json.Marshal(m)
	_, err := p.file.Write(append(b, '\n'))
	if err != nil {
		return err
	}
	// Commenting out sync to avoid blocking HTTP responses
	// return p.file.Sync()
	return nil
}

func (p *Partition) loadFromFile() error {
	p.fileMu.Lock()
	defer p.fileMu.Unlock()
	// read from beginning
	if _, err := p.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	scanner := bufio.NewScanner(p.file)
	for scanner.Scan() {
		var m Message
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			log.Printf("partition %s-%d: skip bad line: %v", p.topic, p.index, err)
			continue
		}
		// push into queue (non-blocking)
		select {
		case p.queue <- m:
			// Successfully loaded message
		default:
			// Queue is full, skip this persisted message
			log.Printf("partition %s-%d: skipping persisted message %s - queue full", p.topic, p.index, m.ID)
		}
	}
	// seek to end for future appends
	_, _ = p.file.Seek(0, io.SeekEnd)
	return nil
}

func (p *Partition) enqueue(m Message) error {
	// persist then push to queue
	if err := p.persist(m); err != nil {
		return err
	}
	log.Printf("partition %s-%d: queue size before enqueue: %d", p.topic, p.index, len(p.queue))

	// Non-blocking enqueue to prevent HTTP handler from hanging
	select {
	case p.queue <- m:
		return nil
	default:
		// Queue is full - return error instead of blocking
		log.Printf("partition %s-%d: queue full (%d messages), rejecting message %s", p.topic, p.index, len(p.queue), m.ID)
		return fmt.Errorf("queue full (%d messages)", len(p.queue))
	}
}

func (p *Partition) monitorPending() {
	ticker := time.NewTicker(50 * time.Second)

	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case now := <-ticker.C:
			p.pendingMu.Lock()
			for id, pd := range p.pending {
				if now.After(pd.deadline) {
					// requeue the message
					log.Printf("visibility timeout: requeue msg %s (topic=%s p=%d group=%s)", id, p.topic, p.index, pd.group)
					// remove from pending and re-enqueue
					delete(p.pending, id)
					// push back to queue (as new attempt; ID remains same)
					log.Printf("partition %s-%d: queue size before requeue: %d", p.topic, p.index, len(p.queue))
					select {
					case p.queue <- pd.msg:
						// Successfully requeued
						delete(p.pending, id)
					default:
						// Queue is full, cannot requeue - message will be lost
						log.Printf("partition %s-%d: cannot requeue message %s - queue full, message lost", p.topic, p.index, id)
					}
				}
			}
			p.pendingMu.Unlock()
		}
	}
}

func (p *Partition) fetchAndTrack(group string) (Message, error) {
	select {
	case <-p.ctx.Done():
		return Message{}, errors.New("partition closed")
	case msg := <-p.queue:
		// track as pending for this group
		p.pendingMu.Lock()
		p.pending[msg.ID] = pending{
			msg:      msg,
			deadline: time.Now().Add(p.visTO),
			group:    group,
		}
		p.pendingMu.Unlock()
		return msg, nil
	case <-time.After(5 * time.Second):
		// Return empty message after timeout - consumer will retry
		return Message{}, errors.New("no messages available")
	}
}

func (p *Partition) ack(msgID string, group string) bool {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()
	pd, ok := p.pending[msgID]
	if !ok {
		return false
	}
	if pd.group != group {
		// ack from wrong group/consumer
		return false
	}
	delete(p.pending, msgID)
	return true
}

// Broker coordinates topics and partitions.
type Broker struct {
	topics       map[string]int // topic -> partitions count
	partitions   map[string]map[int]*Partition
	visTO        time.Duration
	brokerIndex  int
	brokerCount  int
	partitionsMu sync.RWMutex
}

func NewBroker(topics map[string]int, visTO time.Duration, brokerIndex, brokerCount int) (*Broker, error) {
	b := &Broker{
		topics:      topics,
		partitions:  make(map[string]map[int]*Partition),
		visTO:       visTO,
		brokerIndex: brokerIndex,
		brokerCount: brokerCount,
	}
	// Initialize partition maps for topics but don't create partitions yet
	for topic := range topics {
		b.partitions[topic] = make(map[int]*Partition)
		log.Printf("initialized topic %s (partitions will be created on-demand)", topic)
	}
	return b, nil
}

func (b *Broker) Close() {
	for _, pm := range b.partitions {
		for _, p := range pm {
			p.Close()
		}
	}
}

// createPartitionIfNotExists creates a partition if it doesn't exist
func (b *Broker) createPartitionIfNotExists(topic string, partition int) (*Partition, error) {
	b.partitionsMu.Lock()
	defer b.partitionsMu.Unlock()

	// Check if topic exists
	pm, ok := b.partitions[topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic")
	}

	// Check if partition already exists
	if p, exists := pm[partition]; exists {
		return p, nil
	}

	// Check if partition is within valid range
	maxPartitions, ok := b.topics[topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic")
	}
	if partition >= maxPartitions {
		return nil, fmt.Errorf("partition %d exceeds max partitions %d for topic %s", partition, maxPartitions, topic)
	}

	// Create new partition
	p, err := newPartition(topic, partition, b.visTO)
	if err != nil {
		return nil, fmt.Errorf("create partition %s-%d error: %w", topic, partition, err)
	}

	pm[partition] = p
	log.Printf("dynamically created partition %s-%d", topic, partition)
	return p, nil
}

func (b *Broker) getPartition(topic string, partition int, isProduceHandling bool) (*Partition, error) {
	b.partitionsMu.RLock()
	pm, ok := b.partitions[topic]
	if !ok {
		b.partitionsMu.RUnlock()
		return nil, fmt.Errorf("unknown topic")
	}
	p, exists := pm[partition]
	b.partitionsMu.RUnlock()

	if !exists {
		// Only create partition if this is produce handling
		if !isProduceHandling {
			return nil, fmt.Errorf("partition %d does not exist for topic %s", partition, topic)
		}
		// Partition doesn't exist, create it dynamically
		return b.createPartitionIfNotExists(topic, partition)
	}

	return p, nil
}

// produceHandler: POST /produce?topic=foo&partition=0
// body: raw payload (text) or JSON {"payload":"..."}
// If partition is not specified, auto-assign to an available partition
func (b *Broker) produceHandler(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	partStr := r.URL.Query().Get("partition")
	log.Printf("Broker received produce request: topic=%s, partition=%s", topic, partStr)

	if topic == "" || partStr == "" {
		log.Printf("Rejecting request: topic and partition required")
		http.Error(w, "topic and partition required", http.StatusBadRequest)
		return
	}

	var part int
	var err error

	part, err = strconv.Atoi(partStr)
	if err != nil {
		http.Error(w, "bad partition", http.StatusBadRequest)
		return
	}
	log.Printf("Publishing message for partition %d for topic %s", part, topic)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body error", http.StatusBadRequest)
		return
	}
	payload := strings.TrimSpace(string(body))
	// If JSON with payload field, try to decode
	if strings.HasPrefix(payload, "{") {
		var tmp struct {
			Payload string `json:"payload"`
		}
		if err := json.Unmarshal([]byte(payload), &tmp); err == nil && tmp.Payload != "" {
			payload = tmp.Payload
		}
	}
	msg := Message{
		ID:        genID(),
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
		Topic:     topic,
		Partition: part,
	}
	p, err := b.getPartition(topic, part, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := p.enqueue(msg); err != nil {
		http.Error(w, "enqueue failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": msg.ID})
}

// consumeHandler: GET /consume?topic=foo&partition=0&group=g1
// uses Server-Sent Events (text/event-stream)
// If partition is not specified, auto-assign to an owned partition
func (b *Broker) consumeHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	topic := r.URL.Query().Get("topic")
	partStr := r.URL.Query().Get("partition")
	group := r.URL.Query().Get("group")
	log.Printf("Broker received consume request: topic=%s, partition=%s, group=%s", topic, partStr, group)

	if topic == "" || partStr == "" || group == "" {
		log.Printf("Rejecting consume request: topic, partition and group required")
		http.Error(w, "topic, partition and group required", http.StatusBadRequest)
		return
	}

	var part int
	var err error

	part, err = strconv.Atoi(partStr)
	if err != nil {
		http.Error(w, "bad partition", http.StatusBadRequest)
		return
	}
	p, err := b.getPartition(topic, part, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	// consumer loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msg, err := p.fetchAndTrack(group)
		if err != nil {
			// Check if it's a timeout (no messages available) vs partition closed
			if err.Error() == "no messages available" {
				// Just continue polling - don't send anything to client
				time.Sleep(1 * time.Second) // Small delay before retry
				continue
			}
			// partition closed or other error
			return
		}
		data, _ := json.Marshal(msg)
		// SSE format
		fmt.Fprintf(w, "id: %s\n", msg.ID)
		fmt.Fprintf(w, "data: %s\n", string(data))
		fmt.Fprintf(w, "partition: %d\n\n", msg.Partition)
		flusher.Flush()
		// continue to next message
	}
}

// ackHandler: POST /ack?topic=foo&partition=0&group=g1
// body: {"id":"..."}
func (b *Broker) ackHandler(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	partStr := r.URL.Query().Get("partition")
	group := r.URL.Query().Get("group")
	if topic == "" || partStr == "" || group == "" {
		http.Error(w, "topic, partition and group required", http.StatusBadRequest)
		return
	}
	part, err := strconv.Atoi(partStr)
	if err != nil {
		http.Error(w, "bad partition", http.StatusBadRequest)
		return
	}
	p, err := b.getPartition(topic, part, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	ok := p.ack(body.ID, group)
	if !ok {
		http.Error(w, "ack failed (unknown id or wrong group)", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (b *Broker) topicsHandler(w http.ResponseWriter, r *http.Request) {
	// returns partitions owned by this broker
	out := make(map[string][]int)
	for t, pm := range b.partitions {
		for idx := range pm {
			out[t] = append(out[t], idx)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (b *Broker) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Simple health check - return owned partitions count
	b.partitionsMu.RLock()
	totalPartitions := 0
	for _, pm := range b.partitions {
		totalPartitions += len(pm)
	}
	b.partitionsMu.RUnlock()

	health := map[string]interface{}{
		"status":           "healthy",
		"broker_index":     b.brokerIndex,
		"broker_count":     b.brokerCount,
		"owned_partitions": totalPartitions,
		"timestamp":        time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(health)
}

func main() {
	// Initialize Prometheus metrics
	metrics.InitMetrics("msg-queue-service")
	log.Println("Prometheus metrics initialized")

	// Configuration (could be flags/env)
	topicsConf := map[string]int{
		"events":  8,
		"orders":  4,
		"default": 8,
	}
	visTO := defaultVisibilityTimeout

	// Broker index/count for partition ownership (env)
	brokerIndex := 0
	brokerCount := 1
	if v := os.Getenv("BROKER_INDEX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			brokerIndex = n
		}
	}
	if v := os.Getenv("BROKER_COUNT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			brokerCount = n
		}
	}
	// override topics via env TOPICS=events:8,orders:4
	if tcfg := os.Getenv("TOPICS"); tcfg != "" {
		topicsConf = map[string]int{}
		for _, part := range strings.Split(tcfg, ",") {
			if part == "" {
				continue
			}
			kv := strings.Split(part, ":")
			if len(kv) != 2 {
				continue
			}
			n, _ := strconv.Atoi(kv[1])
			topicsConf[kv[0]] = n
		}
	}

	// Create storage dir
	_ = os.MkdirAll(storageDir, 0o755)

	broker, err := NewBroker(topicsConf, visTO, brokerIndex, brokerCount)
	if err != nil {
		log.Fatalf("broker init failed: %v", err)
	}
	defer broker.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/produce", broker.produceHandler)
	mux.HandleFunc("/consume", broker.consumeHandler)
	mux.HandleFunc("/ack", broker.ackHandler)
	mux.HandleFunc("/topics", broker.topicsHandler)
	mux.HandleFunc("/health", broker.healthHandler)

	// Add Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.MetricsHandler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("broker starting on %s (index=%d count=%d)", addr, brokerIndex, brokerCount)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// genID generates a URL-safe random id (~22 chars).
func genID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
