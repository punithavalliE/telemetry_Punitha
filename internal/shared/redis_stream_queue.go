package shared

import (
	"context"
	"time"
	"log"
	"fmt"
	"github.com/redis/go-redis/v9"
)

type RedisStreamQueue struct {
	client *redis.Client
	stream string
	group  string
	name   string
}

func NewRedisStreamQueue(addr, stream, group, name string) (*RedisStreamQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	ctx := context.Background()
	// Create consumer group if not exists
	_ = client.XGroupCreateMkStream(ctx, stream, group, "$")
	return &RedisStreamQueue{client: client, stream: stream, group: group, name: name}, nil
}

func (q *RedisStreamQueue) Publish(topic string, body []byte) error {
	ctx := context.Background()
	id, err :=  q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]interface{}{
			"topic": topic,
			"body":  body,
		},
	}).Result()
	if err != nil {
		log.Fatalf("xadd failed: %v", err)
		return err
	}
	fmt.Println("sent message id:", id)	
	return nil
}

func (q *RedisStreamQueue) Subscribe(handler func(topic string, body []byte, id string) error) error {
	ctx := context.Background()
	for {
		msgs, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.group,
			Consumer: q.name,
			Streams:  []string{q.stream, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()

		if err != nil && err != redis.Nil {
			log.Fatalf("xreadgroup failed: %v", err)
			return err
		}
		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				fmt.Printf("Processing %s: %v\n", msg.ID, msg.Values)
				topic, _ := msg.Values["topic"].(string)
				bodyStr, _ := msg.Values["body"].(string)
				body := []byte(bodyStr)
				fmt.Printf("Received message id=%s topic=%s body=%s, len= %d\n", msg.ID, topic, string(body), len(body))
				if err := handler(topic, body, msg.ID); err == nil {
					q.client.XAck(ctx, q.stream, q.group, msg.ID)
				}
			}
		}
	}
}

func (q *RedisStreamQueue) Close() error {
	return q.client.Close()
}
