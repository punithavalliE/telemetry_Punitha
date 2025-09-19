FROM golang:1.20-alpine as builder
WORKDIR /app
COPY . .
RUN cd /app && go build -mod=vendor -o collector-service ./cmd/collector-service

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/collector-service /app/collector-service
ENTRYPOINT ["/app/collector-service"]
