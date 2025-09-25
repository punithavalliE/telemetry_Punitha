package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"time"

	"github.com/example/telemetry/internal/metrics"
)

// StreamCSV reads telemetry data from a CSV file and publishes the entire CSV record to the queue.
// CSV format: timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
func (ss *StreamerService) StreamCSV(filePath string, delay time.Duration) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	recordCount := 0
	ss.logger.Printf("Starting CSV streaming with %v delay between records", delay)

	// Skip the header row on first read
	skipHeader := true

	//for i := 0; i < 10; i++ {
	for {
		rec, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				ss.logger.Printf("Reached end of CSV file, restarting from beginning (processed %d records so far)", recordCount)
				f.Seek(0, 0)
				r = csv.NewReader(f)
				skipHeader = true // Reset header skip flag when restarting
				continue
			}
			return err
		}

		// Skip header row
		if skipHeader {
			ss.logger.Printf("Skipping CSV header row: %v", rec)
			skipHeader = false
			continue
		}

		if len(rec) < 12 {
			ss.logger.Printf("Skipping incomplete record (only %d fields)", len(rec))
			continue
		}

		// Send the entire CSV record as JSON array
		msgBody, err := json.Marshal(rec)
		if err != nil {
			ss.logger.Printf("Failed to marshal record %d: %v", recordCount, err)
			continue
		}

		recordCount++

		// Retry publish with exponential backoff
		maxRetries := 3
		published := false
		for attempt := 0; attempt < maxRetries && !published; attempt++ {
			if err := ss.queue.Publish("telemetry", msgBody); err != nil {
				if attempt == maxRetries-1 {
					ss.logger.Printf("Failed to publish record %d after %d attempts: %v (skipping)", recordCount, maxRetries, err)
				} else {
					retryDelay := time.Duration(attempt+1) * time.Second
					ss.logger.Printf("Failed to publish record %d (attempt %d/%d): %v (retrying in %v)", recordCount, attempt+1, maxRetries, err, retryDelay)
					time.Sleep(retryDelay)
				}
			} else {
				published = true
			}
		}

		// Record metrics only if message was successfully published
		if published {
			metrics.RecordMessageProduced("streamer-service", "telemetry")
			metrics.RecordTelemetryDataPoint("streamer-service", "csv_record")
		}

		// Log every 10th record to show activity without flooding logs
		if recordCount%10 == 0 {
			ss.logger.Printf("Published record %d: GPU ID=%s, Metric=%s, Timestamp=%s",
				recordCount, rec[2], rec[1], rec[0])
		}

		time.Sleep(delay)
	}
	// Note: This function runs an infinite loop, so this return is never reached
}
