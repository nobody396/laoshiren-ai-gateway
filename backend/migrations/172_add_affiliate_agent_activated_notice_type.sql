-- Allow the agent activation notice type used by the qualify-and-activate flow.

ALTER TABLE affiliate_agent_notices
    DROP CONSTRAINT IF EXISTS chk_affiliate_agent_notice_type;
ALTER TABLE affiliate_agent_notices
    ADD CONSTRAINT chk_affiliate_agent_notice_type CHECK (
        notice_type IN (
            'community_invite',
            'withdrawal_paid',
            'withdrawal_failed',
            'risk_review',
            'risk_blocked',
            'risk_cleared',
            'commission_reversed',
            'agent_activated'
        )
    );
