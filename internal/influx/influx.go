
package influx

import (
	"context"
	"time"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/example/telemetry/internal/telemetry"
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

func (iw *InfluxWriter) WriteTelemetry(record telemetry.TelemetryRecord) error {
	fmt.Printf("Writing to InfluxDB: device=%s, metric=%s, value=%f, time=%s\n", record.DeviceID, record.Metric, record.Value, record.Time.Format(time.RFC3339))
	writeAPI := iw.client.WriteAPIBlocking(iw.org, iw.bucket)
	p := influxdb2.NewPoint(
		record.Metric,
		map[string]string{
			"device_id": record.DeviceID,
			"gpu_id": record.GPUID,
			"uuid": record.UUID,
			"modelName": record.ModelName,
			"Hostname": record.Hostname,
			"container": record.Container,
			"pod": record.Pod,
			"namespace": record.Namespace,
			"labels_raw": record.LabelsRaw,
		},
		map[string]interface{}{
			"value": record.Value,
		},
		record.Time, // This is the point's official timestamp
	)
	return writeAPI.WritePoint(context.Background(), p)
}

func (iw *InfluxWriter) Close() {
	iw.client.Close()
}

// QueryRecentTelemetry fetches the most recent N telemetry records from InfluxDB
func (iw *InfluxWriter) QueryRecentTelemetry(limit int) ([]telemetry.TelemetryRecord, error) {
       queryAPI := iw.client.QueryAPI(iw.org)
       flux := `from(bucket: "` + iw.bucket + `") |> range(start: -24h) |> sort(columns:["_time"], desc:true) |> limit(n:` +  fmt.Sprintf("%d", limit) + `)`
       result, err := queryAPI.Query(context.Background(), flux)
       if err != nil {
	       return nil, err
       }
       return iw.parseQueryResults(result)
}

/*from(bucket: "telem_bucket")
  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)
  |> group(columns: ["uuid"])
  |> keep(columns: ["uuid"])
  |> yield(name: "unique") */
func (iw *InfluxWriter) QueryUniqueUUIDs() ([]string, error) {
	queryAPI := iw.client.QueryAPI(iw.org)
	flux := fmt.Sprintf(`from(bucket: "%s") |> range(start: 0) |> group(columns: ["uuid"]) |> keep(columns: ["uuid"]) |> distinct(column: "uuid")`, iw.bucket)
	result, err := queryAPI.Query(context.Background(), flux)
	if err != nil {
		return nil, err
	}
	uuids := []string{}
	for result.Next() {
		if v := result.Record().ValueByKey("uuid"); v != nil {
			if s, ok := v.(string); ok {
				uuids = append(uuids, s)
			}
		}
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return uuids, nil
}

// QueryTelemetryByDevice fetches telemetry records for a specific device
func (iw *InfluxWriter) QueryTelemetryByDevice(uuid string) ([]telemetry.TelemetryRecord, error) {
	queryAPI := iw.client.QueryAPI(iw.org)
	flux := `from(bucket: "` + iw.bucket + `") |> range(start: 0) |> filter(fn: (r) => r.uuid == "` + uuid + `") |> sort(columns:["_time"], desc:true)`
	result, err := queryAPI.Query(context.Background(), flux)
	if err != nil {
		return nil, err
	}
	return iw.parseQueryResults(result)
}

// QueryTelemetryByDeviceTimeRange fetches telemetry records for a specific device within a time range
func (iw *InfluxWriter) QueryTelemetryByDeviceTimeRange(uuid string, startTime, endTime string) ([]telemetry.TelemetryRecord, error) {
	queryAPI := iw.client.QueryAPI(iw.org)
	// Parse the time strings to ensure they're valid RFC3339 format
	parsedStart, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start time format: %v", err)
	}
	parsedEnd, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, fmt.Errorf("invalid end time format: %v", err)
	}
	
	// Use proper RFC3339 formatting for InfluxDB
	flux := fmt.Sprintf(`from(bucket: "%s") |> range(start: %s, stop: %s) |> filter(fn: (r) => r.uuid == "%s") |> sort(columns:["_time"], desc:true)`, 
		iw.bucket, 
		parsedStart.Format(time.RFC3339), 
		parsedEnd.Format(time.RFC3339), 
		uuid)
	
	result, err := queryAPI.Query(context.Background(), flux)
	if err != nil {
		return nil, err
	}
	return iw.parseQueryResults(result)
}

// parseQueryResults is a helper function to parse query results into TelemetryRecord structs
func (iw *InfluxWriter) parseQueryResults(result *api.QueryTableResult) ([]telemetry.TelemetryRecord, error) {
	records := []telemetry.TelemetryRecord{}
	for result.Next() {
		var deviceID, metric, gpuID, uuid, modelName, hostname, container, pod, namespace, labelsRaw string
		var value float64
		
		if v := result.Record().ValueByKey("device_id"); v != nil {
			if s, ok := v.(string); ok {
				deviceID = s
			}
		}
		if v := result.Record().ValueByKey("_measurement"); v != nil {
			if s, ok := v.(string); ok {
				metric = s
			}
		}
		if v := result.Record().ValueByKey("_value"); v != nil {
			switch val := v.(type) {
			case int64:
				value = float64(val)
			case float64:
				value = val
			case int:
				value = float64(val)
			}
		}
		if v := result.Record().ValueByKey("gpu_id"); v != nil {
			if s, ok := v.(string); ok {
				gpuID = s
			}
		}
		if v := result.Record().ValueByKey("uuid"); v != nil {
			if s, ok := v.(string); ok {
				uuid = s
			}
		}
		if v := result.Record().ValueByKey("modelName"); v != nil {
			if s, ok := v.(string); ok {
				modelName = s
			}
		}
		if v := result.Record().ValueByKey("Hostname"); v != nil {
			if s, ok := v.(string); ok {
				hostname = s
			}
		}
		if v := result.Record().ValueByKey("container"); v != nil {
			if s, ok := v.(string); ok {
				container = s
			}
		}
		if v := result.Record().ValueByKey("pod"); v != nil {
			if s, ok := v.(string); ok {
				pod = s
			}
		}
		if v := result.Record().ValueByKey("namespace"); v != nil {
			if s, ok := v.(string); ok {
				namespace = s
			}
		}
		if v := result.Record().ValueByKey("labels_raw"); v != nil {
			if s, ok := v.(string); ok {
				labelsRaw = s
			}
		}
		
		rec := telemetry.TelemetryRecord{
			DeviceID:  deviceID,
			Metric:    metric,
			Value:     value,
			Time:      result.Record().Time(),
			GPUID:     gpuID,
			UUID:      uuid,
			ModelName: modelName,
			Hostname:  hostname,
			Container: container,
			Pod:       pod,
			Namespace: namespace,
			LabelsRaw: labelsRaw,
		}
		records = append(records, rec)
	}
	if result.Err() != nil {
		return nil, result.Err()
	}
	return records, nil
}