package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/example/telemetry/internal/shared"
	"github.com/example/telemetry/internal/telemetry"
	"github.com/example/telemetry/config"
)


type StreamerService struct {
	queue  shared.MessageQueue
	logger *log.Logger
	config config.Config
}

func NewStreamerService() *StreamerService {
       logger := log.New(os.Stdout, "[streamer-service] ", log.LstdFlags)
       cfg := config.Load()

       // Check if we should use HTTP message queue or Redis
       useHTTPQueue := os.Getenv("USE_HTTP_QUEUE")
       var queue shared.MessageQueue
       var err error
       
       if useHTTPQueue == "true" {
	       // Use HTTP message queue
	       queueAddr := os.Getenv("MSG_QUEUE_ADDR")
	       if queueAddr == "" {
		       queueAddr = "http://msg_queue:8080"
	       }
	       topic := os.Getenv("MSG_QUEUE_TOPIC")
	       if topic == "" {
		       topic = "telemetry"
	       }
	       group := os.Getenv("MSG_QUEUE_GROUP")
	       if group == "" {
		       group = "telemetry_group"
	       }
	       name := os.Getenv("MSG_QUEUE_PRODUCER_NAME")
	       if name == "" {
		       name = "streamer"
	       }

	       queue, err = shared.NewHTTPMessageQueue(queueAddr, topic, group, name)
	       if err != nil {
		       logger.Fatalf("Failed to create HTTP message queue: %v", err)
	       }

	       logger.Printf("Using HTTP message queue at %s, topic=%s, group=%s, name=%s", queueAddr, topic, group, name)
       } else {
	       // Use Redis (existing behavior)
	       redisAddr := os.Getenv("REDIS_ADDR")
	       if redisAddr == "" {
		       redisAddr = "redis:6379"
	       }
	       stream := os.Getenv("REDIS_STREAM")
	       if stream == "" {
		       stream = "telemetry"
	       }
	       group := os.Getenv("REDIS_GROUP")
	       if group == "" {
		       group = "telemetry_group"
	       }
	       name := os.Getenv("REDIS_PRODUCER_NAME")
	       if name == "" {
		       name = "streamer"
	       }

	       queue, err = shared.NewRedisStreamQueue(redisAddr, stream, group, name)
	       if err != nil {
		       logger.Fatalf("Failed to create Redis stream queue: %v", err)
	       }

	       logger.Printf("Using Redis stream queue at %s, stream=%s, group=%s, name=%s", redisAddr, stream, group, name)
       }

       return &StreamerService{
	       queue:  queue,
	       logger: logger,
	       config: cfg,
       }
}

func (ss *StreamerService) publishTelemetryHandler(w http.ResponseWriter, r *http.Request) {
       if r.Method != http.MethodPost {
	       http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	       return
       }

       var data telemetry.TelemetryRecord
       if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
	       ss.logger.Printf("Invalid JSON: %v", err)
	       http.Error(w, "Invalid JSON", http.StatusBadRequest)
	       return
       }

       msgBody, err := json.Marshal(data)
       if err != nil {
	       ss.logger.Printf("Failed to marshal telemetry: %v", err)
	       http.Error(w, "Failed to process telemetry", http.StatusInternalServerError)
	       return
       }

       if err := ss.queue.Publish("telemetry", msgBody); err != nil {
	       ss.logger.Printf("Failed to publish telemetry: %v", err)
	       http.Error(w, "Failed to publish telemetry", http.StatusInternalServerError)
	       return
       }

       ss.logger.Printf("Published telemetry: %+v", data)
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(map[string]string{"status": "published"})
}

func (ps *StreamerService) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}



func (ps *StreamerService) Start() {
	http.HandleFunc("/telemetry", ps.publishTelemetryHandler)
	http.HandleFunc("/health", ps.healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ps.logger.Printf("Streamer service starting on port %s", port)
	ps.logger.Printf("Endpoints:")
	ps.logger.Printf("  POST /telemetry - Publish telemetry data")
	ps.logger.Printf("  GET  /health    - Health check")
	ps.logger.Printf("  GET  /stats     - Queue statistics")

	// Start HTTP server in a goroutine so health checks work
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			ps.logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(1 * time.Second)
	
       // If CSV_PATH env var is set, stream from CSV but keep server running
       csvPath := os.Getenv("CSV_PATH")
       if csvPath != "" {
	       delay := 1 * time.Second
	       if d := os.Getenv("CSV_DELAY_MS"); d != "" {
		       if ms, err := strconv.Atoi(d); err == nil {
			       delay = time.Duration(ms) * time.Millisecond
		       }
	       }
	       ps.logger.Printf("Streaming telemetry from CSV: %s", csvPath)
	       if err := ps.StreamCSV(csvPath, delay); err != nil {
		       ps.logger.Fatalf("CSV streaming failed: %v", err)
	       }
	       ps.logger.Println("CSV streaming complete. HTTP server continues running...")
       }

	// Keep the main goroutine alive for HTTP server
	select {}
}

func (ss *StreamerService) Close() {
	ss.queue.Close()
}

func main() {
       service := NewStreamerService()
       defer service.Close()
       service.Start()
}