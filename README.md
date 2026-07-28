# Real-Time AI Inference Platform

Görüntü yükleme → asenkron işleme (Kafka) → YOLO inference → PostgreSQL kaydı
→ WebSocket ile gerçek zamanlı bildirim akışını uygulayan, mikroservis
mimarisiyle tasarlanmış bir platform.

## Mimari

```
[Kullanıcı] --upload--> [API (Go)] --produce--> [Kafka]
                                                    |
                                              [Worker (Go)] --HTTP--> [Inference (Python/YOLO)]
                                                    |
                                            [PostgreSQL] <--write
                                                    |
                                            [Redis pub/sub] --> [WebSocket (Go)] --push--> [Kullanıcı]
```

## Servisler

| Servis | Dil | Sorumluluk 
|---|---|---|---|
| `api/` | Go | Görüntü yükleme, Kafka'ya mesaj atma 
| `worker/` | Go | Kafka consumer, inference'ı tetikleme, DB güncelleme 
| `inference/` | Python (FastAPI) | YOLO ile nesne tespiti 
| `websocket/` | Go | Sonuçları gerçek zamanlı kullanıcıya push etme 
| `deploy/` | - | Docker Compose, K8s manifest'leri, SQL migration 

## Geliştirme Fazları

-  **Faz 1** — Senkron MVP: API → Inference servisi → PostgreSQL
-  **Faz 2** — Kafka ile asenkron hale getirme (API → Kafka → Worker)
-  **Faz 3** — Redis pub/sub + WebSocket ile gerçek zamanlı bildirim
- **Faz 4** — Tüm servisleri Docker'a alma + Kubernetes'e deploy

## Lokal Çalıştırma

```bash
docker compose up --build
```

- API: http://localhost:8080
- Inference servisi: http://localhost:8001
- WebSocket Servisi: ws://localhost:8082/ws?image_id=<image_id>
- PostgreSQL: localhost:5432 (user: app, pass: app, db: inference_db)
- Redis: localhost:6379
- Kafka: localhost:9092

### Test akışı (Faz 2)
1. Asenkron İşleme ve HTTP Polling (Faz 2)
```bash
# 1. Görüntü yükle - hemen 202 + image_id dönecek
curl -X POST -F "file=@test.jpg" http://localhost:8080/upload

# 2. Worker arka planda işlerken, sonucu sorgula (birkaç saniye sonra "done" görmelisin)
curl http://localhost:8080/results/<image_id>

# 3. Worker'ı ölçeklendirmeyi test et (Kafka mesajlarının paylaşıldığını logs'tan gözlemle)
docker compose up --build --scale worker=3
```
2. Gerçek Zamanlı Bildirim Testi (Faz 3)
- Tarayıcı konsolundan (F12) WebSocket bağlantısı açın:
  const imageId = "<SİZE_GELEN_IMAGE_ID>";
  const ws = new WebSocket(`ws://localhost:8082/ws?image_id=${imageId}`);
  ws.onmessage = (e) => console.log("🔥 CANLI SONUÇ:", JSON.parse(e.data));
-Görseli /upload endpoint'ine gönderdiğiniz an sonuç WebSocket konsoluna düşecektir.

**Kubernetes (Minikube) Deployment (Faz 4)**

  minikube addons enable ingress
  
  kubectl apply -f deploy/k8s/
  
  minikube tunnel

## Neden bu teknolojiler?

- **Go**: API ve worker'larda concurrency ve düşük memory footprint için.
- **Kafka**: API'yi inference süresinden bağımsızlaştırmak, worker'ları
  bağımsız ölçeklendirebilmek için (backpressure yönetimi).
- **Redis**: Worker ile WebSocket servisi arasında pub/sub üzerinden
  gevşek bağlı (loosely coupled) haberleşme sağlamak için.
- **PostgreSQL**: Sonuçların kalıcı ve sorgulanabilir şekilde saklanması için.
- **Kubernetes**: Servislerin bağımsız ölçeklendirilmesi ve health-check
  ile kendi kendini onarması için.
