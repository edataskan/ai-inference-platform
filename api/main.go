package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const uploadsDir = "/app/uploads"

func main() {
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("uploads klasörü oluşturulamadı: %v", err)
	}

	db, err := connectDB()
	if err != nil {
		log.Fatalf("veritabanına bağlanılamadı: %v", err)
	}
	defer db.Close()

	producer := newKafkaProducer()
	defer producer.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		handleUpload(w, r, db, producer)
	})

	mux.HandleFunc("/results/", func(w http.ResponseWriter, r *http.Request) {
		handleGetResult(w, r, db)
	})

	log.Println("API servisi :8080 portunda dinliyor")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request, db *DB, producer *KafkaProducer) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "sadece POST destekleniyor")
		return
	}

	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "form parse edilemedi: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "'file' alanı bulunamadı: "+err.Error())
		return
	}
	defer file.Close()

	imageID := uuid.New().String()
	savedPath, err := saveUploadedFile(file, header, imageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "dosya kaydedilemedi: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := db.InsertPending(ctx, imageID, header.Filename); err != nil {
		writeError(w, http.StatusInternalServerError, "DB kaydı oluşturulamadı: "+err.Error())
		return
	}

	msg := ImageUploadedMessage{
		ImageID:          imageID,
		ImagePath:        savedPath,
		OriginalFilename: header.Filename,
	}
	if err := producer.Publish(ctx, msg); err != nil {
		_ = db.UpdateStatus(ctx, imageID, "failed", nil)
		writeError(w, http.StatusInternalServerError, "Kafka'ya mesaj gönderilemedi: "+err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"image_id": imageID,
		"status":   "pending",
		"message":  "Görüntü işleme kuyruğuna alındı. Sonucu /results/" + imageID + " üzerinden sorgulayabilirsiniz.",
	})
}

func handleGetResult(w http.ResponseWriter, r *http.Request, db *DB) {
	imageID := filepath.Base(r.URL.Path)
	if imageID == "" || imageID == "results" {
		writeError(w, http.StatusBadRequest, "image_id belirtilmedi")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := db.GetResult(ctx, imageID)
	if err != nil {
		writeError(w, http.StatusNotFound, "kayıt bulunamadı: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func saveUploadedFile(file multipart.File, header *multipart.FileHeader, imageID string) (string, error) {
	ext := filepath.Ext(header.Filename)
	destPath := filepath.Join(uploadsDir, imageID+ext)

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return destPath, nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
