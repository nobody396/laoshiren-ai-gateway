-- Persist collector completeness across graceful application restarts. These
-- rows contain only aggregate operational counters; no customer identity,
-- prompt, response, credential, or raw upstream URL is stored.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS openai_route_evidence_epochs (
  epoch_id VARCHAR(64) PRIMARY KEY,
  instance_id VARCHAR(64) NOT NULL,
  component VARCHAR(32) NOT NULL CHECK (component IN ('audit', 'observation')),
  started_at TIMESTAMPTZ NOT NULL,
  heartbeat_at TIMESTAMPTZ NOT NULL,
  stopped_at TIMESTAMPTZ,
  clean_shutdown BOOLEAN NOT NULL DEFAULT FALSE,

  attempted BIGINT NOT NULL DEFAULT 0 CHECK (attempted >= 0),
  written BIGINT NOT NULL DEFAULT 0 CHECK (written >= 0),
  failed BIGINT NOT NULL DEFAULT 0 CHECK (failed >= 0),
  dropped BIGINT NOT NULL DEFAULT 0 CHECK (dropped >= 0),
  rejected BIGINT NOT NULL DEFAULT 0 CHECK (rejected >= 0),
  outcome_expected BIGINT NOT NULL DEFAULT 0 CHECK (outcome_expected >= 0),
  outcome_applied BIGINT NOT NULL DEFAULT 0 CHECK (outcome_applied >= 0),
  outcome_failed BIGINT NOT NULL DEFAULT 0 CHECK (outcome_failed >= 0),
  storage_checks BIGINT NOT NULL DEFAULT 0 CHECK (storage_checks >= 0),
  storage_failed BIGINT NOT NULL DEFAULT 0 CHECK (storage_failed >= 0),
  last_error VARCHAR(512) NOT NULL DEFAULT '',

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CHECK (heartbeat_at >= started_at),
  CHECK (stopped_at IS NULL OR stopped_at >= started_at),
  CHECK (NOT clean_shutdown OR stopped_at IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_openai_route_evidence_epochs_window
  ON openai_route_evidence_epochs (component, started_at, heartbeat_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_openai_route_evidence_epochs_instance_component
  ON openai_route_evidence_epochs (instance_id, component);

COMMENT ON TABLE openai_route_evidence_epochs IS
  'Durable per-process collector epochs used to prove audit and observation completeness across graceful releases.';
