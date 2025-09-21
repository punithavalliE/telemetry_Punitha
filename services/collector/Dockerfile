FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN cd /app && go build -mod=vendor -o collector-service ./services/collector

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/collector-service /app/collector-service
ENTRYPOINT ["/app/collector-service"]
