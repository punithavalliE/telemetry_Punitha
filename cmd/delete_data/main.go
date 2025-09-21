package main

import (
	"fmt"
	"log"
	"os"

	"github.com/example/telemetry/internal/influx"
)

func main() {
	// Get InfluxDB connection details from environment variables
	url := os.Getenv("INFLUXDB_URL")
	token := os.Getenv("INFLUXDB_TOKEN")
	org := os.Getenv("INFLUXDB_ORG")
	bucket := os.Getenv("INFLUXDB_BUCKET")

	// Set defaults if environment variables are not set
	if url == "" {
		url = "http://localhost:8086"
	}
	if token == "" {
		token = "supersecrettoken"
	}
	if org == "" {
		org = "telemetryorg"
	}
	if bucket == "" {
		bucket = "telem_bucket"
	}

	fmt.Printf("Connecting to InfluxDB:\n")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Org: %s\n", org)
	fmt.Printf("  Bucket: %s\n", bucket)
	fmt.Printf("  Token: %s...\n", token[:10]+"***")

	// Create InfluxDB writer
	writer := influx.NewInfluxWriter(url, token, org, bucket)
	defer writer.Close()

	// Check command line arguments for delete options
	if len(os.Args) < 2 {
		fmt.Println("\nUsage:")
		fmt.Println("  go run main.go all              - Delete all data from bucket")
		fmt.Println("  go run main.go telemetry         - Delete all telemetry measurement data")
		fmt.Println("  go run main.go device <deviceID> - Delete data for specific device")
		fmt.Println("\nExample:")
		fmt.Println("  go run main.go all")
		fmt.Println("  go run main.go device GPU-001")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "all":
		fmt.Println("\nDeleting ALL data from bucket...")
		err := writer.DeleteAllData()
		if err != nil {
			log.Fatalf("Failed to delete all data: %v", err)
		}
		fmt.Println("✅ Successfully deleted all data from the bucket!")

	case "telemetry":
		fmt.Println("\nDeleting all telemetry measurement data...")
		err := writer.DeleteTelemetryData()
		if err != nil {
			log.Fatalf("Failed to delete telemetry data: %v", err)
		}
		fmt.Println("✅ Successfully deleted all telemetry data!")

	case "device":
		if len(os.Args) < 3 {
			log.Fatal("Device ID required. Usage: go run main.go device <deviceID>")
		}
		deviceID := os.Args[2]
		fmt.Printf("\nDeleting data for device: %s...\n", deviceID)
		err := writer.DeleteDataByDevice(deviceID)
		if err != nil {
			log.Fatalf("Failed to delete data for device %s: %v", deviceID, err)
		}
		fmt.Printf("✅ Successfully deleted data for device: %s!\n", deviceID)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Available commands: all, telemetry, device")
		os.Exit(1)
	}
}