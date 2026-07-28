package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

type ResultNotification struct {
	ImageID    string      `json:"image_id"`
	Status     string      `json:"status"`
	Detections interface{} `json:"detections,omitempty"`
}

func newRedisClient() *RedisClient {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &RedisClient{client: rdb}
}

func (r *RedisClient) PublishResult(ctx context.Context, imageID string, status string, detections interface{}) error {
	payload := ResultNotification{
		ImageID:    imageID,
		Status:     status,
		Detections: detections,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return r.client.Publish(ctx, "inference-results", data).Err()
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
