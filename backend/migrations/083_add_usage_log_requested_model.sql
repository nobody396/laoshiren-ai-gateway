-- Migration: 083_add_usage_log_requested_model
-- Add requested_model for stable requested/upstream model tracking.

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS requested_model VARCHAR(100);
