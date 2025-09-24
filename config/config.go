package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	// InfluxDB configuration
	InfluxDBURL    string
	InfluxDBToken  string
	InfluxDBOrg    string
	InfluxDBBucket string

	// Message Queue configuration
	UseHTTPQueue         bool
	MsgQueueAddr         string
	MsgQueueTopic        string
	MsgQueueGroup        string
	MsgQueueConsumerName string
	MsgQueueProducerName string
	MaxPartitions        int

	// CSV Streaming configuration
	CSVPath    string
	CSVDelayMs int

	// Server configuration
	Port string
}

// Load loads configuration from environment variables
func Load() Config {
	cfg := Config{
		// InfluxDB defaults
		InfluxDBURL:    getEnv("INFLUXDB_URL", "http://influxdb:8086"),
		InfluxDBToken:  getEnv("INFLUXDB_TOKEN", "supersecrettoken"),
		InfluxDBOrg:    getEnv("INFLUXDB_ORG", "telemetryorg"),
		InfluxDBBucket: getEnv("INFLUXDB_BUCKET", "telem_bucket"),

		// Message Queue defaults
		UseHTTPQueue:         getEnv("USE_HTTP_QUEUE", "true") == "true",
		MsgQueueAddr:         getEnv("MSG_QUEUE_ADDR", "http://msg-queue-proxy-service:8080"),
		MsgQueueTopic:        getEnv("MSG_QUEUE_TOPIC", "telemetry"),
		MsgQueueGroup:        getEnv("MSG_QUEUE_GROUP", "telemetry_group"),
		MsgQueueConsumerName: getEnv("MSG_QUEUE_CONSUMER_NAME", "collector"),
		MsgQueueProducerName: getEnv("MSG_QUEUE_PRODUCER_NAME", "streamer"),
		MaxPartitions:        getEnvInt("MAX_PARTITIONS", 2),

		// CSV Streaming defaults
		CSVPath:    getEnv("CSV_PATH", "/data/dcgm_metrics_20250718_134233.csv"),
		CSVDelayMs: getEnvInt("CSV_DELAY_MS", 1000),

		// Server defaults
		Port: getEnv("PORT", "8080"),
	}

	return cfg
}

// getEnv gets an environment variable with a fallback default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as integer with a fallback default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
