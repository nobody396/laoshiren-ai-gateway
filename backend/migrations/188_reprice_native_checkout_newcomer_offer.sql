-- Reprice the stable once-only newcomer offer from ¥1 -> ¥5 to ¥5 -> ¥10.
-- Keep the historical offer code unchanged: existing orders and redeemed
-- inventory must continue to count toward the same lifetime purchase limit.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE native_checkout_offers
SET name = '新人专享 · 10 元余额包',
    description = '',
    pay_amount_cny_fen = 500,
    benefit_amount_cny_fen = 1000,
    redeem_type = 'balance',
    redeem_value = 10,
    redeem_paid_value = 0,
    redeem_purpose = 'gift',
    redeem_sales_status = 'gifted',
    redeem_group_ids = '[]'::jsonb,
    redeem_validity_days = 0,
    once_per_user = TRUE,
    enabled = TRUE,
    updated_at = NOW()
WHERE code = 'trial-balance-1-to-5';

-- Environments without the controlled replacement batch legitimately insert
-- zero rows. Old ¥5 cards stay restricted for audit and must be removed from
-- LDXP stock before the merchant product is re-enabled.
INSERT INTO native_checkout_redeem_inventory (redeem_code_id, offer_code)
SELECT rc.id, 'trial-balance-1-to-5'
FROM redeem_codes rc
JOIN redeem_code_batches batch ON batch.id = rc.batch_id
WHERE batch.name = 'native-checkout-newcomer-5-to-10-20260813'
  AND rc.type = 'balance'
  AND rc.value = 10
  AND COALESCE(rc.paid_value, 0) = 0
  AND rc.purpose = 'gift'
  AND rc.sales_status = 'gifted'
  AND rc.validity_days = 0
ON CONFLICT (redeem_code_id) DO NOTHING;

RESET statement_timeout;
RESET lock_timeout;
