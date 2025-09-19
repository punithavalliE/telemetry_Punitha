package telemetry

import "encoding/json"

// TelemetryData represents a single telemetry record.
type TelemetryData struct {
	DeviceID string  `json:"device_id"`
	Metric   string  `json:"metric"`
	Value    int64   `json:"value"`
	Time     string `json:"time"`
}

// Marshal marshals TelemetryData to JSON.
func Marshal(data TelemetryData) ([]byte, error) {
	return json.Marshal(data)
}
