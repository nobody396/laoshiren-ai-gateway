-- Affiliate V2 dynamic-link lookup and immutable direct-edge hardening.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE INDEX IF NOT EXISTS idx_affiliate_links_code_active
    ON affiliate_links (code)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_affiliate_bindings_agent_link
    ON affiliate_bindings (agent_id, affiliate_link_id, bound_at DESC)
    WHERE binding_kind = 'agent';

COMMENT ON COLUMN affiliate_bindings.customer_rebate_rate_snapshot_bps IS
    'Customer floor snapshot: link decreases do not reduce an existing customer; increases upgrade it';
