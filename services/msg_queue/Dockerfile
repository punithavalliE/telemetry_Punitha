FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
WORKDIR /app/services/msg_queue
RUN go build -o msg_queue

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/services/msg_queue/msg_queue .
RUN mkdir -p /root/data
ENV PORT=9090
ENV BROKER_INDEX=0
ENV BROKER_COUNT=1
ENV TOPICS=telemetry:4,events:8,orders:4,default:8
EXPOSE 9090
CMD ["./msg_queue"]