-- Configure the approved once-only newcomer offer in one canonical row:
-- pay ¥5 and receive ¥10 of pure-gift balance. Customer copy, expected LDXP
-- price, order snapshots, and inventory validation all derive from this row.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO native_checkout_offers (
    code, provider, provider_goods_key, name, description, product_kind,
    pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type, redeem_value,
    redeem_paid_value, redeem_purpose, redeem_sales_status, redeem_group_ids,
    redeem_validity_days, once_per_user, enabled, sort_order
) VALUES (
    'newcomer-balance-5-to-10', 'ldxp', 'oc3w4r',
    '新人专享 · 10 元余额包', '',
    'balance', 500, 1000, 'balance', 10, 0, 'gift', 'gifted', '[]'::jsonb,
    0, TRUE, FALSE, 10
)
ON CONFLICT (code) DO UPDATE SET
    provider = EXCLUDED.provider,
    provider_goods_key = EXCLUDED.provider_goods_key,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    product_kind = EXCLUDED.product_kind,
    pay_amount_cny_fen = EXCLUDED.pay_amount_cny_fen,
    benefit_amount_cny_fen = EXCLUDED.benefit_amount_cny_fen,
    redeem_type = EXCLUDED.redeem_type,
    redeem_value = EXCLUDED.redeem_value,
    redeem_paid_value = EXCLUDED.redeem_paid_value,
    redeem_purpose = EXCLUDED.redeem_purpose,
    redeem_sales_status = EXCLUDED.redeem_sales_status,
    redeem_group_ids = EXCLUDED.redeem_group_ids,
    redeem_validity_days = EXCLUDED.redeem_validity_days,
    once_per_user = EXCLUDED.once_per_user,
    enabled = FALSE,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();

-- Environments without the controlled replacement batch legitimately insert
-- zero rows. The migration never generates or exports card secrets.
INSERT INTO native_checkout_redeem_inventory (redeem_code_id, offer_code)
SELECT rc.id, 'newcomer-balance-5-to-10'
FROM redeem_codes rc
JOIN redeem_code_batches batch ON batch.id = rc.batch_id
WHERE batch.name = 'native-checkout-newcomer-5-to-10-20260813'
ON CONFLICT (redeem_code_id) DO NOTHING;

RESET statement_timeout;
RESET lock_timeout;
