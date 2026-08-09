-- Persist every enabled OpenAI shadow-routing evaluation. This table is
-- append-only operational evidence: it intentionally stores no prompt,
-- response body, credential, account name, or raw upstream URL.

CREATE TABLE IF NOT EXISTS openai_route_shadow_decisions (
  id BIGSERIAL PRIMARY KEY,
  decision_id VARCHAR(64) NOT NULL UNIQUE,
  request_id VARCHAR(128) NOT NULL DEFAULT '',
  client_request_id VARCHAR(128) NOT NULL DEFAULT '',
  attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt > 0),
  group_id BIGINT NOT NULL CHECK (group_id > 0),
  model VARCHAR(128) NOT NULL,
  policy_mode VARCHAR(16) NOT NULL,
  policy_version INTEGER NOT NULL DEFAULT 0,
  reason VARCHAR(64) NOT NULL,
  evaluated BOOLEAN NOT NULL DEFAULT FALSE,
  evaluation_duration_us BIGINT NOT NULL DEFAULT 0 CHECK (evaluation_duration_us >= 0),
  legacy_selected_account_id BIGINT,
  adaptive_selected_account_id BIGINT,
  adaptive_selected_rate NUMERIC(10,4),
  candidate_count INTEGER NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
  excluded_count INTEGER NOT NULL DEFAULT 0 CHECK (excluded_count >= 0),
  diverged BOOLEAN NOT NULL DEFAULT FALSE,
  emergency BOOLEAN NOT NULL DEFAULT FALSE,
  snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_created_at
  ON openai_route_shadow_decisions (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_request_id
  ON openai_route_shadow_decisions (request_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_client_request_id
  ON openai_route_shadow_decisions (client_request_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_group_model_created
  ON openai_route_shadow_decisions (group_id, model, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_policy_created
  ON openai_route_shadow_decisions (policy_version, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_diverged_created
  ON openai_route_shadow_decisions (created_at DESC)
  WHERE diverged = TRUE;

COMMENT ON TABLE openai_route_shadow_decisions IS
  'Append-only, non-sensitive audit evidence for enabled OpenAI shadow-routing evaluations.';
