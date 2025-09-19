FROM golang:1.21-alpine as builder
WORKDIR /app
COPY . .
WORKDIR /app/cmd/api-service
RUN go build -o api-service

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cmd/api-service/api-service .
COPY --from=builder /app/internal/influx /app/internal/influx
ENV INFLUXDB_URL=http://influxdb:8086
ENV INFLUXDB_TOKEN=supersecrettoken
ENV INFLUXDB_ORG=telemetryorg
ENV INFLUXDB_BUCKET=telem_bucket
EXPOSE 8080
CMD ["./api-service"]
