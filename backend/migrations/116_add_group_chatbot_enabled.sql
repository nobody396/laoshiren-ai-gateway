-- Add per-group Chatbot exposure switch.
-- Only groups with chatbot_enabled=true can show Chatbot actions and be selected in chatbot.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS chatbot_enabled BOOLEAN NOT NULL DEFAULT FALSE;
