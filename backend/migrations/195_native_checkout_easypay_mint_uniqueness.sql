-- EasyPay native checkout mints one internal redeem code per paid order,
-- keyed by our NC- order number stored in redeem_codes.external_order_no.
-- The mint path is lookup-then-insert; this index is the database backstop
-- that makes a concurrent second mint fail with a unique violation (the
-- service then re-reads and returns the already-minted code).
--
-- The predicate is deliberately scoped to native checkout order numbers:
-- admin-generated card batches legitimately share one external order number
-- across many codes (one shop order, multiple cards), so an unscoped unique
-- index would reject existing batch workflows and might not even build on
-- production data. LDXP card-shop trade numbers and admin-entered order
-- references do not use the NC- prefix.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE UNIQUE INDEX IF NOT EXISTS uq_redeem_codes_external_order_no_native_checkout
    ON redeem_codes (external_order_no)
    WHERE external_order_no LIKE 'NC-%';

RESET statement_timeout;
RESET lock_timeout;
