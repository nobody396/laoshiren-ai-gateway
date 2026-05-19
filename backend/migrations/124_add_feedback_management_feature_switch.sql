INSERT INTO settings (key, value, updated_at)
VALUES ('feedback_management_enabled', 'true', NOW())
ON CONFLICT (key) DO NOTHING;
