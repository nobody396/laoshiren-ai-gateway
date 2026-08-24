-- Sell the three developer monthly cards through the native EasyPay checkout.
-- The card-shop catalog keeps its existing list prices; these rows contain the
-- owner-approved scan-payment prices and mint the same 31-day entitlements.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO native_checkout_offers (
    code, provider, provider_goods_key, name, description, product_kind,
    pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type, redeem_value,
    redeem_paid_value, redeem_purpose, redeem_sales_status, redeem_group_ids,
    redeem_validity_days, once_per_user, enabled, sort_order,
    manual_redeem_enabled
) VALUES
    (
        'plus', 'easypay', 'plus', 'Plus 月卡',
        '支付宝或微信扫码支付，自动开通 31 天 Plus 开发者计划。',
        'subscription', 25500, 25900, 'subscription', 259,
        0, 'sale_recharge', 'sold', '[40,41]'::jsonb,
        31, FALSE, TRUE, 20, FALSE
    ),
    (
        'pro', 'easypay', 'pro', 'Pro 月卡',
        '支付宝或微信扫码支付，自动开通 31 天 Pro 开发者计划。',
        'subscription', 71500, 72900, 'subscription', 729,
        0, 'sale_recharge', 'sold', '[42,43]'::jsonb,
        31, FALSE, TRUE, 30, FALSE
    ),
    (
        'max', 'easypay', 'max', 'Max 月卡',
        '支付宝或微信扫码支付，自动开通 31 天 Max 开发者计划。',
        'subscription', 152500, 154900, 'subscription', 1549,
        0, 'sale_recharge', 'sold', '[44,45]'::jsonb,
        31, FALSE, TRUE, 40, FALSE
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
    enabled = EXCLUDED.enabled,
    sort_order = EXCLUDED.sort_order,
    manual_redeem_enabled = EXCLUDED.manual_redeem_enabled,
    updated_at = NOW();

RESET statement_timeout;
RESET lock_timeout;
