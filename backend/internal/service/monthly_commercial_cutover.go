package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var (
	ErrMonthlyCommercialCutoverTooEarly = infraerrors.Conflict("MONTHLY_COMMERCIAL_CUTOVER_TOO_EARLY", "monthly commercial cutover cannot execute before its effective time")
	ErrMonthlyCommercialCutoverBlocked  = infraerrors.Conflict("MONTHLY_COMMERCIAL_CUTOVER_BLOCKED", "monthly commercial cutover preflight is blocked")
)

var monthlyCommercialCutoverEffectiveAt = time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC) // 2026-09-01 00:00 Asia/Shanghai

type monthlyCutoverQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type MonthlyCommercialCutoverService struct {
	db           *sql.DB
	billingCache *BillingCacheService
	apiKeys      *APIKeyService
}

func NewMonthlyCommercialCutoverService(db *sql.DB, billingCache *BillingCacheService, apiKeys *APIKeyService) *MonthlyCommercialCutoverService {
	return &MonthlyCommercialCutoverService{db: db, billingCache: billingCache, apiKeys: apiKeys}
}

func (s *MonthlyCommercialCutoverService) Preview(ctx context.Context) (map[string]any, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("monthly commercial cutover is not configured")
	}
	return monthlyCommercialCutoverSnapshot(ctx, s.db)
}

func (s *MonthlyCommercialCutoverService) Execute(ctx context.Context, operatorID int64, idempotencyKey string) (map[string]any, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("monthly commercial cutover is not configured")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 180 {
		return nil, ErrAffiliateIdempotencyKeyRequired
	}
	if time.Now().UTC().Before(monthlyCommercialCutoverEffectiveAt) {
		return nil, ErrMonthlyCommercialCutoverTooEarly
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(202609010000)`); err != nil {
		return nil, err
	}
	// Close the checkout race only for the short cutover transaction. Orders
	// already in flight fail the preflight; new reservations wait until the new
	// offer snapshot is committed and therefore cannot pay an old price for a
	// new entitlement.
	if _, err := tx.ExecContext(ctx, `
		LOCK TABLE native_checkout_offers IN ACCESS EXCLUSIVE MODE;
		LOCK TABLE native_checkout_orders IN SHARE ROW EXCLUSIVE MODE;
	`); err != nil {
		return nil, err
	}
	var existing []byte
	if err := tx.QueryRowContext(ctx, `SELECT result FROM monthly_commercial_cutovers WHERE idempotency_key=$1 OR effective_at=$2 LIMIT 1`, idempotencyKey, monthlyCommercialCutoverEffectiveAt).Scan(&existing); err == nil {
		var result map[string]any
		if jsonErr := json.Unmarshal(existing, &result); jsonErr != nil {
			return nil, jsonErr
		}
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	before, err := monthlyCommercialCutoverSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	if countNumber(before["inflight_order_count"]) != 0 {
		return nil, ErrMonthlyCommercialCutoverBlocked.WithMetadata(map[string]string{"reason": "inflight_monthly_orders"})
	}
	if countNumber(before["unassigned_inventory_count"]) != 0 {
		return nil, ErrMonthlyCommercialCutoverBlocked.WithMetadata(map[string]string{"reason": "unassigned_native_inventory"})
	}
	if countNumber(before["unexpected_offer_count"]) != 0 || countNumber(before["unexpected_group_count"]) != 0 {
		return nil, ErrMonthlyCommercialCutoverBlocked.WithMetadata(map[string]string{"reason": "live_contract_drift"})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE native_checkout_offers SET
			pay_amount_cny_fen=CASE code WHEN 'plus' THEN 29900 WHEN 'pro' THEN 59900 WHEN 'max' THEN 99900 END,
			benefit_amount_cny_fen=CASE code WHEN 'plus' THEN 29900 WHEN 'pro' THEN 59900 WHEN 'max' THEN 99900 END,
			redeem_value=CASE code WHEN 'plus' THEN 299 WHEN 'pro' THEN 599 WHEN 'max' THEN 999 END,
			description=CASE code
				WHEN 'plus' THEN '支付宝或佣金钱包支付，自动开通31天Plus开发者计划。'
				WHEN 'pro' THEN '支付宝或佣金钱包支付，自动开通31天Pro开发者计划。'
				WHEN 'max' THEN '支付宝或佣金钱包支付，自动开通31天Max开发者计划。' END,
			updated_at=NOW()
		WHERE code IN ('plus','pro','max') AND enabled=TRUE
	`); err != nil {
		return nil, fmt.Errorf("update live monthly offers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE groups SET monthly_limit_usd=CASE
			WHEN id IN (40,41,48) THEN 380
			WHEN id IN (42,43,49) THEN 780
			WHEN id IN (44,45,50) THEN 1300 END,
			updated_at=NOW()
		WHERE id IN (40,41,48,42,43,49,44,45,50) AND status='active'
	`); err != nil {
		return nil, fmt.Errorf("update monthly groups: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE monthly_entitlement_cycles c SET credit_limit_micros=380000000, updated_at=NOW()
		WHERE c.ends_at>NOW() AND c.credit_limit_micros=300000000 AND c.used_credit_micros<=380000000
		  AND EXISTS (
			SELECT 1 FROM monthly_entitlement_cycle_subscriptions cs
			WHERE cs.cycle_id=c.id AND cs.group_id IN (40,41,48)
		  )
	`); err != nil {
		return nil, fmt.Errorf("upgrade active plus cycles: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE affiliate_program_settings SET
			commission_wallet_checkout_enabled=TRUE,
			commission_wallet_purchase_rate_bps=8500,
			commission_conversion_enabled=FALSE,
			updated_at=NOW()
		WHERE id=1
	`); err != nil {
		return nil, fmt.Errorf("activate commission wallet checkout: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		WITH plus_users AS (
			SELECT user_id,
			       GREATEST(COALESCE(MAX(monthly_usage_usd),0),0) AS used_raw,
			       MIN(expires_at) AS expires_at
			FROM user_subscriptions
			WHERE group_id IN (40,41,48)
			  AND status='active' AND expires_at>NOW()
			GROUP BY user_id
		)
		INSERT INTO user_notifications(user_id,type,title,body,action_url,dedupe_key)
		SELECT user_id,
		       'subscription',
		       'Plus 月卡额度升级完成',
		       format(
			 '感谢您的支持！您的当前 Plus 月卡已免费补足至最新版本 3,800 AI credits。%s本周期已用 %s AI credits，剩余 %s AI credits，有效期至 %s（北京时间）。%s本次升级不会重置已用额度，也不会改变到期时间；GPT、Claude、Grok 三个 Plus 入口共享同一份额度，无需重新创建 Key 或修改现有配置。',
			 E'\n\n',
			 trim(to_char(ROUND(used_raw*10,2),'FM999999990.00')),
			 trim(to_char(ROUND(GREATEST(380-used_raw,0)*10,2),'FM999999990.00')),
			 to_char(expires_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),
			 E'\n\n'
		       ),
		       '/subscriptions',
		       'monthly-plus-upgrade-20260901:u'||user_id::text
		FROM plus_users
		ON CONFLICT(dedupe_key) DO NOTHING
	`); err != nil {
		return nil, fmt.Errorf("create plus upgrade notifications: %w", err)
	}
	after, err := monthlyCommercialCutoverSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"ok": true, "effective_at": monthlyCommercialCutoverEffectiveAt,
		"before": before, "after": after,
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO monthly_commercial_cutovers(idempotency_key,effective_at,result,executed_by)
		VALUES($1,$2,$3,$4)
	`, idempotencyKey, monthlyCommercialCutoverEffectiveAt, encoded, operatorID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for _, groupID := range []int64{40, 41, 48, 42, 43, 49, 44, 45, 50} {
		if s.apiKeys != nil {
			s.apiKeys.InvalidateAuthCacheByGroupID(context.WithoutCancel(ctx), groupID)
		}
	}
	if s.billingCache != nil {
		rows, queryErr := s.db.QueryContext(context.WithoutCancel(ctx), `
			SELECT user_id,group_id FROM user_subscriptions
			WHERE group_id IN (40,41,48,42,43,49,44,45,50)
			  AND status='active' AND expires_at>NOW()
		`)
		if queryErr == nil {
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var userID, groupID int64
				if rows.Scan(&userID, &groupID) == nil {
					_ = s.billingCache.InvalidateSubscription(context.WithoutCancel(ctx), userID, groupID)
				}
			}
		}
	}
	return result, nil
}

func countNumber(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

func monthlyCommercialCutoverSnapshot(ctx context.Context, q monthlyCutoverQuerier) (map[string]any, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, `
		WITH expected_offers(code,old_price,new_price) AS (VALUES
			('plus',25500::bigint,29900::bigint),('pro',71500::bigint,59900::bigint),('max',152500::bigint,99900::bigint)
		), expected_groups(group_id,old_limit,new_limit) AS (VALUES
			(40,300::numeric,380::numeric),(41,300,380),(48,300,380),
			(42,900,780),(43,900,780),(49,900,780),
			(44,2000,1300),(45,2000,1300),(50,2000,1300)
		)
		SELECT json_build_object(
			'generated_at',NOW(),
			'effective_at',$1::timestamptz,
			'offers',(SELECT json_agg(json_build_object('code',o.code,'pay_amount_cny_fen',o.pay_amount_cny_fen,'benefit_amount_cny_fen',o.benefit_amount_cny_fen,'redeem_value',o.redeem_value,'enabled',o.enabled) ORDER BY o.code) FROM native_checkout_offers o WHERE o.code IN ('plus','pro','max')),
			'groups',(SELECT json_agg(json_build_object('id',g.id,'name',g.name,'monthly_limit',g.monthly_limit_usd,'status',g.status) ORDER BY g.id) FROM groups g WHERE g.id IN (40,41,48,42,43,49,44,45,50)),
			'wallet_settings',(SELECT json_build_object('checkout_enabled',commission_wallet_checkout_enabled,'rate_bps',commission_wallet_purchase_rate_bps,'conversion_enabled',commission_conversion_enabled) FROM affiliate_program_settings WHERE id=1),
			'inflight_order_count',(SELECT count(*) FROM native_checkout_orders WHERE offer_code IN ('plus','pro','max') AND status IN ('creating','pending','checking','fulfilling','manual_review')),
			'unassigned_inventory_count',(SELECT count(*) FROM native_checkout_redeem_inventory WHERE offer_code IN ('plus','pro','max') AND assigned_order_id IS NULL),
			'unexpected_offer_count',(SELECT count(*) FROM expected_offers e LEFT JOIN native_checkout_offers o USING(code) WHERE o.code IS NULL OR o.pay_amount_cny_fen NOT IN (e.old_price,e.new_price) OR NOT o.enabled),
			'unexpected_group_count',(SELECT count(*) FROM expected_groups e LEFT JOIN groups g ON g.id=e.group_id WHERE g.id IS NULL OR g.monthly_limit_usd NOT IN (e.old_limit,e.new_limit) OR g.status<>'active'),
			'refund_candidates',COALESCE((SELECT json_agg(json_build_object('order_no',order_no,'user_id',user_id,'offer_code',offer_code,'paid_fen',pay_amount_cny_fen,'refund_fen',CASE offer_code WHEN 'pro' THEN pay_amount_cny_fen-59900 WHEN 'max' THEN pay_amount_cny_fen-99900 END,'payment_method',payment_method,'completed_at',completed_at)) FROM native_checkout_orders WHERE offer_code IN ('pro','max') AND status='completed' AND completed_at>='2026-08-31 17:27:31+08'::timestamptz AND completed_at<$1::timestamptz), '[]'::json),
			'plus_upgrade_notification_count',(SELECT count(*) FROM user_notifications WHERE dedupe_key LIKE 'monthly-plus-upgrade-20260901:u%'),
			'active_users',(SELECT json_agg(json_build_object('user_id',user_id,'group_id',group_id,'subscription_id',id,'used',monthly_usage_usd,'expires_at',expires_at) ORDER BY user_id,group_id) FROM user_subscriptions WHERE group_id IN (40,41,48,42,43,49,44,45,50) AND status='active' AND expires_at>NOW())
		)::text
	`, monthlyCommercialCutoverEffectiveAt).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}
