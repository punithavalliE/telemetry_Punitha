package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/example/telemetry/internal/influx"
	"github.com/example/telemetry/internal/metrics"
	"github.com/example/telemetry/internal/security"
	"github.com/example/telemetry/internal/telemetry"
	_ "github.com/example/telemetry/services/api/docs"
	httpSwagger "github.com/swaggo/http-swagger"
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

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description API Key authentication using X-API-Key header

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token authentication using Authorization header (format: Bearer <token>)

// @host localhost:30081
// @BasePath /
func main() {
	logger := log.New(os.Stdout, "[api-service] ", log.LstdFlags)

	// Initialize Prometheus metrics
	metrics.InitMetrics("api-service")
	logger.Println("Prometheus metrics initialized")

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

	// Create HTTP router with API key authentication
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("/health", metrics.HTTPMiddleware("api-service", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("API service healthy"))
	}))

	// Prometheus metrics endpoint
	mux.Handle("/metrics", metrics.MetricsHandler())

	// Swagger endpoint (public for documentation)
	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

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
	mux.HandleFunc("/api/v1/gpus/", func(w http.ResponseWriter, r *http.Request) {
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
	// @Security ApiKeyAuth
	// @Success 200 {object} GPUListResponse
	// @Failure 500 {object} ErrorResponse
	// @Router /api/v1/gpus [get]
	// Helper endpoint: GET /api/v1/gpus - List available GPU IDs
	mux.HandleFunc("/api/v1/gpus", func(w http.ResponseWriter, r *http.Request) {
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

	logger.Println("API service started on :8080")
	logger.Println("Available endpoints:")
	logger.Println("  GET /health                            - Health check (no auth)")
	logger.Println("  GET /swagger/                          - Swagger UI documentation (no auth)")
	logger.Println("  GET /api/v1/gpus                       - List available GPUs [API KEY REQUIRED]")
	logger.Println("  GET /api/v1/gpus/{id}/telemetry        - GPU telemetry [API KEY REQUIRED]")
	logger.Println("")
	logger.Println("Authentication: Include 'X-API-Key: <your-secret>' header or 'Authorization: Bearer <your-secret>'")

	// Apply API key authentication middleware to all routes
	securedHandler := security.APIKeyMiddleware(mux)
	log.Fatal(http.ListenAndServe(":8080", securedHandler))
}
