package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/segmentio/kafka-go"
)

const ImageUploadedTopic = "image-uploaded"

type ImageUploadedMessage struct {
	ImageID          string `json:"image_id"`
	ImagePath        string `json:"image_path"`
	OriginalFilename string `json:"original_filename"`
}

type KafkaProducer struct {
	writer *kafka.Writer
}

func newKafkaProducer() *KafkaProducer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Topic:        ImageUploadedTopic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
	}

	return &KafkaProducer{writer: writer}
}

func (p *KafkaProducer) Publish(ctx context.Context, msg ImageUploadedMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(msg.ImageID),
		Value: body,
	})
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
