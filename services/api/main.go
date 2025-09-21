package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/example/telemetry/internal/influx"
	"github.com/example/telemetry/internal/telemetry"
	_ "github.com/example/telemetry/services/api/docs"
)

// @title Telemetry API
// @version 1.0
// @description This is a telemetry data API server for GPU monitoring.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
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

	// Swagger endpoint
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	// @Summary Get recent telemetry data (legacy)
	// @Description Get the 10 most recent telemetry records
	// @Tags legacy
	// @Produce json
	// @Success 200 {array} TelemetryDataResponse
	// @Failure 500 {object} ErrorResponse
	// @Router /gpus [get]
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

	// @Summary Get GPU telemetry data
	// @Description Get telemetry data for a specific GPU with optional time range filtering
	// @Tags telemetry
	// @Param id path string true "GPU ID (UUID)"
	// @Param start_time query string false "Start time in RFC3339 format (e.g., 2023-01-01T00:00:00Z)"
	// @Param end_time query string false "End time in RFC3339 format (e.g., 2023-01-01T23:59:59Z)"
	// @Param limit query int false "Maximum number of records to return (default: 100)"
	// @Produce json
	// @Success 200 {object} TelemetryResponse
	// @Failure 400 {object} ErrorResponse
	// @Failure 404 {object} ErrorResponse
	// @Failure 500 {object} ErrorResponse
	// @Router /api/v1/gpus/{id}/telemetry [get]
	// New endpoint: GET /api/v1/gpus/{id}/telemetry
	http.HandleFunc("/api/v1/gpus/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Parse the URL path to extract GPU ID
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/gpus/")
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("GPU ID is required"))
			return
		}

		// Split path to get ID and check for /telemetry suffix
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "telemetry" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Endpoint not found"))
			return
		}

		gpuID := parts[0]
		logger.Printf("Querying telemetry for GPU ID: %s", gpuID)

		// Check for time range query parameters
		startTimeStr := r.URL.Query().Get("start_time")
		endTimeStr := r.URL.Query().Get("end_time")

		var records []telemetry.TelemetryRecord
		var err error

		if startTimeStr != "" && endTimeStr != "" {
			// Parse time parameters
			_, err1 := time.Parse(time.RFC3339, startTimeStr)
			_, err2 := time.Parse(time.RFC3339, endTimeStr)

			if err1 != nil || err2 != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid time format. Use RFC3339 format (e.g., 2023-01-01T00:00:00Z)"))
				return
			}

			// Query with time range
			records, err = influxClient.QueryTelemetryByDeviceTimeRange(gpuID, startTimeStr, endTimeStr)
		} else {
			records, err = influxClient.QueryTelemetryByDevice(gpuID)
		}

		if err != nil {
			logger.Printf("Failed to query InfluxDB for GPU %s: %v", gpuID, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to query telemetry data"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"gpu_id": gpuID,
			"count":  len(records),
			"data":   records,
		}
		json.NewEncoder(w).Encode(response)
	})

	// @Summary List available GPUs
	// @Description Get a list of all available GPUs with their metadata
	// @Tags gpus
	// @Produce json
	// @Success 200 {object} GPUListResponse
	// @Failure 500 {object} ErrorResponse
	// @Router /api/v1/gpus [get]
	// Helper endpoint: GET /api/v1/gpus - List available GPU IDs
	http.HandleFunc("/api/v1/gpus", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Query recent telemetry to get available GPU IDs
		logger.Printf("Querying recent telemetry for GPU list...")
		records, err := influxClient.QueryUniqueUUIDs()
		if err != nil {
			logger.Printf("Failed to query InfluxDB for GPU list: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to query GPU list"))
			return
		}

		logger.Printf("Retrieved %d records from InfluxDB", len(records))
		if len(records) > 0 {
			logger.Printf("Sample record: %+v", records[0])
		}

		logger.Printf("Found %d unique GPUs", len(records))
		
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"count": len(records),
			"gpus":  records,
		}
		json.NewEncoder(w).Encode(response)
	})

	// @Summary List available hosts
	// @Description Get a list of all hosts with GPU count
	// @Tags infrastructure
	// @Produce json
	// @Success 200 {object} HostListResponse
	// @Failure 500 {object} ErrorResponse
	// @Router /api/v1/hosts [get]
	// New endpoint: GET /api/v1/hosts - List available hosts
	http.HandleFunc("/api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		records, err := influxClient.QueryRecentTelemetry(1000)
		if err != nil {
			logger.Printf("Failed to query InfluxDB for host list: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to query host list"))
			return
		}

		// Extract unique hostnames with GPU count
		hostMap := make(map[string]map[string]bool)
		for _, record := range records {
			if record.Hostname != "" {
				if hostMap[record.Hostname] == nil {
					hostMap[record.Hostname] = make(map[string]bool)
				}
				if record.DeviceID != "" {
					hostMap[record.Hostname][record.DeviceID] = true
				}
			}
		}

		var hostInfo []map[string]interface{}
		for hostname, gpus := range hostMap {
			info := map[string]interface{}{
				"hostname":  hostname,
				"gpu_count": len(gpus),
			}
			hostInfo = append(hostInfo, info)
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"count": len(hostInfo),
			"hosts": hostInfo,
		}
		json.NewEncoder(w).Encode(response)
	})

	// @Summary List available namespaces
	// @Description Get a list of all Kubernetes namespaces with GPU count
	// @Tags infrastructure
	// @Produce json
	// @Success 200 {object} NamespaceListResponse
	// @Failure 500 {object} ErrorResponse
	// @Router /api/v1/namespaces [get]
	// New endpoint: GET /api/v1/namespaces - List available namespaces
	http.HandleFunc("/api/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		records, err := influxClient.QueryRecentTelemetry(1000)
		if err != nil {
			logger.Printf("Failed to query InfluxDB for namespace list: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to query namespace list"))
			return
		}

		// Extract unique namespaces with GPU count
		namespaceMap := make(map[string]map[string]bool)
		for _, record := range records {
			if record.Namespace != "" {
				if namespaceMap[record.Namespace] == nil {
					namespaceMap[record.Namespace] = make(map[string]bool)
				}
				if record.DeviceID != "" {
					namespaceMap[record.Namespace][record.DeviceID] = true
				}
			}
		}

		var namespaceInfo []map[string]interface{}
		for namespace, gpus := range namespaceMap {
			info := map[string]interface{}{
				"namespace": namespace,
				"gpu_count": len(gpus),
			}
			namespaceInfo = append(namespaceInfo, info)
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"count": len(namespaceInfo),
			"namespaces": namespaceInfo,
		}
		json.NewEncoder(w).Encode(response)
	})

	logger.Println("API service started on :8080")
	logger.Println("Available endpoints:")
	logger.Println("  GET /swagger/                          - Swagger UI documentation")
	logger.Println("  GET /gpus                              - Recent telemetry (legacy)")
	logger.Println("  GET /api/v1/gpus                       - List available GPUs with metadata")
	logger.Println("  GET /api/v1/gpus/{id}/telemetry        - GPU telemetry (recent)")
	logger.Println("  GET /api/v1/gpus/{id}/telemetry?start_time=...&end_time=... - GPU telemetry (time range)")
	logger.Println("  GET /api/v1/hosts                      - List available hosts")
	logger.Println("  GET /api/v1/namespaces                 - List available namespaces")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
