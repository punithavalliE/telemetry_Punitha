
package influx

import (
	"context"
	"time"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)


type InfluxWriter struct {
	client influxdb2.Client
	org    string
	bucket string
}

func NewInfluxWriter(url, token, org, bucket string) *InfluxWriter {
	client := influxdb2.NewClient(url, token)
	return &InfluxWriter{client: client, org: org, bucket: bucket}
}

func (iw *InfluxWriter) WriteTelemetry(deviceID string, metric string, value int64, ts time.Time) error {
	writeAPI := iw.client.WriteAPIBlocking(iw.org, iw.bucket)
	p := influxdb2.NewPoint(
		"telemetry",
		map[string]string{"device_id": deviceID, },
		map[string]interface{}{
			metric : value,
			"time": ts,
		},
		ts,
	)
	return writeAPI.WritePoint(context.Background(), p)
}

func (iw *InfluxWriter) Close() {
	iw.client.Close()
}

type TelemetryRecord struct {
       DeviceID string    `json:"device_id"`
       Metric   string    `json:"metric"`
       Value    int64   `json:"value"`
       Time     time.Time `json:"time"`
}

// QueryRecentTelemetry fetches the most recent N telemetry records from InfluxDB
func (iw *InfluxWriter) QueryRecentTelemetry(limit int) ([]TelemetryRecord, error) {
       queryAPI := iw.client.QueryAPI(iw.org)
       //flux := `from(bucket: "` + iw.bucket + `") |> range(start: -1h) |> filter(fn: (r) => r._measurement == "telemetry") |> sort(columns:["_time"], desc:true) |> limit(n:` +  fmt.Sprintf("%d", limit) + `)`
	   flux := `from(bucket: "` + iw.bucket + `") |> range(start: -1h) |> sort(columns:["_time"], desc:true) |> limit(n:` +  fmt.Sprintf("%d", limit) + `)`
       result, err := queryAPI.Query(context.Background(), flux)
       if err != nil {
	       return nil, err
       }
       records := []TelemetryRecord{}
       for result.Next() {
	      var deviceID, metric string
	      var value int64
	      if v := result.Record().ValueByKey("device_id"); v != nil {
		      if s, ok := v.(string); ok {
			      deviceID = s
		      }
	      }
	      if v := result.Record().ValueByKey("_field"); v != nil {
		      if s, ok := v.(string); ok {
			      metric = s
		      }
	      }
	      if v := result.Record().ValueByKey("_value"); v != nil {
		      switch val := v.(type) {
		      case int64:
			      value = val
		      case float64:
			      value = int64(val)
		      case int:
			      value = int64(val)
		      }
	      }
	      rec := TelemetryRecord{
		      DeviceID: deviceID,
		      Metric:   metric,
		      Value:    value,
		      Time:     result.Record().Time(),
	      }
	      records = append(records, rec)
       }
       if result.Err() != nil {
	       return nil, result.Err()
       }
       return records, nil
}