package main

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"
	"github.com/example/telemetry/internal/telemetry"
)

// StreamCSV reads telemetry data from a CSV file and publishes it to the queue.
// CSV format: device_id,metric,value
func (ss *StreamerService) StreamCSV(filePath string, delay time.Duration) error {
       f, err := os.Open(filePath)
       if err != nil {
	       return err
       }
       defer f.Close()

       r := csv.NewReader(f)
	for i := 0; i < 10; i++ {
		rec, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				f.Seek(0, 0)
				r = csv.NewReader(f)
				continue
			}
			return err
		}
		if len(rec) < 3 {
			continue
		}
		val, err := strconv.ParseInt(rec[2], 10, 64)
		if err != nil {
			continue
		}
		data := telemetry.TelemetryData{
			DeviceID: rec[4],
			Metric:   rec[1],
			Value:    val,
			Time:     rec[0],
		}
		msgBody, err := telemetry.Marshal(data)
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
