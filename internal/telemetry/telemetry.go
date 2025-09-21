package telemetry

import (
	"encoding/json"
	"time"
)

// TelemetryRecord represents a telemetry record with parsed time
type TelemetryRecord struct {
	DeviceID string    `json:"device_id"`
	Metric   string    `json:"metric"`
	Value    float64   `json:"value"`
	Time     time.Time `json:"time"`
	GPUID    string `json:"gpu_id"`
	UUID     string `json:"uuid"`
	ModelName string `json:"modelName"`
	Hostname string `json:"Hostname"`
	Container string `json:"container"`
	Pod      string `json:"pod"`
	Namespace string `json:"namespace"`
	LabelsRaw string `json:"labels_raw"`
}

// Marshal marshals TelemetryRecord to JSON.
func Marshal(record TelemetryRecord) ([]byte, error) {
	return json.Marshal(record)
}
