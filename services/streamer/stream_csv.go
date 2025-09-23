package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"time"
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
	
	//for i := 0; i < 10; i++ {
	for {
		rec, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				ss.logger.Printf("Reached end of CSV file, restarting from beginning (processed %d records so far)", recordCount)
				f.Seek(0, 0)
				r = csv.NewReader(f)
				continue
			}
			return err
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
		if err := ss.queue.Publish("telemetry", msgBody); err != nil {
			ss.logger.Printf("Failed to publish record %d: %v", recordCount, err)
			return err
		}
		
		// Log every 10th record to show activity without flooding logs
		if recordCount%10 == 0 {
			ss.logger.Printf("Published record %d: GPU ID=%s, Metric=%s, Timestamp=%s", 
				recordCount, rec[2], rec[1], rec[0])
		}
		
		time.Sleep(delay)
	}
	return nil
}
