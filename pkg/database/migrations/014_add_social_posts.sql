-- +goose Up
-- Add social_posts column to blog_posts table
ALTER TABLE blog_posts ADD COLUMN IF NOT EXISTS social_posts TEXT; -- JSON blob

-- +goose Down
-- Remove social_posts column from blog_posts table
ALTER TABLE blog_posts DROP COLUMN IF EXISTS social_posts;
