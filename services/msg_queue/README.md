# Message Queue Service

This is a custom HTTP-based message queue service that provides topic-based messaging with partitions and consumer groups.

## Features

- **Topics and Partitions**: Messages are organized by topics with configurable partitions per topic
- **Consumer Groups**: Multiple consumers can be part of the same group for load balancing
- **Persistence**: Messages are persisted to disk for durability
- **Visibility Timeout**: In-flight messages are automatically requeued if not acknowledged within timeout
- **HTTP API**: RESTful API for producing, consuming, and acknowledging messages
- **Scalability**: Supports multiple broker instances with partition ownership

## API Endpoints

### Produce Message
```
POST /produce?topic=<topic>&partition=<partition>
Content-Type: application/json

{"payload": "your message content"}
```

### Consume Messages (Server-Sent Events)
```
GET /consume?topic=<topic>&partition=<partition>&group=<group>
```

### Acknowledge Message
```
POST /ack?topic=<topic>&partition=<partition>&group=<group>
Content-Type: application/json

{"id": "message_id"}
```

### Get Topics
```
GET /topics
```

## Environment Variables

- `PORT`: Server port (default: 8080)
- `BROKER_INDEX`: Broker instance index for partition ownership (default: 0)
- `BROKER_COUNT`: Total number of broker instances (default: 1)
- `TOPICS`: Comma-separated list of topics with partition counts (default: events:8,orders:4,default:8)

## Docker Usage

```bash
# Build the container
docker build -t msg_queue -f services/msg_queue/Dockerfile .

# Run the container
docker run -p 8090:8080 \
  -e TOPICS=telemetry:4,events:8 \
  -v msg_queue_data:/root/data \
  msg_queue
```

## Integration with Other Services

Services can be configured to use either Redis or the HTTP message queue by setting:

- `USE_HTTP_QUEUE=true` - Use HTTP message queue
- `MSG_QUEUE_ADDR=http://msg_queue:8080` - Message queue service URL
- `MSG_QUEUE_TOPIC=telemetry` - Topic name
- `MSG_QUEUE_GROUP=telemetry_group` - Consumer group
- `MSG_QUEUE_PRODUCER_NAME=producer_name` - Producer/consumer name

If `USE_HTTP_QUEUE` is not set or false, services will fall back to Redis.

## Storage

Messages are stored in `/root/data` directory with one log file per partition:
- `./data/<topic>/partition-<N>.log`

Each log file contains JSON messages, one per line.

## Partition Assignment

Partitions are assigned to broker instances using: `partition % BROKER_COUNT == BROKER_INDEX`

This allows running multiple broker instances where each owns a subset of partitions.