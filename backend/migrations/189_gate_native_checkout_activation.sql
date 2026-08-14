-- Keep the newcomer checkout dark after deployment.  Production activation
-- is a separate, explicit operation performed only after the LDXP product is
-- priced at ¥5, its stock contains the matching registered ¥10 cards, and the
-- merchant recovery credentials are present.  This prevents a code deploy
-- from exposing a checkout that cannot safely fulfill.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE native_checkout_offers
SET enabled = FALSE,
    updated_at = NOW()
WHERE code = 'trial-balance-1-to-5';

RESET statement_timeout;
RESET lock_timeout;
