package main

import "time"

// GPUInfo represents GPU information response
type GPUInfo struct {
	DeviceID  string    `json:"device_id" example:"nvidia0"`
	GPUID     string    `json:"gpu_id" example:"0"`
	UUID      string    `json:"uuid" example:"GPU-5fd4f087-86f3-7a43-b711-4771313afc50"`
	ModelName string    `json:"model_name" example:"NVIDIA H100 80GB HBM3"`
	Hostname  string    `json:"hostname" example:"mtv5-dgx1-hgpu-031"`
	Container string    `json:"container" example:""`
	Pod       string    `json:"pod" example:""`
	Namespace string    `json:"namespace" example:""`
	LastSeen  time.Time `json:"last_seen" example:"2025-07-18T20:42:34Z"`
}

// GPUListResponse represents the response for GPU list endpoint
type GPUListResponse struct {
	Count int       `json:"count" example:"2"`
	GPUs  []GPUInfo `json:"gpus"`
}

// TelemetryResponse represents the response for telemetry endpoint
type TelemetryResponse struct {
	GPUID string                 `json:"gpu_id" example:"nvidia0"`
	Count int                    `json:"count" example:"100"`
	Data  []TelemetryDataResponse `json:"data"`
}

// TelemetryDataResponse represents individual telemetry data
type TelemetryDataResponse struct {
	DeviceID  string    `json:"device_id" example:"nvidia0"`
	Metric    string    `json:"metric" example:"DCGM_FI_DEV_GPU_UTIL"`
	Value     float64   `json:"value" example:"85.5"`
	Time      time.Time `json:"time" example:"2025-07-18T20:42:34Z"`
	GPUID     string    `json:"gpu_id" example:"0"`
	UUID      string    `json:"uuid" example:"GPU-5fd4f087-86f3-7a43-b711-4771313afc50"`
	ModelName string    `json:"model_name" example:"NVIDIA H100 80GB HBM3"`
	Hostname  string    `json:"hostname" example:"mtv5-dgx1-hgpu-031"`
	Container string    `json:"container" example:""`
	Pod       string    `json:"pod" example:""`
	Namespace string    `json:"namespace" example:""`
	LabelsRaw string    `json:"labels_raw" example:"DCGM_FI_DRIVER_VERSION=\"535.129.03\""`
}

// HostInfo represents host information
type HostInfo struct {
	Hostname string `json:"hostname" example:"mtv5-dgx1-hgpu-031"`
	GPUCount int    `json:"gpu_count" example:"8"`
}

// HostListResponse represents the response for host list endpoint
type HostListResponse struct {
	Count int        `json:"count" example:"1"`
	Hosts []HostInfo `json:"hosts"`
}

// NamespaceInfo represents namespace information
type NamespaceInfo struct {
	Namespace string `json:"namespace" example:"default"`
	GPUCount  int    `json:"gpu_count" example:"4"`
}

// NamespaceListResponse represents the response for namespace list endpoint
type NamespaceListResponse struct {
	Count      int             `json:"count" example:"2"`
	Namespaces []NamespaceInfo `json:"namespaces"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error" example:"Failed to query data"`
	Message string `json:"message,omitempty" example:"Additional error details"`
}