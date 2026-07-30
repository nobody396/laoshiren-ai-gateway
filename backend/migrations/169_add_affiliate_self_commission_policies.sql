-- Per-partner self-consumption commission controls.
--
-- Policies are absent/disabled by default and may only be enabled for active,
-- clear-risk partners without any upstream relationship. The current policy
-- is mutable through the optimistic-revision admin API; every actual change is
-- copied to an append-only event table.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS affiliate_agent_self_commission_policies (
    agent_id BIGINT PRIMARY KEY REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rate_bps INTEGER NOT NULL DEFAULT 1000,
    effective_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_self_commission_rate CHECK (rate_bps = 1000),
    CONSTRAINT chk_affiliate_self_commission_revision CHECK (revision > 0),
    CONSTRAINT chk_affiliate_self_commission_effective CHECK (
        (enabled = TRUE AND effective_at IS NOT NULL)
        OR
        (enabled = FALSE AND effective_at IS NULL)
    ),
    CONSTRAINT chk_affiliate_self_commission_reason CHECK (BTRIM(reason) <> '')
);

CREATE INDEX IF NOT EXISTS idx_affiliate_self_commission_enabled
    ON affiliate_agent_self_commission_policies (agent_id, effective_at)
    WHERE enabled = TRUE;

CREATE TABLE IF NOT EXISTS affiliate_agent_self_commission_events (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    previous_enabled BOOLEAN NOT NULL,
    next_enabled BOOLEAN NOT NULL,
    rate_bps INTEGER NOT NULL DEFAULT 1000,
    effective_at TIMESTAMPTZ,
    revision BIGINT NOT NULL,
    reason TEXT NOT NULL,
    operator_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_self_commission_event_revision UNIQUE (agent_id, revision),
    CONSTRAINT chk_affiliate_self_commission_event_rate CHECK (rate_bps = 1000),
    CONSTRAINT chk_affiliate_self_commission_event_revision CHECK (revision > 0),
    CONSTRAINT chk_affiliate_self_commission_event_effective CHECK (
        (next_enabled = TRUE AND effective_at IS NOT NULL)
        OR
        (next_enabled = FALSE AND effective_at IS NULL)
    ),
    CONSTRAINT chk_affiliate_self_commission_event_reason CHECK (BTRIM(reason) <> ''),
    CONSTRAINT chk_affiliate_self_commission_event_metadata CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_self_commission_events_agent_time
    ON affiliate_agent_self_commission_events (agent_id, created_at DESC, id DESC);

-- One row is the database-level serialization and exclusion point for an
-- upstream relationship versus an enabled self-commission policy. Unlike a
-- lock-only scheme, the row is actually updated, so a waiter observes the
-- committed competing state even when its statement snapshot was older.
CREATE TABLE IF NOT EXISTS affiliate_self_commission_relationship_guards (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    has_upstream BOOLEAN NOT NULL DEFAULT FALSE,
    self_commission_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_relationship_guard_exclusive CHECK (
        NOT (has_upstream AND self_commission_enabled)
    )
);

INSERT INTO affiliate_self_commission_relationship_guards (
    user_id,
    has_upstream,
    self_commission_enabled
)
SELECT
    u.id,
    TRUE,
    FALSE
FROM users u
WHERE u.inviter_id IS NOT NULL
   OR u.agent_id IS NOT NULL
   OR EXISTS (
        SELECT 1
        FROM affiliate_bindings binding
        WHERE binding.customer_user_id = u.id
   )
ON CONFLICT (user_id) DO UPDATE
SET has_upstream = TRUE,
    updated_at = NOW();

CREATE OR REPLACE FUNCTION prevent_affiliate_self_commission_event_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'affiliate self-commission events are immutable'
        USING
            ERRCODE = '55000',
            CONSTRAINT = 'affiliate_self_commission_events_immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_affiliate_self_commission_events_immutable
    ON affiliate_agent_self_commission_events;
CREATE TRIGGER trg_affiliate_self_commission_events_immutable
    BEFORE UPDATE OR DELETE ON affiliate_agent_self_commission_events
    FOR EACH ROW
    EXECUTE FUNCTION prevent_affiliate_self_commission_event_mutation();

-- Locking the user row is the serialization point shared by policy activation,
-- affiliate binding creation, and legacy users.inviter_id/users.agent_id writes.
-- This closes the race where both sides independently observe "no upstream".
CREATE OR REPLACE FUNCTION guard_affiliate_self_commission_policy()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    current_inviter_id BIGINT;
    current_agent_id BIGINT;
    principal_status VARCHAR(24);
    principal_risk_status VARCHAR(24);
BEGIN
    SELECT
        u.inviter_id,
        u.agent_id,
        ap.status,
        ap.risk_status
    INTO
        current_inviter_id,
        current_agent_id,
        principal_status,
        principal_risk_status
    FROM users u
    JOIN agent_principals ap ON ap.agent_id = u.id
    WHERE u.id = NEW.agent_id
      AND u.deleted_at IS NULL
    FOR UPDATE OF u, ap;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'affiliate partner does not exist'
            USING
                ERRCODE = '23503',
                CONSTRAINT = 'affiliate_self_commission_partner_exists';
    END IF;

    IF NEW.enabled THEN
        IF principal_status <> 'active' OR principal_risk_status <> 'clear' THEN
            RAISE EXCEPTION 'affiliate partner must be active with clear risk status'
                USING
                    ERRCODE = '23514',
                    CONSTRAINT = 'affiliate_self_commission_partner_eligible';
        END IF;

        IF current_inviter_id IS NOT NULL
           OR current_agent_id IS NOT NULL
           OR EXISTS (
                SELECT 1
                FROM affiliate_bindings binding
                WHERE binding.customer_user_id = NEW.agent_id
           )
        THEN
            RAISE EXCEPTION 'affiliate partner already has an upstream relationship'
                USING
                    ERRCODE = '23514',
                    CONSTRAINT = 'affiliate_self_commission_no_upstream';
        END IF;
    END IF;

    BEGIN
        INSERT INTO affiliate_self_commission_relationship_guards (
            user_id,
            has_upstream,
            self_commission_enabled
        )
        VALUES (NEW.agent_id, FALSE, NEW.enabled)
        ON CONFLICT (user_id) DO UPDATE
        SET self_commission_enabled = EXCLUDED.self_commission_enabled,
            updated_at = NOW();
    EXCEPTION
        WHEN check_violation THEN
            RAISE EXCEPTION 'affiliate partner already has an upstream relationship'
                USING
                    ERRCODE = '23514',
                    CONSTRAINT = 'affiliate_self_commission_no_upstream';
    END;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_affiliate_self_commission_policy_guard
    ON affiliate_agent_self_commission_policies;
CREATE TRIGGER trg_affiliate_self_commission_policy_guard
    BEFORE INSERT OR UPDATE OF enabled, effective_at
    ON affiliate_agent_self_commission_policies
    FOR EACH ROW
    EXECUTE FUNCTION guard_affiliate_self_commission_policy();

CREATE OR REPLACE FUNCTION prevent_affiliate_self_commission_policy_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'affiliate self-commission policies must be disabled, not deleted'
        USING
            ERRCODE = '55000',
            CONSTRAINT = 'affiliate_self_commission_policy_delete_forbidden';
END;
$$;

DROP TRIGGER IF EXISTS trg_affiliate_self_commission_policy_delete
    ON affiliate_agent_self_commission_policies;
CREATE TRIGGER trg_affiliate_self_commission_policy_delete
    BEFORE DELETE ON affiliate_agent_self_commission_policies
    FOR EACH ROW
    EXECUTE FUNCTION prevent_affiliate_self_commission_policy_delete();

CREATE OR REPLACE FUNCTION guard_affiliate_binding_against_self_commission()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM affiliate_agent_self_commission_policies policy
        WHERE policy.agent_id = NEW.customer_user_id
          AND policy.enabled = TRUE
    )
    THEN
        RAISE EXCEPTION 'enabled self-commission partner cannot gain an upstream binding'
            USING
                ERRCODE = '23514',
                    CONSTRAINT = 'affiliate_binding_self_commission_exclusion';
    END IF;

    INSERT INTO affiliate_self_commission_relationship_guards (
        user_id,
        has_upstream,
        self_commission_enabled
    )
    VALUES (NEW.customer_user_id, TRUE, FALSE)
    ON CONFLICT (user_id) DO UPDATE
    SET has_upstream = TRUE,
        updated_at = NOW();

    RETURN NEW;
EXCEPTION
    WHEN check_violation THEN
        RAISE EXCEPTION 'enabled self-commission partner cannot gain an upstream binding'
            USING
                ERRCODE = '23514',
                CONSTRAINT = 'affiliate_binding_self_commission_exclusion';
END;
$$;

DROP TRIGGER IF EXISTS trg_affiliate_binding_self_commission_guard
    ON affiliate_bindings;
CREATE TRIGGER trg_affiliate_binding_self_commission_guard
    BEFORE INSERT OR UPDATE OF customer_user_id
    ON affiliate_bindings
    FOR EACH ROW
    EXECUTE FUNCTION guard_affiliate_binding_against_self_commission();

CREATE OR REPLACE FUNCTION guard_legacy_upstream_against_self_commission()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF (
        NEW.inviter_id IS DISTINCT FROM OLD.inviter_id
        OR NEW.agent_id IS DISTINCT FROM OLD.agent_id
    )
    AND (
        NEW.inviter_id IS NOT NULL
        OR NEW.agent_id IS NOT NULL
    )
    AND EXISTS (
        SELECT 1
        FROM affiliate_agent_self_commission_policies policy
        WHERE policy.agent_id = NEW.id
          AND policy.enabled = TRUE
    )
    THEN
        RAISE EXCEPTION 'enabled self-commission partner cannot gain a legacy upstream relationship'
            USING
                ERRCODE = '23514',
                    CONSTRAINT = 'users_upstream_self_commission_exclusion';
    END IF;

    IF (
        NEW.inviter_id IS NOT NULL
        OR NEW.agent_id IS NOT NULL
    )
    THEN
        INSERT INTO affiliate_self_commission_relationship_guards (
            user_id,
            has_upstream,
            self_commission_enabled
        )
        VALUES (NEW.id, TRUE, FALSE)
        ON CONFLICT (user_id) DO UPDATE
        SET has_upstream = TRUE,
            updated_at = NOW();
    END IF;

    RETURN NEW;
EXCEPTION
    WHEN check_violation THEN
        RAISE EXCEPTION 'enabled self-commission partner cannot gain a legacy upstream relationship'
            USING
                ERRCODE = '23514',
                CONSTRAINT = 'users_upstream_self_commission_exclusion';
END;
$$;

DROP TRIGGER IF EXISTS trg_users_self_commission_upstream_guard ON users;
CREATE TRIGGER trg_users_self_commission_upstream_guard
    BEFORE UPDATE OF inviter_id, agent_id
    ON users
    FOR EACH ROW
    EXECUTE FUNCTION guard_legacy_upstream_against_self_commission();

COMMENT ON TABLE affiliate_agent_self_commission_policies IS
    'Current per-partner self-consumption cash commission policy; absent means disabled';
COMMENT ON TABLE affiliate_agent_self_commission_events IS
    'Append-only audit history for self-consumption commission policy changes';
COMMENT ON COLUMN affiliate_agent_self_commission_policies.rate_bps IS
    'Fixed 10 percent cash commission rate (1000 basis points)';
COMMENT ON COLUMN affiliate_agent_self_commission_policies.effective_at IS
    'Only purchases at or after this instant may snapshot self-consumption eligibility';

-- Extend the immutable purchase/performance policy vocabulary without
-- weakening the existing 10% pool. Regular partner usage must point to a
-- different consumer; self usage must point back to the same user and allocate
-- the entire fixed pool to partner cash commission.
ALTER TABLE balance_lots
    DROP CONSTRAINT IF EXISTS chk_balance_lot_affiliate_shape,
    DROP CONSTRAINT IF EXISTS chk_balance_lot_affiliate_policy;

ALTER TABLE balance_lots
    ADD CONSTRAINT chk_balance_lot_affiliate_policy CHECK (
        affiliate_policy IN (
            'NONE',
            'ORDINARY_FIRST_PAID',
            'PARTNER_USAGE',
            'PARTNER_SELF_USAGE'
        )
    ),
    ADD CONSTRAINT chk_balance_lot_affiliate_shape CHECK (
        (
            affiliate_policy = 'PARTNER_USAGE'
            AND affiliate_eligible = TRUE
            AND direct_partner_id IS NOT NULL
            AND direct_partner_id <> user_id
            AND customer_rebate_rate_bps + partner_commission_rate_bps = 1000
        )
        OR
        (
            affiliate_policy = 'PARTNER_SELF_USAGE'
            AND affiliate_eligible = TRUE
            AND direct_partner_id IS NOT NULL
            AND direct_partner_id = user_id
            AND customer_rebate_rate_bps = 0
            AND partner_commission_rate_bps = 1000
        )
        OR
        (
            affiliate_policy NOT IN ('PARTNER_USAGE', 'PARTNER_SELF_USAGE')
            AND affiliate_eligible = (affiliate_policy = 'ORDINARY_FIRST_PAID')
            AND customer_rebate_rate_bps = 0
            AND partner_commission_rate_bps = 0
        )
    );

ALTER TABLE monthly_entitlement_cycles
    DROP CONSTRAINT IF EXISTS chk_monthly_entitlement_affiliate_shape,
    DROP CONSTRAINT IF EXISTS chk_monthly_entitlement_affiliate_policy;

ALTER TABLE monthly_entitlement_cycles
    ADD CONSTRAINT chk_monthly_entitlement_affiliate_policy CHECK (
        affiliate_policy IN (
            'NONE',
            'ORDINARY_FIRST_PAID',
            'PARTNER_USAGE',
            'PARTNER_SELF_USAGE'
        )
    ),
    ADD CONSTRAINT chk_monthly_entitlement_affiliate_shape CHECK (
        (
            affiliate_policy = 'PARTNER_USAGE'
            AND affiliate_eligible = TRUE
            AND direct_partner_id IS NOT NULL
            AND direct_partner_id <> user_id
            AND customer_rebate_rate_bps + partner_commission_rate_bps = 1000
        )
        OR
        (
            affiliate_policy = 'PARTNER_SELF_USAGE'
            AND affiliate_eligible = TRUE
            AND direct_partner_id IS NOT NULL
            AND direct_partner_id = user_id
            AND customer_rebate_rate_bps = 0
            AND partner_commission_rate_bps = 1000
        )
        OR
        (
            affiliate_policy NOT IN ('PARTNER_USAGE', 'PARTNER_SELF_USAGE')
            AND affiliate_eligible = (affiliate_policy = 'ORDINARY_FIRST_PAID')
            AND customer_rebate_rate_bps = 0
            AND partner_commission_rate_bps = 0
        )
    );

ALTER TABLE affiliate_performance_events
    DROP CONSTRAINT IF EXISTS chk_affiliate_performance_rates,
    DROP CONSTRAINT IF EXISTS chk_affiliate_performance_policy;

ALTER TABLE affiliate_performance_events
    ADD CONSTRAINT chk_affiliate_performance_policy CHECK (
        affiliate_policy IN (
            'NONE',
            'ORDINARY_FIRST_PAID',
            'PARTNER_USAGE',
            'PARTNER_SELF_USAGE'
        )
    ),
    ADD CONSTRAINT chk_affiliate_performance_rates CHECK (
        customer_rebate_rate_bps BETWEEN 0 AND 1000
        AND partner_commission_rate_bps BETWEEN 0 AND 1000
        AND (
            (
                affiliate_policy = 'PARTNER_USAGE'
                AND direct_agent_id IS NOT NULL
                AND direct_agent_id <> user_id
                AND customer_rebate_rate_bps + partner_commission_rate_bps = 1000
            )
            OR
            (
                affiliate_policy = 'PARTNER_SELF_USAGE'
                AND direct_agent_id IS NOT NULL
                AND direct_agent_id = user_id
                AND customer_rebate_rate_bps = 0
                AND partner_commission_rate_bps = 1000
                AND (metadata ->> 'attribution_policy')
                    IS NOT DISTINCT FROM 'PARTNER_SELF_USAGE'
            )
            OR
            (
                affiliate_policy NOT IN ('PARTNER_USAGE', 'PARTNER_SELF_USAGE')
            )
        )
    );

ALTER TABLE agent_cash_commission_entries
    DROP CONSTRAINT IF EXISTS chk_agent_cash_self_commission_shape;

ALTER TABLE agent_cash_commission_entries
    ADD CONSTRAINT chk_agent_cash_self_commission_shape CHECK (
        entry_type <> 'earned'
        OR consumer_user_id IS NULL
        OR agent_id IS DISTINCT FROM consumer_user_id
        OR (
            agent_id IS NOT DISTINCT FROM consumer_user_id
            AND customer_rebate_rate_bps IS NOT DISTINCT FROM 0
            AND agent_commission_rate_bps IS NOT DISTINCT FROM 1000
            AND (metadata ->> 'attribution_policy')
                IS NOT DISTINCT FROM 'PARTNER_SELF_USAGE'
        )
    );

COMMENT ON CONSTRAINT chk_agent_cash_self_commission_shape
    ON agent_cash_commission_entries IS
    'Self earned cash is valid only for the explicit unsplit PARTNER_SELF_USAGE pool';
