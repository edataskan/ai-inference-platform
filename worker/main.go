package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/segmentio/kafka-go"
)

const (
	imageUploadedTopic = "image-uploaded"
	consumerGroupID    = "inference-workers"
)

type ImageUploadedMessage struct {
	ImageID          string `json:"image_id"`
	ImagePath        string `json:"image_path"`
	OriginalFilename string `json:"original_filename"`
}

func main() {
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}
	defer db.Close()

	redisClient := newRedisClient()
	defer redisClient.Close()

	reader := newKafkaReader()
	defer reader.Close()

	log.Println("Worker başlatıldı, Kafka mesajları dinleniyor...")

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Kafka'dan mesaj okunamadı: %v", err)
			continue
		}

		var payload ImageUploadedMessage
		if err := json.Unmarshal(msg.Value, &payload); err != nil {
			log.Printf("Mesaj parse edilemedi: %v", err)
			continue
		}

		processMessage(db, redisClient, payload)
	}
}

func processMessage(db *DB, redisClient *RedisClient, payload ImageUploadedMessage) {
	log.Printf("İşleniyor: image_id=%s file=%s", payload.ImageID, payload.OriginalFilename)
	ctx := context.Background()

	detections, err := callInferenceService(payload.ImagePath, payload.OriginalFilename)
	if err != nil {
		log.Printf("Inference başarısız (image_id=%s): %v", payload.ImageID, err)
		if dbErr := db.UpdateStatus(ctx, payload.ImageID, "failed", nil); dbErr != nil {
			log.Printf("DB güncellenemedi: %v", dbErr)
		}
		_ = redisClient.PublishResult(ctx, payload.ImageID, "failed", nil)
		return
	}

	if err := db.UpdateStatus(ctx, payload.ImageID, "done", detections); err != nil {
		log.Printf("DB güncellenemedi: %v", err)
		return
	}

	// Faz 3: Redis Pub/Sub üzerinden WebSocket servisine bildir
	if err := redisClient.PublishResult(ctx, payload.ImageID, "done", detections); err != nil {
		log.Printf("Redis Pub/Sub mesajı gönderilemedi: %v", err)
	}

	log.Printf("Tamamlandı ve Redis'e bildirildi: image_id=%s", payload.ImageID)
}

func newKafkaReader() *kafka.Reader {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(brokers, ","),
		GroupID: consumerGroupID,
		Topic:   imageUploadedTopic,
	})
}
