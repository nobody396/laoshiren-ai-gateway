-- Give every separately authorized Shadow run an immutable identity and exact
-- T0. Historical rows remain explicitly unassigned and cannot satisfy future
-- activation-scoped promotion gates.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE openai_route_shadow_decisions
  ADD COLUMN IF NOT EXISTS activation_id VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS shadow_started_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_activation_scope
  ON openai_route_shadow_decisions (
    activation_id,
    group_id,
    model,
    request_class,
    policy_version,
    created_at DESC
  );

COMMENT ON COLUMN openai_route_shadow_decisions.activation_id IS
  'Immutable operator-approved identity for one Shadow enablement cycle; empty only for historical or invalid-policy evidence.';

COMMENT ON COLUMN openai_route_shadow_decisions.shadow_started_at IS
  'Exact UTC T0 recorded by the authorized Shadow policy; NULL only for historical or invalid-policy evidence.';
