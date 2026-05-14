-- Add upstream_model field to usage_logs.
-- Stores the actual upstream model after mapping for admin analytics.

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS upstream_model VARCHAR(100);
