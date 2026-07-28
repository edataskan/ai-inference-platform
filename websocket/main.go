package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ClientManager struct {
	clients map[string][]*websocket.Conn
	mu      sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string][]*websocket.Conn),
	}
}

func (cm *ClientManager) AddClient(imageID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.clients[imageID] = append(cm.clients[imageID], conn)
}

func (cm *ClientManager) RemoveClient(imageID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conns := cm.clients[imageID]
	for i, c := range conns {
		if c == conn {
			cm.clients[imageID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	if len(cm.clients[imageID]) == 0 {
		delete(cm.clients, imageID)
	}
}

func (cm *ClientManager) Broadcast(imageID string, message []byte) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if conns, ok := cm.clients[imageID]; ok {
		for _, conn := range conns {
			_ = conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

type Notification struct {
	ImageID    string      `json:"image_id"`
	Status     string      `json:"status"`
	Detections interface{} `json:"detections"`
}

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	cm := NewClientManager()

	// Redis Pub/Sub Dinleyici Routine
	go listenRedisChannel(rdb, cm)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		imageID := r.URL.Query().Get("image_id")
		if imageID == "" {
			http.Error(w, "image_id parametresi gerekli", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket bağlantı hatası: %v", err)
			return
		}

		cm.AddClient(imageID, conn)
		log.Printf("Yeni WebSocket bağlantısı: image_id=%s", imageID)

		// Connection açık tutulduğu sürece okuma döngüsü (disconnect yakalamak için)
		defer func() {
			cm.RemoveClient(imageID, conn)
			conn.Close()
			log.Printf("WebSocket bağlantısı kapatıldı: image_id=%s", imageID)
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	})

	log.Println("WebSocket servisi :8082 portunda dinliyor...")
	if err := http.ListenAndServe(":8082", cmManagerHandler(http.DefaultServeMux)); err != nil {
		log.Fatal(err)
	}
}

func cmManagerHandler(mux *http.ServeMux) http.Handler {
	return mux
}

func listenRedisChannel(rdb *redis.Client, cm *ClientManager) {
	pubsub := rdb.Subscribe(context.Background(), "inference-results")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("Redis Pub/Sub 'inference-results' kanalı dinleniyor...")

	for msg := range ch {
		var notification Notification
		if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
			log.Printf("Redis mesajı parse edilemedi: %v", err)
			continue
		}

		// İlgili image_id'yi dinleyen istemcilere push et
		cm.Broadcast(notification.ImageID, []byte(msg.Payload))
	}
}
