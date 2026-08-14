-- Retire the native payment flow for the newcomer offer while keeping its
-- existing ¥5 -> ¥10 card stock usable through the public manual redeem path.
-- The offer stays disabled for native checkout. A database claim row is the
-- final authority for the once-per-account rule, including concurrent attempts
-- with two different card codes.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE native_checkout_offers
    ADD COLUMN IF NOT EXISTS manual_redeem_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS native_checkout_manual_claims (
    offer_code VARCHAR(64) NOT NULL REFERENCES native_checkout_offers(code) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    redeem_code_id BIGINT NOT NULL UNIQUE REFERENCES redeem_codes(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (offer_code, user_id)
);

CREATE INDEX IF NOT EXISTS idx_native_checkout_manual_claims_user
    ON native_checkout_manual_claims (user_id, offer_code);

-- Preserve any historical use of stocked codes before turning on the manual
-- path. This makes the new claim table consistent with the existing lifetime
-- entitlement check.
INSERT INTO native_checkout_manual_claims (offer_code, user_id, redeem_code_id, created_at)
SELECT inventory.offer_code, code.used_by, code.id, COALESCE(code.used_at, NOW())
FROM native_checkout_redeem_inventory inventory
JOIN native_checkout_offers offer ON offer.code = inventory.offer_code
JOIN redeem_codes code ON code.id = inventory.redeem_code_id
WHERE offer.once_per_user = TRUE
  AND code.status = 'used'
  AND code.used_by IS NOT NULL
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION claim_manual_checkout_offer_once()
RETURNS TRIGGER AS $$
DECLARE
    claim_offer_code VARCHAR(64);
BEGIN
    IF NEW.status <> 'used' OR NEW.used_by IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT inventory.offer_code INTO claim_offer_code
    FROM native_checkout_redeem_inventory inventory
    JOIN native_checkout_offers offer ON offer.code = inventory.offer_code
    WHERE inventory.redeem_code_id = NEW.id
      AND offer.manual_redeem_enabled = TRUE
      AND offer.once_per_user = TRUE;

    IF claim_offer_code IS NOT NULL THEN
        INSERT INTO native_checkout_manual_claims (offer_code, user_id, redeem_code_id)
        VALUES (claim_offer_code, NEW.used_by, NEW.id);
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_claim_manual_checkout_offer_once ON redeem_codes;
CREATE TRIGGER trg_claim_manual_checkout_offer_once
    BEFORE UPDATE OF status, used_by ON redeem_codes
    FOR EACH ROW
    WHEN (
        NEW.status = 'used'
        AND NEW.used_by IS NOT NULL
        AND (OLD.status IS DISTINCT FROM NEW.status OR OLD.used_by IS DISTINCT FROM NEW.used_by)
    )
    EXECUTE FUNCTION claim_manual_checkout_offer_once();

UPDATE native_checkout_offers
SET manual_redeem_enabled = TRUE,
    enabled = FALSE,
    updated_at = NOW()
WHERE code = 'newcomer-balance-5-to-10';

DELETE FROM native_checkout_offer_testers
WHERE offer_code = 'newcomer-balance-5-to-10';

RESET statement_timeout;
RESET lock_timeout;
