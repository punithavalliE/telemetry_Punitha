package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"encoding/json"

	"github.com/example/telemetry/internal/shared"
	"github.com/example/telemetry/config"
	"github.com/example/telemetry/internal/influx"
)



	type CollectorService struct {
		queue  *shared.RedisStreamQueue
		logger *log.Logger
		config config.Config
		influx *influx.InfluxWriter
	}

func NewCollectorService() *CollectorService {
	logger := log.New(os.Stdout, "[collector-service] ", log.LstdFlags)
    cfg := config.Load()
       
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

       queue, err := shared.NewRedisStreamQueue(redisAddr, stream, group, name)
       if err != nil {
	       logger.Fatalf("Failed to create Redis stream queue: %v", err)
       }

       logger.Printf("Using Redis stream queue at %s, stream=%s, group=%s, name=%s", redisAddr, stream, group, name)

	influxURL := os.Getenv("INFLUXDB_URL")
	if influxURL == "" {
		influxURL = "http://influxdb:8086"
	}
	influxToken := os.Getenv("INFLUXDB_TOKEN")
	if influxToken == "" {
		influxToken = "supersecrettoken"
	}
	influxOrg := os.Getenv("INFLUXDB_ORG")
	if influxOrg == "" {
		influxOrg = "telemetryorg"
	}
	influxBucket := os.Getenv("INFLUXDB_BUCKET")
	if influxBucket == "" {
		influxBucket = "telem_bucket"
	}

	influxWriter := influx.NewInfluxWriter(influxURL, influxToken, influxOrg, influxBucket)

       return &CollectorService{
	      queue:  queue,
	      logger: logger,
	      config: cfg,
	      influx: influxWriter,
       }
	}

	func (cs *CollectorService) Start() {
	   cs.logger.Println("Starting collector service...")
       // Start consuming telemetry messages from Redis stream
	   go func() {
	   _ = cs.queue.Subscribe(func(topic string, body []byte, id string) error {
		   if len(body) == 0 {
			   cs.logger.Printf("Skipped empty message body for id %s", id)
			   return nil
		   }
		   var data struct {
			   DeviceID string  `json:"device_id"`
			   Metric   string  `json:"metric"`
			   Value    int64   `json:"value"`
			   Time     time.Time `json:"time"`
		   }
		   if err := json.Unmarshal(body, &data); err != nil {
			   cs.logger.Printf("Invalid message for id %s: %v. Raw body: %s", id, err, string(body))
			   return err
		   }
		   cs.logger.Printf("Received telemetry [%s]: %+v", id, data)
		   // Write to InfluxDB
		   err := cs.influx.WriteTelemetry(data.DeviceID, data.Metric, data.Value, data.Time)//time.Now())
		   if err != nil {
			   cs.logger.Printf("Failed to write to InfluxDB: %v", err)
		   }
		   return err
	   })
	   }()

       // For demonstration, let's also add a periodic stats reporter
       go cs.reportStats()

       // Wait for interrupt signal
       sigChan := make(chan os.Signal, 1)
       signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
       <-sigChan

       cs.logger.Println("Shutting down collector service...")
}

func (cs *CollectorService) reportStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cs.logger.Printf("Stats reporting not implemented for RedisStreamQueue.")
	}
}

func (cs *CollectorService) Close() {
	cs.queue.Close()
}

func main() {
	service := NewCollectorService()
	defer service.Close()
	service.Start()
}