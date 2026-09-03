-- Allow an active partner's cash commission to pay for a platform entitlement
-- without pretending that cash was withdrawn or converting it into API credit.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE agent_cash_commission_entries
    DROP CONSTRAINT IF EXISTS chk_agent_cash_entry_type;

ALTER TABLE agent_cash_commission_entries
    ADD CONSTRAINT chk_agent_cash_entry_type CHECK (
        entry_type IN (
            'earned', 'withdrawal_hold', 'withdrawal_release', 'conversion',
            'reversal', 'risk_release', 'platform_purchase'
        )
    );

ALTER TABLE agent_cash_commission_entries
    DROP CONSTRAINT IF EXISTS chk_agent_cash_entry_direction;

ALTER TABLE agent_cash_commission_entries
    ADD CONSTRAINT chk_agent_cash_entry_direction CHECK (
        (entry_type IN ('earned', 'withdrawal_release', 'risk_release') AND amount_micros > 0) OR
        (entry_type IN ('withdrawal_hold', 'conversion', 'reversal', 'platform_purchase') AND amount_micros < 0)
    );

COMMENT ON COLUMN agent_cash_commission_entries.entry_type IS
    'Cash-wallet movement type; platform_purchase is an in-kind platform entitlement purchase, not a withdrawal or commission reversal';

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_cash_platform_purchase_reference
    ON agent_cash_commission_entries (agent_id, ((metadata ->> 'external_reference')))
    WHERE entry_type = 'platform_purchase';

ALTER TABLE affiliate_agent_notices
    DROP CONSTRAINT IF EXISTS chk_affiliate_agent_notice_type;

ALTER TABLE affiliate_agent_notices
    ADD CONSTRAINT chk_affiliate_agent_notice_type CHECK (
        notice_type IN (
            'community_invite', 'withdrawal_paid', 'withdrawal_failed',
            'risk_review', 'risk_blocked', 'risk_cleared',
            'commission_reversed', 'agent_activated', 'commission_purchase'
        )
    );

RESET statement_timeout;
RESET lock_timeout;
