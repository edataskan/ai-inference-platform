-- Faz 1'de kullanılacak ana tablo

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- gen_random_uuid() için

CREATE TABLE IF NOT EXISTS inference_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    image_name TEXT NOT NULL,
    detections JSONB,
    status TEXT NOT NULL DEFAULT 'pending', -- pending | processing | done | failed
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inference_results_status ON inference_results(status);
