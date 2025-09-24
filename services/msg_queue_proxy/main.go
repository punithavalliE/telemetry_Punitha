package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	consistenthash "github.com/example/telemetry/internal/consistent_hash"
	"github.com/example/telemetry/internal/metrics"
)

// ProxyConfig holds configuration for the smart proxy
type ProxyConfig struct {
	Port           string
	BrokerService  string // Kubernetes service name for brokers
	BrokerCount    int
	VirtualNodes   int
	MaxPartitions  int
	HealthInterval time.Duration
}

// SmartProxy routes requests to appropriate brokers using consistent hashing
type SmartProxy struct {
	config          ProxyConfig
	consistentHash  *consistenthash.ConsistentHash
	brokerEndpoints []string
	healthyBrokers  map[string]bool
	mu              sync.RWMutex
	client          *http.Client

	// Metrics tracking
	stats     ProxyStats
	startTime time.Time
}

// ProxyStats holds detailed statistics for monitoring
type ProxyStats struct {
	// Request counters (atomic for thread safety)
	TotalRequests   int64
	ProduceRequests int64
	ConsumeRequests int64
	AckRequests     int64
	HealthRequests  int64

	// Response counters
	SuccessfulRequests int64
	FailedRequests     int64

	// Latency tracking
	TotalLatencyMs int64
	RequestCount   int64

	// Per-broker request distribution
	BrokerRequestCounts map[string]int64
	BrokerErrors        map[string]int64

	// Health check stats
	HealthCheckCount int64
	BrokerFailures   int64

	mu sync.RWMutex
}

// NewSmartProxy creates a new smart proxy instance
func NewSmartProxy(config ProxyConfig) *SmartProxy {
	return &SmartProxy{
		config:         config,
		healthyBrokers: make(map[string]bool),
		startTime:      time.Now(),
		stats: ProxyStats{
			BrokerRequestCounts: make(map[string]int64),
			BrokerErrors:        make(map[string]int64),
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Start initializes the proxy and starts the HTTP server
func (sp *SmartProxy) Start() error {
	// Initialize Prometheus metrics
	metrics.InitMetrics("msg-queue-proxy")
	log.Println("Prometheus metrics initialized for smart proxy")

	// Discover brokers
	if err := sp.discoverBrokers(); err != nil {
		return fmt.Errorf("failed to discover brokers: %w", err)
	}

	// Initialize consistent hash
	sp.initConsistentHash()

	// Initialize broker metrics maps
	sp.initBrokerMetrics()

	// Start health checking
	go sp.healthCheckLoop()

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/produce", sp.produceHandler)
	mux.HandleFunc("/consume", sp.consumeHandler)
	mux.HandleFunc("/ack", sp.ackHandler)
	mux.HandleFunc("/topics", sp.topicsHandler)
	mux.HandleFunc("/health", sp.healthHandler)
	mux.HandleFunc("/status", sp.statusHandler)
	mux.HandleFunc("/stats", sp.statsHandler)

	// Add Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.MetricsHandler())

	log.Printf("Smart proxy starting on port %s", sp.config.Port)
	log.Printf("Routing to %d brokers with %d virtual nodes",
		len(sp.brokerEndpoints), sp.config.VirtualNodes)

	server := &http.Server{
		Addr:         ":" + sp.config.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server.ListenAndServe()
}

// discoverBrokers discovers broker endpoints from Kubernetes service
func (sp *SmartProxy) discoverBrokers() error {
	sp.brokerEndpoints = make([]string, 0, sp.config.BrokerCount)

	// Check if we're dealing with a single broker deployment (brokerCount = 1)
	if sp.config.BrokerCount == 1 {
		// For single broker, connect directly to the service
		endpoint := fmt.Sprintf("http://%s:8080", strings.Split(sp.config.BrokerService, ".")[0])
		sp.brokerEndpoints = append(sp.brokerEndpoints, endpoint)
		sp.healthyBrokers[endpoint] = true // Assume healthy initially
	} else {
		// For Kubernetes StatefulSet, brokers are named: service-0, service-1, etc.
		for i := 0; i < sp.config.BrokerCount; i++ {
			endpoint := fmt.Sprintf("http://%s-%d.%s:8080",
				strings.Split(sp.config.BrokerService, ".")[0], i, sp.config.BrokerService)
			sp.brokerEndpoints = append(sp.brokerEndpoints, endpoint)
			sp.healthyBrokers[endpoint] = true // Assume healthy initially
		}
	}

	log.Printf("Discovered %d broker endpoints: %v", len(sp.brokerEndpoints), sp.brokerEndpoints)
	return nil
}

// initConsistentHash initializes the consistent hash ring
func (sp *SmartProxy) initConsistentHash() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.consistentHash = consistenthash.NewConsistentHash(sp.brokerEndpoints, sp.config.VirtualNodes)

	// Log partition distribution
	distribution := sp.consistentHash.GetPartitionDistribution(sp.config.MaxPartitions)
	for broker, partitions := range distribution {
		log.Printf("Broker %s owns partitions: %v", broker, partitions)
	}
}

// getBrokerForTopicPartition returns the broker responsible for a topic-partition combination
func (sp *SmartProxy) getBrokerForTopicPartition(topic string, partition int) string {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	broker := sp.consistentHash.GetBrokerByTopicPartition(topic, partition)

	// If broker is unhealthy, find next healthy broker
	if !sp.healthyBrokers[broker] {
		for _, endpoint := range sp.brokerEndpoints {
			if sp.healthyBrokers[endpoint] {
				return endpoint
			}
		}
	}

	return broker
}

// assignPartition assigns a partition for a given topic/key
/*func (sp *SmartProxy) assignPartition(topic, key string) int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	if key != "" {
		return sp.consistentHash.HashPartition(key, sp.config.MaxPartitions)
	}

	// Simple round-robin for now
	hash := sp.consistentHash.HashPartition(topic, sp.config.MaxPartitions)
	return hash
}*/

// initBrokerMetrics initializes broker-specific metrics maps
func (sp *SmartProxy) initBrokerMetrics() {
	sp.stats.mu.Lock()
	defer sp.stats.mu.Unlock()

	for _, endpoint := range sp.brokerEndpoints {
		sp.stats.BrokerRequestCounts[endpoint] = 0
		sp.stats.BrokerErrors[endpoint] = 0
	}
}

// recordRequest tracks request metrics in both internal stats and Prometheus
func (sp *SmartProxy) recordRequest(requestType string, broker string, latency time.Duration, success bool) {
	// Internal counters for /stats endpoint
	atomic.AddInt64(&sp.stats.TotalRequests, 1)

	// Track by request type
	switch requestType {
	case "produce":
		atomic.AddInt64(&sp.stats.ProduceRequests, 1)
	case "consume":
		atomic.AddInt64(&sp.stats.ConsumeRequests, 1)
	case "ack":
		atomic.AddInt64(&sp.stats.AckRequests, 1)
	case "health":
		atomic.AddInt64(&sp.stats.HealthRequests, 1)
	}

	// Track success/failure
	status := "success"
	if success {
		atomic.AddInt64(&sp.stats.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&sp.stats.FailedRequests, 1)
		status = "failure"
	}

	// Track latency
	atomic.AddInt64(&sp.stats.TotalLatencyMs, latency.Milliseconds())
	atomic.AddInt64(&sp.stats.RequestCount, 1)

	// Track per-broker stats
	sp.stats.mu.Lock()
	if success {
		sp.stats.BrokerRequestCounts[broker]++
	} else {
		sp.stats.BrokerErrors[broker]++
	}
	sp.stats.mu.Unlock()

	// Prometheus metrics
	serviceName := "msg-queue-proxy"
	metrics.ProxyRequestsTotal.WithLabelValues(serviceName, requestType, status).Inc()
	metrics.ProxyRequestDuration.WithLabelValues(serviceName, requestType).Observe(latency.Seconds())
	metrics.ProxyBrokerRequests.WithLabelValues(serviceName, broker, status).Inc()
}

// produceHandler handles message production
func (sp *SmartProxy) produceHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received produce request: method=%s, url=%s", r.Method, r.URL.String())

	if r.Method != http.MethodPost {
		log.Printf("Rejecting non-POST request: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topic := r.URL.Query().Get("topic")
	partStr := r.URL.Query().Get("partition")
	//key := r.URL.Query().Get("key")

	log.Printf("Produce request params: topic=%s, partition=%s, key=%s", topic, partStr, key)

	if topic == "" || partStr == "" {
		http.Error(w, "topic and partition required", http.StatusBadRequest)
		return
	}

	var partition int
	var err error

	partition, err = strconv.Atoi(partStr)
	if err != nil {
		http.Error(w, "invalid partition", http.StatusBadRequest)
		return
	}

	// Get target broker using topic-partition combination
	targetBroker := sp.getBrokerForTopicPartition(topic, partition)
	if targetBroker == "" {
		http.Error(w, "no healthy brokers available", http.StatusServiceUnavailable)
		return
	}

	// Forward request to target broker
	targetURL := fmt.Sprintf("%s/produce?topic=%s&partition=%d", targetBroker, topic, partition)
	log.Printf("Forwarding to broker: %s", targetURL)
	sp.forwardRequest(w, r, targetURL, "produce")
}

// consumeHandler handles message consumption
func (sp *SmartProxy) consumeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topic := r.URL.Query().Get("topic")
	partStr := r.URL.Query().Get("partition")
	group := r.URL.Query().Get("group")
	//key := r.URL.Query().Get("key")

	if topic == "" || partStr == "" || group == "" {
		http.Error(w, "topic, partition and group required", http.StatusBadRequest)
		return
	}
	partition, err := strconv.Atoi(partStr)
	if err != nil {
		http.Error(w, "invalid partition", http.StatusBadRequest)
		return
	}

	// Get target broker using topic-partition combination
	targetBroker := sp.getBrokerForTopicPartition(topic, partition)
	if targetBroker == "" {
		http.Error(w, "no healthy brokers available", http.StatusServiceUnavailable)
		return
	}

	// Forward request to target broker
	targetURL := fmt.Sprintf("%s/consume?topic=%s&partition=%d&group=%s",
		targetBroker, topic, partition, group)
	sp.forwardRequest(w, r, targetURL, "consume")
}

// ackHandler handles message acknowledgment
func (sp *SmartProxy) ackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topic := r.URL.Query().Get("topic")
	partStr := r.URL.Query().Get("partition")
	group := r.URL.Query().Get("group")

	if topic == "" || partStr == "" || group == "" {
		http.Error(w, "topic, partition and group required", http.StatusBadRequest)
		return
	}

	partition, err := strconv.Atoi(partStr)
	if err != nil {
		http.Error(w, "invalid partition", http.StatusBadRequest)
		return
	}

	// Get target broker using topic-partition combination (same as the one that served the message)
	targetBroker := sp.getBrokerForTopicPartition(topic, partition)
	if targetBroker == "" {
		http.Error(w, "no healthy brokers available", http.StatusServiceUnavailable)
		return
	}

	// Forward request to target broker
	targetURL := fmt.Sprintf("%s/ack?topic=%s&partition=%d&group=%s",
		targetBroker, topic, partition, group)
	sp.forwardRequest(w, r, targetURL, "ack")
}

// topicsHandler handles topics listing
func (sp *SmartProxy) topicsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Forward to any healthy broker (they should all have the same topics)
	for endpoint := range sp.healthyBrokers {
		if sp.healthyBrokers[endpoint] {
			targetURL := fmt.Sprintf("%s/topics", endpoint)
			sp.forwardRequest(w, r, targetURL, "topics")
			return
		}
	}

	http.Error(w, "no healthy brokers available", http.StatusServiceUnavailable)
}

// healthHandler returns proxy health status
func (sp *SmartProxy) healthHandler(w http.ResponseWriter, r *http.Request) {
	sp.mu.RLock()
	healthyCount := 0
	for _, healthy := range sp.healthyBrokers {
		if healthy {
			healthyCount++
		}
	}
	sp.mu.RUnlock()

	status := map[string]interface{}{
		"status":          "healthy",
		"brokers_total":   len(sp.brokerEndpoints),
		"brokers_healthy": healthyCount,
		"timestamp":       time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// statusHandler returns detailed proxy status
func (sp *SmartProxy) statusHandler(w http.ResponseWriter, r *http.Request) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	brokerStatus := make(map[string]bool)
	for endpoint, healthy := range sp.healthyBrokers {
		brokerStatus[endpoint] = healthy
	}

	distribution := sp.consistentHash.GetPartitionDistribution(sp.config.MaxPartitions)

	status := map[string]interface{}{
		"proxy_config":           sp.config,
		"broker_status":          brokerStatus,
		"partition_distribution": distribution,
		"timestamp":              time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// statsHandler returns detailed proxy statistics
func (sp *SmartProxy) statsHandler(w http.ResponseWriter, r *http.Request) {
	// Calculate uptime
	uptime := time.Since(sp.startTime)

	// Get current stats (atomic reads)
	totalRequests := atomic.LoadInt64(&sp.stats.TotalRequests)
	successfulRequests := atomic.LoadInt64(&sp.stats.SuccessfulRequests)
	failedRequests := atomic.LoadInt64(&sp.stats.FailedRequests)
	produceRequests := atomic.LoadInt64(&sp.stats.ProduceRequests)
	consumeRequests := atomic.LoadInt64(&sp.stats.ConsumeRequests)
	ackRequests := atomic.LoadInt64(&sp.stats.AckRequests)
	healthRequests := atomic.LoadInt64(&sp.stats.HealthRequests)
	totalLatencyMs := atomic.LoadInt64(&sp.stats.TotalLatencyMs)
	requestCount := atomic.LoadInt64(&sp.stats.RequestCount)
	healthCheckCount := atomic.LoadInt64(&sp.stats.HealthCheckCount)
	brokerFailures := atomic.LoadInt64(&sp.stats.BrokerFailures)

	// Calculate averages
	var avgLatencyMs float64
	var successRate float64
	var requestsPerSecond float64

	if requestCount > 0 {
		avgLatencyMs = float64(totalLatencyMs) / float64(requestCount)
	}

	if totalRequests > 0 {
		successRate = float64(successfulRequests) / float64(totalRequests) * 100
	}

	if uptime.Seconds() > 0 {
		requestsPerSecond = float64(totalRequests) / uptime.Seconds()
	}

	// Get broker-specific stats
	sp.stats.mu.RLock()
	brokerRequestCounts := make(map[string]int64)
	brokerErrors := make(map[string]int64)
	for broker, count := range sp.stats.BrokerRequestCounts {
		brokerRequestCounts[broker] = count
	}
	for broker, errors := range sp.stats.BrokerErrors {
		brokerErrors[broker] = errors
	}
	sp.stats.mu.RUnlock()

	// Get broker health status
	sp.mu.RLock()
	healthyBrokers := 0
	totalBrokers := len(sp.brokerEndpoints)
	for _, healthy := range sp.healthyBrokers {
		if healthy {
			healthyBrokers++
		}
	}
	sp.mu.RUnlock()

	stats := map[string]interface{}{
		"uptime_seconds":       uptime.Seconds(),
		"uptime_human":         uptime.String(),
		"total_requests":       totalRequests,
		"successful_requests":  successfulRequests,
		"failed_requests":      failedRequests,
		"success_rate_percent": successRate,
		"requests_per_second":  requestsPerSecond,
		"average_latency_ms":   avgLatencyMs,

		"request_breakdown": map[string]int64{
			"produce": produceRequests,
			"consume": consumeRequests,
			"ack":     ackRequests,
			"health":  healthRequests,
		},

		"broker_distribution": map[string]interface{}{
			"request_counts": brokerRequestCounts,
			"error_counts":   brokerErrors,
			"healthy_count":  healthyBrokers,
			"total_count":    totalBrokers,
		},

		"health_monitoring": map[string]interface{}{
			"health_checks_performed":  healthCheckCount,
			"broker_failures_detected": brokerFailures,
		},

		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// forwardRequest forwards HTTP request to target broker with metrics tracking
func (sp *SmartProxy) forwardRequest(w http.ResponseWriter, r *http.Request, targetURL string, requestType string) {
	startTime := time.Now()
	log.Printf("Forwarding %s request to: %s", requestType, targetURL)

	// Create new request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		sp.recordRequest(requestType, targetURL, time.Since(startTime), false)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewBuffer(body))
	if err != nil {
		sp.recordRequest(requestType, targetURL, time.Since(startTime), false)
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Execute request
	resp, err := sp.client.Do(req)
	if err != nil {
		sp.recordRequest(requestType, targetURL, time.Since(startTime), false)
		log.Printf("Failed to forward request to %s: %v", targetURL, err)
		http.Error(w, "broker unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)

	// Record successful request
	success := resp.StatusCode >= 200 && resp.StatusCode < 400
	sp.recordRequest(requestType, targetURL, time.Since(startTime), success)

	if success {
		log.Printf("Successfully forwarded %s request to %s (status: %d)", requestType, targetURL, resp.StatusCode)
	} else {
		log.Printf("Forward request failed with status %d for %s", resp.StatusCode, targetURL)
	}
}

// healthCheckLoop periodically checks broker health
func (sp *SmartProxy) healthCheckLoop() {
	ticker := time.NewTicker(sp.config.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sp.checkBrokerHealth()
		}
	}
}

// checkBrokerHealth checks health of all brokers
func (sp *SmartProxy) checkBrokerHealth() {
	atomic.AddInt64(&sp.stats.HealthCheckCount, 1)
	metrics.ProxyHealthChecks.WithLabelValues("msg-queue-proxy").Inc()

	sp.mu.Lock()
	defer sp.mu.Unlock()

	for _, endpoint := range sp.brokerEndpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/health", nil)

		if err != nil {
			if sp.healthyBrokers[endpoint] {
				atomic.AddInt64(&sp.stats.BrokerFailures, 1)
				log.Printf("Broker %s became unhealthy: %v", endpoint, err)
			}
			sp.healthyBrokers[endpoint] = false
			metrics.ProxyBrokerHealth.WithLabelValues("msg-queue-proxy", endpoint).Set(0)
			cancel()
			continue
		}

		resp, err := sp.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if sp.healthyBrokers[endpoint] {
				atomic.AddInt64(&sp.stats.BrokerFailures, 1)
				log.Printf("Broker %s became unhealthy: status %d", endpoint, getStatusCode(resp))
			}
			sp.healthyBrokers[endpoint] = false
			metrics.ProxyBrokerHealth.WithLabelValues("msg-queue-proxy", endpoint).Set(0)
		} else {
			if !sp.healthyBrokers[endpoint] {
				log.Printf("Broker %s recovered and is now healthy", endpoint)
			}
			sp.healthyBrokers[endpoint] = true
			metrics.ProxyBrokerHealth.WithLabelValues("msg-queue-proxy", endpoint).Set(1)
		}

		if resp != nil {
			resp.Body.Close()
		}
		cancel()
	}
}

// Helper function to safely get status code
func getStatusCode(resp *http.Response) int {
	if resp != nil {
		return resp.StatusCode
	}
	return 0
}

func loadConfig() ProxyConfig {
	config := ProxyConfig{
		Port:           getEnv("PORT", "8080"),
		BrokerService:  getEnv("BROKER_SERVICE", "msg-queue"),
		BrokerCount:    getEnvInt("BROKER_COUNT", 3),
		VirtualNodes:   getEnvInt("VIRTUAL_NODES", 150),
		MaxPartitions:  getEnvInt("MAX_PARTITIONS", 12),
		HealthInterval: time.Duration(getEnvInt("HEALTH_INTERVAL_SECONDS", 30)) * time.Second,
	}

	log.Printf("Proxy configuration: %+v", config)
	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func main() {
	config := loadConfig()
	proxy := NewSmartProxy(config)

	log.Printf("Starting Smart Message Queue Proxy")
	if err := proxy.Start(); err != nil {
		log.Fatalf("Failed to start proxy: %v", err)
	}
}
