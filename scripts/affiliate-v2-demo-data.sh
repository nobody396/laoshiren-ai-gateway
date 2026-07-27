#!/usr/bin/env bash
set -euo pipefail
set +x

readonly EXPECTED_WORKTREE="/Users/fujunhao/laoshirenai/worktrees/affiliate-program-v2"
readonly EXPECTED_BRANCH="feat/affiliate-program-v2-20260726"
readonly PG_CONTAINER="laoshirenai-affiliate-v2-staging-postgres-1"
readonly APP_CONTAINER="laoshirenai-affiliate-v2-staging-app-1"

require_checkout() {
  local current_root current_branch
  current_root="$(git -C "$EXPECTED_WORKTREE" rev-parse --show-toplevel)"
  current_branch="$(git -C "$EXPECTED_WORKTREE" branch --show-current)"
  [[ "$current_root" == "$EXPECTED_WORKTREE" ]] || {
    echo "unexpected staging worktree: $current_root" >&2
    exit 1
  }
  [[ "$current_branch" == "$EXPECTED_BRANCH" ]] || {
    echo "unexpected staging branch: $current_branch" >&2
    exit 1
  }
}

require_containers() {
  docker inspect "$PG_CONTAINER" >/dev/null
  docker inspect "$APP_CONTAINER" >/dev/null
}

write_demo_qr_assets() {
  local tmp_png="${TMPDIR:-/tmp}/affiliate-v2-demo-qr.png"
  python3 - "$tmp_png" <<'PY'
import binascii
import struct
import sys
import zlib

path = sys.argv[1]
w = h = 160

def chunk(kind: bytes, payload: bytes) -> bytes:
    return (
        struct.pack(">I", len(payload))
        + kind
        + payload
        + struct.pack(">I", binascii.crc32(kind + payload) & 0xFFFFFFFF)
    )

rows = []
for y in range(h):
    row = bytearray([0])
    for x in range(w):
        finder = (
            (8 <= x < 48 and 8 <= y < 48)
            or (112 <= x < 152 and 8 <= y < 48)
            or (8 <= x < 48 and 112 <= y < 152)
        )
        core = (58 <= x < 104 and 58 <= y < 104 and ((x // 8 + y // 8) % 2 == 0))
        stripes = (x % 29 < 6 and 52 <= y < 132) or (y % 31 < 6 and 52 <= x < 132)
        black = finder or core or stripes
        row.extend((0, 0, 0) if black else (255, 255, 255))
    rows.append(bytes(row))

payload = b"".join(rows)
png = (
    b"\x89PNG\r\n\x1a\n"
    + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
    + chunk(b"IDAT", zlib.compress(payload, 9))
    + chunk(b"IEND", b"")
)
with open(path, "wb") as f:
    f.write(png)
PY

  docker exec "$APP_CONTAINER" mkdir -p \
    /app/data/uploads/agent-payment-qrcodes \
    /app/data/uploads/affiliate-community
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/agent-payment-qrcodes/demo-alpha.png"
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/agent-payment-qrcodes/demo-review.png"
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/agent-payment-qrcodes/demo-blocked.png"
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/affiliate-community/demo-community.png"
  docker exec --user 0 "$APP_CONTAINER" chmod 644 \
    /app/data/uploads/agent-payment-qrcodes/demo-alpha.png \
    /app/data/uploads/agent-payment-qrcodes/demo-review.png \
    /app/data/uploads/agent-payment-qrcodes/demo-blocked.png \
    /app/data/uploads/affiliate-community/demo-community.png
}

seed_database() {
  docker exec -i "$PG_CONTAINER" psql -U affiliate_staging -d affiliate_staging -v ON_ERROR_STOP=1 <<'SQL'
DO $$
DECLARE
  demo_password_hash text := '$2a$10$8F1HImqy1ZYH8EJevN6BEulUZrKn2k2JMkOoy2BnYJUlRUPptzvz6';
  admin_id bigint;
  demo_group_id bigint;
  demo_account_id bigint;
  alpha_id bigint;
  review_id bigint;
  blocked_id bigint;
  candidate_id bigint;
  ordinary_id bigint;
  invitee_id bigint;
  customer_id bigint;
  topup_id bigint;
  demo_api_key_id bigint;
  alpha_default_link_id bigint;
  alpha_zero_link_id bigint;
  alpha_vip_link_id bigint;
  alpha_paused_link_id bigint;
  review_default_link_id bigint;
  blocked_default_link_id bigint;
  selected_link_id bigint;
  event_id bigint;
  reward_id bigint;
  cash_id bigint;
  conversion_id bigint;
  withdrawal_request_id bigint;
  i integer;
  amount_micros bigint;
  reward_micros bigint;
  cash_micros bigint;
  customer_bps integer;
  agent_bps integer;
  customer_email text;
  demo_order_no text;
  usage_id bigint;
BEGIN
  SELECT id INTO admin_id
  FROM users
  WHERE email = 'affiliate-staging@local.invalid'
    AND deleted_at IS NULL
  LIMIT 1;
  IF admin_id IS NULL THEN
    RAISE EXCEPTION 'staging admin user is missing';
  END IF;

  UPDATE affiliate_program_settings
  SET mode = 'live',
      started_at = NOW() - INTERVAL '14 days',
      revision = revision + 1,
      updated_by = admin_id,
      updated_at = NOW()
  WHERE id = 1;

  UPDATE affiliate_community_settings
  SET enabled = TRUE,
      title = '合伙人内测社群',
      message = '这里是 staging 演示数据：成为合伙人后会看到这张加群卡片。实际生产可以替换成飞书群、微信群或运营群二维码。',
      qr_object_key = 'affiliate-community/demo-community.png',
      qr_content_type = 'image/png',
      qr_original_filename = 'demo-community.png',
      qr_size = 1200,
      revision = revision + 1,
      updated_by = admin_id,
      updated_at = NOW()
  WHERE id = 1;

  INSERT INTO groups (
    name, description, rate_multiplier, status, platform, subscription_type,
    monthly_limit_usd, daily_limit_usd, default_validity_days, sort_order,
    supported_model_scopes
  )
  VALUES
    ('GPT Starter 月卡组', '', 0.5000, 'active', 'openai', 'credit', 240, 8, 31, 710, '["openai"]'::jsonb),
    ('GPT Lite 月卡组', '', 0.5000, 'active', 'openai', 'credit', 450, 15, 31, 720, '["openai"]'::jsonb),
    ('GPT Pro 月卡组', '', 0.5000, 'active', 'openai', 'credit', 850, 28, 31, 730, '["openai"]'::jsonb),
    ('Claude Lite 演示组', '', 2.4000, 'active', 'anthropic', 'credit', 450, 15, 31, 820, '["claude"]'::jsonb),
    ('Grok Lite 演示组', '', 0.4000, 'active', 'openai', 'credit', 450, 15, 31, 920, '["openai"]'::jsonb)
  ON CONFLICT (name) WHERE deleted_at IS NULL DO UPDATE SET
    rate_multiplier = EXCLUDED.rate_multiplier,
    status = EXCLUDED.status,
    platform = EXCLUDED.platform,
    subscription_type = EXCLUDED.subscription_type,
    monthly_limit_usd = EXCLUDED.monthly_limit_usd,
    daily_limit_usd = EXCLUDED.daily_limit_usd,
    default_validity_days = EXCLUDED.default_validity_days,
    sort_order = EXCLUDED.sort_order,
    supported_model_scopes = EXCLUDED.supported_model_scopes,
    updated_at = NOW();

  UPDATE groups
  SET rate_multiplier = 0.5000,
      updated_at = NOW()
  WHERE deleted_at IS NULL
    AND name LIKE 'GPT %月卡组';

  SELECT id INTO demo_group_id
  FROM groups
  WHERE name = 'GPT Lite 月卡组'
    AND deleted_at IS NULL
  LIMIT 1;

  SELECT id INTO demo_account_id
  FROM accounts
  WHERE name = 'Demo GPT upstream · staging only'
    AND deleted_at IS NULL
  ORDER BY id
  LIMIT 1;
  IF demo_account_id IS NULL THEN
    INSERT INTO accounts (
      name, platform, type, credentials, extra, concurrency, priority, status,
      schedulable, rate_multiplier, notes
    )
    VALUES (
      'Demo GPT upstream · staging only', 'openai', 'demo',
      '{}'::jsonb, '{"staging_demo": true}'::jsonb, 20, 10, 'active',
      TRUE, 0.1850, 'staging demo data only'
    )
    RETURNING id INTO demo_account_id;
  ELSE
    UPDATE accounts
    SET platform = 'openai',
        type = 'demo',
        extra = '{"staging_demo": true}'::jsonb,
        concurrency = 20,
        priority = 10,
        status = 'active',
        schedulable = TRUE,
        rate_multiplier = 0.1850,
        notes = 'staging demo data only',
        updated_at = NOW()
    WHERE id = demo_account_id;
  END IF;

  INSERT INTO account_groups (account_id, group_id, priority)
  VALUES (demo_account_id, demo_group_id, 1)
  ON CONFLICT (account_id, group_id) DO UPDATE SET
    priority = EXCLUDED.priority;

  INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, total_recharged, first_recharged, last_active_at, wechat)
  VALUES
    ('agent-alpha@demo.local', demo_password_hash, 'agent', 0, 8, 'active', 'Alpha 合伙人', 'staging demo active partner', 'AGENTALPHA', 3888, TRUE, NOW() - INTERVAL '1 hour', 'alpha-demo'),
    ('agent-review@demo.local', demo_password_hash, 'agent', 0, 8, 'active', '待确认合伙人', 'staging demo review partner', 'AGENTREVIEW', 860, TRUE, NOW() - INTERVAL '2 hours', 'review-demo'),
    ('agent-blocked@demo.local', demo_password_hash, 'agent', 0, 8, 'active', '已暂停合伙人', 'staging demo paused partner', 'AGENTBLOCK', 640, TRUE, NOW() - INTERVAL '3 hours', 'blocked-demo'),
    ('agent-candidate@demo.local', demo_password_hash, 'user', 0, 5, 'active', 'Candidate 待升级用户', 'staging demo qualified candidate', 'CANDIDATE', 220, TRUE, NOW() - INTERVAL '4 hours', 'candidate-demo'),
    ('ordinary-referrer@demo.local', demo_password_hash, 'user', 0, 5, 'active', '普通邀请人', 'staging demo ordinary referrer', 'ORDREF', 160, TRUE, NOW() - INTERVAL '5 hours', 'ordinary-demo'),
    ('ordinary-invitee@demo.local', demo_password_hash, 'user', 0, 5, 'active', '普通被邀请人', 'staging demo ordinary invitee', 'ORDINVITEE', 80, TRUE, NOW() - INTERVAL '6 hours', 'invitee-demo')
  ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    role = EXCLUDED.role,
    status = EXCLUDED.status,
    username = EXCLUDED.username,
    notes = EXCLUDED.notes,
    invite_code = EXCLUDED.invite_code,
    total_recharged = EXCLUDED.total_recharged,
    first_recharged = EXCLUDED.first_recharged,
    last_active_at = EXCLUDED.last_active_at,
    wechat = EXCLUDED.wechat,
    updated_at = NOW();

  SELECT id INTO alpha_id FROM users WHERE email = 'agent-alpha@demo.local' AND deleted_at IS NULL;
  SELECT id INTO review_id FROM users WHERE email = 'agent-review@demo.local' AND deleted_at IS NULL;
  SELECT id INTO blocked_id FROM users WHERE email = 'agent-blocked@demo.local' AND deleted_at IS NULL;
  SELECT id INTO candidate_id FROM users WHERE email = 'agent-candidate@demo.local' AND deleted_at IS NULL;
  SELECT id INTO ordinary_id FROM users WHERE email = 'ordinary-referrer@demo.local' AND deleted_at IS NULL;
  SELECT id INTO invitee_id FROM users WHERE email = 'ordinary-invitee@demo.local' AND deleted_at IS NULL;

  -- Keep the qualified candidate reusable across repeated E2E runs. If a
  -- previous staging test clicked "立即成为合伙人", reset only this demo
  -- candidate back to the pre-activation state so the upgrade path remains
  -- testable without wiping the whole staging database.
  UPDATE agent_withdrawal_requests
  SET status = 'failed',
      failed_at = NOW(),
      failure_reason = 'staging demo candidate reset',
      updated_at = NOW()
  WHERE agent_id = candidate_id
    AND status = 'processing';

  DELETE FROM agent_withdrawal_events
  WHERE withdrawal_id IN (
    SELECT id FROM agent_withdrawal_requests WHERE agent_id = candidate_id
  );
  DELETE FROM agent_withdrawal_requests WHERE agent_id = candidate_id;
  DELETE FROM agent_commission_conversions WHERE agent_id = candidate_id;
  DELETE FROM agent_cash_commission_entries WHERE agent_id = candidate_id;
  DELETE FROM agent_payment_qr_access_events WHERE agent_id = candidate_id;
  DELETE FROM agent_payment_profiles WHERE agent_id = candidate_id;
  DELETE FROM affiliate_agent_notices WHERE agent_id = candidate_id;
  DELETE FROM affiliate_bindings WHERE agent_id = candidate_id;
  DELETE FROM affiliate_link_rate_versions
  WHERE link_id IN (SELECT id FROM affiliate_links WHERE agent_id = candidate_id);
  DELETE FROM affiliate_links WHERE agent_id = candidate_id;
  DELETE FROM affiliate_performance_events
  WHERE (user_id = candidate_id AND event_type = 'agent_activated')
     OR direct_agent_id = candidate_id;
  UPDATE agent_principals
  SET status = 'candidate',
      risk_status = 'clear',
      risk_note = '',
      qualified_at = NULL,
      activated_at = NULL,
      reviewed_at = NULL,
      reviewed_by = NULL,
      updated_at = NOW()
  WHERE agent_id = candidate_id;

  -- The production guard correctly prevents payment-profile edits while a
  -- withdrawal is processing. For this staging demo re-seed, temporarily move
  -- our own demo processing withdrawals out of the way; the demo withdrawals
  -- are re-created as processing with fresh snapshots later in this script.
  UPDATE agent_withdrawal_requests
  SET status = 'failed',
      failed_at = NOW(),
      failure_reason = 'staging demo refresh',
      updated_at = NOW()
  WHERE idempotency_key IN ('demo:withdrawal:alpha:processing', 'demo:withdrawal:review:processing')
    AND status = 'processing';

  INSERT INTO agent_principals (agent_id, status, risk_status, risk_note, qualified_at, activated_at, reviewed_at, reviewed_by)
  VALUES
    (alpha_id, 'active', 'clear', '', NOW() - INTERVAL '10 days', NOW() - INTERVAL '9 days', NOW() - INTERVAL '9 days', admin_id),
    (review_id, 'active', 'review', 'staging 演示：有异常订单，先暂停发放，确认后再恢复。', NOW() - INTERVAL '8 days', NOW() - INTERVAL '7 days', NOW() - INTERVAL '1 day', admin_id),
    (blocked_id, 'active', 'blocked', 'staging 演示：疑似用小号互刷，已暂停邀请和提现。', NOW() - INTERVAL '8 days', NOW() - INTERVAL '7 days', NOW() - INTERVAL '1 day', admin_id)
  ON CONFLICT (agent_id) DO UPDATE SET
    status = EXCLUDED.status,
    risk_status = EXCLUDED.risk_status,
    risk_note = EXCLUDED.risk_note,
    qualified_at = EXCLUDED.qualified_at,
    activated_at = EXCLUDED.activated_at,
    reviewed_at = EXCLUDED.reviewed_at,
    reviewed_by = EXCLUDED.reviewed_by,
    updated_at = NOW();

  INSERT INTO agent_payment_profiles (
    agent_id, alipay_real_name, alipay_account, contact_phone, payment_note,
    alipay_qr_object_key, alipay_qr_content_type, alipay_qr_original_filename, alipay_qr_size,
    identity_fingerprint_hash, verification_status, verification_note, verified_at, verified_by
  )
  VALUES
    (alpha_id, '张三', 'alpha-pay@example.com', '13800000001', '演示账号，可扫码预览。', 'agent-payment-qrcodes/demo-alpha.png', 'image/png', 'demo-alpha.png', 1200, 'demo-alpha-fingerprint', 'verified', 'staging verified', NOW() - INTERVAL '6 days', admin_id),
    (review_id, '李四', 'review-pay@example.com', '13800000002', '演示：待审核资料。', 'agent-payment-qrcodes/demo-review.png', 'image/png', 'demo-review.png', 1200, 'demo-review-fingerprint', 'pending_review', '', NULL, NULL),
    (blocked_id, '王五', 'blocked-pay@example.com', '13800000003', '演示：已暂停合伙人的收款资料。', 'agent-payment-qrcodes/demo-blocked.png', 'image/png', 'demo-blocked.png', 1200, 'demo-blocked-fingerprint', 'verified', 'staging verified', NOW() - INTERVAL '5 days', admin_id)
  ON CONFLICT (agent_id) DO UPDATE SET
    alipay_real_name = EXCLUDED.alipay_real_name,
    alipay_account = EXCLUDED.alipay_account,
    contact_phone = EXCLUDED.contact_phone,
    payment_note = EXCLUDED.payment_note,
    alipay_qr_object_key = EXCLUDED.alipay_qr_object_key,
    alipay_qr_content_type = EXCLUDED.alipay_qr_content_type,
    alipay_qr_original_filename = EXCLUDED.alipay_qr_original_filename,
    alipay_qr_size = EXCLUDED.alipay_qr_size,
    identity_fingerprint_hash = EXCLUDED.identity_fingerprint_hash,
    verification_status = EXCLUDED.verification_status,
    verification_note = EXCLUDED.verification_note,
    verified_at = EXCLUDED.verified_at,
    verified_by = EXCLUDED.verified_by,
    updated_at = NOW()
  WHERE NOT EXISTS (
    SELECT 1
    FROM agent_withdrawal_requests w
    WHERE w.agent_id = agent_payment_profiles.agent_id
      AND w.status = 'processing'
  );

  INSERT INTO affiliate_links (agent_id, code, name, channel, is_default, status, current_rate_version)
  VALUES (alpha_id, 'AGALPHA5', '默认 5% / 5%', 'default', TRUE, 'active', 1)
  ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, is_default = EXCLUDED.is_default,
    status = EXCLUDED.status, current_rate_version = EXCLUDED.current_rate_version, updated_at = NOW()
  RETURNING id INTO alpha_default_link_id;
  INSERT INTO affiliate_link_rate_versions (link_id, version, customer_rebate_rate_bps, agent_commission_rate_bps, created_by, effective_at)
  VALUES (alpha_default_link_id, 1, 500, 500, alpha_id, NOW() - INTERVAL '9 days')
  ON CONFLICT (link_id, version) DO UPDATE SET
    customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
    agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
    created_by = EXCLUDED.created_by,
    effective_at = EXCLUDED.effective_at;

  INSERT INTO affiliate_links (agent_id, code, name, channel, is_default, status, current_rate_version)
  VALUES (alpha_id, 'AGALPHA0', '高佣金链接 0% / 10%', '社群成交', FALSE, 'active', 1)
  ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, status = EXCLUDED.status, current_rate_version = EXCLUDED.current_rate_version, updated_at = NOW()
  RETURNING id INTO alpha_zero_link_id;
  INSERT INTO affiliate_link_rate_versions (link_id, version, customer_rebate_rate_bps, agent_commission_rate_bps, created_by, effective_at)
  VALUES (alpha_zero_link_id, 1, 0, 1000, alpha_id, NOW() - INTERVAL '8 days')
  ON CONFLICT (link_id, version) DO UPDATE SET
    customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
    agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
    created_by = EXCLUDED.created_by,
    effective_at = EXCLUDED.effective_at;

  INSERT INTO affiliate_links (agent_id, code, name, channel, is_default, status, current_rate_version)
  VALUES (alpha_id, 'AGALPHA8', '客户返利链接 8% / 2%', '朋友圈活动', FALSE, 'active', 1)
  ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, status = EXCLUDED.status, current_rate_version = EXCLUDED.current_rate_version, updated_at = NOW()
  RETURNING id INTO alpha_vip_link_id;
  INSERT INTO affiliate_link_rate_versions (link_id, version, customer_rebate_rate_bps, agent_commission_rate_bps, created_by, effective_at)
  VALUES (alpha_vip_link_id, 1, 800, 200, alpha_id, NOW() - INTERVAL '7 days')
  ON CONFLICT (link_id, version) DO UPDATE SET
    customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
    agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
    created_by = EXCLUDED.created_by,
    effective_at = EXCLUDED.effective_at;

  INSERT INTO affiliate_links (agent_id, code, name, channel, is_default, status, current_rate_version)
  VALUES (alpha_id, 'AGALPHAPAUSE', '已停用历史活动', '旧海报', FALSE, 'paused', 1)
  ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, status = EXCLUDED.status, current_rate_version = EXCLUDED.current_rate_version, updated_at = NOW()
  RETURNING id INTO alpha_paused_link_id;
  INSERT INTO affiliate_link_rate_versions (link_id, version, customer_rebate_rate_bps, agent_commission_rate_bps, created_by, effective_at)
  VALUES (alpha_paused_link_id, 1, 300, 700, alpha_id, NOW() - INTERVAL '6 days')
  ON CONFLICT (link_id, version) DO UPDATE SET
    customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
    agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
    created_by = EXCLUDED.created_by,
    effective_at = EXCLUDED.effective_at;

  INSERT INTO affiliate_links (agent_id, code, name, channel, is_default, status, current_rate_version)
  VALUES (review_id, 'AGREVIEW5', '待确认默认链接', 'default', TRUE, 'active', 1)
  ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, is_default = EXCLUDED.is_default,
    status = EXCLUDED.status, current_rate_version = EXCLUDED.current_rate_version, updated_at = NOW()
  RETURNING id INTO review_default_link_id;
  INSERT INTO affiliate_link_rate_versions (link_id, version, customer_rebate_rate_bps, agent_commission_rate_bps, created_by, effective_at)
  VALUES (review_default_link_id, 1, 500, 500, review_id, NOW() - INTERVAL '7 days')
  ON CONFLICT (link_id, version) DO UPDATE SET
    customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
    agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
    created_by = EXCLUDED.created_by,
    effective_at = EXCLUDED.effective_at;

  INSERT INTO affiliate_links (agent_id, code, name, channel, is_default, status, current_rate_version)
  VALUES (blocked_id, 'AGBLOCK5', '暂停演示链接', 'default', TRUE, 'active', 1)
  ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name, channel = EXCLUDED.channel, is_default = EXCLUDED.is_default,
    status = EXCLUDED.status, current_rate_version = EXCLUDED.current_rate_version, updated_at = NOW()
  RETURNING id INTO blocked_default_link_id;
  INSERT INTO affiliate_link_rate_versions (link_id, version, customer_rebate_rate_bps, agent_commission_rate_bps, created_by, effective_at)
  VALUES (blocked_default_link_id, 1, 500, 500, blocked_id, NOW() - INTERVAL '7 days')
  ON CONFLICT (link_id, version) DO UPDATE SET
    customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
    agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
    created_by = EXCLUDED.created_by,
    effective_at = EXCLUDED.effective_at;

  INSERT INTO affiliate_performance_events (user_id, direct_agent_id, event_type, amount_micros, source_type, source_id, event_key, occurred_at, metadata)
  VALUES
    (alpha_id, NULL, 'agent_activated', 0, 'qualification', NULL, 'demo:agent-activated:alpha', NOW() - INTERVAL '9 days', '{"program_mode":"live","staging_demo":true}'::jsonb),
    (review_id, NULL, 'agent_activated', 0, 'qualification', NULL, 'demo:agent-activated:review', NOW() - INTERVAL '7 days', '{"program_mode":"live","staging_demo":true}'::jsonb),
    (blocked_id, NULL, 'agent_activated', 0, 'qualification', NULL, 'demo:agent-activated:blocked', NOW() - INTERVAL '7 days', '{"program_mode":"live","staging_demo":true}'::jsonb),
    (alpha_id, NULL, 'confirmed_consumption', 350000000, 'usage', NULL, 'demo:confirmed:self:alpha', NOW() - INTERVAL '5 days', '{"program_mode":"live","staging_demo":true,"kind":"self"}'::jsonb),
    (candidate_id, NULL, 'confirmed_consumption', 150000000, 'usage', NULL, 'demo:confirmed:self:candidate', NOW() - INTERVAL '5 days', '{"program_mode":"live","staging_demo":true,"kind":"self"}'::jsonb)
  ON CONFLICT (event_key) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    direct_agent_id = EXCLUDED.direct_agent_id,
    event_type = EXCLUDED.event_type,
    amount_micros = EXCLUDED.amount_micros,
    source_type = EXCLUDED.source_type,
    source_id = EXCLUDED.source_id,
    occurred_at = EXCLUDED.occurred_at,
    metadata = EXCLUDED.metadata;

  FOR i IN 1..12 LOOP
    customer_email := 'alpha-customer-' || lpad(i::text, 2, '0') || '@demo.local';
    amount_micros := (550 + i * 55)::bigint * 1000000;
    IF i <= 6 THEN
      selected_link_id := alpha_default_link_id;
      customer_bps := 500;
      agent_bps := 500;
    ELSIF i <= 9 THEN
      selected_link_id := alpha_zero_link_id;
      customer_bps := 0;
      agent_bps := 1000;
    ELSE
      selected_link_id := alpha_vip_link_id;
      customer_bps := 800;
      agent_bps := 200;
    END IF;

    INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, agent_id, total_recharged, first_recharged, first_invited_topup_at, last_active_at, wechat)
    VALUES (
      customer_email, demo_password_hash, 'user', 0, 5, 'active',
      'Alpha 客户 ' || lpad(i::text, 2, '0'), 'staging demo alpha customer',
      'ALPHACUST' || lpad(i::text, 2, '0'), alpha_id, alpha_id,
      (amount_micros::numeric / 1000000), TRUE, NOW() - (i || ' days')::interval, NOW() - (i || ' hours')::interval, ''
    )
    ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET
      password_hash = EXCLUDED.password_hash,
      username = EXCLUDED.username,
      notes = EXCLUDED.notes,
      invite_code = EXCLUDED.invite_code,
      inviter_id = EXCLUDED.inviter_id,
      agent_id = EXCLUDED.agent_id,
      total_recharged = EXCLUDED.total_recharged,
      first_recharged = EXCLUDED.first_recharged,
      first_invited_topup_at = EXCLUDED.first_invited_topup_at,
      last_active_at = EXCLUDED.last_active_at,
      updated_at = NOW()
    RETURNING id INTO customer_id;

    INSERT INTO affiliate_bindings (
      customer_user_id, inviter_user_id, binding_kind, agent_id,
      affiliate_link_id, link_rate_version,
      customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps,
      bound_at
    )
    VALUES (customer_id, alpha_id, 'agent', alpha_id, selected_link_id, 1, customer_bps, agent_bps, NOW() - INTERVAL '8 days')
    ON CONFLICT (customer_user_id) DO UPDATE SET
      inviter_user_id = EXCLUDED.inviter_user_id,
      binding_kind = EXCLUDED.binding_kind,
      agent_id = EXCLUDED.agent_id,
      affiliate_link_id = EXCLUDED.affiliate_link_id,
      link_rate_version = EXCLUDED.link_rate_version,
      customer_rebate_rate_snapshot_bps = EXCLUDED.customer_rebate_rate_snapshot_bps,
      agent_commission_rate_snapshot_bps = EXCLUDED.agent_commission_rate_snapshot_bps,
      updated_at = NOW();

    demo_order_no := 'DEMO-TOPUP-ALPHA-' || lpad(i::text, 2, '0');
    INSERT INTO topup_orders (order_no, user_id, amount_cny_fen, pay_type, status, completed_at, invoice_status)
    VALUES (demo_order_no, customer_id, (amount_micros / 10000)::integer, 'alipay', 'completed', NOW() - (i || ' days')::interval, 'none')
    ON CONFLICT (order_no) DO UPDATE SET
      user_id = EXCLUDED.user_id,
      amount_cny_fen = EXCLUDED.amount_cny_fen,
      pay_type = EXCLUDED.pay_type,
      status = EXCLUDED.status,
      completed_at = EXCLUDED.completed_at,
      invoice_status = EXCLUDED.invoice_status,
      updated_at = NOW()
    RETURNING id INTO topup_id;

    INSERT INTO affiliate_first_paid_purchases (user_id, purchase_type, source_id, purchase_key, amount_micros, occurred_at)
    VALUES (customer_id, 'balance_topup', topup_id, 'demo:first-paid:alpha:' || i, amount_micros, NOW() - (i || ' days')::interval)
    ON CONFLICT (user_id) DO UPDATE SET
      purchase_type = EXCLUDED.purchase_type,
      source_id = EXCLUDED.source_id,
      purchase_key = EXCLUDED.purchase_key,
      amount_micros = EXCLUDED.amount_micros,
      occurred_at = EXCLUDED.occurred_at;

    INSERT INTO affiliate_performance_events (
      user_id, direct_agent_id, event_type, amount_micros,
      source_type, source_id, event_key, occurred_at, metadata
    )
    VALUES (
      customer_id, alpha_id, 'confirmed_consumption', amount_micros,
      'usage', NULL, 'demo:confirmed:alpha:' || i,
      NOW() - (i || ' days')::interval,
      jsonb_build_object('program_mode', 'live', 'staging_demo', TRUE, 'link_id', selected_link_id, 'rate_version', 1)
    )
    ON CONFLICT (event_key) DO UPDATE SET
      user_id = EXCLUDED.user_id,
      direct_agent_id = EXCLUDED.direct_agent_id,
      amount_micros = EXCLUDED.amount_micros,
      occurred_at = EXCLUDED.occurred_at,
      metadata = EXCLUDED.metadata
    RETURNING id INTO event_id;

    reward_micros := (amount_micros * customer_bps / 10000);
    IF reward_micros > 0 THEN
      INSERT INTO affiliate_reward_entries (
        beneficiary_user_id, consumer_user_id, reward_type, asset_type,
        amount_micros, source_amount_micros, rate_bps, status, available_at,
        source_type, source_id, idempotency_key, metadata, posted_at
      )
      VALUES (
        customer_id, customer_id, 'customer_rebate', 'platform_credit',
        reward_micros, amount_micros, customer_bps, 'posted', NOW() - (i || ' days')::interval,
        'confirmed_consumption', event_id, 'demo:reward:alpha:' || i,
        '{"staging_demo":true}'::jsonb, NOW() - (i || ' days')::interval
      )
      ON CONFLICT (idempotency_key) DO UPDATE SET
        beneficiary_user_id = EXCLUDED.beneficiary_user_id,
        consumer_user_id = EXCLUDED.consumer_user_id,
        amount_micros = EXCLUDED.amount_micros,
        source_amount_micros = EXCLUDED.source_amount_micros,
        rate_bps = EXCLUDED.rate_bps,
        status = EXCLUDED.status,
        available_at = EXCLUDED.available_at,
        source_id = EXCLUDED.source_id,
        metadata = EXCLUDED.metadata,
        posted_at = EXCLUDED.posted_at
      RETURNING id INTO reward_id;

      INSERT INTO balance_lots (user_id, source_type, source_id, source_key, original_amount_micros, remaining_amount_micros, affiliate_eligible, occurred_at)
      VALUES (customer_id, 'customer_rebate', reward_id, 'demo:lot:customer-rebate:' || reward_id, reward_micros, reward_micros, FALSE, NOW() - (i || ' days')::interval)
      ON CONFLICT (source_key) DO UPDATE SET
        original_amount_micros = EXCLUDED.original_amount_micros,
        remaining_amount_micros = EXCLUDED.remaining_amount_micros,
        updated_at = NOW();
    END IF;

    cash_micros := (amount_micros * agent_bps / 10000);
    IF cash_micros > 0 THEN
      INSERT INTO agent_cash_commission_entries (
        agent_id, consumer_user_id, entry_type, amount_micros, source_amount_micros,
        customer_rebate_rate_bps, agent_commission_rate_bps, posting_status,
        source_type, source_id, idempotency_key, metadata, occurred_at
      )
      VALUES (
        alpha_id, customer_id, 'earned', cash_micros, amount_micros,
        customer_bps, agent_bps, 'posted',
        'confirmed_consumption', event_id, 'demo:cash:alpha:' || i,
        '{"staging_demo":true}'::jsonb, NOW() - (i || ' days')::interval
      )
      ON CONFLICT (idempotency_key) DO UPDATE SET
        agent_id = EXCLUDED.agent_id,
        consumer_user_id = EXCLUDED.consumer_user_id,
        amount_micros = EXCLUDED.amount_micros,
        source_amount_micros = EXCLUDED.source_amount_micros,
        customer_rebate_rate_bps = EXCLUDED.customer_rebate_rate_bps,
        agent_commission_rate_bps = EXCLUDED.agent_commission_rate_bps,
        posting_status = EXCLUDED.posting_status,
        source_id = EXCLUDED.source_id,
        metadata = EXCLUDED.metadata,
        occurred_at = EXCLUDED.occurred_at
      RETURNING id INTO cash_id;
    END IF;

    INSERT INTO api_keys (user_id, key, name, group_id, status, quota, quota_used, last_used_at)
    VALUES (customer_id, 'sk-demo-alpha-' || lpad(i::text, 2, '0'), 'Demo Alpha Key ' || lpad(i::text, 2, '0'), demo_group_id, 'active', 0, amount_micros::numeric / 1000000, NOW() - (i || ' hours')::interval)
    ON CONFLICT (key) DO UPDATE SET
      user_id = EXCLUDED.user_id,
      name = EXCLUDED.name,
      group_id = EXCLUDED.group_id,
      status = EXCLUDED.status,
      quota_used = EXCLUDED.quota_used,
      last_used_at = EXCLUDED.last_used_at,
      updated_at = NOW()
    RETURNING id INTO demo_api_key_id;

    INSERT INTO usage_logs (
      user_id, api_key_id, account_id, request_id, model,
      input_tokens, output_tokens, input_cost, output_cost, total_cost, actual_cost,
      stream, duration_ms, group_id, rate_multiplier, billing_type,
      account_rate_multiplier, account_stats_cost, created_at,
      requested_model, upstream_model, inbound_endpoint, upstream_endpoint
    )
    VALUES (
      customer_id, demo_api_key_id, demo_account_id, 'demo-alpha-' || lpad(i::text, 2, '0'), 'gpt-5.6-sol',
      2000 + i * 100, 800 + i * 50, 0.0100000000, 0.0200000000,
      (amount_micros::numeric / 1000000) / 0.5, amount_micros::numeric / 1000000,
      TRUE, 1200 + i * 10, demo_group_id, 0.5000, 0,
      0.1850, ((amount_micros::numeric / 1000000) / 0.5) * 0.1850,
      NOW() - (i || ' hours')::interval,
      'gpt-5.6-sol', 'gpt-5.6-sol', '/v1/responses', '/v1/responses'
    )
    ON CONFLICT (request_id, api_key_id) DO UPDATE SET
      actual_cost = EXCLUDED.actual_cost,
      total_cost = EXCLUDED.total_cost,
      rate_multiplier = EXCLUDED.rate_multiplier,
      account_rate_multiplier = EXCLUDED.account_rate_multiplier,
      account_stats_cost = EXCLUDED.account_stats_cost,
      created_at = EXCLUDED.created_at
    RETURNING id INTO usage_id;
  END LOOP;

  FOR i IN 1..10 LOOP
    customer_email := 'candidate-customer-' || lpad(i::text, 2, '0') || '@demo.local';
    amount_micros := 120000000;
    INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, total_recharged, first_recharged, first_invited_topup_at, last_active_at, wechat)
    VALUES (
      customer_email, demo_password_hash, 'user', 0, 5, 'active',
      'Candidate 客户 ' || lpad(i::text, 2, '0'), 'staging demo candidate customer',
      'CANDCUST' || lpad(i::text, 2, '0'), candidate_id,
      120, TRUE, NOW() - (i || ' days')::interval, NOW() - (i || ' hours')::interval, ''
    )
    ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET
      password_hash = EXCLUDED.password_hash,
      username = EXCLUDED.username,
      notes = EXCLUDED.notes,
      invite_code = EXCLUDED.invite_code,
      inviter_id = EXCLUDED.inviter_id,
      total_recharged = EXCLUDED.total_recharged,
      first_recharged = EXCLUDED.first_recharged,
      first_invited_topup_at = EXCLUDED.first_invited_topup_at,
      last_active_at = EXCLUDED.last_active_at,
      updated_at = NOW()
    RETURNING id INTO customer_id;

    INSERT INTO affiliate_bindings (
      customer_user_id, inviter_user_id, binding_kind,
      customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps,
      bound_at
    )
    VALUES (customer_id, candidate_id, 'ordinary', 0, 0, NOW() - INTERVAL '7 days')
    ON CONFLICT (customer_user_id) DO UPDATE SET
      inviter_user_id = EXCLUDED.inviter_user_id,
      binding_kind = EXCLUDED.binding_kind,
      agent_id = NULL,
      affiliate_link_id = NULL,
      link_rate_version = NULL,
      customer_rebate_rate_snapshot_bps = 0,
      agent_commission_rate_snapshot_bps = 0,
      updated_at = NOW();

    INSERT INTO affiliate_performance_events (
      user_id, direct_agent_id, event_type, amount_micros,
      source_type, source_id, event_key, occurred_at, metadata
    )
    VALUES (
      customer_id, candidate_id, 'confirmed_consumption', amount_micros,
      'usage', NULL, 'demo:confirmed:candidate:' || i,
      NOW() - (i || ' days')::interval,
      '{"program_mode":"live","staging_demo":true,"ordinary_invite":true}'::jsonb
    )
    ON CONFLICT (event_key) DO UPDATE SET
      user_id = EXCLUDED.user_id,
      direct_agent_id = EXCLUDED.direct_agent_id,
      amount_micros = EXCLUDED.amount_micros,
      occurred_at = EXCLUDED.occurred_at,
      metadata = EXCLUDED.metadata;
  END LOOP;

  INSERT INTO affiliate_bindings (
    customer_user_id, inviter_user_id, binding_kind,
    customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps,
    bound_at
  )
  VALUES (invitee_id, ordinary_id, 'ordinary', 0, 0, NOW() - INTERVAL '3 days')
  ON CONFLICT (customer_user_id) DO UPDATE SET
    inviter_user_id = EXCLUDED.inviter_user_id,
    binding_kind = EXCLUDED.binding_kind,
    agent_id = NULL,
    affiliate_link_id = NULL,
    link_rate_version = NULL,
    customer_rebate_rate_snapshot_bps = 0,
    agent_commission_rate_snapshot_bps = 0,
    updated_at = NOW();
  UPDATE users
  SET inviter_id = ordinary_id,
      total_recharged = 80,
      first_recharged = TRUE,
      first_invited_topup_at = NOW() - INTERVAL '2 days',
      updated_at = NOW()
  WHERE id = invitee_id;

  INSERT INTO topup_orders (order_no, user_id, amount_cny_fen, pay_type, status, completed_at, invoice_status)
  VALUES ('DEMO-TOPUP-ORDINARY-80', invitee_id, 8000, 'alipay', 'completed', NOW() - INTERVAL '2 days', 'none')
  ON CONFLICT (order_no) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    amount_cny_fen = EXCLUDED.amount_cny_fen,
    status = EXCLUDED.status,
    completed_at = EXCLUDED.completed_at,
    invoice_status = EXCLUDED.invoice_status,
    updated_at = NOW()
  RETURNING id INTO topup_id;

  INSERT INTO affiliate_first_paid_purchases (user_id, purchase_type, source_id, purchase_key, amount_micros, occurred_at)
  VALUES (invitee_id, 'balance_topup', topup_id, 'demo:first-paid:ordinary-invitee', 80000000, NOW() - INTERVAL '2 days')
  ON CONFLICT (user_id) DO UPDATE SET
    purchase_type = EXCLUDED.purchase_type,
    source_id = EXCLUDED.source_id,
    purchase_key = EXCLUDED.purchase_key,
    amount_micros = EXCLUDED.amount_micros,
    occurred_at = EXCLUDED.occurred_at;

  INSERT INTO affiliate_reward_entries (
    beneficiary_user_id, consumer_user_id, reward_type, asset_type,
    amount_micros, source_amount_micros, rate_bps, status, available_at,
    source_type, source_id, idempotency_key, metadata, posted_at
  )
  VALUES (
    ordinary_id, invitee_id, 'ordinary_referral', 'platform_credit',
    4000000, 80000000, 500, 'posted', NOW() - INTERVAL '2 days',
    'first_paid_purchase', topup_id, 'demo:reward:ordinary-referrer',
    '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '2 days'
  )
  ON CONFLICT (idempotency_key) DO UPDATE SET
    beneficiary_user_id = EXCLUDED.beneficiary_user_id,
    consumer_user_id = EXCLUDED.consumer_user_id,
    amount_micros = EXCLUDED.amount_micros,
    source_amount_micros = EXCLUDED.source_amount_micros,
    status = EXCLUDED.status,
    source_id = EXCLUDED.source_id,
    posted_at = EXCLUDED.posted_at
  RETURNING id INTO reward_id;
  INSERT INTO balance_lots (user_id, source_type, source_id, source_key, original_amount_micros, remaining_amount_micros, affiliate_eligible, occurred_at)
  VALUES (ordinary_id, 'referral_bonus', reward_id, 'demo:lot:ordinary-referrer', 4000000, 4000000, FALSE, NOW() - INTERVAL '2 days')
  ON CONFLICT (source_key) DO UPDATE SET
    original_amount_micros = EXCLUDED.original_amount_micros,
    remaining_amount_micros = EXCLUDED.remaining_amount_micros,
    updated_at = NOW();

  INSERT INTO affiliate_reward_entries (
    beneficiary_user_id, consumer_user_id, reward_type, asset_type,
    amount_micros, source_amount_micros, rate_bps, status, available_at,
    source_type, source_id, idempotency_key, metadata
  )
  VALUES (
    invitee_id, invitee_id, 'first_paid_bonus', 'platform_credit',
    5000000, 80000000, NULL, 'pending', NOW() + INTERVAL '1 day',
    'first_paid_purchase', topup_id, 'demo:reward:ordinary-invitee-t1',
    '{"staging_demo":true,"beijing_t_plus_1":true}'::jsonb
  )
  ON CONFLICT (idempotency_key) DO UPDATE SET
    beneficiary_user_id = EXCLUDED.beneficiary_user_id,
    consumer_user_id = EXCLUDED.consumer_user_id,
    amount_micros = EXCLUDED.amount_micros,
    source_amount_micros = EXCLUDED.source_amount_micros,
    status = EXCLUDED.status,
    available_at = EXCLUDED.available_at,
    source_id = EXCLUDED.source_id;

  -- 暂缓发放演示：review / blocked 各一条消费事件、客户额度暂缓发放、现金暂缓发放。
  FOR i IN 1..2 LOOP
    IF i = 1 THEN
      customer_email := 'review-customer-01@demo.local';
      selected_link_id := review_default_link_id;
      amount_micros := 400000000;
      customer_id := NULL;
      INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, agent_id, total_recharged, first_recharged, wechat)
      VALUES (customer_email, demo_password_hash, 'user', 0, 5, 'active', '待确认客户', 'staging demo review customer', 'REVCUST01', review_id, review_id, 400, TRUE, '')
      ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET
        inviter_id = EXCLUDED.inviter_id, agent_id = EXCLUDED.agent_id, username = EXCLUDED.username, updated_at = NOW()
      RETURNING id INTO customer_id;
      agent_bps := 500;
      customer_bps := 500;
      INSERT INTO affiliate_bindings (customer_user_id, inviter_user_id, binding_kind, agent_id, affiliate_link_id, link_rate_version, customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps, bound_at)
      VALUES (customer_id, review_id, 'agent', review_id, selected_link_id, 1, customer_bps, agent_bps, NOW() - INTERVAL '3 days')
      ON CONFLICT (customer_user_id) DO UPDATE SET inviter_user_id = EXCLUDED.inviter_user_id, binding_kind = EXCLUDED.binding_kind, agent_id = EXCLUDED.agent_id, affiliate_link_id = EXCLUDED.affiliate_link_id, link_rate_version = 1, customer_rebate_rate_snapshot_bps = customer_bps, agent_commission_rate_snapshot_bps = agent_bps, updated_at = NOW();
      INSERT INTO affiliate_performance_events (user_id, direct_agent_id, event_type, amount_micros, source_type, source_id, event_key, occurred_at, metadata)
      VALUES (customer_id, review_id, 'confirmed_consumption', amount_micros, 'usage', NULL, 'demo:confirmed:review-hold', NOW() - INTERVAL '2 days', '{"program_mode":"live","staging_demo":true}'::jsonb)
      ON CONFLICT (event_key) DO UPDATE SET user_id = EXCLUDED.user_id, direct_agent_id = EXCLUDED.direct_agent_id, amount_micros = EXCLUDED.amount_micros, occurred_at = EXCLUDED.occurred_at, metadata = EXCLUDED.metadata
      RETURNING id INTO event_id;
      INSERT INTO affiliate_reward_entries (beneficiary_user_id, consumer_user_id, reward_type, asset_type, amount_micros, source_amount_micros, rate_bps, status, available_at, source_type, source_id, idempotency_key, metadata)
      VALUES (customer_id, customer_id, 'customer_rebate', 'platform_credit', 20000000, amount_micros, 500, 'risk_hold', NOW() - INTERVAL '2 days', 'confirmed_consumption', event_id, 'demo:reward:review-hold', '{"staging_demo":true}'::jsonb)
      ON CONFLICT (idempotency_key) DO UPDATE SET beneficiary_user_id = EXCLUDED.beneficiary_user_id, consumer_user_id = EXCLUDED.consumer_user_id, amount_micros = EXCLUDED.amount_micros, source_amount_micros = EXCLUDED.source_amount_micros, status = EXCLUDED.status, source_id = EXCLUDED.source_id;
      INSERT INTO agent_cash_commission_entries (agent_id, consumer_user_id, entry_type, amount_micros, source_amount_micros, customer_rebate_rate_bps, agent_commission_rate_bps, posting_status, source_type, source_id, idempotency_key, metadata, occurred_at)
      VALUES (review_id, customer_id, 'earned', 20000000, amount_micros, 500, 500, 'risk_hold', 'confirmed_consumption', event_id, 'demo:cash:review-hold', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '2 days')
      ON CONFLICT (idempotency_key) DO UPDATE SET agent_id = EXCLUDED.agent_id, consumer_user_id = EXCLUDED.consumer_user_id, amount_micros = EXCLUDED.amount_micros, source_amount_micros = EXCLUDED.source_amount_micros, posting_status = EXCLUDED.posting_status, source_id = EXCLUDED.source_id, occurred_at = EXCLUDED.occurred_at;
    ELSE
      customer_email := 'blocked-customer-01@demo.local';
      selected_link_id := blocked_default_link_id;
      amount_micros := 300000000;
      customer_id := NULL;
      INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, agent_id, total_recharged, first_recharged, wechat)
      VALUES (customer_email, demo_password_hash, 'user', 0, 5, 'active', '暂停演示客户', 'staging demo paused customer', 'BLKCUST01', blocked_id, blocked_id, 300, TRUE, '')
      ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET
        inviter_id = EXCLUDED.inviter_id, agent_id = EXCLUDED.agent_id, username = EXCLUDED.username, updated_at = NOW()
      RETURNING id INTO customer_id;
      agent_bps := 500;
      customer_bps := 500;
      INSERT INTO affiliate_bindings (customer_user_id, inviter_user_id, binding_kind, agent_id, affiliate_link_id, link_rate_version, customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps, bound_at)
      VALUES (customer_id, blocked_id, 'agent', blocked_id, selected_link_id, 1, customer_bps, agent_bps, NOW() - INTERVAL '3 days')
      ON CONFLICT (customer_user_id) DO UPDATE SET inviter_user_id = EXCLUDED.inviter_user_id, binding_kind = EXCLUDED.binding_kind, agent_id = EXCLUDED.agent_id, affiliate_link_id = EXCLUDED.affiliate_link_id, link_rate_version = 1, customer_rebate_rate_snapshot_bps = customer_bps, agent_commission_rate_snapshot_bps = agent_bps, updated_at = NOW();
      INSERT INTO affiliate_performance_events (user_id, direct_agent_id, event_type, amount_micros, source_type, source_id, event_key, occurred_at, metadata)
      VALUES (customer_id, blocked_id, 'confirmed_consumption', amount_micros, 'usage', NULL, 'demo:confirmed:blocked-hold', NOW() - INTERVAL '2 days', '{"program_mode":"live","staging_demo":true}'::jsonb)
      ON CONFLICT (event_key) DO UPDATE SET user_id = EXCLUDED.user_id, direct_agent_id = EXCLUDED.direct_agent_id, amount_micros = EXCLUDED.amount_micros, occurred_at = EXCLUDED.occurred_at, metadata = EXCLUDED.metadata
      RETURNING id INTO event_id;
      INSERT INTO affiliate_reward_entries (beneficiary_user_id, consumer_user_id, reward_type, asset_type, amount_micros, source_amount_micros, rate_bps, status, available_at, source_type, source_id, idempotency_key, metadata)
      VALUES (customer_id, customer_id, 'customer_rebate', 'platform_credit', 15000000, amount_micros, 500, 'risk_hold', NOW() - INTERVAL '2 days', 'confirmed_consumption', event_id, 'demo:reward:blocked-hold', '{"staging_demo":true}'::jsonb)
      ON CONFLICT (idempotency_key) DO UPDATE SET beneficiary_user_id = EXCLUDED.beneficiary_user_id, consumer_user_id = EXCLUDED.consumer_user_id, amount_micros = EXCLUDED.amount_micros, source_amount_micros = EXCLUDED.source_amount_micros, status = EXCLUDED.status, source_id = EXCLUDED.source_id;
      INSERT INTO agent_cash_commission_entries (agent_id, consumer_user_id, entry_type, amount_micros, source_amount_micros, customer_rebate_rate_bps, agent_commission_rate_bps, posting_status, source_type, source_id, idempotency_key, metadata, occurred_at)
      VALUES (blocked_id, customer_id, 'earned', 15000000, amount_micros, 500, 500, 'risk_hold', 'confirmed_consumption', event_id, 'demo:cash:blocked-hold', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '2 days')
      ON CONFLICT (idempotency_key) DO UPDATE SET agent_id = EXCLUDED.agent_id, consumer_user_id = EXCLUDED.consumer_user_id, amount_micros = EXCLUDED.amount_micros, source_amount_micros = EXCLUDED.source_amount_micros, posting_status = EXCLUDED.posting_status, source_id = EXCLUDED.source_id, occurred_at = EXCLUDED.occurred_at;
    END IF;
  END LOOP;

  INSERT INTO affiliate_risk_actions (agent_id, action_type, previous_risk_status, next_risk_status, reason, released_reward_count, released_reward_micros, released_cash_count, released_cash_micros, operator_id, metadata)
  SELECT review_id, 'review', 'clear', 'review', 'staging 演示：异常订单待确认', 0, 0, 0, 0, admin_id, '{"staging_demo":true}'::jsonb
  WHERE NOT EXISTS (SELECT 1 FROM affiliate_risk_actions WHERE agent_id = review_id AND next_risk_status = 'review' AND reason = 'staging 演示：异常订单待确认');

  INSERT INTO affiliate_risk_actions (agent_id, action_type, previous_risk_status, next_risk_status, reason, released_reward_count, released_reward_micros, released_cash_count, released_cash_micros, operator_id, metadata)
  SELECT blocked_id, 'block', 'clear', 'blocked', 'staging 演示：疑似自循环拉新', 0, 0, 0, 0, admin_id, '{"staging_demo":true}'::jsonb
  WHERE NOT EXISTS (SELECT 1 FROM affiliate_risk_actions WHERE agent_id = blocked_id AND next_risk_status = 'blocked' AND reason = 'staging 演示：疑似自循环拉新');

  INSERT INTO agent_commission_conversions (agent_id, cash_amount_micros, credit_amount_micros, multiplier_millis, idempotency_key)
  VALUES (alpha_id, 50000000, 60000000, 1200, 'demo:conversion:alpha:50')
  ON CONFLICT (agent_id, idempotency_key) DO UPDATE SET
    cash_amount_micros = EXCLUDED.cash_amount_micros,
    credit_amount_micros = EXCLUDED.credit_amount_micros,
    multiplier_millis = EXCLUDED.multiplier_millis
  RETURNING id INTO conversion_id;

  INSERT INTO agent_cash_commission_entries (
    agent_id, entry_type, amount_micros, posting_status,
    source_type, source_id, idempotency_key, metadata, occurred_at
  )
  VALUES (
    alpha_id, 'conversion', -50000000, 'posted',
    'commission_conversion', conversion_id, 'demo:cash:conversion:alpha:50',
    '{"staging_demo":true,"credit_amount_micros":60000000}'::jsonb, NOW() - INTERVAL '1 day'
  )
  ON CONFLICT (idempotency_key) DO UPDATE SET
    amount_micros = EXCLUDED.amount_micros,
    posting_status = EXCLUDED.posting_status,
    source_id = EXCLUDED.source_id,
    metadata = EXCLUDED.metadata,
    occurred_at = EXCLUDED.occurred_at;

  INSERT INTO balance_lots (user_id, source_type, source_id, source_key, original_amount_micros, remaining_amount_micros, affiliate_eligible, occurred_at)
  VALUES (alpha_id, 'commission_conversion', conversion_id, 'demo:lot:conversion:alpha:50', 60000000, 60000000, FALSE, NOW() - INTERVAL '1 day')
  ON CONFLICT (source_key) DO UPDATE SET
    original_amount_micros = EXCLUDED.original_amount_micros,
    remaining_amount_micros = EXCLUDED.remaining_amount_micros,
    updated_at = NOW();

  INSERT INTO agent_withdrawal_requests (
    agent_id, amount_micros, status, idempotency_key,
    payment_alipay_real_name, payment_alipay_account, payment_contact_phone, payment_note,
    payment_qr_object_key, payment_qr_content_type, payment_qr_original_filename,
    requested_at, due_at, payment_reference, failure_reason
  )
  VALUES (
    alpha_id, 120000000, 'processing', 'demo:withdrawal:alpha:processing',
    '张三', 'alpha-pay@example.com', '13800000001', '演示账号，可扫码预览。',
    'agent-payment-qrcodes/demo-alpha.png', 'image/png', 'demo-alpha.png',
    NOW() - INTERVAL '6 hours', NOW() + INTERVAL '18 hours', '', ''
  )
  ON CONFLICT (agent_id, idempotency_key) DO UPDATE SET
    amount_micros = EXCLUDED.amount_micros,
    status = EXCLUDED.status,
    payment_alipay_real_name = EXCLUDED.payment_alipay_real_name,
    payment_alipay_account = EXCLUDED.payment_alipay_account,
    payment_contact_phone = EXCLUDED.payment_contact_phone,
    payment_note = EXCLUDED.payment_note,
    payment_qr_object_key = EXCLUDED.payment_qr_object_key,
    payment_qr_content_type = EXCLUDED.payment_qr_content_type,
    payment_qr_original_filename = EXCLUDED.payment_qr_original_filename,
    requested_at = EXCLUDED.requested_at,
    due_at = EXCLUDED.due_at,
    paid_at = NULL,
    failed_at = NULL,
    payment_reference = '',
    failure_reason = '',
    updated_at = NOW()
  RETURNING id INTO withdrawal_request_id;
  INSERT INTO agent_cash_commission_entries (agent_id, entry_type, amount_micros, posting_status, source_type, source_id, idempotency_key, metadata, occurred_at)
  VALUES (alpha_id, 'withdrawal_hold', -120000000, 'posted', 'withdrawal', withdrawal_request_id, 'demo:cash:withdrawal-hold:alpha:processing', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '6 hours')
  ON CONFLICT (idempotency_key) DO UPDATE SET amount_micros = EXCLUDED.amount_micros, source_id = EXCLUDED.source_id, occurred_at = EXCLUDED.occurred_at;
  INSERT INTO agent_withdrawal_events (withdrawal_id, agent_id, event_type, previous_status, next_status, operator_id, note, metadata, created_at)
  SELECT withdrawal_request_id, alpha_id, 'requested', NULL, 'processing', alpha_id, 'staging demo request', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '6 hours'
  WHERE NOT EXISTS (SELECT 1 FROM agent_withdrawal_events WHERE agent_withdrawal_events.withdrawal_id = withdrawal_request_id AND event_type = 'requested');

  INSERT INTO agent_withdrawal_requests (
    agent_id, amount_micros, status, idempotency_key,
    payment_alipay_real_name, payment_alipay_account, payment_contact_phone, payment_note,
    payment_qr_object_key, payment_qr_content_type, payment_qr_original_filename,
    requested_at, due_at, paid_at, handled_by, payment_reference, failure_reason
  )
  VALUES (
    alpha_id, 80000000, 'paid', 'demo:withdrawal:alpha:paid',
    '张三', 'alpha-pay@example.com', '13800000001', '演示账号，可扫码预览。',
    'agent-payment-qrcodes/demo-alpha.png', 'image/png', 'demo-alpha.png',
    NOW() - INTERVAL '4 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', admin_id, 'ALI-DEMO-PAID-001', ''
  )
  ON CONFLICT (agent_id, idempotency_key) DO UPDATE SET
    amount_micros = EXCLUDED.amount_micros,
    status = EXCLUDED.status,
    payment_alipay_real_name = EXCLUDED.payment_alipay_real_name,
    payment_alipay_account = EXCLUDED.payment_alipay_account,
    payment_contact_phone = EXCLUDED.payment_contact_phone,
    payment_note = EXCLUDED.payment_note,
    payment_qr_object_key = EXCLUDED.payment_qr_object_key,
    payment_qr_content_type = EXCLUDED.payment_qr_content_type,
    payment_qr_original_filename = EXCLUDED.payment_qr_original_filename,
    requested_at = EXCLUDED.requested_at,
    due_at = EXCLUDED.due_at,
    paid_at = EXCLUDED.paid_at,
    handled_by = EXCLUDED.handled_by,
    payment_reference = EXCLUDED.payment_reference,
    failure_reason = '',
    updated_at = NOW()
  RETURNING id INTO withdrawal_request_id;
  INSERT INTO agent_cash_commission_entries (agent_id, entry_type, amount_micros, posting_status, source_type, source_id, idempotency_key, metadata, occurred_at)
  VALUES (alpha_id, 'withdrawal_hold', -80000000, 'posted', 'withdrawal', withdrawal_request_id, 'demo:cash:withdrawal-hold:alpha:paid', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '4 days')
  ON CONFLICT (idempotency_key) DO UPDATE SET amount_micros = EXCLUDED.amount_micros, source_id = EXCLUDED.source_id, occurred_at = EXCLUDED.occurred_at;

  INSERT INTO agent_withdrawal_requests (
    agent_id, amount_micros, status, idempotency_key,
    payment_alipay_real_name, payment_alipay_account, payment_contact_phone, payment_note,
    payment_qr_object_key, payment_qr_content_type, payment_qr_original_filename,
    requested_at, due_at, payment_reference, failure_reason
  )
  VALUES (
    review_id, 60000000, 'processing', 'demo:withdrawal:review:processing',
    '李四', 'review-pay@example.com', '13800000002', '演示：待审核资料。',
    'agent-payment-qrcodes/demo-review.png', 'image/png', 'demo-review.png',
    NOW() - INTERVAL '12 hours', NOW() + INTERVAL '12 hours', '', ''
  )
  ON CONFLICT (agent_id, idempotency_key) DO UPDATE SET
    amount_micros = EXCLUDED.amount_micros,
    status = EXCLUDED.status,
    payment_alipay_real_name = EXCLUDED.payment_alipay_real_name,
    payment_alipay_account = EXCLUDED.payment_alipay_account,
    payment_contact_phone = EXCLUDED.payment_contact_phone,
    payment_note = EXCLUDED.payment_note,
    payment_qr_object_key = EXCLUDED.payment_qr_object_key,
    payment_qr_content_type = EXCLUDED.payment_qr_content_type,
    payment_qr_original_filename = EXCLUDED.payment_qr_original_filename,
    requested_at = EXCLUDED.requested_at,
    due_at = EXCLUDED.due_at,
    paid_at = NULL,
    failed_at = NULL,
    payment_reference = '',
    failure_reason = '',
    updated_at = NOW()
  RETURNING id INTO withdrawal_request_id;

  INSERT INTO affiliate_agent_notices (agent_id, notice_type, title, message, source_type, source_id, idempotency_key, metadata, created_at)
  VALUES
    (alpha_id, 'community_invite', '欢迎加入合伙人社群', '你的合伙人权限已开通。扫码加入社群，获取素材、话术和结算通知。', 'agent_activation', alpha_id, 'demo:notice:alpha:community', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '6 days'),
    (alpha_id, 'withdrawal_paid', '一笔提现已到账', '演示：¥80 提现已标记到账，用户侧只看到“已到账”。', 'withdrawal', NULL, 'demo:notice:alpha:paid', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '3 days'),
    (review_id, 'risk_review', '账户状态待确认', '演示：待确认期间会暂缓发放奖励，并暂停新增链接、提现和转换。', 'risk', NULL, 'demo:notice:review:risk', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '1 day')
  ON CONFLICT (idempotency_key) DO UPDATE SET
    title = EXCLUDED.title,
    message = EXCLUDED.message,
    metadata = EXCLUDED.metadata,
    created_at = EXCLUDED.created_at;

  INSERT INTO affiliate_qualification_states (
    user_id, direct_valid_consumer_count, direct_team_consumption_micros,
    self_consumption_micros, combined_consumption_micros,
    qualifying_route, status, qualified_at, evaluated_at
  )
  VALUES
    (alpha_id, 12, 10890000000, 350000000, 11240000000, 'direct_team', 'active', NOW() - INTERVAL '9 days', NOW()),
    (candidate_id, 10, 1200000000, 150000000, 1350000000, 'direct_team', 'qualified', NOW(), NOW()),
    (review_id, 1, 400000000, 0, 400000000, NULL, 'active', NOW() - INTERVAL '7 days', NOW()),
    (blocked_id, 1, 300000000, 0, 300000000, NULL, 'active', NOW() - INTERVAL '7 days', NOW())
  ON CONFLICT (user_id) DO UPDATE SET
    direct_valid_consumer_count = EXCLUDED.direct_valid_consumer_count,
    direct_team_consumption_micros = EXCLUDED.direct_team_consumption_micros,
    self_consumption_micros = EXCLUDED.self_consumption_micros,
    combined_consumption_micros = EXCLUDED.combined_consumption_micros,
    qualifying_route = EXCLUDED.qualifying_route,
    status = EXCLUDED.status,
    qualified_at = EXCLUDED.qualified_at,
    evaluated_at = NOW(),
    updated_at = NOW();

  UPDATE users u
  SET balance = COALESCE((
      SELECT SUM(bl.remaining_amount_micros)::numeric / 1000000
      FROM balance_lots bl
      WHERE bl.user_id = u.id
    ), 0),
    updated_at = NOW()
  WHERE u.email LIKE '%@demo.local'
    AND u.deleted_at IS NULL;
END $$;

SELECT 'demo users' AS section, id, email, role, username, balance, invite_code
FROM users
WHERE email LIKE '%@demo.local'
ORDER BY id
LIMIT 20;

SELECT 'gpt groups' AS section, id, name, rate_multiplier
FROM groups
WHERE deleted_at IS NULL
  AND name LIKE 'GPT %月卡组'
ORDER BY id;

SELECT
  'affiliate queues' AS section,
  (SELECT COUNT(*) FROM agent_principals) AS agents,
  (SELECT COUNT(*) FROM agent_payment_profiles WHERE verification_status = 'pending_review') AS pending_profiles,
  (SELECT COUNT(*) FROM agent_withdrawal_requests WHERE status = 'processing') AS processing_withdrawals,
  (SELECT COUNT(*) FROM affiliate_reward_entries WHERE status = 'risk_hold') AS held_rewards,
  (SELECT COUNT(*) FROM agent_cash_commission_entries WHERE posting_status = 'risk_hold') AS held_cash_entries;
SQL
}

main() {
  require_checkout
  require_containers
  write_demo_qr_assets
  seed_database
  echo "affiliate v2 staging demo data seeded"
  echo "demo login password: Demo123456"
}

main "$@"
