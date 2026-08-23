-- Shadow-only Compensation Control. This migration has no balance,
-- entitlement, notification, email, approval, or execution write path.

SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='2min';

CREATE TABLE IF NOT EXISTS compensation_policy_versions (
    version INTEGER PRIMARY KEY,
    final_failure_threshold INTEGER NOT NULL,
    customer_relationship_cap_bps INTEGER NOT NULL,
    high_value_threshold_bps INTEGER NOT NULL,
    rolling_window_days INTEGER NOT NULL,
    standard_rolling_fixed_cap_cny_fen BIGINT NOT NULL,
    standard_rolling_paid_value_bps INTEGER NOT NULL,
    priority_rolling_fixed_cap_cny_fen BIGINT NOT NULL,
    priority_rolling_paid_value_bps INTEGER NOT NULL,
    shadow_minimum_days INTEGER NOT NULL,
    shadow_minimum_incidents INTEGER NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compensation_policy_bps_check CHECK(customer_relationship_cap_bps BETWEEN 1 AND 10000 AND high_value_threshold_bps BETWEEN 1 AND 10000 AND standard_rolling_paid_value_bps BETWEEN 1 AND 10000 AND priority_rolling_paid_value_bps BETWEEN 1 AND 10000),
    CONSTRAINT compensation_policy_shadow_gate_check CHECK(shadow_minimum_days>=30 AND shadow_minimum_incidents>=3)
);
INSERT INTO compensation_policy_versions VALUES(1,4,1000,1000,30,2000,2000,15000,3000,30,3,'2026-08-23T00:00:00Z',NOW()) ON CONFLICT(version) DO NOTHING;

CREATE TABLE IF NOT EXISTS compensation_product_rate_versions (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES service_status_products(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    rate_cny_fen_per_hour BIGINT NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id,version),
    CONSTRAINT compensation_product_rate_positive CHECK(rate_cny_fen_per_hour>0)
);
INSERT INTO compensation_product_rate_versions(product_id,version,rate_cny_fen_per_hour,effective_from)
SELECT id,1,3000,'2026-08-23T00:00:00Z' FROM service_status_products ON CONFLICT(product_id,version) DO NOTHING;

CREATE TABLE IF NOT EXISTS compensation_group_weight_versions (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    weight_bps INTEGER NOT NULL,
    source_rate_multiplier NUMERIC(10,4) NOT NULL,
    benefit_channel VARCHAR(24) NOT NULL,
    group_display_name VARCHAR(100) NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(group_id,version),
    CONSTRAINT compensation_group_weight_positive CHECK(weight_bps>0),
    CONSTRAINT compensation_group_weight_channel_check CHECK(benefit_channel IN('balance','builder_pass'))
);
INSERT INTO compensation_group_weight_versions(group_id,version,weight_bps,source_rate_multiplier,benefit_channel,group_display_name,effective_from)
SELECT id,1,GREATEST(1,ROUND(rate_multiplier*10000)::integer),rate_multiplier,CASE WHEN subscription_type='credit' THEN 'builder_pass' ELSE 'balance' END,name,NOW() FROM groups ON CONFLICT(group_id,version) DO NOTHING;

CREATE OR REPLACE FUNCTION snapshot_compensation_group_weight() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE next_version integer;
BEGIN
  PERFORM pg_advisory_xact_lock(NEW.id);
  SELECT COALESCE(MAX(version),0)+1 INTO next_version FROM compensation_group_weight_versions WHERE group_id=NEW.id;
  INSERT INTO compensation_group_weight_versions(group_id,version,weight_bps,source_rate_multiplier,benefit_channel,group_display_name,effective_from)
  VALUES(NEW.id,next_version,GREATEST(1,ROUND(NEW.rate_multiplier*10000)::integer),NEW.rate_multiplier,CASE WHEN NEW.subscription_type='credit' THEN 'builder_pass' ELSE 'balance' END,NEW.name,NOW());
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_groups_snapshot_compensation_weight ON groups;
CREATE TRIGGER trg_groups_snapshot_compensation_weight AFTER INSERT OR UPDATE OF rate_multiplier,subscription_type,name ON groups
FOR EACH ROW EXECUTE FUNCTION snapshot_compensation_group_weight();

CREATE TABLE IF NOT EXISTS compensation_shadow_periods (
    id BIGSERIAL PRIMARY KEY,
    activation_id UUID NOT NULL UNIQUE,
    policy_version INTEGER NOT NULL REFERENCES compensation_policy_versions(version) ON DELETE RESTRICT,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_by_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compensation_shadow_period_window CHECK(ended_at IS NULL OR ended_at>=started_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_compensation_shadow_one_active ON compensation_shadow_periods((TRUE)) WHERE ended_at IS NULL;

CREATE TABLE IF NOT EXISTS compensation_drafts (
    id BIGSERIAL PRIMARY KEY,
    series_id UUID NOT NULL,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE RESTRICT,
    shadow_period_id BIGINT NOT NULL REFERENCES compensation_shadow_periods(id) ON DELETE RESTRICT,
    policy_version INTEGER NOT NULL REFERENCES compensation_policy_versions(version) ON DELETE RESTRICT,
    revision_number INTEGER NOT NULL,
    replaces_draft_id BIGINT REFERENCES compensation_drafts(id) ON DELETE RESTRICT,
    state VARCHAR(32) NOT NULL,
    revision_reason VARCHAR(500) NOT NULL DEFAULT '',
    created_by_user_id BIGINT,
    redesigned BOOLEAN NOT NULL DEFAULT FALSE,
    eligible_user_count INTEGER NOT NULL,
    affected_user_count INTEGER NOT NULL,
    proposed_total_cny_fen BIGINT NOT NULL,
    affected_product_rolling_paid_value_cny_fen BIGINT NOT NULL,
    high_value_threshold_cny_fen BIGINT NOT NULL,
    high_value BOOLEAN NOT NULL,
    evidence_hash CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(series_id,revision_number),
    UNIQUE(incident_id,revision_number),
    CONSTRAINT compensation_draft_state_check CHECK(state IN('shadow','redesign_required')),
    CONSTRAINT compensation_draft_values_check CHECK(revision_number>0 AND eligible_user_count>=0 AND affected_user_count>=0 AND proposed_total_cny_fen>=0 AND affected_product_rolling_paid_value_cny_fen>=0 AND high_value_threshold_cny_fen>=0),
    CONSTRAINT compensation_draft_hash_check CHECK(evidence_hash~'^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_compensation_drafts_incident_revision ON compensation_drafts(incident_id,revision_number DESC,id DESC);

CREATE TABLE IF NOT EXISTS compensation_draft_users (
    id BIGSERIAL PRIMARY KEY,
    draft_id BIGINT NOT NULL REFERENCES compensation_drafts(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    tier_snapshot_id BIGINT REFERENCES customer_tier_incident_snapshots(id) ON DELETE RESTRICT,
    eligible BOOLEAN NOT NULL,
    included BOOLEAN NOT NULL,
    evidence_complete BOOLEAN NOT NULL,
    final_failure_count INTEGER NOT NULL,
    first_qualifying_failure_at TIMESTAMPTZ,
    verified_paid_value_cny_fen BIGINT NOT NULL,
    rolling_goodwill_executed_cny_fen BIGINT NOT NULL DEFAULT 0,
    raw_value_cny_micros BIGINT NOT NULL,
    tier_cap_cny_fen BIGINT NOT NULL,
    relationship_cap_cny_fen BIGINT NOT NULL,
    rolling_cap_cny_fen BIGINT,
    rolling_remaining_cny_fen BIGINT,
    proposed_total_cny_fen BIGINT NOT NULL,
    balance_benefit_cny_fen BIGINT NOT NULL,
    builder_pass_benefit_cny_fen BIGINT NOT NULL,
    limiting_cap VARCHAR(32) NOT NULL,
    exclusion_reason VARCHAR(100) NOT NULL DEFAULT '',
    evidence JSONB NOT NULL,
    evidence_hash CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(draft_id,user_id),
    CONSTRAINT compensation_draft_user_values_check CHECK(final_failure_count>=0 AND verified_paid_value_cny_fen>=0 AND rolling_goodwill_executed_cny_fen>=0 AND raw_value_cny_micros>=0 AND tier_cap_cny_fen>=0 AND relationship_cap_cny_fen>=0 AND proposed_total_cny_fen>=0 AND balance_benefit_cny_fen>=0 AND builder_pass_benefit_cny_fen>=0),
    CONSTRAINT compensation_draft_user_hash_check CHECK(evidence_hash~'^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_compensation_draft_users_user ON compensation_draft_users(user_id,draft_id DESC);

CREATE TABLE IF NOT EXISTS compensation_draft_items (
    id BIGSERIAL PRIMARY KEY,
    draft_id BIGINT NOT NULL REFERENCES compensation_drafts(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    product_id BIGINT NOT NULL REFERENCES service_status_products(id) ON DELETE RESTRICT,
    group_id BIGINT REFERENCES groups(id) ON DELETE RESTRICT,
    benefit_channel VARCHAR(24) NOT NULL,
    first_qualifying_failure_at TIMESTAMPTZ NOT NULL,
    compensable_duration_ms BIGINT NOT NULL,
    product_rate_version_id BIGINT REFERENCES compensation_product_rate_versions(id) ON DELETE RESTRICT,
    group_weight_version_id BIGINT REFERENCES compensation_group_weight_versions(id) ON DELETE RESTRICT,
    tier_multiplier_bps INTEGER NOT NULL,
    raw_value_cny_micros BIGINT NOT NULL,
    proposed_cny_fen BIGINT NOT NULL,
    exclusion_reason VARCHAR(100) NOT NULL DEFAULT '',
    evidence JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT(draft_id,user_id,product_id,group_id),
    CONSTRAINT compensation_draft_item_channel_check CHECK(benefit_channel IN('balance','builder_pass')),
    CONSTRAINT compensation_draft_item_values_check CHECK(compensable_duration_ms>=0 AND tier_multiplier_bps>=0 AND raw_value_cny_micros>=0 AND proposed_cny_fen>=0)
);
CREATE INDEX IF NOT EXISTS idx_compensation_draft_items_draft_user ON compensation_draft_items(draft_id,user_id,id);

CREATE TABLE IF NOT EXISTS compensation_evidence_snapshots (
    id BIGSERIAL PRIMARY KEY,
    draft_id BIGINT NOT NULL REFERENCES compensation_drafts(id) ON DELETE RESTRICT,
    user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    snapshot_kind VARCHAR(24) NOT NULL,
    source_identifier_count INTEGER NOT NULL,
    payload JSONB NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    retention_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT(draft_id,snapshot_kind,user_id),
    CONSTRAINT compensation_evidence_kind_check CHECK(snapshot_kind IN('draft','user')),
    CONSTRAINT compensation_evidence_hash_check CHECK(payload_hash~'^[0-9a-f]{64}$'),
    CONSTRAINT compensation_evidence_retention_check CHECK(retention_until>=created_at+INTERVAL '3 years')
);

CREATE TABLE IF NOT EXISTS compensation_shadow_reviews (
    id BIGSERIAL PRIMARY KEY,
    draft_id BIGINT NOT NULL UNIQUE REFERENCES compensation_drafts(id) ON DELETE RESTRICT,
    owner_judgement_total_cny_fen BIGINT NOT NULL,
    variance_cny_fen BIGINT NOT NULL,
    result VARCHAR(24) NOT NULL,
    notes VARCHAR(1000) NOT NULL,
    created_by_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compensation_shadow_review_result_check CHECK(result IN('aligned','redesign_required')),
    CONSTRAINT compensation_shadow_review_value_check CHECK(owner_judgement_total_cny_fen>=0)
);

CREATE TABLE IF NOT EXISTS compensation_draft_failures (
    shadow_period_id BIGINT NOT NULL REFERENCES compensation_shadow_periods(id) ON DELETE CASCADE,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    error_message VARCHAR(1000) NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    PRIMARY KEY(shadow_period_id,incident_id)
);

CREATE OR REPLACE FUNCTION prevent_compensation_shadow_evidence_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'compensation shadow evidence is immutable' USING ERRCODE='23514',CONSTRAINT='compensation_shadow_evidence_immutable'; END; $$;

DO $$ DECLARE table_name text; BEGIN
  FOREACH table_name IN ARRAY ARRAY['compensation_policy_versions','compensation_product_rate_versions','compensation_group_weight_versions','compensation_drafts','compensation_draft_users','compensation_draft_items','compensation_evidence_snapshots','compensation_shadow_reviews'] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%s_immutable ON %I',table_name,table_name);
    EXECUTE format('CREATE TRIGGER trg_%s_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION prevent_compensation_shadow_evidence_mutation()',table_name,table_name);
  END LOOP;
END $$;

INSERT INTO settings(key,value,updated_at) VALUES('compensation_shadow_draft_enabled','false',NOW()) ON CONFLICT(key) DO NOTHING;
