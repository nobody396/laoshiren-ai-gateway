-- Durable, non-sensitive hourly checkpoints for OpenAI route observations.
-- Redis remains the low-latency recent window, while this table is the
-- authoritative source for the seven-day and seasonal evidence windows.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS openai_route_observation_hourly (
  route_fingerprint VARCHAR(32) NOT NULL,
  hour_start TIMESTAMPTZ NOT NULL,

  group_id BIGINT NOT NULL CHECK (group_id > 0),
  account_id BIGINT NOT NULL CHECK (account_id > 0),
  failure_domain TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  request_class VARCHAR(16) NOT NULL CHECK (request_class IN ('text', 'image')),
  endpoint_hash TEXT NOT NULL,
  transport TEXT NOT NULL,

  metrics JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metrics) = 'object'),
  actual_base_cost_usd NUMERIC(30, 12) NOT NULL DEFAULT 0 CHECK (actual_base_cost_usd >= 0),
  actual_account_cost_usd NUMERIC(30, 12) NOT NULL DEFAULT 0 CHECK (actual_account_cost_usd >= 0),
  last_observed_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  PRIMARY KEY (route_fingerprint, hour_start),
  CHECK (route_fingerprint ~ '^[0-9a-f]{32}$'),
  CHECK (last_observed_at >= hour_start),
  CHECK (last_observed_at < hour_start + INTERVAL '1 hour')
);

CREATE INDEX IF NOT EXISTS idx_openai_route_observation_hourly_hour_start
  ON openai_route_observation_hourly (hour_start DESC);

COMMENT ON TABLE openai_route_observation_hourly IS
  'Non-sensitive hourly aggregate checkpoints for adaptive OpenAI routing; no URL, prompt, response, credential, or user identity.';

COMMENT ON COLUMN openai_route_observation_hourly.metrics IS
  'Sparse integer counters and histogram buckets. Keys are internal metric names only.';
