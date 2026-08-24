-- Explicitly approved, resumable Compensation execution. Every runtime switch
-- remains disabled and approval is impossible until Shadow exit gates pass.

SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='2min';

CREATE TABLE IF NOT EXISTS compensation_draft_approvals (
    id BIGSERIAL PRIMARY KEY,
    draft_id BIGINT NOT NULL UNIQUE REFERENCES compensation_drafts(id) ON DELETE RESTRICT,
    draft_evidence_hash CHAR(64) NOT NULL,
    approved_preview_hash CHAR(64) NOT NULL,
    approval_key VARCHAR(180) NOT NULL UNIQUE,
    approval_reason VARCHAR(1000) NOT NULL,
    owner_confirmation BOOLEAN NOT NULL,
    approved_by_user_id BIGINT NOT NULL,
    approved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compensation_approval_hash_check CHECK(draft_evidence_hash~'^[0-9a-f]{64}$'),
    CONSTRAINT compensation_approval_preview_hash_check CHECK(approved_preview_hash~'^[0-9a-f]{64}$'),
    CONSTRAINT compensation_approval_owner_check CHECK(owner_confirmation=TRUE)
);

CREATE TABLE IF NOT EXISTS compensation_approval_notices (
    id BIGSERIAL PRIMARY KEY,
    approval_id BIGINT NOT NULL REFERENCES compensation_draft_approvals(id) ON DELETE RESTRICT,
    draft_user_id BIGINT NOT NULL UNIQUE REFERENCES compensation_draft_users(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    balance_cny_fen BIGINT NOT NULL,
    builder_pass_cny_fen BIGINT NOT NULL,
    body TEXT NOT NULL,
    payload JSONB NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(approval_id,user_id),
    CONSTRAINT compensation_approval_notice_amount_check CHECK(balance_cny_fen>=0 AND builder_pass_cny_fen>=0 AND balance_cny_fen+builder_pass_cny_fen>0),
    CONSTRAINT compensation_approval_notice_hash_check CHECK(payload_hash~'^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS compensation_execution_permission_audit (
    id BIGSERIAL PRIMARY KEY,
    enabled BOOLEAN NOT NULL,
    actor_user_id BIGINT NOT NULL,
    confirmation_hash CHAR(64) NOT NULL,
    assessment JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compensation_permission_hash_check CHECK(confirmation_hash~'^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS compensation_executions (
    id BIGSERIAL PRIMARY KEY,
    execution_key VARCHAR(180) NOT NULL UNIQUE,
    approval_id BIGINT NOT NULL REFERENCES compensation_draft_approvals(id) ON DELETE RESTRICT,
    draft_user_id BIGINT NOT NULL REFERENCES compensation_draft_users(id) ON DELETE RESTRICT,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    benefit_channel VARCHAR(24) NOT NULL,
    amount_cny_fen BIGINT NOT NULL,
    state VARCHAR(24) NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    asset_before NUMERIC(20,8),
    asset_after NUMERIC(20,8),
    applied_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    last_error VARCHAR(1000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(draft_user_id,benefit_channel),
    CONSTRAINT compensation_execution_channel_check CHECK(benefit_channel IN('balance','builder_pass')),
    CONSTRAINT compensation_execution_amount_check CHECK(amount_cny_fen>0),
    CONSTRAINT compensation_execution_state_check CHECK(state IN('pending','failed','applied','verified'))
);
CREATE INDEX IF NOT EXISTS idx_compensation_executions_approval_state ON compensation_executions(approval_id,state,id);
CREATE INDEX IF NOT EXISTS idx_compensation_executions_user_incident ON compensation_executions(user_id,incident_id,id);

CREATE TABLE IF NOT EXISTS compensation_execution_assets (
    id BIGSERIAL PRIMARY KEY,
    execution_id BIGINT NOT NULL REFERENCES compensation_executions(id) ON DELETE RESTRICT,
    group_id BIGINT REFERENCES groups(id) ON DELETE RESTRICT,
    amount_cny_fen BIGINT NOT NULL,
    asset_type VARCHAR(32) NOT NULL,
    asset_id BIGINT NOT NULL,
    benefit_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT(execution_id,group_id),
    CONSTRAINT compensation_execution_asset_type_check CHECK(asset_type IN('balance_lot','monthly_entitlement_cycle')),
    CONSTRAINT compensation_execution_asset_amount_check CHECK(amount_cny_fen>0)
);

CREATE TABLE IF NOT EXISTS compensation_execution_events (
    id BIGSERIAL PRIMARY KEY,
    execution_id BIGINT NOT NULL REFERENCES compensation_executions(id) ON DELETE RESTRICT,
    event_type VARCHAR(32) NOT NULL,
    state VARCHAR(24) NOT NULL,
    details JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_compensation_execution_events_execution ON compensation_execution_events(execution_id,id);

CREATE TABLE IF NOT EXISTS compensation_execution_receipts (
    id BIGSERIAL PRIMARY KEY,
    execution_id BIGINT NOT NULL UNIQUE REFERENCES compensation_executions(id) ON DELETE RESTRICT,
    payload JSONB NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT compensation_receipt_hash_check CHECK(payload_hash~'^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS compensation_notices (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    notification_id BIGINT NOT NULL UNIQUE REFERENCES user_notifications(id) ON DELETE RESTRICT,
    approval_notice_id BIGINT NOT NULL UNIQUE REFERENCES compensation_approval_notices(id) ON DELETE RESTRICT,
    payload JSONB NOT NULL,
    payload_hash CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(incident_id,user_id),
    CONSTRAINT compensation_notice_hash_check CHECK(payload_hash~'^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS erroneous_charge_refunds (
    id BIGSERIAL PRIMARY KEY,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    usage_log_id BIGINT NOT NULL UNIQUE REFERENCES usage_logs(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount_micros BIGINT NOT NULL,
    billing_type SMALLINT NOT NULL,
    asset_type VARCHAR(32) NOT NULL,
    asset_id BIGINT NOT NULL,
    reason VARCHAR(500) NOT NULL,
    created_by_user_id BIGINT NOT NULL,
    evidence JSONB NOT NULL,
    evidence_hash CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT erroneous_charge_refund_amount_check CHECK(amount_micros>0),
    CONSTRAINT erroneous_charge_refund_billing_check CHECK(billing_type IN(0,1)),
    CONSTRAINT erroneous_charge_refund_asset_check CHECK(asset_type IN('balance_lot','monthly_entitlement_cycle')),
    CONSTRAINT erroneous_charge_refund_hash_check CHECK(evidence_hash~'^[0-9a-f]{64}$')
);

CREATE TABLE IF NOT EXISTS monthly_entitlement_consumption_reversals (
    id BIGSERIAL PRIMARY KEY,
    refund_id BIGINT NOT NULL UNIQUE REFERENCES erroneous_charge_refunds(id) ON DELETE RESTRICT,
    consumption_id BIGINT NOT NULL UNIQUE REFERENCES monthly_entitlement_consumptions(id) ON DELETE RESTRICT,
    confirmed_amount_micros BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monthly_consumption_reversal_amount_check CHECK(confirmed_amount_micros>0)
);

ALTER TABLE balance_lots DROP CONSTRAINT IF EXISTS chk_balance_lot_source_type;
ALTER TABLE balance_lots ADD CONSTRAINT chk_balance_lot_source_type CHECK(source_type IN(
 'paid_redeem','paid_topup','gift','referral_bonus','customer_rebate','commission_conversion','compensation',
 'erroneous_charge_refund','internal_test','legacy_unattributed','admin_adjustment'));

CREATE OR REPLACE FUNCTION protect_verified_compensation_execution() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR OLD.state='verified' THEN
    RAISE EXCEPTION 'verified compensation execution is immutable' USING ERRCODE='23514',CONSTRAINT='verified_compensation_execution_immutable';
  END IF;
  IF NEW.execution_key IS DISTINCT FROM OLD.execution_key OR NEW.approval_id IS DISTINCT FROM OLD.approval_id OR NEW.draft_user_id IS DISTINCT FROM OLD.draft_user_id OR NEW.incident_id IS DISTINCT FROM OLD.incident_id OR NEW.user_id IS DISTINCT FROM OLD.user_id OR NEW.benefit_channel IS DISTINCT FROM OLD.benefit_channel OR NEW.amount_cny_fen IS DISTINCT FROM OLD.amount_cny_fen THEN
    RAISE EXCEPTION 'compensation execution identity is immutable' USING ERRCODE='23514',CONSTRAINT='compensation_execution_identity_immutable';
  END IF;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_compensation_executions_protect ON compensation_executions;
CREATE TRIGGER trg_compensation_executions_protect BEFORE UPDATE OR DELETE ON compensation_executions FOR EACH ROW EXECUTE FUNCTION protect_verified_compensation_execution();

CREATE OR REPLACE FUNCTION prevent_approved_compensation_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.replaces_draft_id IS NOT NULL AND EXISTS(
    SELECT 1 FROM compensation_draft_approvals a
    JOIN compensation_drafts d ON d.id=a.draft_id
    WHERE d.series_id=NEW.series_id
  ) THEN
    RAISE EXCEPTION 'approved compensation series cannot be revised' USING ERRCODE='23514',CONSTRAINT='approved_compensation_series_frozen';
  END IF;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_compensation_drafts_prevent_approved_revision ON compensation_drafts;
CREATE TRIGGER trg_compensation_drafts_prevent_approved_revision BEFORE INSERT ON compensation_drafts FOR EACH ROW EXECUTE FUNCTION prevent_approved_compensation_revision();

CREATE OR REPLACE FUNCTION protect_compensation_user_notification() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' THEN
    IF EXISTS(SELECT 1 FROM compensation_notices WHERE notification_id=OLD.id) THEN
      RAISE EXCEPTION 'compensation notification is immutable' USING ERRCODE='23514',CONSTRAINT='compensation_notification_immutable';
    END IF;
    RETURN OLD;
  END IF;
  IF EXISTS(SELECT 1 FROM compensation_notices WHERE notification_id=OLD.id) THEN
    IF NEW.user_id IS DISTINCT FROM OLD.user_id OR NEW.feedback_id IS DISTINCT FROM OLD.feedback_id OR NEW.type IS DISTINCT FROM OLD.type OR NEW.title IS DISTINCT FROM OLD.title OR NEW.body IS DISTINCT FROM OLD.body OR NEW.action_url IS DISTINCT FROM OLD.action_url OR NEW.dedupe_key IS DISTINCT FROM OLD.dedupe_key OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
      RAISE EXCEPTION 'compensation notification copy is immutable' USING ERRCODE='23514',CONSTRAINT='compensation_notification_immutable';
    END IF;
  END IF;
  RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_user_notifications_protect_compensation ON user_notifications;
CREATE TRIGGER trg_user_notifications_protect_compensation BEFORE UPDATE OR DELETE ON user_notifications FOR EACH ROW EXECUTE FUNCTION protect_compensation_user_notification();

DO $$ DECLARE table_name text; BEGIN
  FOREACH table_name IN ARRAY ARRAY['compensation_draft_approvals','compensation_approval_notices','compensation_execution_permission_audit','compensation_execution_assets','compensation_execution_events','compensation_execution_receipts','compensation_notices','erroneous_charge_refunds','monthly_entitlement_consumption_reversals'] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%s_immutable ON %I',table_name,table_name);
    EXECUTE format('CREATE TRIGGER trg_%s_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION prevent_compensation_shadow_evidence_mutation()',table_name,table_name);
  END LOOP;
END $$;

INSERT INTO settings(key,value,updated_at) VALUES('compensation_execution_enabled','false',NOW()) ON CONFLICT(key) DO NOTHING;
