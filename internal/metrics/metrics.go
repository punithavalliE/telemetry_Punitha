package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "endpoint", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "endpoint"},
	)

	// Message queue metrics
	MessagesProduced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_produced_total",
			Help: "Total number of messages produced to the queue",
		},
		[]string{"service", "topic"},
	)

	MessagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_consumed_total",
			Help: "Total number of messages consumed from the queue",
		},
		[]string{"service", "topic"},
	)

	MessageProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "message_processing_duration_seconds",
			Help:    "Duration of message processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "topic"},
	)

	// Database metrics
	DatabaseOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"service", "operation", "status"},
	)

	DatabaseOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "operation"},
	)

	// Service health metrics
	ServiceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_health",
			Help: "Health status of the service (1 = healthy, 0 = unhealthy)",
		},
		[]string{"service"},
	)

	// Telemetry specific metrics
	TelemetryDataPoints = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "telemetry_data_points_total",
			Help: "Total number of telemetry data points processed",
		},
		[]string{"service", "type"},
	)

	ActiveConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
		[]string{"service"},
	)
)

// InitMetrics registers all metrics with Prometheus
func InitMetrics(serviceName string) {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		MessagesProduced,
		MessagesConsumed,
		MessageProcessingDuration,
		DatabaseOperations,
		DatabaseOperationDuration,
		ServiceHealth,
		TelemetryDataPoints,
		ActiveConnections,
	)

	// Set initial health status
	ServiceHealth.WithLabelValues(serviceName).Set(1)
}

// HTTPMiddleware creates a middleware for HTTP metrics collection
func HTTPMiddleware(serviceName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a wrapper to capture status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start).Seconds()
		statusCode := wrapper.statusCode

		HTTPRequestsTotal.WithLabelValues(
			serviceName,
			r.Method,
			r.URL.Path,
			http.StatusText(statusCode),
		).Inc()

		HTTPRequestDuration.WithLabelValues(
			serviceName,
			r.Method,
			r.URL.Path,
		).Observe(duration)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordMessageProduced records a message production event
func RecordMessageProduced(serviceName, topic string) {
	MessagesProduced.WithLabelValues(serviceName, topic).Inc()
}

// RecordMessageConsumed records a message consumption event
func RecordMessageConsumed(serviceName, topic string) {
	MessagesConsumed.WithLabelValues(serviceName, topic).Inc()
}

// RecordMessageProcessing records message processing duration
func RecordMessageProcessing(serviceName, topic string, duration time.Duration) {
	MessageProcessingDuration.WithLabelValues(serviceName, topic).Observe(duration.Seconds())
}

// RecordDatabaseOperation records a database operation
func RecordDatabaseOperation(serviceName, operation, status string, duration time.Duration) {
	DatabaseOperations.WithLabelValues(serviceName, operation, status).Inc()
	DatabaseOperationDuration.WithLabelValues(serviceName, operation).Observe(duration.Seconds())
}

// RecordTelemetryDataPoint records a telemetry data point
func RecordTelemetryDataPoint(serviceName, dataType string) {
	TelemetryDataPoints.WithLabelValues(serviceName, dataType).Inc()
}

// SetActiveConnections sets the number of active connections
func SetActiveConnections(serviceName string, count float64) {
	ActiveConnections.WithLabelValues(serviceName).Set(count)
}

// SetServiceHealth sets the service health status
func SetServiceHealth(serviceName string, healthy bool) {
	if healthy {
		ServiceHealth.WithLabelValues(serviceName).Set(1)
	} else {
		ServiceHealth.WithLabelValues(serviceName).Set(0)
	}
}
