FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN cd /app && go build -mod=vendor -o streamer-service ./services/streamer-service

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/streamer-service /app/streamer-service
EXPOSE 9090
ENTRYPOINT ["/app/streamer-service"]