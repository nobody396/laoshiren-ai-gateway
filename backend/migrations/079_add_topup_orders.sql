-- 079_add_topup_orders.sql
-- 新增虎皮椒充值订单表
-- 用于记录用户通过虎皮椒发起的余额充值订单

CREATE TABLE IF NOT EXISTS topup_orders (
    id          BIGSERIAL PRIMARY KEY,
    order_no    VARCHAR(32)  NOT NULL UNIQUE,
    user_id     BIGINT       NOT NULL REFERENCES users(id),
    -- 充值金额，单位：分（人民币）。2000 = ¥20
    amount_cny_fen INTEGER    NOT NULL CHECK (amount_cny_fen > 0),
    -- 支付渠道：alipay / wechat
    pay_type    VARCHAR(16)  NOT NULL,
    -- 订单状态：pending / completed / expired
    status      VARCHAR(20)  NOT NULL DEFAULT 'pending',
    -- 虎皮椒平台交易号（回调写入）
    xunhu_trade_no  VARCHAR(64),
    -- 二维码图片 URL
    qr_code_url TEXT,
    completed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS topup_orders_user_id_idx  ON topup_orders(user_id);
CREATE INDEX IF NOT EXISTS topup_orders_status_idx   ON topup_orders(status);
CREATE INDEX IF NOT EXISTS topup_orders_order_no_idx ON topup_orders(order_no);
