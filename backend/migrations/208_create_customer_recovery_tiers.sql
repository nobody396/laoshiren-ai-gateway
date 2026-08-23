-- Auditable Customer Tier policy, daily evidence, immutable history,
-- time-bounded overrides, mutable current projection, and Incident snapshots.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS customer_tier_policy_versions (
    id BIGSERIAL PRIMARY KEY,
    version INTEGER NOT NULL UNIQUE,
    rolling_window_days INTEGER NOT NULL,
    downgrade_grace_days INTEGER NOT NULL,
    priority_threshold_cny_fen BIGINT NOT NULL,
    strategic_threshold_cny_fen BIGINT NOT NULL,
    standard_multiplier NUMERIC(8,4) NOT NULL,
    priority_multiplier NUMERIC(8,4) NOT NULL,
    strategic_multiplier NUMERIC(8,4) NOT NULL,
    standard_cap_cny_fen BIGINT NOT NULL,
    priority_cap_cny_fen BIGINT NOT NULL,
    strategic_cap_cny_fen BIGINT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_tier_policy_thresholds_check CHECK (
        priority_threshold_cny_fen > 0 AND strategic_threshold_cny_fen > priority_threshold_cny_fen
    )
);

INSERT INTO customer_tier_policy_versions(
 version,rolling_window_days,downgrade_grace_days,
 priority_threshold_cny_fen,strategic_threshold_cny_fen,
 standard_multiplier,priority_multiplier,strategic_multiplier,
 standard_cap_cny_fen,priority_cap_cny_fen,strategic_cap_cny_fen,effective_from
) VALUES(1,90,30,25000,100000,1.0,1.25,1.5,1000,5000,20000,'2026-08-23T00:00:00Z')
ON CONFLICT(version) DO NOTHING;

CREATE TABLE IF NOT EXISTS customer_tier_overrides (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    tier VARCHAR(16) NOT NULL,
    reason VARCHAR(500) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_by_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_tier_override_tier_check CHECK (tier IN ('standard','priority','strategic')),
    CONSTRAINT customer_tier_override_window_check CHECK (expires_at > starts_at)
);

CREATE TABLE IF NOT EXISTS customer_tier_override_refreshes (
    override_id BIGINT PRIMARY KEY REFERENCES customer_tier_overrides(id) ON DELETE RESTRICT,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error VARCHAR(1000),
    applied_evaluation_id BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_tier_override_refresh_status_check CHECK(status IN ('pending','applied','failed'))
);

-- Structured reversals are the only supported way to remove refunded cash from
-- Verified Paid Value.  Notes and free-text finance ledgers are deliberately
-- not consulted because they cannot prove which paid source was reversed.
CREATE TABLE IF NOT EXISTS customer_paid_value_refunds (
    id BIGSERIAL PRIMARY KEY,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT NOT NULL,
    amount_cny_fen BIGINT NOT NULL,
    reason VARCHAR(500) NOT NULL,
    created_by_user_id BIGINT,
    refunded_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_paid_value_refunds_source_check CHECK (source_type IN ('topup_order','payment_order','native_checkout_order','redeem_code')),
    CONSTRAINT customer_paid_value_refunds_amount_check CHECK (amount_cny_fen > 0)
);

CREATE INDEX IF NOT EXISTS idx_customer_paid_value_refunds_source
    ON customer_paid_value_refunds(user_id,source_type,source_id,refunded_at,id);

CREATE TABLE IF NOT EXISTS monthly_entitlement_consumptions (
    id BIGSERIAL PRIMARY KEY,
    cycle_id BIGINT NOT NULL REFERENCES monthly_entitlement_cycles(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    usage_log_id BIGINT,
    usage_event_key VARCHAR(180) NOT NULL UNIQUE,
    confirmed_amount_micros BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monthly_entitlement_consumption_amount_check CHECK (confirmed_amount_micros > 0)
);

CREATE INDEX IF NOT EXISTS idx_monthly_entitlement_consumptions_user_time
    ON monthly_entitlement_consumptions(user_id,created_at,id);

CREATE INDEX IF NOT EXISTS idx_customer_tier_overrides_active
    ON customer_tier_overrides(user_id,starts_at,expires_at,id DESC);

CREATE TABLE IF NOT EXISTS customer_tier_evaluations (
    id BIGSERIAL PRIMARY KEY,
    evaluation_key VARCHAR(180) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    policy_version INTEGER NOT NULL REFERENCES customer_tier_policy_versions(version) ON DELETE RESTRICT,
    window_started_at TIMESTAMPTZ NOT NULL,
    window_ended_at TIMESTAMPTZ NOT NULL,
    verified_paid_value_cny_fen BIGINT NOT NULL,
    verified_paid_consumption_micros BIGINT NOT NULL DEFAULT 0,
    calculated_tier VARCHAR(16) NOT NULL,
    effective_tier VARCHAR(16) NOT NULL,
    resolution_reason VARCHAR(64) NOT NULL,
    override_id BIGINT REFERENCES customer_tier_overrides(id) ON DELETE RESTRICT,
    grace_expires_at TIMESTAMPTZ,
    evidence JSONB NOT NULL,
    policy_snapshot JSONB NOT NULL,
    evidence_hash CHAR(64) NOT NULL,
    evaluated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_tier_evaluation_tier_check CHECK (
        calculated_tier IN ('standard','priority','strategic') AND effective_tier IN ('standard','priority','strategic')
    ),
    CONSTRAINT customer_tier_evaluation_window_check CHECK (window_ended_at > window_started_at),
    CONSTRAINT customer_tier_evaluation_values_check CHECK (
        verified_paid_value_cny_fen >= 0 AND verified_paid_consumption_micros >= 0
    ),
    CONSTRAINT customer_tier_evaluation_hash_check CHECK (evidence_hash ~ '^[0-9a-f]{64}$')
);

CREATE INDEX IF NOT EXISTS idx_customer_tier_evaluations_user_time
    ON customer_tier_evaluations(user_id,evaluated_at DESC,id DESC);

CREATE TABLE IF NOT EXISTS customer_tier_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    evaluation_id BIGINT NOT NULL UNIQUE REFERENCES customer_tier_evaluations(id) ON DELETE RESTRICT,
    previous_tier VARCHAR(16),
    new_tier VARCHAR(16) NOT NULL,
    change_reason VARCHAR(64) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_tier_history_previous_check CHECK (previous_tier IS NULL OR previous_tier IN ('standard','priority','strategic')),
    CONSTRAINT customer_tier_history_new_check CHECK (new_tier IN ('standard','priority','strategic'))
);

CREATE INDEX IF NOT EXISTS idx_customer_tier_history_user_time
    ON customer_tier_history(user_id,changed_at DESC,id DESC);

CREATE TABLE IF NOT EXISTS customer_tier_current (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    policy_version INTEGER NOT NULL REFERENCES customer_tier_policy_versions(version) ON DELETE RESTRICT,
    calculated_tier VARCHAR(16) NOT NULL,
    effective_tier VARCHAR(16) NOT NULL,
    verified_paid_value_cny_fen BIGINT NOT NULL,
    verified_paid_consumption_micros BIGINT NOT NULL DEFAULT 0,
    grace_expires_at TIMESTAMPTZ,
    override_id BIGINT REFERENCES customer_tier_overrides(id) ON DELETE RESTRICT,
    last_evaluation_id BIGINT NOT NULL REFERENCES customer_tier_evaluations(id) ON DELETE RESTRICT,
    evaluated_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT customer_tier_current_tier_check CHECK (
        calculated_tier IN ('standard','priority','strategic') AND effective_tier IN ('standard','priority','strategic')
    ),
    CONSTRAINT customer_tier_current_values_check CHECK (
        verified_paid_value_cny_fen >= 0 AND verified_paid_consumption_micros >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_customer_tier_current_effective
    ON customer_tier_current(effective_tier,verified_paid_value_cny_fen DESC,user_id);

CREATE TABLE IF NOT EXISTS customer_tier_incident_snapshots (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    policy_version INTEGER NOT NULL REFERENCES customer_tier_policy_versions(version) ON DELETE RESTRICT,
    tier VARCHAR(16) NOT NULL,
    multiplier NUMERIC(8,4) NOT NULL,
    cap_cny_fen BIGINT NOT NULL,
    verified_paid_value_cny_fen BIGINT NOT NULL,
    verified_paid_consumption_micros BIGINT NOT NULL DEFAULT 0,
    override_id BIGINT REFERENCES customer_tier_overrides(id) ON DELETE RESTRICT,
    customer_impact_started_at TIMESTAMPTZ NOT NULL,
    evidence JSONB NOT NULL,
    policy_snapshot JSONB NOT NULL,
    evidence_hash CHAR(64) NOT NULL,
    snapshotted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(incident_id,user_id),
    CONSTRAINT customer_tier_incident_snapshot_tier_check CHECK (tier IN ('standard','priority','strategic')),
    CONSTRAINT customer_tier_incident_snapshot_values_check CHECK (
        cap_cny_fen >= 0 AND verified_paid_value_cny_fen >= 0 AND verified_paid_consumption_micros >= 0
    ),
    CONSTRAINT customer_tier_incident_snapshot_hash_check CHECK (evidence_hash ~ '^[0-9a-f]{64}$')
);

CREATE INDEX IF NOT EXISTS idx_customer_tier_incident_snapshots_user
    ON customer_tier_incident_snapshots(user_id,incident_id);

-- A daily run is resumable. Per-user failures are durable and do not prevent
-- later users or incident snapshot draining from progressing.
CREATE TABLE IF NOT EXISTS customer_tier_evaluator_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton),
    run_id UUID,
    cutoff_at TIMESTAMPTZ,
    last_user_id BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO customer_tier_evaluator_state(singleton) VALUES(TRUE) ON CONFLICT(singleton) DO NOTHING;

CREATE TABLE IF NOT EXISTS customer_tier_evaluation_failures (
    id BIGSERIAL PRIMARY KEY,
    run_id UUID NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    error_message VARCHAR(1000) NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    UNIQUE(run_id,user_id)
);
CREATE INDEX IF NOT EXISTS idx_customer_tier_evaluation_failures_open
    ON customer_tier_evaluation_failures(attempted_at DESC,user_id) WHERE resolved_at IS NULL;

CREATE OR REPLACE FUNCTION prevent_customer_tier_evidence_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'customer tier evidence is immutable'
        USING ERRCODE='23514', CONSTRAINT='customer_tier_evidence_immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_customer_tier_evaluations_immutable ON customer_tier_evaluations;
CREATE TRIGGER trg_customer_tier_evaluations_immutable BEFORE UPDATE OR DELETE ON customer_tier_evaluations
FOR EACH ROW EXECUTE FUNCTION prevent_customer_tier_evidence_mutation();
DROP TRIGGER IF EXISTS trg_customer_tier_history_immutable ON customer_tier_history;
CREATE TRIGGER trg_customer_tier_history_immutable BEFORE UPDATE OR DELETE ON customer_tier_history
FOR EACH ROW EXECUTE FUNCTION prevent_customer_tier_evidence_mutation();
DROP TRIGGER IF EXISTS trg_customer_tier_incident_snapshots_immutable ON customer_tier_incident_snapshots;
CREATE TRIGGER trg_customer_tier_incident_snapshots_immutable BEFORE UPDATE OR DELETE ON customer_tier_incident_snapshots
FOR EACH ROW EXECUTE FUNCTION prevent_customer_tier_evidence_mutation();
DROP TRIGGER IF EXISTS trg_customer_tier_overrides_immutable ON customer_tier_overrides;
CREATE TRIGGER trg_customer_tier_overrides_immutable BEFORE UPDATE OR DELETE ON customer_tier_overrides
FOR EACH ROW EXECUTE FUNCTION prevent_customer_tier_evidence_mutation();
DROP TRIGGER IF EXISTS trg_customer_tier_policy_versions_immutable ON customer_tier_policy_versions;
CREATE TRIGGER trg_customer_tier_policy_versions_immutable BEFORE UPDATE OR DELETE ON customer_tier_policy_versions
FOR EACH ROW EXECUTE FUNCTION prevent_customer_tier_evidence_mutation();
DROP TRIGGER IF EXISTS trg_customer_paid_value_refunds_immutable ON customer_paid_value_refunds;
CREATE TRIGGER trg_customer_paid_value_refunds_immutable BEFORE UPDATE OR DELETE ON customer_paid_value_refunds
FOR EACH ROW EXECUTE FUNCTION prevent_customer_tier_evidence_mutation();

INSERT INTO settings(key,value,updated_at)
VALUES('customer_tier_evaluation_enabled','false',NOW())
ON CONFLICT(key) DO NOTHING;
