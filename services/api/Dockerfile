FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN cd /app && go build -mod=vendor -o api-service ./services/api

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/api-service .
COPY --from=builder /app/internal/influx /app/internal/influx
ENV INFLUXDB_URL=http://influxdb:8086
ENV INFLUXDB_TOKEN=supersecrettoken
ENV INFLUXDB_ORG=telemetryorg
ENV INFLUXDB_BUCKET=telem_bucket
EXPOSE 8080
CMD ["./api-service"]
