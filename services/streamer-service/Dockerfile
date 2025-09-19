FROM golang:1.20-alpine as builder
WORKDIR /app
COPY . .
RUN cd /app && go build -mod=vendor -o streamer-service ./cmd/streamer-service

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/streamer-service /app/streamer-service
COPY ../../dcgm_metrics_20250718_134233.csv /data/dcgm_metrics_20250718_134233.csv
ENTRYPOINT ["/app/streamer-service"]
