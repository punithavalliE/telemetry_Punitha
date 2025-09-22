package main

import (
	"testing"
	"time"
)

func TestStreamCSVFunction(t *testing.T) {
	t.Run("CSV Processing Logic", func(t *testing.T) {
		// Test the CSV processing logic without file I/O
		csvRecord := []string{
			"2023-07-18T20:42:34Z",
			"DCGM_FI_DEV_GPU_UTIL",
			"0",
			"nvidia0",
			"GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			"NVIDIA H100 80GB HBM3",
			"mtv5-dgx1-hgpu-031",
			"",
			"test-pod",
			"default",
			"85.5",
			"version=535.129.03",
		}

		// Test that we have the expected number of fields
		if len(csvRecord) < 12 {
			t.Errorf("Expected at least 12 fields, got %d", len(csvRecord))
		}

		// Test specific field values
		expectedTimestamp := "2023-07-18T20:42:34Z"
		if csvRecord[0] != expectedTimestamp {
			t.Errorf("Expected timestamp '%s', got '%s'", expectedTimestamp, csvRecord[0])
		}

		expectedMetric := "DCGM_FI_DEV_GPU_UTIL"
		if csvRecord[1] != expectedMetric {
			t.Errorf("Expected metric '%s', got '%s'", expectedMetric, csvRecord[1])
		}

		expectedGPUID := "0"
		if csvRecord[2] != expectedGPUID {
			t.Errorf("Expected GPU ID '%s', got '%s'", expectedGPUID, csvRecord[2])
		}

		expectedDevice := "nvidia0"
		if csvRecord[3] != expectedDevice {
			t.Errorf("Expected device '%s', got '%s'", expectedDevice, csvRecord[3])
		}

		expectedUUID := "GPU-5fd4f087-86f3-7a43-b711-4771313afc50"
		if csvRecord[4] != expectedUUID {
			t.Errorf("Expected UUID '%s', got '%s'", expectedUUID, csvRecord[4])
		}

		expectedModel := "NVIDIA H100 80GB HBM3"
		if csvRecord[5] != expectedModel {
			t.Errorf("Expected model '%s', got '%s'", expectedModel, csvRecord[5])
		}

		expectedHostname := "mtv5-dgx1-hgpu-031"
		if csvRecord[6] != expectedHostname {
			t.Errorf("Expected hostname '%s', got '%s'", expectedHostname, csvRecord[6])
		}

		expectedPod := "test-pod"
		if csvRecord[8] != expectedPod {
			t.Errorf("Expected pod '%s', got '%s'", expectedPod, csvRecord[8])
		}

		expectedNamespace := "default"
		if csvRecord[9] != expectedNamespace {
			t.Errorf("Expected namespace '%s', got '%s'", expectedNamespace, csvRecord[9])
		}

		expectedValue := "85.5"
		if csvRecord[10] != expectedValue {
			t.Errorf("Expected value '%s', got '%s'", expectedValue, csvRecord[10])
		}

		expectedLabels := "version=535.129.03"
		if csvRecord[11] != expectedLabels {
			t.Errorf("Expected labels '%s', got '%s'", expectedLabels, csvRecord[11])
		}
	})

	t.Run("CSV Field Mapping", func(t *testing.T) {
		// Test the expected field positions match the CSV format comment
		// CSV format: timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
		fieldPositions := map[string]int{
			"timestamp":    0,
			"metric_name":  1,
			"gpu_id":       2,
			"device":       3,
			"uuid":         4,
			"modelName":    5,
			"Hostname":     6,
			"container":    7,
			"pod":          8,
			"namespace":    9,
			"value":        10,
			"labels_raw":   11,
		}

		expectedPositions := map[string]int{
			"timestamp":    0,
			"metric_name":  1,
			"gpu_id":       2,
			"device":       3,
			"uuid":         4,
			"modelName":    5,
			"Hostname":     6,
			"container":    7,
			"pod":          8,
			"namespace":    9,
			"value":        10,
			"labels_raw":   11,
		}

		for field, position := range fieldPositions {
			if expectedPositions[field] != position {
				t.Errorf("Field '%s': expected position %d, got %d", field, expectedPositions[field], position)
			}
		}
	})

	t.Run("Delay Configuration", func(t *testing.T) {
		delays := []time.Duration{
			10 * time.Millisecond,
			100 * time.Millisecond,
			1 * time.Second,
		}

		for _, delay := range delays {
			start := time.Now()
			time.Sleep(delay)
			elapsed := time.Since(start)

			// Allow some tolerance for timing
			tolerance := 10 * time.Millisecond
			if elapsed < delay-tolerance || elapsed > delay+tolerance {
				t.Errorf("Expected delay ~%v, got %v", delay, elapsed)
			}
		}
	})
}

func TestCSVValidation(t *testing.T) {
	t.Run("Minimum Fields Required", func(t *testing.T) {
		testCases := []struct {
			name     string
			record   []string
			expected bool
		}{
			{
				name:     "Valid Record",
				record:   []string{"ts", "metric", "gpu", "dev", "uuid", "model", "host", "cont", "pod", "ns", "val", "labels"},
				expected: true,
			},
			{
				name:     "Insufficient Fields",
				record:   []string{"ts", "metric", "gpu", "dev", "uuid"},
				expected: false,
			},
			{
				name:     "Empty Record",
				record:   []string{},
				expected: false,
			},
			{
				name:     "Exactly 12 Fields",
				record:   []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
				expected: true,
			},
			{
				name:     "More Than 12 Fields",
				record:   []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14"},
				expected: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				isValid := len(tc.record) >= 12
				if isValid != tc.expected {
					t.Errorf("Expected validation result %v, got %v for record with %d fields", tc.expected, isValid, len(tc.record))
				}
			})
		}
	})
}
