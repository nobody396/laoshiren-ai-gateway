-- Keep commission-wallet purchase refunds as an immutable positive ledger
-- movement instead of rewriting the original debit or misclassifying it as
-- newly earned commission.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE agent_cash_commission_entries
    DROP CONSTRAINT IF EXISTS chk_agent_cash_entry_type;
ALTER TABLE agent_cash_commission_entries
    ADD CONSTRAINT chk_agent_cash_entry_type CHECK (
        entry_type IN (
            'earned', 'withdrawal_hold', 'withdrawal_release', 'conversion',
            'reversal', 'risk_release', 'platform_purchase',
            'platform_purchase_refund'
        )
    );

ALTER TABLE agent_cash_commission_entries
    DROP CONSTRAINT IF EXISTS chk_agent_cash_entry_direction;
ALTER TABLE agent_cash_commission_entries
    ADD CONSTRAINT chk_agent_cash_entry_direction CHECK (
        (entry_type IN (
            'earned', 'withdrawal_release', 'risk_release',
            'platform_purchase_refund'
        ) AND amount_micros > 0) OR
        (entry_type IN (
            'withdrawal_hold', 'conversion', 'reversal', 'platform_purchase'
        ) AND amount_micros < 0)
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_cash_platform_purchase_rate_refund
    ON agent_cash_commission_entries (
        agent_id,
        related_entry_id,
        ((metadata ->> 'rate_bps'))
    )
    WHERE entry_type = 'platform_purchase_refund';

COMMENT ON COLUMN agent_cash_commission_entries.entry_type IS
    'Cash-wallet movement type; platform_purchase_refund is a positive correction linked to an immutable platform_purchase debit';

RESET statement_timeout;
RESET lock_timeout;
