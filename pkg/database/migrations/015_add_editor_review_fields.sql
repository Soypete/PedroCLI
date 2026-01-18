-- +goose Up
-- Add editor review workflow fields to blog_posts table
ALTER TABLE blog_posts ADD COLUMN IF NOT EXISTS editor_applied_content TEXT;
ALTER TABLE blog_posts ADD COLUMN IF NOT EXISTS editor_diff TEXT;

-- Update status column comment to include new statuses
COMMENT ON COLUMN blog_posts.status IS 'dictated, drafted, edited, editor_applied, approved, rejected_edits, published, public';

-- +goose Down
ALTER TABLE blog_posts DROP COLUMN IF EXISTS editor_diff;
ALTER TABLE blog_posts DROP COLUMN IF EXISTS editor_applied_content;
