-- Migration: 004_prompt_templates_table
-- Description: Create prompt_templates table for image generation prompts
-- Created: 2025-01-01

CREATE TABLE IF NOT EXISTS prompt_templates (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    category VARCHAR(50) NOT NULL, -- 'blog_hero', 'technical_diagram', etc.
    base_prompt TEXT NOT NULL,
    style_modifiers JSONB, -- Additional style keywords
    recommended_model VARCHAR(100),
    recommended_params JSONB, -- CFG scale, steps, sampler, etc.
    aspect_ratio VARCHAR(20) DEFAULT '16:9',
    default_width INTEGER DEFAULT 1024,
    default_height INTEGER DEFAULT 576,
    alt_text_instructions TEXT, -- Instructions for alt text generation
    example_outputs JSONB, -- Links to example generated images
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_prompt_templates_category ON prompt_templates(category);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_active ON prompt_templates(is_active);

-- Comment: Categories include:
-- 'blog_hero_image' - Hero images for blog posts
-- 'technical_diagram' - Technical/architecture diagrams
-- 'code_visualization' - Code concept visualizations
-- 'social_preview' - Social media preview images (OG images)
-- 'newsletter_header' - Email newsletter header images
