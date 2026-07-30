#!/usr/bin/env bash
set -euo pipefail
set +x

readonly SCRIPT_WORKTREE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly EXPECTED_WORKTREE="${AFFILIATE_STAGING_WORKTREE:-$SCRIPT_WORKTREE}"
readonly EXPECTED_BRANCH="${AFFILIATE_STAGING_BRANCH:-$(git -C "$EXPECTED_WORKTREE" branch --show-current)}"
readonly PROJECT_NAME="${AFFILIATE_STAGING_PROJECT_NAME:-laoshirenai-affiliate-v2-staging}"
readonly PG_CONTAINER="${PROJECT_NAME}-postgres-1"
readonly APP_CONTAINER="${PROJECT_NAME}-app-1"
readonly REDIS_CONTAINER="${PROJECT_NAME}-redis-1"

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
  case "$current_branch" in
    main|master|release/*)
      echo "staging demo data must target a non-release feature or fix branch: $current_branch" >&2
      exit 1
      ;;
  esac
  make -C "$EXPECTED_WORKTREE" checkout-validate >/dev/null
}

require_containers() {
  docker inspect "$PG_CONTAINER" >/dev/null
  docker inspect "$APP_CONTAINER" >/dev/null
  docker inspect "$REDIS_CONTAINER" >/dev/null
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
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/agent-payment-qrcodes/partner-alpha.png"
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/agent-payment-qrcodes/partner-review.png"
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/agent-payment-qrcodes/partner-blocked.png"
  docker cp "$tmp_png" "$APP_CONTAINER:/app/data/uploads/affiliate-community/community.png"
  docker exec --user 0 "$APP_CONTAINER" chmod 644 \
    /app/data/uploads/agent-payment-qrcodes/partner-alpha.png \
    /app/data/uploads/agent-payment-qrcodes/partner-review.png \
    /app/data/uploads/agent-payment-qrcodes/partner-blocked.png \
    /app/data/uploads/affiliate-community/community.png
}

seed_database() {
  docker exec -i "$PG_CONTAINER" psql -U affiliate_staging -d affiliate_staging -v ON_ERROR_STOP=1 <<'SQL'
DO $$
DECLARE
  demo_password_hash text := '$2a$10$8F1HImqy1ZYH8EJevN6BEulUZrKn2k2JMkOoy2BnYJUlRUPptzvz6';
  admin_id bigint;
  browser_admin_id bigint;
  demo_group_id bigint;
  demo_account_id bigint;
  demo_claude_account_id bigint;
  alpha_id bigint;
  review_id bigint;
  blocked_id bigint;
  candidate_id bigint;
  applicant_id bigint;
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
  topup_micros bigint;
  reward_micros bigint;
  cash_micros bigint;
  customer_bps integer;
  agent_bps integer;
  customer_email text;
  legacy_email text;
  demo_order_no text;
  usage_id bigint;
BEGIN
  SELECT id INTO admin_id
  FROM users
  WHERE email IN ('ops-admin@partner.local', 'affiliate-staging@local.invalid')
    AND deleted_at IS NULL
  ORDER BY CASE WHEN email = 'ops-admin@partner.local' THEN 0 ELSE 1 END
  LIMIT 1;
  IF admin_id IS NULL THEN
    RAISE EXCEPTION 'admin user is missing';
  END IF;

  UPDATE users
  SET email = 'ops-admin@partner.local', username = '运营管理员', notes = '运营管理账号', updated_at = NOW()
  WHERE id = admin_id;

  INSERT INTO settings (key, value, updated_at)
  VALUES
    ('site_name', '老实人AI', NOW()),
    ('registration_enabled', 'true', NOW()),
    ('invitation_code_enabled', 'false', NOW()),
    ('promo_code_enabled', 'false', NOW())
  ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;

  UPDATE affiliate_program_settings
  SET mode = 'live',
      started_at = NOW() - INTERVAL '14 days',
      ordinary_referral_rate_bps = 500,
      ordinary_invitee_rate_bps = 500,
      first_paid_bonus_threshold_micros = 0,
      first_paid_bonus_micros = 0,
      stress_cost_per_raw_credit_micros = 530000,
      revision = revision + 1,
      updated_by = admin_id,
      updated_at = NOW()
  WHERE id = 1;

  UPDATE affiliate_community_settings
  SET enabled = TRUE,
      title = '合伙人社群',
      message = '扫码加入合伙人社群，获取最新物料、运营通知和结算提醒。',
      qr_object_key = 'affiliate-community/community.png',
      qr_content_type = 'image/png',
      qr_original_filename = 'community.png',
      qr_size = 1200,
      revision = revision + 1,
      updated_by = admin_id,
      updated_at = NOW()
  WHERE id = 1;

  IF (
    SELECT COUNT(*)
    FROM groups
    WHERE deleted_at IS NULL
      AND name IN (
        'GPT Plus 月卡组', 'Claude Plus 月卡组',
        'GPT Pro V3 月卡组', 'Claude Pro V3 月卡组',
        'GPT Max V3 月卡组', 'Claude Max V3 月卡组'
      )
  ) <> 6 THEN
    RAISE EXCEPTION 'V3 Plus/Pro/Max monthly groups are missing';
  END IF;

  SELECT id INTO demo_group_id
  FROM groups
  WHERE name = 'GPT Plus 月卡组'
    AND deleted_at IS NULL
  LIMIT 1;

  SELECT id INTO demo_account_id
  FROM accounts
  WHERE name = 'GPT 混合上游账号'
    AND deleted_at IS NULL
  ORDER BY id
  LIMIT 1;
  IF demo_account_id IS NULL THEN
    INSERT INTO accounts (
      name, platform, type, credentials, extra, concurrency, priority, status,
      schedulable, rate_multiplier, notes
    )
    VALUES (
      'GPT 混合上游账号', 'openai', 'openai',
      '{}'::jsonb, '{"staging_demo": true}'::jsonb, 20, 10, 'active',
      TRUE, 0.1850, 'GPT 混合成本账号'
    )
    RETURNING id INTO demo_account_id;
  ELSE
    UPDATE accounts
    SET platform = 'openai',
        type = 'openai',
        extra = '{"staging_demo": true}'::jsonb,
        concurrency = 20,
        priority = 10,
        status = 'active',
        schedulable = TRUE,
        rate_multiplier = 0.1850,
        notes = 'GPT 混合成本账号',
        updated_at = NOW()
    WHERE id = demo_account_id;
  END IF;

  INSERT INTO account_groups (account_id, group_id, priority)
  SELECT demo_account_id, id, 1
  FROM groups
  WHERE deleted_at IS NULL
    AND name IN ('GPT Plus 月卡组', 'GPT Pro V3 月卡组', 'GPT Max V3 月卡组')
  ON CONFLICT (account_id, group_id) DO UPDATE SET
    priority = EXCLUDED.priority;

  SELECT id INTO demo_claude_account_id
  FROM accounts
  WHERE name = 'Claude 月卡上游账号'
    AND deleted_at IS NULL
  ORDER BY id
  LIMIT 1;
  IF demo_claude_account_id IS NULL THEN
    INSERT INTO accounts (
      name, platform, type, credentials, extra, concurrency, priority, status,
      schedulable, rate_multiplier, notes
    )
    VALUES (
      'Claude 月卡上游账号', 'anthropic', 'apikey',
      '{}'::jsonb, '{"staging_demo": true}'::jsonb, 20, 10, 'active',
      TRUE, 1.0000, 'Claude 月卡路由账号'
    )
    RETURNING id INTO demo_claude_account_id;
  ELSE
    UPDATE accounts
    SET platform = 'anthropic',
        type = 'apikey',
        extra = '{"staging_demo": true}'::jsonb,
        concurrency = 20,
        priority = 10,
        status = 'active',
        schedulable = TRUE,
        rate_multiplier = 1.0000,
        notes = 'Claude 月卡路由账号',
        updated_at = NOW()
    WHERE id = demo_claude_account_id;
  END IF;

  INSERT INTO account_groups (account_id, group_id, priority)
  SELECT demo_claude_account_id, id, 1
  FROM groups
  WHERE deleted_at IS NULL
    AND name IN ('Claude Plus 月卡组', 'Claude Pro V3 月卡组', 'Claude Max V3 月卡组')
  ON CONFLICT (account_id, group_id) DO UPDATE SET
    priority = EXCLUDED.priority;

  UPDATE users u
  SET email = REPLACE(u.email, '@demo.local', '@partner.local'), updated_at = NOW()
  WHERE u.email LIKE '%@demo.local'
    AND u.deleted_at IS NULL
    AND NOT EXISTS (
      SELECT 1
      FROM users existing
      WHERE existing.email = REPLACE(u.email, '@demo.local', '@partner.local')
        AND existing.id <> u.id
        AND existing.deleted_at IS NULL
    );

  -- Normalize older local seed rows so repeat runs stay production-like instead
  -- of showing legacy technical names in the UI.
  UPDATE users duplicate
  SET deleted_at = COALESCE(duplicate.deleted_at, NOW()),
      email = duplicate.email || '.archived-' || duplicate.id::text,
      invite_code = NULL,
      updated_at = NOW()
  WHERE duplicate.email = 'partner-upgrade@partner.local'
    AND duplicate.deleted_at IS NULL
    AND EXISTS (
      SELECT 1 FROM users legacy
      WHERE legacy.email = 'agent-candidate@partner.local'
        AND legacy.deleted_at IS NULL
    );

  UPDATE users
  SET email = 'partner-upgrade@partner.local',
      invite_code = 'UPGRADE',
      wechat = 'upgrade-partner',
      updated_at = NOW()
  WHERE email = 'agent-candidate@partner.local'
    AND deleted_at IS NULL;

  FOR i IN 1..10 LOOP
    legacy_email := 'candidate-customer-' || lpad(i::text, 2, '0') || '@partner.local';
    customer_email := 'upgrade-customer-' || lpad(i::text, 2, '0') || '@partner.local';

    UPDATE users duplicate
    SET deleted_at = COALESCE(duplicate.deleted_at, NOW()),
        email = duplicate.email || '.archived-' || duplicate.id::text,
        invite_code = NULL,
        updated_at = NOW()
    WHERE duplicate.email = customer_email
      AND duplicate.deleted_at IS NULL
      AND EXISTS (
        SELECT 1 FROM users legacy
        WHERE legacy.email = legacy_email
          AND legacy.deleted_at IS NULL
      );

    UPDATE users
    SET email = customer_email,
        invite_code = 'UPCUST' || lpad(i::text, 2, '0'),
        updated_at = NOW()
    WHERE email = legacy_email
      AND deleted_at IS NULL;
  END LOOP;

  INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, total_recharged, first_recharged, last_active_at, wechat)
  VALUES
    ('browser-admin@partner.local', demo_password_hash, 'admin', 0, 20, 'active', '浏览器验收管理员', '仅限隔离 Staging 的浏览器验收账号', 'BROWSERADMIN', 0, FALSE, NOW(), 'browser-admin'),
    ('agent-alpha@partner.local', demo_password_hash, 'agent', 0, 8, 'active', 'Alpha 合伙人', '合伙人账号', 'AGENTALPHA', 3888, TRUE, NOW() - INTERVAL '1 hour', 'alpha-partner'),
    ('agent-review@partner.local', demo_password_hash, 'agent', 0, 8, 'active', '待审核合伙人', '待审核合伙人账号', 'AGENTREVIEW', 860, TRUE, NOW() - INTERVAL '2 hours', 'review-partner'),
    ('agent-blocked@partner.local', demo_password_hash, 'agent', 0, 8, 'active', '已暂停合伙人', '已暂停合伙人账号', 'AGENTBLOCK', 640, TRUE, NOW() - INTERVAL '3 hours', 'blocked-partner'),
    ('partner-upgrade@partner.local', demo_password_hash, 'user', 0, 5, 'active', '待升级用户', '已满足合伙人开通条件', 'UPGRADE', 220, TRUE, NOW() - INTERVAL '4 hours', 'upgrade-partner'),
    ('partner-applicant@partner.local', demo_password_hash, 'user', 0, 5, 'active', '申请审核用户', '已提交合伙人申请', 'APPLICANT', 260, TRUE, NOW() - INTERVAL '4 hours', 'applicant-partner'),
    ('ordinary-referrer@partner.local', demo_password_hash, 'user', 0, 5, 'active', '普通邀请人', '普通邀请账号', 'ORDREF', 160, TRUE, NOW() - INTERVAL '5 hours', 'ordinary-partner'),
    ('ordinary-invitee@partner.local', demo_password_hash, 'user', 0, 5, 'active', '普通被邀请人', '普通被邀请账号', 'ORDINVITEE', 80, TRUE, NOW() - INTERVAL '6 hours', 'invitee-partner')
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

  SELECT id INTO alpha_id FROM users WHERE email = 'agent-alpha@partner.local' AND deleted_at IS NULL;
  SELECT id INTO browser_admin_id FROM users WHERE email = 'browser-admin@partner.local' AND deleted_at IS NULL;
  SELECT id INTO review_id FROM users WHERE email = 'agent-review@partner.local' AND deleted_at IS NULL;
  SELECT id INTO blocked_id FROM users WHERE email = 'agent-blocked@partner.local' AND deleted_at IS NULL;
  SELECT id INTO candidate_id FROM users WHERE email = 'partner-upgrade@partner.local' AND deleted_at IS NULL;
  SELECT id INTO applicant_id FROM users WHERE email = 'partner-applicant@partner.local' AND deleted_at IS NULL;
  SELECT id INTO ordinary_id FROM users WHERE email = 'ordinary-referrer@partner.local' AND deleted_at IS NULL;
  SELECT id INTO invitee_id FROM users WHERE email = 'ordinary-invitee@partner.local' AND deleted_at IS NULL;

  INSERT INTO admin_user_roles (user_id, role_id)
  SELECT browser_admin_id, role_id
  FROM admin_user_roles
  WHERE user_id = admin_id
  ON CONFLICT (user_id, role_id) DO NOTHING;

  -- Keep repeated API/UI E2E runs deterministic. The E2E script intentionally
  -- creates random idempotency keys, so a re-seed must remove only those
  -- staging test conversion/withdrawal rows before recreating the fixed demo
  -- ledger below.
  DELETE FROM agent_payment_qr_access_events
  WHERE withdrawal_id IN (
    SELECT id
    FROM agent_withdrawal_requests
    WHERE agent_id = alpha_id
      AND (idempotency_key LIKE 'e2e-withdraw-%' OR idempotency_key LIKE 'affiliate-withdraw-%')
  );
  DELETE FROM agent_withdrawal_events
  WHERE withdrawal_id IN (
    SELECT id
    FROM agent_withdrawal_requests
    WHERE agent_id = alpha_id
      AND idempotency_key LIKE 'e2e-withdraw-%'
  );
  DELETE FROM agent_cash_commission_entries
  WHERE agent_id = alpha_id
    AND (
      idempotency_key LIKE 'e2e-%'
      OR (
        source_type = 'withdrawal'
        AND source_id IN (
          SELECT id
          FROM agent_withdrawal_requests
          WHERE agent_id = alpha_id
            AND (idempotency_key LIKE 'e2e-withdraw-%' OR idempotency_key LIKE 'affiliate-withdraw-%')
        )
      )
      OR (
        source_type = 'withdrawal_request'
        AND idempotency_key LIKE 'withdrawal:%:hold'
      )
      OR (
        source_type = 'commission_conversion'
        AND source_id IN (
          SELECT id
          FROM agent_commission_conversions
          WHERE agent_id = alpha_id
            AND (
              idempotency_key LIKE 'e2e-convert-%'
              OR idempotency_key LIKE 'affiliate-convert-%'
            )
        )
      )
    );
  DELETE FROM balance_lots
  WHERE user_id = alpha_id
    AND source_type = 'commission_conversion'
    AND source_id IN (
      SELECT id
      FROM agent_commission_conversions
      WHERE agent_id = alpha_id
        AND (
          idempotency_key LIKE 'e2e-convert-%'
          OR idempotency_key LIKE 'affiliate-convert-%'
        )
    );
  DELETE FROM agent_commission_conversions
  WHERE agent_id = alpha_id
    AND (
      idempotency_key LIKE 'e2e-convert-%'
      OR idempotency_key LIKE 'affiliate-convert-%'
    );
  DELETE FROM agent_withdrawal_events
  WHERE withdrawal_id IN (
    SELECT id
    FROM agent_withdrawal_requests
    WHERE agent_id = alpha_id
      AND (idempotency_key LIKE 'e2e-withdraw-%' OR idempotency_key LIKE 'affiliate-withdraw-%')
  );
  DELETE FROM agent_withdrawal_requests
  WHERE agent_id = alpha_id
    AND (idempotency_key LIKE 'e2e-withdraw-%' OR idempotency_key LIKE 'affiliate-withdraw-%');

  DELETE FROM affiliate_link_rate_versions
  WHERE link_id IN (
    SELECT id
    FROM affiliate_links
    WHERE agent_id = alpha_id
      AND code NOT IN ('AGALPHA5', 'AGALPHA0', 'AGALPHA8', 'AGALPHAPAUSE')
  );
  DELETE FROM affiliate_links
  WHERE agent_id = alpha_id
    AND code NOT IN ('AGALPHA5', 'AGALPHA0', 'AGALPHA8', 'AGALPHAPAUSE');

  DELETE FROM affiliate_agent_notices
  WHERE agent_id IN (alpha_id, review_id, blocked_id)
    AND idempotency_key LIKE 'withdrawal:%:paid-notice';

  -- Keep the qualified candidate reusable across repeated E2E runs. If a
  -- previous staging test clicked "立即成为合伙人", reset only this demo
  -- candidate back to the pre-activation state so the upgrade path remains
  -- testable without wiping the whole staging database.
  UPDATE agent_withdrawal_requests
  SET status = 'failed',
      failed_at = NOW(),
      failure_reason = '数据重置',
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
  DELETE FROM affiliate_agent_status_events
  WHERE agent_id IN (candidate_id, applicant_id);
  DELETE FROM affiliate_agent_applications
  WHERE user_id IN (candidate_id, applicant_id);
  DELETE FROM affiliate_performance_events
  WHERE (user_id = candidate_id AND event_type = 'agent_activated')
     OR direct_agent_id = candidate_id;
  UPDATE agent_principals
  SET status = 'candidate',
      risk_status = 'clear',
      risk_note = '',
      qualified_at = NULL,
      activated_at = NULL,
      applied_at = NULL,
      application_note = '',
      decision_note = '',
      reviewed_at = NULL,
      reviewed_by = NULL,
      terminated_at = NULL,
      updated_at = NOW()
  WHERE agent_id = candidate_id;

  -- The production guard correctly prevents payment-profile edits while a
  -- withdrawal is processing. For this staging demo re-seed, temporarily move
  -- our own demo processing withdrawals out of the way; the demo withdrawals
  -- are re-created as processing with fresh snapshots later in this script.
  UPDATE agent_withdrawal_requests
  SET status = 'failed',
      failed_at = NOW(),
      failure_reason = '数据刷新',
      updated_at = NOW()
  WHERE idempotency_key IN ('demo:withdrawal:alpha:processing', 'demo:withdrawal:review:processing', 'demo:withdrawal:blocked:processing')
    AND status = 'processing';

  UPDATE affiliate_risk_actions
  SET reason = CASE
      WHEN reason LIKE '%异常订单%' THEN '异常订单待审核'
      WHEN reason LIKE '%自循环%' OR reason LIKE '%小号%' OR reason LIKE '%互刷%' THEN '异常邀请行为待处理'
      ELSE reason
    END
  WHERE agent_id IN (review_id, blocked_id)
    AND (reason LIKE '%staging%' OR reason LIKE '%演示%' OR reason LIKE '%小号%' OR reason LIKE '%互刷%' OR reason LIKE '%自循环%');

  INSERT INTO agent_principals (agent_id, status, risk_status, risk_note, qualified_at, activated_at, reviewed_at, reviewed_by)
  VALUES
    (alpha_id, 'active', 'clear', '', NOW() - INTERVAL '10 days', NOW() - INTERVAL '9 days', NOW() - INTERVAL '9 days', admin_id),
    (review_id, 'active', 'review', '存在异常订单，正在审核；审核完成后再恢复。', NOW() - INTERVAL '8 days', NOW() - INTERVAL '7 days', NOW() - INTERVAL '1 day', admin_id),
    (blocked_id, 'active', 'blocked', '存在异常邀请行为，已暂停邀请和提现。', NOW() - INTERVAL '8 days', NOW() - INTERVAL '7 days', NOW() - INTERVAL '1 day', admin_id)
  ON CONFLICT (agent_id) DO UPDATE SET
    status = EXCLUDED.status,
    risk_status = EXCLUDED.risk_status,
    risk_note = EXCLUDED.risk_note,
    qualified_at = EXCLUDED.qualified_at,
    activated_at = EXCLUDED.activated_at,
    reviewed_at = EXCLUDED.reviewed_at,
    reviewed_by = EXCLUDED.reviewed_by,
    updated_at = NOW();

  INSERT INTO agent_principals (
    agent_id, status, risk_status, risk_note,
    qualified_at, applied_at, application_note
  )
  VALUES (
    applicant_id, 'pending_review', 'clear', '',
    NOW() - INTERVAL '2 days', NOW() - INTERVAL '1 day',
    '主要服务独立开发者，计划通过技术社群进行真实分享。'
  )
  ON CONFLICT (agent_id) DO UPDATE SET
    status = EXCLUDED.status,
    risk_status = EXCLUDED.risk_status,
    risk_note = EXCLUDED.risk_note,
    qualified_at = EXCLUDED.qualified_at,
    applied_at = EXCLUDED.applied_at,
    application_note = EXCLUDED.application_note,
    activated_at = NULL,
    reviewed_at = NULL,
    reviewed_by = NULL,
    decision_note = '',
    terminated_at = NULL,
    updated_at = NOW();

  INSERT INTO affiliate_agent_applications (
    user_id, status, qualifying_route,
    direct_valid_consumer_count, direct_team_consumption_micros,
    application_note, submitted_at
  )
  VALUES (
    applicant_id, 'pending_review', 'direct_volume',
    7, 2280000000,
    '主要服务独立开发者，计划通过技术社群进行真实分享。',
    NOW() - INTERVAL '1 day'
  )
  RETURNING id INTO event_id;

  INSERT INTO affiliate_agent_status_events (
    agent_id, previous_status, next_status, reason, application_id, effective_at
  )
  VALUES (
    applicant_id, 'candidate', 'pending_review',
    '用户提交合伙人申请', event_id, NOW() - INTERVAL '1 day'
  );

  -- The pending application must be backed by the same immutable consumption
  -- ledger used during real approval. Otherwise the admin table would show a
  -- qualified snapshot that the review endpoint correctly refuses to approve.
  FOR i IN 1..7 LOOP
    customer_email := 'applicant-customer-' || lpad(i::text, 2, '0') || '@partner.local';
    amount_micros := CASE WHEN i = 7 THEN 480000000 ELSE 300000000 END;
    INSERT INTO users (
      email, password_hash, role, balance, concurrency, status,
      username, notes, invite_code, inviter_id,
      total_recharged, first_recharged, first_invited_topup_at,
      last_active_at, wechat
    )
    VALUES (
      customer_email, demo_password_hash, 'user', 0, 5, 'active',
      '申请审核用户客户 ' || lpad(i::text, 2, '0'), '申请审核用户直属客户',
      'APPCUST' || lpad(i::text, 2, '0'), applicant_id,
      amount_micros::numeric / 1000000, TRUE,
      NOW() - (i || ' days')::interval,
      NOW() - (i || ' hours')::interval, ''
    )
    ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET
      password_hash = EXCLUDED.password_hash,
      username = EXCLUDED.username,
      notes = EXCLUDED.notes,
      invite_code = EXCLUDED.invite_code,
      inviter_id = EXCLUDED.inviter_id,
      agent_id = NULL,
      total_recharged = EXCLUDED.total_recharged,
      first_recharged = EXCLUDED.first_recharged,
      first_invited_topup_at = EXCLUDED.first_invited_topup_at,
      last_active_at = EXCLUDED.last_active_at,
      updated_at = NOW()
    RETURNING id INTO customer_id;

    INSERT INTO affiliate_bindings (
      customer_user_id, inviter_user_id, binding_kind,
      customer_rebate_rate_snapshot_bps,
      agent_commission_rate_snapshot_bps,
      bound_at
    )
    VALUES (
      customer_id, applicant_id, 'ordinary',
      0, 0, NOW() - INTERVAL '7 days'
    )
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
      customer_id, applicant_id, 'confirmed_consumption', amount_micros,
      'usage', NULL, 'demo:confirmed:applicant:' || i,
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

  INSERT INTO agent_payment_profiles (
    agent_id, alipay_real_name, alipay_account, contact_phone, payment_note,
    alipay_qr_object_key, alipay_qr_content_type, alipay_qr_original_filename, alipay_qr_size,
    identity_fingerprint_hash, verification_status, verification_note, verified_at, verified_by
  )
  VALUES
    (alpha_id, '张三', 'alpha-pay@example.com', '13800000001', '常用收款账号，可扫码打款。', 'agent-payment-qrcodes/partner-alpha.png', 'image/png', 'partner-alpha.png', 1200, 'partner-alpha-fingerprint', 'verified', '已核对', NOW() - INTERVAL '6 days', admin_id),
    (review_id, '李四', 'review-pay@example.com', '13800000002', '资料待审核，请核对实名和收款码。', 'agent-payment-qrcodes/partner-review.png', 'image/png', 'partner-review.png', 1200, 'partner-review-fingerprint', 'pending_review', '', NULL, NULL),
    (blocked_id, '王五', 'blocked-pay@example.com', '13800000003', '当前合作已暂停，收款资料暂不处理。', 'agent-payment-qrcodes/partner-blocked.png', 'image/png', 'partner-blocked.png', 1200, 'partner-blocked-fingerprint', 'verified', '已核对', NOW() - INTERVAL '5 days', admin_id)
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
  VALUES (review_id, 'AGREVIEW5', '待审核默认链接', 'default', TRUE, 'active', 1)
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
  VALUES (blocked_id, 'AGBLOCK5', '暂停链接', 'default', TRUE, 'active', 1)
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
    customer_email := 'alpha-customer-' || lpad(i::text, 2, '0') || '@partner.local';
    amount_micros := (550 + i * 55)::bigint * 1000000;
    -- Keep recharge and confirmed consumption deliberately different so the
    -- partner table demonstrates the real meanings of both columns.
    topup_micros := amount_micros + (90 + i * 10)::bigint * 1000000;
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
      'Alpha 客户 ' || lpad(i::text, 2, '0'), 'Alpha 直属客户',
      'ALPHACUST' || lpad(i::text, 2, '0'), alpha_id, alpha_id,
      (topup_micros::numeric / 1000000), TRUE, NOW() - (i || ' days')::interval, NOW() - (i || ' hours')::interval, ''
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

    demo_order_no := 'TOPUP-ALPHA-' || lpad(i::text, 2, '0');
    INSERT INTO topup_orders (order_no, user_id, amount_cny_fen, pay_type, status, completed_at, invoice_status)
    VALUES (demo_order_no, customer_id, (topup_micros / 10000)::integer, 'alipay', 'completed', NOW() - (i || ' days')::interval, 'none')
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
    VALUES (customer_id, 'balance_topup', topup_id, 'demo:first-paid:alpha:' || i, topup_micros, NOW() - (i || ' days')::interval)
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
    VALUES (customer_id, 'sk-alpha-' || lpad(i::text, 2, '0'), 'Alpha Key ' || lpad(i::text, 2, '0'), demo_group_id, 'active', 0, amount_micros::numeric / 1000000, NOW() - (i || ' hours')::interval)
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

  -- E2E 可提现余额补足：Alpha 账号需要在“佣金转额度”之后仍能发起一笔真实提现，
  -- 否则提现申请 -> 后台扫码打款 -> 标记到账这条链路只能被脚本跳过。
  INSERT INTO agent_cash_commission_entries (
    agent_id, entry_type, amount_micros, posting_status,
    source_type, source_id, idempotency_key, metadata, occurred_at
  )
  VALUES (
    alpha_id, 'earned', 220000000, 'posted',
    'staging_demo_adjustment', NULL, 'demo:cash:alpha:e2e-withdrawable-topup',
    '{"staging_demo":true,"purpose":"e2e_withdrawal_coverage"}'::jsonb,
    NOW() - INTERVAL '2 hours'
  )
  ON CONFLICT (idempotency_key) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    amount_micros = EXCLUDED.amount_micros,
    posting_status = EXCLUDED.posting_status,
    source_type = EXCLUDED.source_type,
    source_id = EXCLUDED.source_id,
    metadata = EXCLUDED.metadata,
    occurred_at = EXCLUDED.occurred_at;

  FOR i IN 1..10 LOOP
    customer_email := 'upgrade-customer-' || lpad(i::text, 2, '0') || '@partner.local';
    amount_micros := 120000000;
    INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, total_recharged, first_recharged, first_invited_topup_at, last_active_at, wechat)
    VALUES (
      customer_email, demo_password_hash, 'user', 0, 5, 'active',
      '待升级用户客户 ' || lpad(i::text, 2, '0'), '待升级用户直属客户',
      'UPCUST' || lpad(i::text, 2, '0'), candidate_id,
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
  VALUES ('TOPUP-ORDINARY-80', invitee_id, 8000, 'alipay', 'completed', NOW() - INTERVAL '2 days', 'none')
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

  INSERT INTO balance_lots (
    user_id, source_type, source_id, source_key,
    original_amount_micros, remaining_amount_micros,
    affiliate_eligible, affiliate_policy, occurred_at
  )
  VALUES (
    invitee_id, 'paid_topup', topup_id, 'demo:lot:ordinary-invitee-paid',
    80000000, 80000000, FALSE, 'NONE', NOW() - INTERVAL '2 days'
  )
  ON CONFLICT (source_key) DO UPDATE SET
    source_id = EXCLUDED.source_id,
    original_amount_micros = EXCLUDED.original_amount_micros,
    remaining_amount_micros = EXCLUDED.remaining_amount_micros,
    affiliate_eligible = FALSE,
    affiliate_policy = 'NONE',
    updated_at = NOW();

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
    source_type, source_id, idempotency_key, metadata, posted_at
  )
  VALUES (
    invitee_id, invitee_id, 'ordinary_invitee', 'platform_credit',
    4000000, 80000000, 500, 'posted', NOW() - INTERVAL '2 days',
    'first_paid_purchase', topup_id, 'demo:reward:ordinary-invitee',
    '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '2 days'
  )
  ON CONFLICT (idempotency_key) DO UPDATE SET
    beneficiary_user_id = EXCLUDED.beneficiary_user_id,
    consumer_user_id = EXCLUDED.consumer_user_id,
    amount_micros = EXCLUDED.amount_micros,
    source_amount_micros = EXCLUDED.source_amount_micros,
    status = EXCLUDED.status,
    available_at = EXCLUDED.available_at,
    source_id = EXCLUDED.source_id,
    posted_at = EXCLUDED.posted_at
  RETURNING id INTO reward_id;

  DELETE FROM affiliate_reward_entries
  WHERE idempotency_key = 'demo:reward:ordinary-invitee-t1';

  INSERT INTO balance_lots (
    user_id, source_type, source_id, source_key,
    original_amount_micros, remaining_amount_micros,
    affiliate_eligible, affiliate_policy, occurred_at
  )
  VALUES (
    invitee_id, 'referral_bonus', reward_id, 'demo:lot:ordinary-invitee',
    4000000, 4000000, FALSE, 'NONE', NOW() - INTERVAL '2 days'
  )
  ON CONFLICT (source_key) DO UPDATE SET
    source_id = EXCLUDED.source_id,
    original_amount_micros = EXCLUDED.original_amount_micros,
    remaining_amount_micros = EXCLUDED.remaining_amount_micros,
    affiliate_eligible = FALSE,
    affiliate_policy = 'NONE',
    updated_at = NOW();

  UPDATE users SET balance = 4, updated_at = NOW() WHERE id = ordinary_id;
  UPDATE users SET balance = 84, updated_at = NOW() WHERE id = invitee_id;

  -- 暂缓发放：review / blocked 各一条消费事件、客户额度暂缓发放、现金暂缓发放。
  FOR i IN 1..2 LOOP
    IF i = 1 THEN
      customer_email := 'review-customer-01@partner.local';
      selected_link_id := review_default_link_id;
      amount_micros := 400000000;
      customer_id := NULL;
      INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, agent_id, total_recharged, first_recharged, wechat)
      VALUES (customer_email, demo_password_hash, 'user', 0, 5, 'active', '待审核客户', '待审核合伙人直属客户', 'REVCUST01', review_id, review_id, 400, TRUE, '')
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
      customer_email := 'blocked-customer-01@partner.local';
      selected_link_id := blocked_default_link_id;
      amount_micros := 300000000;
      customer_id := NULL;
      INSERT INTO users (email, password_hash, role, balance, concurrency, status, username, notes, invite_code, inviter_id, agent_id, total_recharged, first_recharged, wechat)
      VALUES (customer_email, demo_password_hash, 'user', 0, 5, 'active', '暂停客户', '已暂停合伙人直属客户', 'BLKCUST01', blocked_id, blocked_id, 300, TRUE, '')
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
  SELECT review_id, 'review', 'clear', 'review', '异常订单待审核', 0, 0, 0, 0, admin_id, '{"staging_demo":true}'::jsonb
  WHERE NOT EXISTS (SELECT 1 FROM affiliate_risk_actions WHERE agent_id = review_id AND next_risk_status = 'review' AND reason = '异常订单待审核');

  INSERT INTO affiliate_risk_actions (agent_id, action_type, previous_risk_status, next_risk_status, reason, released_reward_count, released_reward_micros, released_cash_count, released_cash_micros, operator_id, metadata)
  SELECT blocked_id, 'block', 'clear', 'blocked', '异常邀请行为待处理', 0, 0, 0, 0, admin_id, '{"staging_demo":true}'::jsonb
  WHERE NOT EXISTS (SELECT 1 FROM affiliate_risk_actions WHERE agent_id = blocked_id AND next_risk_status = 'blocked' AND reason = '异常邀请行为待处理');

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
    '张三', 'alpha-pay@example.com', '13800000001', '常用收款账号，可扫码打款。',
    'agent-payment-qrcodes/partner-alpha.png', 'image/png', 'partner-alpha.png',
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
  SELECT withdrawal_request_id, alpha_id, 'requested', NULL, 'processing', alpha_id, '用户提交提现申请', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '6 hours'
  WHERE NOT EXISTS (SELECT 1 FROM agent_withdrawal_events WHERE agent_withdrawal_events.withdrawal_id = withdrawal_request_id AND event_type = 'requested');

  INSERT INTO agent_withdrawal_requests (
    agent_id, amount_micros, status, idempotency_key,
    payment_alipay_real_name, payment_alipay_account, payment_contact_phone, payment_note,
    payment_qr_object_key, payment_qr_content_type, payment_qr_original_filename,
    requested_at, due_at, paid_at, handled_by, payment_reference, failure_reason
  )
  VALUES (
    alpha_id, 80000000, 'paid', 'demo:withdrawal:alpha:paid',
    '张三', 'alpha-pay@example.com', '13800000001', '常用收款账号，可扫码打款。',
    'agent-payment-qrcodes/partner-alpha.png', 'image/png', 'partner-alpha.png',
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
    blocked_id, 60000000, 'processing', 'demo:withdrawal:blocked:processing',
    '王五', 'blocked-pay@example.com', '13800000003', '当前合作已暂停，收款资料暂不处理。',
    'agent-payment-qrcodes/partner-blocked.png', 'image/png', 'partner-blocked.png',
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
    (alpha_id, 'withdrawal_paid', '一笔提现已到账', '¥80 提现已标记到账。', 'withdrawal', NULL, 'demo:notice:alpha:paid', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '3 days'),
    (review_id, 'risk_review', '合伙人权限待审核', '审核期间会暂缓发放奖励，并暂停新增链接、提现和转换。', 'risk', NULL, 'demo:notice:review:risk', '{"staging_demo":true}'::jsonb, NOW() - INTERVAL '1 day')
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
  WHERE u.email LIKE '%@partner.local'
    AND u.deleted_at IS NULL;
END $$;

SELECT 'sample users' AS section, id, email, role, username, balance, invite_code
FROM users
WHERE email LIKE '%@partner.local'
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

invalidate_browser_admin_rbac_cache() {
  local browser_admin_id
  browser_admin_id="$(
    docker exec "$PG_CONTAINER" \
      psql -U affiliate_staging -d affiliate_staging -Atc \
      "SELECT id FROM users WHERE email = 'browser-admin@partner.local' AND deleted_at IS NULL"
  )"
  [[ "$browser_admin_id" =~ ^[0-9]+$ ]] || {
    echo "browser admin fixture is missing" >&2
    exit 1
  }
  docker exec "$REDIS_CONTAINER" redis-cli DEL \
    "rbac:perms:${browser_admin_id}" \
    "rbac:perms:${browser_admin_id}:empty" \
    "rbac:menu:${browser_admin_id}" >/dev/null
}

main() {
  require_checkout
  require_containers
  write_demo_qr_assets
  seed_database
  invalidate_browser_admin_rbac_cache
  echo "affiliate v3 acceptance data seeded"
  echo "login password: Demo123456"
}

main "$@"
