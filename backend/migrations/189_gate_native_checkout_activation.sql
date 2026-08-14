-- Keep the approved newcomer offer dark after deployment. Production
-- activation is a separate, explicit operation performed only after the LDXP
-- product costs ¥5, stock contains matching registered ¥10 pure-gift cards,
-- recovery credentials are present, and the real payment gate passes.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE native_checkout_offers
SET enabled = FALSE,
    updated_at = NOW()
WHERE code = 'newcomer-balance-5-to-10';

DELETE FROM native_checkout_offer_testers
WHERE offer_code = 'newcomer-balance-5-to-10';

RESET statement_timeout;
RESET lock_timeout;
