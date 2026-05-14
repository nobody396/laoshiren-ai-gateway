-- Migration: 082_add_group_messages_dispatch_model_config
-- Add fine-grained OpenAI /v1/messages dispatch model config to groups.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS messages_dispatch_model_config JSONB NOT NULL DEFAULT '{}'::jsonb;
