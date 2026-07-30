package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateRewardRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewAffiliateRewardRepository(client *dbent.Client, db *sql.DB) service.AffiliateRewardRepository {
	return &affiliateRewardRepository{client: client, db: db}
}

func (r *affiliateRewardRepository) ClaimFirstPaidPurchase(
	ctx context.Context,
	input service.AffiliateFirstPaidPurchaseInput,
) (*service.AffiliateFirstPaidContext, error) {
	client, err := r.clientForContext(ctx)
	if err != nil {
		return nil, err
	}
	result := &service.AffiliateFirstPaidContext{}
	var (
		startedAt            sql.NullTime
		bindingKind          sql.NullString
		bindingBoundAt       sql.NullTime
		inviterID            sql.NullInt64
		bindingAgentID       sql.NullInt64
		inviterPartnerStatus sql.NullString
		inviterActivatedAt   sql.NullTime
		legacyInviterID      sql.NullInt64
		legacyAgentID        sql.NullInt64
		selfPartnerStatus    sql.NullString
		selfPartnerRisk      sql.NullString
		selfPolicyEnabled    sql.NullBool
		selfPolicyRateBPS    sql.NullInt32
		selfPolicyEffective  sql.NullTime
		mode                 string
	)
	rows := &entsql.Rows{}
	if err := client.Driver().Query(ctx, `
		SELECT
			s.mode,
			s.started_at,
			s.ordinary_referral_rate_bps,
			s.ordinary_invitee_rate_bps,
			b.binding_kind,
			b.bound_at,
			b.inviter_user_id,
			b.agent_id,
			COALESCE(b.customer_rebate_rate_snapshot_bps, 0),
			COALESCE(b.agent_commission_rate_snapshot_bps, 0),
			ap.status,
			ap.activated_at,
			u.inviter_id,
			u.agent_id,
			self_ap.status,
			self_ap.risk_status,
			self_policy.enabled,
			self_policy.rate_bps,
			self_policy.effective_at
		FROM affiliate_program_settings s
		LEFT JOIN users u
			ON u.id = $1
			AND u.deleted_at IS NULL
		LEFT JOIN affiliate_bindings b
			ON b.customer_user_id = $1
		LEFT JOIN agent_principals ap
			ON ap.agent_id = b.inviter_user_id
		LEFT JOIN agent_principals self_ap
			ON self_ap.agent_id = $1
		LEFT JOIN affiliate_agent_self_commission_policies self_policy
			ON self_policy.agent_id = $1
		WHERE s.id = 1
		FOR SHARE OF s
	`, []any{input.UserID}, rows); err != nil {
		return nil, err
	}
	if !rows.Next() {
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("affiliate program settings not found")
	}
	if err := rows.Scan(
		&mode,
		&startedAt,
		&result.OrdinaryReferralRateBPS,
		&result.OrdinaryInviteeRateBPS,
		&bindingKind,
		&bindingBoundAt,
		&inviterID,
		&bindingAgentID,
		&result.BindingCustomerRateBPS,
		&result.BindingPartnerRateBPS,
		&inviterPartnerStatus,
		&inviterActivatedAt,
		&legacyInviterID,
		&legacyAgentID,
		&selfPartnerStatus,
		&selfPartnerRisk,
		&selfPolicyEnabled,
		&selfPolicyRateBPS,
		&selfPolicyEffective,
	); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	result.ProgramMode = mode
	result.ProgramLive = mode == service.AffiliateProgramModeLive
	if startedAt.Valid {
		result.ProgramStartedAt = &startedAt.Time
	}
	result.HasUpstreamRelationship = bindingKind.Valid ||
		legacyInviterID.Valid ||
		legacyAgentID.Valid
	if selfPartnerStatus.Valid {
		result.SelfPartnerStatus = selfPartnerStatus.String
	}
	if selfPartnerRisk.Valid {
		result.SelfPartnerRiskStatus = selfPartnerRisk.String
	}
	result.SelfCommissionEnabled = selfPolicyEnabled.Valid && selfPolicyEnabled.Bool
	if selfPolicyRateBPS.Valid {
		result.SelfCommissionRateBPS = selfPolicyRateBPS.Int32
	}
	if selfPolicyEffective.Valid {
		result.SelfCommissionEffectiveAt = &selfPolicyEffective.Time
	}
	if mode == service.AffiliateProgramModeOff {
		return result, nil
	}
	if result.ProgramLive && (!startedAt.Valid || input.OccurredAt.Before(startedAt.Time)) {
		return result, nil
	}
	if bindingKind.Valid && inviterID.Valid {
		result.BindingKind = bindingKind.String
		if bindingBoundAt.Valid {
			result.BindingBoundAt = &bindingBoundAt.Time
		}
		result.InviterUserID = inviterID.Int64
		if bindingAgentID.Valid {
			result.BindingAgentID = bindingAgentID.Int64
		}
		if inviterPartnerStatus.Valid {
			result.InviterPartnerStatus = inviterPartnerStatus.String
		}
		if inviterActivatedAt.Valid {
			result.InviterPartnerActivatedAt = &inviterActivatedAt.Time
		}
	}
	if mode == service.AffiliateProgramModeShadow {
		return result, nil
	}
	if result.BindingKind == "" {
		// Self-attributed purchases never consume the one-time ordinary
		// first-paid claim. The policy is snapshotted onto every future paid
		// lot/entitlement by the service result instead.
		return result, nil
	}

	insertRows := &entsql.Rows{}
	if err := client.Driver().Query(ctx, `
		INSERT INTO affiliate_first_paid_purchases (
			user_id, purchase_type, source_id, purchase_key,
			amount_micros, occurred_at
		)
		VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING user_id
	`, []any{
		input.UserID,
		input.PurchaseType,
		input.SourceID,
		input.PurchaseKey,
		input.AmountMicros,
		input.OccurredAt,
	}, insertRows); err != nil {
		return nil, err
	}
	if insertRows.Next() {
		if err := insertRows.Scan(&result.PurchaseID); err != nil {
			_ = insertRows.Close()
			return nil, err
		}
		result.Claimed = true
	}
	if err := insertRows.Err(); err != nil {
		_ = insertRows.Close()
		return nil, err
	}
	if err := insertRows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *affiliateRewardRepository) PostPlatformReward(
	ctx context.Context,
	input service.AffiliatePlatformRewardInput,
) error {
	client, err := r.clientForContext(ctx)
	if err != nil {
		return err
	}
	if input.AvailableAt.IsZero() {
		input.AvailableAt = time.Now()
	}
	var rate any
	if input.RateBPS != nil {
		rate = *input.RateBPS
	}
	rows := &entsql.Rows{}
	if err := client.Driver().Query(ctx, `
		INSERT INTO affiliate_reward_entries (
			beneficiary_user_id, consumer_user_id, reward_type,
			amount_micros, source_amount_micros, rate_bps,
			status, available_at, posted_at, source_type, source_id,
			idempotency_key, metadata
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			'posted', $7, NOW(), $8, NULLIF($9, 0),
			$10, jsonb_build_object('asset_symbol', '⚡')
		)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`, []any{
		input.BeneficiaryUserID,
		input.ConsumerUserID,
		input.RewardType,
		input.AmountMicros,
		input.SourceAmountMicros,
		rate,
		input.AvailableAt,
		input.SourceType,
		input.SourceID,
		input.IdempotencyKey,
	}, rows); err != nil {
		return err
	}
	var rewardID int64
	if !rows.Next() {
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}
	if err := rows.Scan(&rewardID); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var updateResult sql.Result
	if err := client.Driver().Exec(ctx, `
		UPDATE users
		SET balance = balance + ($1::numeric / 1000000),
			updated_at = NOW()
		WHERE id = $2
			AND deleted_at IS NULL
	`, []any{input.AmountMicros, input.BeneficiaryUserID}, &updateResult); err != nil {
		return err
	}
	affected, err := updateResult.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrUserNotFound
	}
	var lotResult sql.Result
	return client.Driver().Exec(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_id, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $5, FALSE, NOW())
		ON CONFLICT (source_key) DO NOTHING
	`, []any{
		input.BeneficiaryUserID,
		affiliateRewardBalanceLotSource(input.RewardType),
		rewardID,
		fmt.Sprintf("affiliate_reward:%d", rewardID),
		input.AmountMicros,
	}, &lotResult)
}

func (r *affiliateRewardRepository) SchedulePlatformReward(
	ctx context.Context,
	input service.AffiliatePlatformRewardInput,
) error {
	client, err := r.clientForContext(ctx)
	if err != nil {
		return err
	}
	var rate any
	if input.RateBPS != nil {
		rate = *input.RateBPS
	}
	var result sql.Result
	return client.Driver().Exec(ctx, `
		INSERT INTO affiliate_reward_entries (
			beneficiary_user_id, consumer_user_id, reward_type,
			amount_micros, source_amount_micros, rate_bps,
			status, available_at, source_type, source_id,
			idempotency_key, metadata
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			'pending', $7, $8, NULLIF($9, 0),
			$10, jsonb_build_object('asset_symbol', '⚡', 'maturity', 'T+1 Beijing')
		)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, []any{
		input.BeneficiaryUserID,
		input.ConsumerUserID,
		input.RewardType,
		input.AmountMicros,
		input.SourceAmountMicros,
		rate,
		input.AvailableAt,
		input.SourceType,
		input.SourceID,
		input.IdempotencyKey,
	}, &result)
}

type dueAffiliateReward struct {
	ID            int64
	BeneficiaryID int64
	RewardType    string
	AmountMicros  int64
}

func (r *affiliateRewardRepository) PostDuePlatformRewards(ctx context.Context, limit int) (_ int, err error) {
	if r == nil || r.db == nil {
		return 0, errors.New("affiliate reward repository db is nil")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, beneficiary_user_id, reward_type, amount_micros
		FROM affiliate_reward_entries
		WHERE status = 'pending'
			AND available_at <= NOW()
		ORDER BY available_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return 0, err
	}
	rewards := make([]dueAffiliateReward, 0, limit)
	for rows.Next() {
		var reward dueAffiliateReward
		if err := rows.Scan(&reward.ID, &reward.BeneficiaryID, &reward.RewardType, &reward.AmountMicros); err != nil {
			_ = rows.Close()
			return 0, err
		}
		rewards = append(rewards, reward)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	for _, reward := range rewards {
		var lotID int64
		insertErr := tx.QueryRowContext(ctx, `
			INSERT INTO balance_lots (
				user_id, source_type, source_id, source_key,
				original_amount_micros, remaining_amount_micros,
				affiliate_eligible, occurred_at
			)
			VALUES ($1, $2, $3, $4, $5, $5, FALSE, NOW())
			ON CONFLICT (source_key) DO NOTHING
			RETURNING id
		`,
			reward.BeneficiaryID,
			affiliateRewardBalanceLotSource(reward.RewardType),
			reward.ID,
			fmt.Sprintf("affiliate_reward:%d", reward.ID),
			reward.AmountMicros,
		).Scan(&lotID)
		switch {
		case insertErr == nil:
			result, updateErr := tx.ExecContext(ctx, `
				UPDATE users
				SET balance = balance + ($1::numeric / 1000000),
					updated_at = NOW()
				WHERE id = $2
					AND deleted_at IS NULL
			`, reward.AmountMicros, reward.BeneficiaryID)
			if updateErr != nil {
				return 0, updateErr
			}
			affected, updateErr := result.RowsAffected()
			if updateErr != nil {
				return 0, updateErr
			}
			if affected != 1 {
				return 0, service.ErrUserNotFound
			}
		case errors.Is(insertErr, sql.ErrNoRows):
			// A pre-existing lot proves this reward was already credited by an
			// earlier idempotent attempt; only repair the ledger status.
		default:
			return 0, insertErr
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE affiliate_reward_entries
			SET status = 'posted',
				posted_at = NOW()
			WHERE id = $1
				AND status = 'pending'
		`, reward.ID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	tx = nil
	return len(rewards), nil
}

func (r *affiliateRewardRepository) clientForContext(ctx context.Context) (*dbent.Client, error) {
	if client, ok := transactionClientFromContext(ctx); ok {
		return client, nil
	}
	if r == nil || r.client == nil {
		return nil, errors.New("affiliate reward repository client is nil")
	}
	return r.client, nil
}

func affiliateRewardBalanceLotSource(rewardType string) string {
	if strings.EqualFold(rewardType, "customer_rebate") {
		return "customer_rebate"
	}
	return "referral_bonus"
}
