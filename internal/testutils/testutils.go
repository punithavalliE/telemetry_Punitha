package testutils

import (
	"log"
	"os"
	"time"
)

// TestLogger creates a logger for testing that can be silenced
func TestLogger(prefix string, silent bool) *log.Logger {
	if silent {
		return log.New(os.NewFile(0, os.DevNull), prefix, log.LstdFlags)
	}
	return log.New(os.Stdout, prefix, log.LstdFlags)
}

// TestTelemetryData provides sample telemetry data for testing
type TestTelemetryData struct {
	DeviceID  string
	Metric    string
	Value     float64
	Time      time.Time
	GPUID     string
	UUID      string
	ModelName string
	Hostname  string
	Container string
	Pod       string
	Namespace string
	Labels    map[string]string
}

// SampleTelemetryData returns a slice of sample telemetry data for testing
func SampleTelemetryData() []TestTelemetryData {
	now := time.Now()
	return []TestTelemetryData{
		{
			DeviceID:  "nvidia0",
			Metric:    "DCGM_FI_DEV_GPU_UTIL",
			Value:     85.5,
			Time:      now,
			GPUID:     "0",
			UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-031",
			Container: "",
			Pod:       "test-pod-1",
			Namespace: "default",
			Labels:    map[string]string{"version": "535.129.03"},
		},
		{
			DeviceID:  "nvidia1",
			Metric:    "DCGM_FI_DEV_MEM_COPY_UTIL",
			Value:     72.3,
			Time:      now.Add(-1 * time.Minute),
			GPUID:     "1",
			UUID:      "GPU-6fd4f087-86f3-7a43-b711-4771313afc51",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-031",
			Container: "",
			Pod:       "test-pod-2",
			Namespace: "production",
			Labels:    map[string]string{"version": "535.129.03"},
		},
		{
			DeviceID:  "nvidia0",
			Metric:    "DCGM_FI_DEV_GPU_TEMP",
			Value:     65.0,
			Time:      now.Add(-2 * time.Minute),
			GPUID:     "0",
			UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-032",
			Container: "container-1",
			Pod:       "test-pod-3",
			Namespace: "development",
			Labels:    map[string]string{"version": "535.129.03"},
		},
	}
}

// SampleCSVData returns sample CSV data for testing streamer service
func SampleCSVData() [][]string {
	return [][]string{
		{
			"timestamp", "metric_name", "gpu_id", "device", "uuid", "modelName", 
			"Hostname", "container", "pod", "namespace", "value", "labels_raw",
		},
		{
			"2023-07-18T20:42:34Z", "DCGM_FI_DEV_GPU_UTIL", "0", "nvidia0",
			"GPU-5fd4f087-86f3-7a43-b711-4771313afc50", "NVIDIA H100 80GB HBM3",
			"mtv5-dgx1-hgpu-031", "", "test-pod", "default", "85.5",
			"DCGM_FI_DRIVER_VERSION=\"535.129.03\"",
		},
		{
			"2023-07-18T20:42:35Z", "DCGM_FI_DEV_MEM_COPY_UTIL", "1", "nvidia1",
			"GPU-6fd4f087-86f3-7a43-b711-4771313afc51", "NVIDIA H100 80GB HBM3",
			"mtv5-dgx1-hgpu-031", "", "test-pod-2", "production", "72.3",
			"DCGM_FI_DRIVER_VERSION=\"535.129.03\"",
		},
	}
}

// SetupTestEnvironment sets up common environment variables for testing
func SetupTestEnvironment() {
	os.Setenv("INFLUXDB_URL", "http://test-influxdb:8086")
	os.Setenv("INFLUXDB_TOKEN", "test-token")
	os.Setenv("INFLUXDB_ORG", "test-org")
	os.Setenv("INFLUXDB_BUCKET", "test-bucket")
	os.Setenv("USE_HTTP_QUEUE", "true")
	os.Setenv("MSG_QUEUE_ADDR", "http://test-queue:8080")
	os.Setenv("MSG_QUEUE_TOPIC", "test-topic")
	os.Setenv("MSG_QUEUE_GROUP", "test-group")
}

// CleanupTestEnvironment cleans up test environment variables
func CleanupTestEnvironment() {
	os.Unsetenv("INFLUXDB_URL")
	os.Unsetenv("INFLUXDB_TOKEN")
	os.Unsetenv("INFLUXDB_ORG")
	os.Unsetenv("INFLUXDB_BUCKET")
	os.Unsetenv("USE_HTTP_QUEUE")
	os.Unsetenv("MSG_QUEUE_ADDR")
	os.Unsetenv("MSG_QUEUE_TOPIC")
	os.Unsetenv("MSG_QUEUE_GROUP")
}

// AssertNoError is a helper function to check for errors in tests
func AssertNoError(t interface{ Errorf(string, ...interface{}) }, err error, msg string) {
	if err != nil {
		t.Errorf("%s: %v", msg, err)
	}
}

// AssertError is a helper function to check that an error occurred
func AssertError(t interface{ Errorf(string, ...interface{}) }, err error, msg string) {
	if err == nil {
		t.Errorf("%s: expected error but got nil", msg)
	}
}

// AssertEqual is a helper function to check equality
func AssertEqual(t interface{ Errorf(string, ...interface{}) }, expected, actual interface{}, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertNotEqual is a helper function to check inequality
func AssertNotEqual(t interface{ Errorf(string, ...interface{}) }, expected, actual interface{}, msg string) {
	if expected == actual {
		t.Errorf("%s: expected %v to not equal %v", msg, expected, actual)
	}
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(condition func() bool, timeout time.Duration, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}
