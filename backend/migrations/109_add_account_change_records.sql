-- 109_add_account_change_records.sql
-- 新增统一账户变动流水表，并将历史充值/兑换/管理员调整记录回填进去。

CREATE TABLE IF NOT EXISTS account_change_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    asset_type VARCHAR(20) NOT NULL,
    reason VARCHAR(32) NOT NULL,
    delta DECIMAL(20,8) NOT NULL DEFAULT 0,
    source_type VARCHAR(32) NOT NULL DEFAULT '',
    source_id BIGINT,
    reference_no VARCHAR(128),
    operator_user_id BIGINT,
    notes TEXT,
    group_id BIGINT REFERENCES groups(id),
    validity_days INTEGER NOT NULL DEFAULT 0,
    dedupe_key VARCHAR(128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS account_change_records_user_id_idx
    ON account_change_records(user_id);

CREATE INDEX IF NOT EXISTS account_change_records_asset_type_idx
    ON account_change_records(asset_type);

CREATE INDEX IF NOT EXISTS account_change_records_reason_idx
    ON account_change_records(reason);

CREATE INDEX IF NOT EXISTS account_change_records_source_idx
    ON account_change_records(source_type, source_id);

CREATE INDEX IF NOT EXISTS account_change_records_user_created_at_idx
    ON account_change_records(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS account_change_records_user_asset_created_at_idx
    ON account_change_records(user_id, asset_type, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS account_change_records_dedupe_key_idx
    ON account_change_records(dedupe_key);

INSERT INTO account_change_records (
    user_id,
    asset_type,
    reason,
    delta,
    source_type,
    source_id,
    reference_no,
    notes,
    group_id,
    validity_days,
    dedupe_key,
    created_at
)
SELECT
    rc.used_by AS user_id,
    CASE
        WHEN rc.type IN ('balance', 'admin_balance', 'topup') THEN 'balance'
        WHEN rc.type IN ('concurrency', 'admin_concurrency') THEN 'concurrency'
        WHEN rc.type = 'subscription' THEN 'subscription'
    END AS asset_type,
    CASE
        WHEN rc.type IN ('balance', 'concurrency', 'subscription') THEN 'redeem_code'
        WHEN rc.type IN ('admin_balance', 'admin_concurrency') THEN 'admin_adjustment'
        WHEN rc.type = 'topup' THEN 'topup'
    END AS reason,
    CASE
        WHEN rc.type = 'subscription' AND rc.validity_days > 0 THEN rc.validity_days::DECIMAL(20,8)
        ELSE rc.value
    END AS delta,
    CASE
        WHEN rc.type = 'topup' AND t.id IS NOT NULL THEN 'topup_order'
        WHEN rc.type IN ('balance', 'concurrency', 'subscription') THEN 'redeem_code'
        ELSE 'legacy_redeem_code'
    END AS source_type,
    CASE
        WHEN rc.type = 'topup' AND t.id IS NOT NULL THEN t.id
        ELSE rc.id
    END AS source_id,
    rc.code AS reference_no,
    rc.notes,
    rc.group_id,
    CASE
        WHEN rc.type = 'subscription' THEN rc.validity_days
        ELSE 0
    END AS validity_days,
    'legacy_redeem_code:' || rc.id::TEXT AS dedupe_key,
    COALESCE(rc.used_at, rc.created_at) AS created_at
FROM redeem_codes rc
LEFT JOIN topup_orders t
    ON rc.type = 'topup'
   AND t.order_no = rc.code
WHERE rc.used_by IS NOT NULL
  AND rc.type IN ('balance', 'concurrency', 'subscription', 'admin_balance', 'admin_concurrency', 'topup')
  AND NOT EXISTS (
      SELECT 1
      FROM account_change_records acr
      WHERE acr.dedupe_key = 'legacy_redeem_code:' || rc.id::TEXT
  );

UPDATE users u
SET total_recharged = COALESCE(src.total_recharged, 0)
FROM (
    SELECT
        user_id,
        SUM(amount_cny_fen)::DECIMAL(20,8) / 100.0 AS total_recharged
    FROM topup_orders
    WHERE status = 'completed'
    GROUP BY user_id
) src
WHERE u.id = src.user_id;

UPDATE users
SET total_recharged = 0
WHERE id NOT IN (
    SELECT DISTINCT user_id
    FROM topup_orders
    WHERE status = 'completed'
);
