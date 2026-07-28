package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func connectDB() (*DB, error) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "postgres://app:app@localhost:5432/inference_db"
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}

	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) InsertPending(ctx context.Context, imageID string, imageName string) error {
	_, err := d.pool.Exec(ctx,
		`INSERT INTO inference_results (id, image_name, status) VALUES ($1, $2, 'pending')`,
		imageID, imageName,
	)
	return err
}

func (d *DB) UpdateStatus(ctx context.Context, imageID string, status string, detections interface{}) error {
	var detectionsJSON []byte
	var err error

	if detections != nil {
		detectionsJSON, err = json.Marshal(detections)
		if err != nil {
			return err
		}
	}

	_, err = d.pool.Exec(ctx,
		`UPDATE inference_results SET status = $1, detections = $2, updated_at = now() WHERE id = $3`,
		status, detectionsJSON, imageID,
	)
	return err
}

type InferenceResult struct {
	ID         string          `json:"image_id"`
	ImageName  string          `json:"image_name"`
	Status     string          `json:"status"`
	Detections json.RawMessage `json:"detections,omitempty"`
}

func (d *DB) GetResult(ctx context.Context, imageID string) (*InferenceResult, error) {
	var r InferenceResult
	err := d.pool.QueryRow(ctx,
		`SELECT id, image_name, status, COALESCE(detections, 'null') FROM inference_results WHERE id = $1`,
		imageID,
	).Scan(&r.ID, &r.ImageName, &r.Status, &r.Detections)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
