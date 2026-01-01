-- +goose Up
-- Migration: 008_vision_models_table
-- Description: Create vision_models table for tracking model configurations

CREATE TABLE IF NOT EXISTS vision_models (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    model_type VARCHAR(50) NOT NULL, -- 'vision' or 'generation'
    backend VARCHAR(50) NOT NULL, -- 'llamacpp', 'ollama', 'comfyui'
    model_path VARCHAR(500),
    mmproj_path VARCHAR(500), -- For vision models (multimodal projector)
    hardware_target VARCHAR(50), -- 'rtx5090', 'mac64', etc.
    context_size INTEGER,
    default_params JSONB, -- Default generation parameters
    capabilities JSONB, -- Supported features/operations
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_vision_models_type ON vision_models(model_type);
CREATE INDEX IF NOT EXISTS idx_vision_models_backend ON vision_models(backend);
CREATE INDEX IF NOT EXISTS idx_vision_models_active ON vision_models(is_active);

-- Comment: Model types:
-- 'vision' - Image understanding models (Qwen2-VL, LLaVA, Llama 3.2 Vision)
-- 'generation' - Image generation models (SDXL, Flux.1)

-- +goose Down
DROP TABLE IF EXISTS vision_models;
