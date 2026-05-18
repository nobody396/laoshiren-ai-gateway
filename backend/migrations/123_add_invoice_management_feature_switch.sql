INSERT INTO settings (key, value, updated_at)
VALUES ('invoice_management_enabled', 'false', NOW())
ON CONFLICT (key) DO NOTHING;
