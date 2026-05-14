-- Ops error logs: add endpoint, model mapping, and request_type observability fields.

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS inbound_endpoint VARCHAR(256),
    ADD COLUMN IF NOT EXISTS upstream_endpoint VARCHAR(256);

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS requested_model VARCHAR(100),
    ADD COLUMN IF NOT EXISTS upstream_model VARCHAR(100);

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS request_type SMALLINT;
