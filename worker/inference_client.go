package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"time"
)

func callInferenceService(imagePath string, originalFilename string) (interface{}, error) {
	inferenceURL := os.Getenv("INFERENCE_URL")
	if inferenceURL == "" {
		inferenceURL = "http://localhost:8001"
	}

	f, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("görüntü açılamadı: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	ext := filepath.Ext(originalFilename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "image/jpeg" // Fallback varsayılan görsel tipi
	}

	// CreateFormFile yerine özel header ile Part oluştur
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(originalFilename)))
	h.Set("Content-Type", contentType)

	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, inferenceURL+"/infer", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inference servisine istek başarısız: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference servisi %d döndü: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("inference yanıtı parse edilemedi: %w", err)
	}

	detections, ok := result["detections"]
	if !ok {
		return nil, fmt.Errorf("inference yanıtında 'detections' alanı yok")
	}

	return detections, nil
}
