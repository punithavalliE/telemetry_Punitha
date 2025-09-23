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
	//for i := 0; i < 10; i++ {
	for {
		rec, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				f.Seek(0, 0)
				r = csv.NewReader(f)
				continue
			}
			return err
		}
		if len(rec) < 12 {
			continue
		}

		// Send the entire CSV record as JSON array
		msgBody, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		if err := ss.queue.Publish("telemetry", msgBody); err != nil {
			return err
		}
		time.Sleep(delay)
	}
	return nil
}
