FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN cd /app && go build -mod=vendor -o streamer-service ./services/streamer

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/streamer-service /app/streamer-service
# Copy the CSV data file
COPY --from=builder /app/dcgm_metrics_20250718_134233.csv /data/dcgm_metrics_20250718_134233.csv
EXPOSE 9090
ENTRYPOINT ["/app/streamer-service"]