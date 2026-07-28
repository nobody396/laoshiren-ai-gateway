-- Affiliate V2 direct-team qualification reads only live confirmed consumption.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE INDEX IF NOT EXISTS idx_affiliate_performance_direct_consumption
    ON affiliate_performance_events (direct_agent_id, user_id, occurred_at)
    WHERE event_type IN ('confirmed_consumption', 'consumption_reversal')
      AND direct_agent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_affiliate_performance_user_consumption
    ON affiliate_performance_events (user_id, occurred_at)
    WHERE event_type IN ('confirmed_consumption', 'consumption_reversal');
