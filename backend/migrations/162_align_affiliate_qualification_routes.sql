-- Affiliate V3 renamed the second qualification route from the legacy
-- "combined" route to "direct_volume". Keep persisted qualification snapshots
-- and the database constraint aligned with the service contract.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

UPDATE affiliate_qualification_states
SET qualifying_route = 'direct_volume',
    updated_at = NOW()
WHERE qualifying_route = 'combined';

ALTER TABLE affiliate_qualification_states
    DROP CONSTRAINT IF EXISTS chk_affiliate_qualification_route;

ALTER TABLE affiliate_qualification_states
    ADD CONSTRAINT chk_affiliate_qualification_route
    CHECK (
        qualifying_route IS NULL
        OR qualifying_route IN ('direct_team', 'direct_volume')
    );
