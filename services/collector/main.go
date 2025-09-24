package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/example/telemetry/config"
	"github.com/example/telemetry/internal/influx"
	"github.com/example/telemetry/internal/metrics"
	"github.com/example/telemetry/internal/shared"
	"github.com/example/telemetry/internal/telemetry"
)

type CollectorService struct {
	queue  shared.MessageQueue
	logger *log.Logger
	config config.Config
	influx *influx.InfluxWriter
}

func NewCollectorService() *CollectorService {
	logger := log.New(os.Stdout, "[collector-service] ", log.LstdFlags)

	// Initialize Prometheus metrics
	metrics.InitMetrics("collector-service")
	logger.Println("Prometheus metrics initialized")

	cfg := config.Load()

	// Check if we should use HTTP message queue or Redis
	var queue shared.MessageQueue
	var err error

	if cfg.UseHTTPQueue {
		// Use HTTP message queue
		queue, err = shared.NewHTTPMessageQueue(cfg.MsgQueueAddr, cfg.MsgQueueTopic, cfg.MsgQueueGroup, cfg.MsgQueueConsumerName)
		if err != nil {
			logger.Fatalf("Failed to create HTTP message queue: %v", err)
		}
		logger.Printf("Using HTTP message queue at %s, topic=%s, group=%s, name=%s", cfg.MsgQueueAddr, cfg.MsgQueueTopic, cfg.MsgQueueGroup, cfg.MsgQueueConsumerName)
	} else {
		// Use Redis (initial trial version)
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
		name := os.Getenv("REDIS_CONSUMER_NAME")
		if name == "" {
			name = "Collector"
		}

		queue, err = shared.NewRedisStreamQueue(redisAddr, stream, group, name)
		if err != nil {
			logger.Fatalf("Failed to create Redis stream queue: %v", err)
		}
		logger.Printf("Using Redis stream queue at %s, stream=%s, group=%s, name=%s", redisAddr, stream, group, name)
	}

	influxWriter := influx.NewInfluxWriter(cfg.InfluxDBURL, cfg.InfluxDBToken, cfg.InfluxDBOrg, cfg.InfluxDBBucket)

	return &CollectorService{
		queue:  queue,
		logger: logger,
		config: cfg,
		influx: influxWriter,
	}
}

func (cs *CollectorService) Start() {
	cs.logger.Println("Starting collector service...")

	// Start HTTP server for health checks
	port := cs.config.Port

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Add Prometheus metrics endpoint
	http.Handle("/metrics", metrics.MetricsHandler())

	go func() {
		cs.logger.Printf("Starting HTTP server on port %s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			cs.logger.Printf("HTTP server error: %v", err)
		}
	}()

	// Start consuming telemetry messages from message queue
	go func() {
		cs.logger.Printf("Starting message consumption...")
		if err := cs.queue.Subscribe(func(topic string, body []byte, id string) error {
			start := time.Now()

			// Record message consumption
			metrics.RecordMessageConsumed("collector-service", topic)

			if len(body) == 0 {
				cs.logger.Printf("Skipped empty message body for id %s", id)
				metrics.RecordMessageProcessing("collector-service", topic, time.Since(start))
				return nil
			}

			// Parse the CSV record array
			var csvRecord []string
			if err := json.Unmarshal(body, &csvRecord); err != nil {
				cs.logger.Printf("Invalid CSV record for id %s: %v. Raw body: %s", id, err, string(body))
				metrics.RecordMessageProcessing("collector-service", topic, time.Since(start))
				return err
			}

			// Validate CSV record has enough fields
			if len(csvRecord) < 12 {
				cs.logger.Printf("Invalid CSV record length for id %s: expected 12 fields, got %d", id, len(csvRecord))
				metrics.RecordMessageProcessing("collector-service", topic, time.Since(start))
				return nil
			}

			// Parse value field
			value, err := strconv.ParseFloat(csvRecord[10], 64)
			if err != nil {
				cs.logger.Printf("Failed to parse value field '%s' for id %s: %v", csvRecord[10], id, err)
				metrics.RecordMessageProcessing("collector-service", topic, time.Since(start))
				return nil
			}

			// Parse timestamp
			timestamp, err := time.Parse(time.RFC3339, csvRecord[0])
			if err != nil {
				cs.logger.Printf("Failed to parse timestamp '%s' for id %s: %v", csvRecord[0], id, err)
				metrics.RecordMessageProcessing("collector-service", topic, time.Since(start))
				return nil
			}

			// Convert CSV record to TelemetryRecord
			data := telemetry.TelemetryRecord{
				DeviceID:  csvRecord[3],  // device
				Metric:    csvRecord[1],  // metric_name
				Value:     value,         // value (parsed)
				Time:      timestamp,     // timestamp (parsed)
				GPUID:     csvRecord[2],  // gpu_id
				UUID:      csvRecord[4],  // uuid
				ModelName: csvRecord[5],  // modelName
				Hostname:  csvRecord[6],  // Hostname
				Container: csvRecord[7],  // container
				Pod:       csvRecord[8],  // pod
				Namespace: csvRecord[9],  // namespace
				LabelsRaw: csvRecord[11], // labels_raw
			}

			cs.logger.Printf("Received telemetry [%s]: device=%s, metric=%s, value=%f", id, data.DeviceID, data.Metric, data.Value)

			// Write to InfluxDB
			dbStart := time.Now()
			err = cs.influx.WriteTelemetry(data)
			if err != nil {
				cs.logger.Printf("Failed to write to InfluxDB: %v", err)
				metrics.RecordDatabaseOperation("collector-service", "write", "error", time.Since(dbStart))
			} else {
				metrics.RecordDatabaseOperation("collector-service", "write", "success", time.Since(dbStart))
				metrics.RecordTelemetryDataPoint("collector-service", "gpu_metric")
			}

			// Record overall message processing time
			metrics.RecordMessageProcessing("collector-service", topic, time.Since(start))
			return err
		}); err != nil {
			cs.logger.Printf("Failed to subscribe to message queue: %v", err)
		}
	}()

	// For demonstration, let's also add a periodic stats reporter
	//go cs.reportStats()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	cs.logger.Println("Shutting down collector service...")
}

/*func (cs *CollectorService) reportStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cs.logger.Printf("Stats reporting not implemented for RedisStreamQueue.")
	}
}*/

func (cs *CollectorService) Close() {
	cs.queue.Close()
}

func main() {
	service := NewCollectorService()
	defer service.Close()
	service.Start()
}
