package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/telemetry/internal/influx"
)

type TelemetryRecord struct {
	DeviceID string    `json:"device_id"`
	Metric   string    `json:"metric"`
	Value    int64   `json:"value"`
	Time     time.Time `json:"time"`
}

func main() {
	logger := log.New(os.Stdout, "[api-service] ", log.LstdFlags)

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

	influxClient := influx.NewInfluxWriter(influxURL, influxToken, influxOrg, influxBucket)
	defer influxClient.Close()

	http.HandleFunc("/gpus", func(w http.ResponseWriter, r *http.Request) {
		records, err := influxClient.QueryRecentTelemetry(10)
		if err != nil {
			logger.Printf("Failed to query InfluxDB: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to query data"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	})

	logger.Println("API service started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
