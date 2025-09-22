package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T, prefix string) string {
	dir, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Clean up when test finishes
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	
	return dir
}

// TempFile creates a temporary file with content for testing
func TempFile(t *testing.T, dir, pattern, content string) string {
	if dir == "" {
		dir = TempDir(t, "test-files")
	}
	
	file, err := ioutil.TempFile(dir, pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	
	if content != "" {
		if _, err := file.WriteString(content); err != nil {
			file.Close()
			t.Fatalf("Failed to write to temp file: %v", err)
		}
	}
	
	filename := file.Name()
	file.Close()
	
	// Clean up when test finishes
	t.Cleanup(func() {
		os.Remove(filename)
	})
	
	return filename
}

// CreateTestCSVFile creates a CSV file with test data
func CreateTestCSVFile(t *testing.T, dir string) string {
	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
2023-07-18T20:42:34Z,DCGM_FI_DEV_GPU_UTIL,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,test-pod,default,85.5,"version=535.129.03"
2023-07-18T20:42:35Z,DCGM_FI_DEV_MEM_COPY_UTIL,1,nvidia1,GPU-6fd4f087-86f3-7a43-b711-4771313afc51,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-031,,test-pod-2,production,72.3,"version=535.129.03"
2023-07-18T20:42:36Z,DCGM_FI_DEV_GPU_TEMP,0,nvidia0,GPU-5fd4f087-86f3-7a43-b711-4771313afc50,NVIDIA H100 80GB HBM3,mtv5-dgx1-hgpu-032,container-1,test-pod-3,development,65.0,"version=535.129.03"`

	return TempFile(t, dir, "test-*.csv", csvContent)
}

// CreateTestJSONFile creates a JSON file with test data
func CreateTestJSONFile(t *testing.T, dir, content string) string {
	if content == "" {
		content = `{
			"device_id": "nvidia0",
			"metric": "DCGM_FI_DEV_GPU_UTIL",
			"value": 85.5,
			"timestamp": "2023-07-18T20:42:34Z",
			"gpu_id": "0",
			"uuid": "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
			"model_name": "NVIDIA H100 80GB HBM3",
			"hostname": "mtv5-dgx1-hgpu-031",
			"container": "",
			"pod": "test-pod",
			"namespace": "default",
			"labels": {"version": "535.129.03"}
		}`
	}
	
	return TempFile(t, dir, "test-*.json", content)
}

// FileExists checks if a file exists
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// DirExists checks if a directory exists
func DirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// CreateTestFile creates a file with specific content at the given path
func CreateTestFile(t *testing.T, filePath, content string) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}
	
	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", filePath, err)
	}
	
	// Clean up when test finishes
	t.Cleanup(func() {
		os.Remove(filePath)
	})
}

// ReadTestFile reads the content of a file
func ReadTestFile(t *testing.T, filePath string) string {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filePath, err)
	}
	return string(content)
}

// CreateTestDirectory creates a directory for testing
func CreateTestDirectory(t *testing.T, dirPath string) {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dirPath, err)
	}
	
	// Clean up when test finishes
	t.Cleanup(func() {
		os.RemoveAll(dirPath)
	})
}
