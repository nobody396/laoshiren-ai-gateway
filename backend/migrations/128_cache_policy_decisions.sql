-- Claude / Anthropic prompt caching policy decisions.
-- Non-sensitive: stores only routing, timing, token, TTL, and policy metadata.

CREATE TABLE IF NOT EXISTS cache_policy_decisions (
  id BIGSERIAL PRIMARY KEY,
  request_id VARCHAR(128) NOT NULL DEFAULT '',
  client_request_id VARCHAR(128) NOT NULL DEFAULT '',
  user_id BIGINT,
  api_key_id BIGINT,
  group_id BIGINT,
  account_id BIGINT,
  client_type VARCHAR(64) NOT NULL DEFAULT 'unknown',
  model VARCHAR(255) NOT NULL DEFAULT '',
  policy_mode VARCHAR(32) NOT NULL DEFAULT 'safe_5m',
  policy_version VARCHAR(32) NOT NULL DEFAULT 'v1',
  actual_ttl VARCHAR(8) NOT NULL DEFAULT '5m',
  shadow_ttl VARCHAR(8) NOT NULL DEFAULT '',
  decision_reason TEXT NOT NULL DEFAULT '',
  cache_control_paths_count INTEGER NOT NULL DEFAULT 0,
  normalized BOOLEAN NOT NULL DEFAULT FALSE,
  downgraded BOOLEAN NOT NULL DEFAULT FALSE,
  retried BOOLEAN NOT NULL DEFAULT FALSE,
  duration_ms INTEGER,
  first_token_ms INTEGER,
  cache_creation_5m_tokens INTEGER NOT NULL DEFAULT 0,
  cache_creation_1h_tokens INTEGER NOT NULL DEFAULT 0,
  cache_read_tokens INTEGER NOT NULL DEFAULT 0,
  cost NUMERIC(18,8) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cache_policy_decisions_created_at
  ON cache_policy_decisions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_cache_policy_decisions_request_id
  ON cache_policy_decisions (request_id);

CREATE INDEX IF NOT EXISTS idx_cache_policy_decisions_user_created
  ON cache_policy_decisions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_cache_policy_decisions_group_created
  ON cache_policy_decisions (group_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_cache_policy_decisions_account_created
  ON cache_policy_decisions (account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_cache_policy_decisions_policy_created
  ON cache_policy_decisions (policy_mode, created_at DESC);

CREATE TABLE IF NOT EXISTS cache_policy_daily_reports (
  id BIGSERIAL PRIMARY KEY,
  report_date DATE NOT NULL UNIQUE,
  phase VARCHAR(32) NOT NULL DEFAULT 'phase0_protection',
  policy_version VARCHAR(32) NOT NULL DEFAULT 'v1',
  client_profiles JSONB NOT NULL DEFAULT '{}'::jsonb,
  group_performance JSONB NOT NULL DEFAULT '{}'::jsonb,
  recommended_thresholds JSONB NOT NULL DEFAULT '{}'::jsonb,
  auto_action VARCHAR(64) NOT NULL DEFAULT 'none',
  auto_action_reason TEXT NOT NULL DEFAULT '',
  risk_metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
  ttl_order_400_count INTEGER NOT NULL DEFAULT 0,
  adaptive_enabled_ratio NUMERIC(8,4) NOT NULL DEFAULT 0,
  cost_delta_pct NUMERIC(8,4) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cache_policy_daily_reports_created_at
  ON cache_policy_daily_reports (created_at DESC);

INSERT INTO settings (key, value, updated_at)
VALUES
  ('anthropic_cache_policy_mode', 'safe_5m', NOW()),
  ('anthropic_cache_policy_version', 'v1', NOW()),
  ('anthropic_cache_policy_phase', 'phase0_protection', NOW()),
  ('anthropic_cache_policy_token_threshold', '40000', NOW()),
  ('anthropic_cache_policy_interval_threshold_minutes', '5', NOW())
ON CONFLICT (key) DO NOTHING;
