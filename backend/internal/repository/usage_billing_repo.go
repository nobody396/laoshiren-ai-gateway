package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, cmd.RequestID, cmd.APIKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, cmd.RequestID, cmd.APIKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		updates, err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost)
		if err != nil {
			return err
		}
		result.SubscriptionUsageUpdates = updates
	}

	if cmd.BalanceCost > 0 {
		newBalance, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	return nil
}

type usageBillingSubscriptionTarget struct {
	ID      int64
	UserID  int64
	GroupID int64
	Notes   string
}

type usageBillingSharedSubscriptionRow struct {
	ID           int64
	UserID       int64
	GroupID      int64
	DailyUsage   float64
	WeeklyUsage  float64
	MonthlyUsage float64
	DailyLimit   sql.NullFloat64
	WeeklyLimit  sql.NullFloat64
	MonthlyLimit sql.NullFloat64
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) ([]service.SubscriptionUsageUpdate, error) {
	target, err := lockUsageBillingSubscriptionTarget(ctx, tx, subscriptionID)
	if err != nil {
		return nil, err
	}
	marker := service.SubscriptionSharedQuotaMarkerFromNotes(target.Notes)
	if marker != "" {
		return incrementUsageBillingSharedSubscription(ctx, tx, target, marker, costUSD)
	}
	if err := incrementUsageBillingSingleSubscription(ctx, tx, subscriptionID, costUSD); err != nil {
		return nil, err
	}
	return []service.SubscriptionUsageUpdate{{
		UserID:  target.UserID,
		GroupID: target.GroupID,
		CostUSD: costUSD,
	}}, nil
}

func lockUsageBillingSubscriptionTarget(ctx context.Context, tx *sql.Tx, subscriptionID int64) (*usageBillingSubscriptionTarget, error) {
	target := &usageBillingSubscriptionTarget{}
	err := tx.QueryRowContext(ctx, `
		SELECT id, user_id, group_id, COALESCE(notes, '')
		FROM user_subscriptions
		WHERE id = $1
			AND deleted_at IS NULL
		FOR UPDATE
	`, subscriptionID).Scan(&target.ID, &target.UserID, &target.GroupID, &target.Notes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, err
	}
	return target, nil
}

func incrementUsageBillingSingleSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
			AND (g.daily_limit_usd IS NULL OR g.daily_limit_usd <= 0 OR us.daily_usage_usd + $1 <= g.daily_limit_usd)
			AND (g.weekly_limit_usd IS NULL OR g.weekly_limit_usd <= 0 OR us.weekly_usage_usd + $1 <= g.weekly_limit_usd)
			AND (g.monthly_limit_usd IS NULL OR g.monthly_limit_usd <= 0 OR us.monthly_usage_usd + $1 <= g.monthly_limit_usd)
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return classifyUsageBillingSubscriptionUpdateMiss(ctx, tx, subscriptionID, costUSD)
}

func incrementUsageBillingSharedSubscription(ctx context.Context, tx *sql.Tx, target *usageBillingSubscriptionTarget, marker string, costUSD float64) ([]service.SubscriptionUsageUpdate, error) {
	explicitNeedle := service.SubscriptionSharedQuotaNoteKey + marker
	legacyNeedle := usageBillingLegacyRedeemNeedle(marker)
	rows, err := tx.QueryContext(ctx, `
		SELECT
			us.id,
			us.user_id,
			us.group_id,
			us.daily_usage_usd,
			us.weekly_usage_usd,
			us.monthly_usage_usd,
			g.daily_limit_usd,
			g.weekly_limit_usd,
			g.monthly_limit_usd
		FROM user_subscriptions us
		JOIN groups g ON us.group_id = g.id
		WHERE us.user_id = $1
			AND us.deleted_at IS NULL
			AND us.status = $2
			AND us.expires_at > NOW()
			AND g.deleted_at IS NULL
			AND (
				STRPOS(COALESCE(us.notes, ''), $3) > 0
				OR ($4 <> '' AND STRPOS(COALESCE(us.notes, ''), $4) > 0)
			)
		ORDER BY us.id
		FOR UPDATE OF us
	`, target.UserID, service.SubscriptionStatusActive, explicitNeedle, legacyNeedle)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	sharedRows := make([]usageBillingSharedSubscriptionRow, 0, 2)
	for rows.Next() {
		var row usageBillingSharedSubscriptionRow
		if err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.GroupID,
			&row.DailyUsage,
			&row.WeeklyUsage,
			&row.MonthlyUsage,
			&row.DailyLimit,
			&row.WeeklyLimit,
			&row.MonthlyLimit,
		); err != nil {
			return nil, err
		}
		sharedRows = append(sharedRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(sharedRows) == 0 {
		return nil, service.ErrSubscriptionNotFound
	}

	dailyUsage, weeklyUsage, monthlyUsage := sharedRows[0].DailyUsage, sharedRows[0].WeeklyUsage, sharedRows[0].MonthlyUsage
	var dailyLimit, weeklyLimit, monthlyLimit sql.NullFloat64
	for _, row := range sharedRows {
		dailyUsage = maxFloat64(dailyUsage, row.DailyUsage)
		weeklyUsage = maxFloat64(weeklyUsage, row.WeeklyUsage)
		monthlyUsage = maxFloat64(monthlyUsage, row.MonthlyUsage)
		dailyLimit = minPositiveUsageBillingLimit(dailyLimit, row.DailyLimit)
		weeklyLimit = minPositiveUsageBillingLimit(weeklyLimit, row.WeeklyLimit)
		monthlyLimit = minPositiveUsageBillingLimit(monthlyLimit, row.MonthlyLimit)
	}

	if usageBillingLimitExceeded(dailyUsage, dailyLimit, costUSD) {
		return nil, service.ErrDailyLimitExceeded
	}
	if usageBillingLimitExceeded(weeklyUsage, weeklyLimit, costUSD) {
		return nil, service.ErrWeeklyLimitExceeded
	}
	if usageBillingLimitExceeded(monthlyUsage, monthlyLimit, costUSD) {
		return nil, service.ErrMonthlyLimitExceeded
	}

	newDailyUsage := dailyUsage + costUSD
	newWeeklyUsage := weeklyUsage + costUSD
	newMonthlyUsage := monthlyUsage + costUSD
	updateRows, err := tx.QueryContext(ctx, `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = $4,
			weekly_usage_usd = $5,
			monthly_usage_usd = $6,
			updated_at = NOW()
		FROM groups g
		WHERE us.user_id = $1
			AND us.deleted_at IS NULL
			AND us.status = $2
			AND us.expires_at > NOW()
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
			AND (
				STRPOS(COALESCE(us.notes, ''), $3) > 0
				OR ($7 <> '' AND STRPOS(COALESCE(us.notes, ''), $7) > 0)
			)
		RETURNING us.user_id, us.group_id
	`, target.UserID, service.SubscriptionStatusActive, explicitNeedle, newDailyUsage, newWeeklyUsage, newMonthlyUsage, legacyNeedle)
	if err != nil {
		return nil, err
	}
	defer func() { _ = updateRows.Close() }()

	updates := make([]service.SubscriptionUsageUpdate, 0, len(sharedRows))
	for updateRows.Next() {
		var update service.SubscriptionUsageUpdate
		if err := updateRows.Scan(&update.UserID, &update.GroupID); err != nil {
			return nil, err
		}
		update.CostUSD = costUSD
		updates = append(updates, update)
	}
	if err := updateRows.Err(); err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		return nil, service.ErrSubscriptionNotFound
	}
	return updates, nil
}

func usageBillingLegacyRedeemNeedle(marker string) string {
	const redeemPrefix = "redeem:"
	code := strings.TrimSpace(strings.TrimPrefix(marker, redeemPrefix))
	if code == "" || code == marker {
		return ""
	}
	return "通过兑换码 " + code + " 兑换"
}

func minPositiveUsageBillingLimit(current, candidate sql.NullFloat64) sql.NullFloat64 {
	if !candidate.Valid || candidate.Float64 <= 0 {
		return current
	}
	if !current.Valid || current.Float64 <= 0 || candidate.Float64 < current.Float64 {
		return candidate
	}
	return current
}

func maxFloat64(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

func classifyUsageBillingSubscriptionUpdateMiss(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	var (
		dailyUsage   float64
		weeklyUsage  float64
		monthlyUsage float64
		dailyLimit   sql.NullFloat64
		weeklyLimit  sql.NullFloat64
		monthlyLimit sql.NullFloat64
	)
	err := tx.QueryRowContext(ctx, `
		SELECT
			us.daily_usage_usd,
			us.weekly_usage_usd,
			us.monthly_usage_usd,
			g.daily_limit_usd,
			g.weekly_limit_usd,
			g.monthly_limit_usd
		FROM user_subscriptions us
		JOIN groups g ON us.group_id = g.id
		WHERE us.id = $1
			AND us.deleted_at IS NULL
			AND g.deleted_at IS NULL
	`, subscriptionID).Scan(&dailyUsage, &weeklyUsage, &monthlyUsage, &dailyLimit, &weeklyLimit, &monthlyLimit)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrSubscriptionNotFound
	}
	if err != nil {
		return err
	}
	if usageBillingLimitExceeded(dailyUsage, dailyLimit, costUSD) {
		return service.ErrDailyLimitExceeded
	}
	if usageBillingLimitExceeded(weeklyUsage, weeklyLimit, costUSD) {
		return service.ErrWeeklyLimitExceeded
	}
	if usageBillingLimitExceeded(monthlyUsage, monthlyLimit, costUSD) {
		return service.ErrMonthlyLimitExceeded
	}
	return service.ErrSubscriptionNotFound
}

func usageBillingLimitExceeded(current float64, limit sql.NullFloat64, cost float64) bool {
	return limit.Valid && limit.Float64 > 0 && current+cost > limit.Float64
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, error) {
	var newBalance float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		exists, existsErr := usageBillingUserExists(ctx, tx, userID)
		if existsErr != nil {
			return 0, existsErr
		}
		if exists {
			return 0, service.ErrInsufficientBalance
		}
		return 0, service.ErrUserNotFound
	}
	if err != nil {
		return 0, err
	}
	return newBalance, nil
}

func usageBillingUserExists(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE id = $1 AND deleted_at IS NULL
		)
	`, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN COALESCE((extra->>'quota_daily_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '24 hours'::interval <= NOW()
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN COALESCE((extra->>'quota_daily_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '24 hours'::interval <= NOW()
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN COALESCE((extra->>'quota_weekly_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '168 hours'::interval <= NOW()
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN COALESCE((extra->>'quota_weekly_start')::timestamptz, '1970-01-01'::timestamptz)
						+ '168 hours'::interval <= NOW()
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
