FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
WORKDIR /app/services/msg_queue
RUN go build -o msg_queue

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/services/msg_queue/msg_queue .
RUN mkdir -p /root/data
#ENV PORT=8080
# BROKER_INDEX and BROKER_COUNT will be set by StatefulSet
# Default values only for standalone testing
ENV BROKER_INDEX=0
ENV BROKER_COUNT=1
ENV TOPICS=telemetry:4
CMD ["./msg_queue"]