package main

import (
	"testing"
	"time"
)

func TestTelemetryDataResponse(t *testing.T) {
	now := time.Now()
	
	telemetry := TelemetryDataResponse{
		DeviceID:  "nvidia0",
		Metric:    "DCGM_FI_DEV_GPU_UTIL",
		Value:     85.5,
		Time:      now,
		GPUID:     "0",
		UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
		ModelName: "NVIDIA H100 80GB HBM3",
		Hostname:  "mtv5-dgx1-hgpu-031",
		Container: "",
		Pod:       "test-pod",
		Namespace: "default",
		LabelsRaw: "DCGM_FI_DRIVER_VERSION=\"535.129.03\"",
	}

	if telemetry.DeviceID != "nvidia0" {
		t.Errorf("Expected DeviceID 'nvidia0', got '%s'", telemetry.DeviceID)
	}
	
	if telemetry.Value != 85.5 {
		t.Errorf("Expected Value 85.5, got %f", telemetry.Value)
	}
	
	if telemetry.GPUID != "0" {
		t.Errorf("Expected GPUID '0', got '%s'", telemetry.GPUID)
	}
}

func TestGPUInfo(t *testing.T) {
	now := time.Now()
	
	gpu := GPUInfo{
		DeviceID:  "nvidia0",
		GPUID:     "0",
		UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
		ModelName: "NVIDIA H100 80GB HBM3",
		Hostname:  "mtv5-dgx1-hgpu-031",
		Container: "",
		Pod:       "test-pod",
		Namespace: "default",
		LastSeen:  now,
	}

	if gpu.DeviceID != "nvidia0" {
		t.Errorf("Expected DeviceID 'nvidia0', got '%s'", gpu.DeviceID)
	}
	
	if gpu.GPUID != "0" {
		t.Errorf("Expected GPUID '0', got '%s'", gpu.GPUID)
	}
	
	if gpu.ModelName != "NVIDIA H100 80GB HBM3" {
		t.Errorf("Expected ModelName 'NVIDIA H100 80GB HBM3', got '%s'", gpu.ModelName)
	}
}

func TestGPUListResponse(t *testing.T) {
	gpus := []GPUInfo{
		{
			DeviceID:  "nvidia0",
			GPUID:     "0",
			UUID:      "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-031",
			LastSeen:  time.Now(),
		},
		{
			DeviceID:  "nvidia1",
			GPUID:     "1",
			UUID:      "GPU-6fd4f087-86f3-7a43-b711-4771313afc51",
			ModelName: "NVIDIA H100 80GB HBM3",
			Hostname:  "mtv5-dgx1-hgpu-031",
			LastSeen:  time.Now(),
		},
	}

	response := GPUListResponse{
		Count: len(gpus),
		GPUs:  gpus,
	}

	if response.Count != 2 {
		t.Errorf("Expected Count 2, got %d", response.Count)
	}
	
	if len(response.GPUs) != 2 {
		t.Errorf("Expected 2 GPUs, got %d", len(response.GPUs))
	}
}

func TestTelemetryResponse(t *testing.T) {
	data := []TelemetryDataResponse{
		{
			DeviceID: "nvidia0",
			Metric:   "DCGM_FI_DEV_GPU_UTIL",
			Value:    85.5,
			GPUID:    "0",
		},
	}

	response := TelemetryResponse{
		GPUID: "0",
		Count: len(data),
		Data:  data,
	}

	if response.GPUID != "0" {
		t.Errorf("Expected GPUID '0', got '%s'", response.GPUID)
	}
	
	if response.Count != 1 {
		t.Errorf("Expected Count 1, got %d", response.Count)
	}
	
	if len(response.Data) != 1 {
		t.Errorf("Expected 1 data point, got %d", len(response.Data))
	}
}

func TestHostInfo(t *testing.T) {
	host := HostInfo{
		Hostname: "mtv5-dgx1-hgpu-031",
		GPUCount: 8,
	}

	if host.Hostname != "mtv5-dgx1-hgpu-031" {
		t.Errorf("Expected Hostname 'mtv5-dgx1-hgpu-031', got '%s'", host.Hostname)
	}
	
	if host.GPUCount != 8 {
		t.Errorf("Expected GPUCount 8, got %d", host.GPUCount)
	}
}

func TestHostListResponse(t *testing.T) {
	hosts := []HostInfo{
		{
			Hostname: "mtv5-dgx1-hgpu-031",
			GPUCount: 8,
		},
		{
			Hostname: "mtv5-dgx1-hgpu-032",
			GPUCount: 4,
		},
	}

	response := HostListResponse{
		Count: len(hosts),
		Hosts: hosts,
	}

	if response.Count != 2 {
		t.Errorf("Expected Count 2, got %d", response.Count)
	}
	
	if len(response.Hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(response.Hosts))
	}
}

func TestNamespaceInfo(t *testing.T) {
	ns := NamespaceInfo{
		Namespace: "default",
		GPUCount:  4,
	}

	if ns.Namespace != "default" {
		t.Errorf("Expected Namespace 'default', got '%s'", ns.Namespace)
	}
	
	if ns.GPUCount != 4 {
		t.Errorf("Expected GPUCount 4, got %d", ns.GPUCount)
	}
}

func TestNamespaceListResponse(t *testing.T) {
	namespaces := []NamespaceInfo{
		{
			Namespace: "default",
			GPUCount:  4,
		},
		{
			Namespace: "production",
			GPUCount:  8,
		},
	}

	response := NamespaceListResponse{
		Count:      len(namespaces),
		Namespaces: namespaces,
	}

	if response.Count != 2 {
		t.Errorf("Expected Count 2, got %d", response.Count)
	}
	
	if len(response.Namespaces) != 2 {
		t.Errorf("Expected 2 namespaces, got %d", len(response.Namespaces))
	}
}

func TestErrorResponse(t *testing.T) {
	err := ErrorResponse{
		Error:   "Failed to query data",
		Message: "Database connection timeout",
	}

	if err.Error != "Failed to query data" {
		t.Errorf("Expected Error 'Failed to query data', got '%s'", err.Error)
	}
	
	if err.Message != "Database connection timeout" {
		t.Errorf("Expected Message 'Database connection timeout', got '%s'", err.Message)
	}
}
