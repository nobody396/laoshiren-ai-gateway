package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

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
	if cmd.UsageLogID > 0 {
		if err := tx.QueryRowContext(ctx, `
			SELECT mode
			FROM affiliate_program_settings
			WHERE id = 1
		`).Scan(&result.AffiliateProgramMode); err != nil {
			return err
		}
	}
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		updates, err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost)
		if err != nil {
			return err
		}
		result.SubscriptionUsageUpdates = updates
		if cmd.UsageLogID > 0 {
			settlement := usageBillingAffiliateSettlement{}
			confirmedMicros, err := attributeUsageBillingMonthlyConsumption(
				ctx,
				tx,
				cmd,
				service.AffiliateMicrosFromFloat(cmd.SubscriptionCost),
				&settlement,
			)
			if err != nil {
				return err
			}
			result.MonthlyConfirmedMicros = confirmedMicros
			result.ConfirmedConsumptionMicros += confirmedMicros
			result.AffiliateCustomerRebateMicros += settlement.CustomerRebateMicros
			result.AffiliateAgentCommissionMicros += settlement.AgentCommissionMicros
		}
	}

	if cmd.BalanceCost > 0 {
		newBalance, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		if cmd.UsageLogID > 0 {
			settlement := usageBillingAffiliateSettlement{}
			confirmedMicros, err := attributeUsageBillingBalanceConsumption(
				ctx,
				tx,
				cmd,
				service.AffiliateMicrosFromFloat(cmd.BalanceCost),
				&settlement,
			)
			if err != nil {
				return err
			}
			result.BalanceConfirmedMicros = confirmedMicros
			result.ConfirmedConsumptionMicros += confirmedMicros
			result.AffiliateCustomerRebateMicros += settlement.CustomerRebateMicros
			result.AffiliateAgentCommissionMicros += settlement.AgentCommissionMicros
		}
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
	if result.AffiliateCustomerRebateMicros > 0 {
		var currentBalance float64
		if err := tx.QueryRowContext(ctx, `
			SELECT balance
			FROM users
			WHERE id = $1
				AND deleted_at IS NULL
		`, cmd.UserID).Scan(&currentBalance); err != nil {
			return err
		}
		result.NewBalance = &currentBalance
	}

	return nil
}

type usageBillingAffiliateSettlement struct {
	CustomerRebateMicros  int64
	AgentCommissionMicros int64
}

type usageBillingBalanceLot struct {
	ID                       int64
	SourceType               string
	RemainingMicros          int64
	AffiliateEligible        bool
	AffiliatePolicy          string
	DirectPartnerID          sql.NullInt64
	CustomerRebateRateBPS    int32
	PartnerCommissionRateBPS int32
	AcquiredAt               time.Time
}

func attributeUsageBillingBalanceConsumption(
	ctx context.Context,
	tx *sql.Tx,
	cmd *service.UsageBillingCommand,
	amountMicros int64,
	settlement *usageBillingAffiliateSettlement,
) (int64, error) {
	if amountMicros <= 0 {
		return 0, nil
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT
			id, source_type, remaining_amount_micros, affiliate_eligible,
			affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps,
			occurred_at
		FROM balance_lots
		WHERE user_id = $1
			AND remaining_amount_micros > 0
		ORDER BY occurred_at, id
		FOR UPDATE
	`, cmd.UserID)
	if err != nil {
		return 0, err
	}
	lots := make([]usageBillingBalanceLot, 0, 4)
	for rows.Next() {
		var lot usageBillingBalanceLot
		if err := rows.Scan(
			&lot.ID,
			&lot.SourceType,
			&lot.RemainingMicros,
			&lot.AffiliateEligible,
			&lot.AffiliatePolicy,
			&lot.DirectPartnerID,
			&lot.CustomerRebateRateBPS,
			&lot.PartnerCommissionRateBPS,
			&lot.AcquiredAt,
		); err != nil {
			_ = rows.Close()
			return 0, err
		}
		lots = append(lots, lot)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	availableMicros := int64(0)
	for _, lot := range lots {
		if lot.RemainingMicros > amountMicros-availableMicros {
			availableMicros = amountMicros
			break
		}
		availableMicros += lot.RemainingMicros
	}
	if gapMicros := amountMicros - availableMicros; gapMicros > 0 {
		var reconciliationLotID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO balance_lots (
				user_id, source_type, source_key,
				original_amount_micros, remaining_amount_micros,
				affiliate_eligible, occurred_at
			)
			VALUES ($1, 'legacy_unattributed', $2, $3, $3, FALSE, NOW())
			ON CONFLICT (source_key) DO UPDATE
			SET source_key = EXCLUDED.source_key
			RETURNING id
		`,
			cmd.UserID,
			fmt.Sprintf("legacy_reconcile:usage:%d", cmd.UsageLogID),
			gapMicros,
		).Scan(&reconciliationLotID)
		if err != nil {
			return 0, err
		}
		lots = append(lots, usageBillingBalanceLot{
			ID:              reconciliationLotID,
			SourceType:      service.AffiliateSourceLegacyUnattributed,
			RemainingMicros: gapMicros,
			AffiliatePolicy: service.AffiliateSourcePolicyNone,
		})
	}

	eventKey := fmt.Sprintf("usage:%d:balance", cmd.UsageLogID)
	remainingMicros := amountMicros
	confirmedMicros := int64(0)
	for _, lot := range lots {
		if remainingMicros <= 0 {
			break
		}
		consumeMicros := lot.RemainingMicros
		if consumeMicros > remainingMicros {
			consumeMicros = remainingMicros
		}
		if consumeMicros <= 0 {
			continue
		}
		eligiblePart := int64(0)
		if lot.AffiliateEligible {
			eligiblePart = consumeMicros
		}
		qualificationPart := int64(0)
		if service.AffiliateSourceTracksQualification(lot.SourceType) {
			qualificationPart = consumeMicros
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE balance_lots
			SET remaining_amount_micros = remaining_amount_micros - $1,
				updated_at = NOW()
			WHERE id = $2
				AND remaining_amount_micros >= $1
		`, consumeMicros, lot.ID)
		if err != nil {
			return 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if affected != 1 {
			return 0, errors.New("affiliate balance lot changed while locked")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO balance_lot_consumptions (
				balance_lot_id, user_id, usage_log_id, usage_event_key,
				amount_micros, affiliate_eligible_amount_micros
			)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (balance_lot_id, usage_event_key) DO NOTHING
		`, lot.ID, cmd.UserID, cmd.UsageLogID, eventKey, consumeMicros, eligiblePart); err != nil {
			return 0, err
		}
		confirmedMicros += qualificationPart
		remainingMicros -= consumeMicros
		if qualificationPart > 0 {
			directPartnerID := int64(0)
			if lot.DirectPartnerID.Valid {
				directPartnerID = lot.DirectPartnerID.Int64
			}
			affiliatePolicy := service.AffiliateSourcePolicyNone
			customerRateBPS := int32(0)
			partnerRateBPS := int32(0)
			if lot.AffiliateEligible {
				affiliatePolicy = lot.AffiliatePolicy
				customerRateBPS = lot.CustomerRebateRateBPS
				partnerRateBPS = lot.PartnerCommissionRateBPS
			} else {
				directPartnerID = 0
			}
			affiliateSettlement, err := recordUsageBillingPerformanceEvent(
				ctx,
				tx,
				cmd.UserID,
				cmd.UsageLogID,
				"balance_usage",
				fmt.Sprintf("confirmed:usage:%d:balance:lot:%d", cmd.UsageLogID, lot.ID),
				qualificationPart,
				affiliatePolicy,
				directPartnerID,
				customerRateBPS,
				partnerRateBPS,
				lot.AcquiredAt,
			)
			if err != nil {
				return 0, err
			}
			if settlement != nil {
				settlement.CustomerRebateMicros += affiliateSettlement.CustomerRebateMicros
				settlement.AgentCommissionMicros += affiliateSettlement.AgentCommissionMicros
			}
		}
	}
	if remainingMicros != 0 {
		return 0, errors.New("affiliate balance attribution did not consume requested amount")
	}
	return confirmedMicros, nil
}

func attributeUsageBillingMonthlyConsumption(
	ctx context.Context,
	tx *sql.Tx,
	cmd *service.UsageBillingCommand,
	creditMicros int64,
	settlement *usageBillingAffiliateSettlement,
) (int64, error) {
	if cmd.SubscriptionID == nil || creditMicros <= 0 {
		return 0, nil
	}
	var (
		sourceType        string
		affiliateEligible bool
		affiliatePolicy   string
		directPartnerID   sql.NullInt64
		customerRateBPS   int32
		partnerRateBPS    int32
		confirmedMicros   int64
		acquiredAt        time.Time
	)
	err := tx.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT
				c.id,
				c.confirmed_consumption_micros,
				c.source_type IN ('paid_redeem', 'paid_topup')
					AS qualification_eligible
			FROM monthly_entitlement_cycles c
			JOIN monthly_entitlement_cycle_subscriptions cs
				ON cs.cycle_id = c.id
			WHERE cs.user_subscription_id = $1
				AND c.user_id = $2
				AND c.starts_at <= NOW()
				AND c.ends_at > NOW()
				AND c.used_credit_micros < c.credit_limit_micros
			ORDER BY c.starts_at, c.id
			LIMIT 1
			FOR UPDATE OF c
		),
		updated AS (
			UPDATE monthly_entitlement_cycles c
			SET
				used_credit_micros = LEAST(c.credit_limit_micros, c.used_credit_micros + $3),
				confirmed_consumption_micros = CASE
					WHEN candidate.qualification_eligible THEN LEAST(
						c.sale_price_micros,
						FLOOR(
							c.sale_price_micros::numeric
							* LEAST(c.credit_limit_micros, c.used_credit_micros + $3)::numeric
							/ c.credit_limit_micros::numeric
						)::bigint
					)
					ELSE 0
				END,
				updated_at = NOW()
			FROM candidate
			WHERE c.id = candidate.id
			RETURNING
				c.source_type,
				c.affiliate_eligible,
				c.affiliate_policy,
				c.direct_partner_id,
				c.customer_rebate_rate_bps,
				c.partner_commission_rate_bps,
				c.created_at,
				c.confirmed_consumption_micros
					- candidate.confirmed_consumption_micros AS confirmed_delta_micros
		)
		SELECT
			source_type,
			affiliate_eligible,
			affiliate_policy,
			direct_partner_id,
			customer_rebate_rate_bps,
			partner_commission_rate_bps,
			created_at,
			GREATEST(0, confirmed_delta_micros)
		FROM updated
	`, *cmd.SubscriptionID, cmd.UserID, creditMicros).Scan(
		&sourceType,
		&affiliateEligible,
		&affiliatePolicy,
		&directPartnerID,
		&customerRateBPS,
		&partnerRateBPS,
		&acquiredAt,
		&confirmedMicros,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if service.AffiliateSourceTracksQualification(sourceType) && confirmedMicros > 0 {
		affiliatePolicyForEvent := service.AffiliateSourcePolicyNone
		directPartnerIDForEvent := int64(0)
		customerRateBPSForEvent := int32(0)
		partnerRateBPSForEvent := int32(0)
		if affiliateEligible {
			affiliatePolicyForEvent = affiliatePolicy
			if directPartnerID.Valid {
				directPartnerIDForEvent = directPartnerID.Int64
			}
			customerRateBPSForEvent = customerRateBPS
			partnerRateBPSForEvent = partnerRateBPS
		}
		affiliateSettlement, err := recordUsageBillingPerformanceEvent(
			ctx,
			tx,
			cmd.UserID,
			cmd.UsageLogID,
			"monthly_usage",
			fmt.Sprintf("confirmed:usage:%d:monthly", cmd.UsageLogID),
			confirmedMicros,
			affiliatePolicyForEvent,
			directPartnerIDForEvent,
			customerRateBPSForEvent,
			partnerRateBPSForEvent,
			acquiredAt,
		)
		if err != nil {
			return 0, err
		}
		if settlement != nil {
			*settlement = affiliateSettlement
		}
	}
	return confirmedMicros, nil
}

func recordUsageBillingPerformanceEvent(
	ctx context.Context,
	tx *sql.Tx,
	userID int64,
	usageLogID int64,
	sourceType string,
	eventKey string,
	amountMicros int64,
	affiliatePolicy string,
	directPartnerID int64,
	customerRateBPS int32,
	partnerRateBPS int32,
	acquiredAt time.Time,
) (usageBillingAffiliateSettlement, error) {
	var (
		mode      string
		startedAt sql.NullTime
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT mode, started_at
		FROM affiliate_program_settings
		WHERE id = 1
	`).Scan(&mode, &startedAt); err != nil {
		return usageBillingAffiliateSettlement{}, err
	}
	if mode == service.AffiliateProgramModeOff {
		return usageBillingAffiliateSettlement{}, nil
	}
	if mode == service.AffiliateProgramModeLive {
		// Fail closed for any source acquired before the frozen live cutover.
		// Shadow lots retain projected attribution for observability, but can
		// never become payable merely because the program later switches live.
		if !startedAt.Valid ||
			acquiredAt.IsZero() ||
			acquiredAt.Before(startedAt.Time) ||
			startedAt.Time.After(time.Now()) {
			return usageBillingAffiliateSettlement{}, nil
		}
	}
	var performanceEventID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id,
			direct_agent_id,
			event_type,
			amount_micros,
			affiliate_policy,
			customer_rebate_rate_bps,
			partner_commission_rate_bps,
			source_type,
			source_id,
			event_key,
			occurred_at,
			metadata
		)
		VALUES (
			$1,
			NULLIF($6, 0),
			'confirmed_consumption',
			$2,
			$7::varchar,
			$8,
			$9,
			$3,
			$4,
			$5,
			NOW(),
			jsonb_build_object(
				'program_mode', $10::text,
				'attribution_policy', $7::varchar
			)
		)
		ON CONFLICT (event_key) DO NOTHING
		RETURNING id
	`,
		userID,
		amountMicros,
		sourceType,
		usageLogID,
		eventKey,
		directPartnerID,
		affiliatePolicy,
		customerRateBPS,
		partnerRateBPS,
		mode,
	).Scan(&performanceEventID)
	if errors.Is(err, sql.ErrNoRows) {
		return usageBillingAffiliateSettlement{}, nil
	}
	if err != nil {
		return usageBillingAffiliateSettlement{}, err
	}
	if mode != service.AffiliateProgramModeLive {
		return usageBillingAffiliateSettlement{}, nil
	}
	switch affiliatePolicy {
	case service.AffiliateSourcePolicyPartnerUsage:
		return settleUsageBillingAgentPool(
			ctx,
			tx,
			performanceEventID,
			userID,
			directPartnerID,
			amountMicros,
			customerRateBPS,
			partnerRateBPS,
		)
	case service.AffiliateSourcePolicyPartnerSelfUsage:
		return settleUsageBillingSelfPool(
			ctx,
			tx,
			performanceEventID,
			userID,
			directPartnerID,
			amountMicros,
			customerRateBPS,
			partnerRateBPS,
		)
	default:
		return usageBillingAffiliateSettlement{}, nil
	}
}

func settleUsageBillingAgentPool(
	ctx context.Context,
	tx *sql.Tx,
	performanceEventID int64,
	consumerUserID int64,
	agentID int64,
	sourceAmountMicros int64,
	customerRateBPS int32,
	agentRateBPS int32,
) (usageBillingAffiliateSettlement, error) {
	var (
		agentStatus     sql.NullString
		agentRiskStatus sql.NullString
	)
	if agentID <= 0 || agentID == consumerUserID {
		return usageBillingAffiliateSettlement{}, errors.New("invalid direct partner attribution")
	}
	err := tx.QueryRowContext(ctx, `
		SELECT status, risk_status
		FROM agent_principals
		WHERE agent_id = $1
	`, agentID).Scan(
		&agentStatus,
		&agentRiskStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		agentStatus = sql.NullString{String: "terminated", Valid: true}
		agentRiskStatus = sql.NullString{String: "blocked", Valid: true}
	} else if err != nil {
		return usageBillingAffiliateSettlement{}, err
	}
	if customerRateBPS < 0 ||
		agentRateBPS < 0 ||
		customerRateBPS+agentRateBPS != service.AffiliateAgentPoolRateBPS {
		return usageBillingAffiliateSettlement{}, errors.New("invalid affiliate source pool snapshot")
	}
	partnerCashPosted := agentStatus.Valid &&
		agentStatus.String == "active" &&
		agentRiskStatus.Valid &&
		agentRiskStatus.String == "clear"
	partnerTerminated := agentStatus.Valid && agentStatus.String == "terminated"
	settlement := usageBillingAffiliateSettlement{}

	totalPoolMicros := usageBillingRateAmountMicros(sourceAmountMicros, service.AffiliateAgentPoolRateBPS)
	customerRebateMicros := usageBillingRateAmountMicros(sourceAmountMicros, customerRateBPS)
	agentCommissionMicros := totalPoolMicros - customerRebateMicros
	if customerRebateMicros > 0 {
		// The customer rebate belongs to the customer's binding snapshot and is
		// never frozen merely because the partner is under review.
		rewardStatus := "posted"
		var rewardID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO affiliate_reward_entries (
				beneficiary_user_id, consumer_user_id, reward_type,
				amount_micros, source_amount_micros, rate_bps,
				status, available_at, posted_at,
				source_type, source_id, idempotency_key, metadata
			)
			VALUES (
				$1, $1, 'customer_rebate',
				$2, $3, $4,
				$5::varchar, NOW(), CASE WHEN $5::text = 'posted' THEN NOW() ELSE NULL END,
				'confirmed_consumption', $6, $7,
				jsonb_build_object('asset_symbol', '⚡')
			)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id
		`,
			consumerUserID,
			customerRebateMicros,
			sourceAmountMicros,
			customerRateBPS,
			rewardStatus,
			performanceEventID,
			fmt.Sprintf("confirmed:%d:customer-rebate", performanceEventID),
		).Scan(&rewardID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return usageBillingAffiliateSettlement{}, err
		}
		if err == nil && rewardStatus == "posted" {
			result, err := tx.ExecContext(ctx, `
				UPDATE users
				SET balance = balance + ($1::numeric / 1000000),
					updated_at = NOW()
				WHERE id = $2
					AND deleted_at IS NULL
			`, customerRebateMicros, consumerUserID)
			if err != nil {
				return usageBillingAffiliateSettlement{}, err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return usageBillingAffiliateSettlement{}, err
			}
			if affected != 1 {
				return usageBillingAffiliateSettlement{}, service.ErrUserNotFound
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO balance_lots (
					user_id, source_type, source_id, source_key,
					original_amount_micros, remaining_amount_micros,
					affiliate_eligible, occurred_at
				)
				VALUES ($1, 'customer_rebate', $2, $3, $4, $4, FALSE, NOW())
				ON CONFLICT (source_key) DO NOTHING
			`, consumerUserID, rewardID, fmt.Sprintf("affiliate_reward:%d", rewardID), customerRebateMicros); err != nil {
				return usageBillingAffiliateSettlement{}, err
			}
			settlement.CustomerRebateMicros = customerRebateMicros
		}
	}

	if agentCommissionMicros > 0 && !partnerTerminated {
		postingStatus := "risk_hold"
		if partnerCashPosted {
			postingStatus = "posted"
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO agent_cash_commission_entries (
				agent_id, consumer_user_id, entry_type,
				amount_micros, source_amount_micros,
				customer_rebate_rate_bps, agent_commission_rate_bps,
				posting_status, source_type, source_id,
				idempotency_key, metadata, occurred_at
			)
			VALUES (
				$1, $2, 'earned',
				$3, $4, $5, $6,
				$7, 'confirmed_consumption', $8,
				$9, '{}'::jsonb, NOW()
			)
			ON CONFLICT (idempotency_key) DO NOTHING
		`,
			agentID,
			consumerUserID,
			agentCommissionMicros,
			sourceAmountMicros,
			customerRateBPS,
			agentRateBPS,
			postingStatus,
			performanceEventID,
			fmt.Sprintf("confirmed:%d:agent-cash", performanceEventID),
		)
		if err != nil {
			return usageBillingAffiliateSettlement{}, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return usageBillingAffiliateSettlement{}, err
		}
		if affected == 1 && postingStatus == "posted" {
			settlement.AgentCommissionMicros = agentCommissionMicros
		}
	}
	return settlement, nil
}

func settleUsageBillingSelfPool(
	ctx context.Context,
	tx *sql.Tx,
	performanceEventID int64,
	consumerUserID int64,
	agentID int64,
	sourceAmountMicros int64,
	customerRateBPS int32,
	agentRateBPS int32,
) (usageBillingAffiliateSettlement, error) {
	if agentID <= 0 || agentID != consumerUserID {
		return usageBillingAffiliateSettlement{}, errors.New("invalid partner self attribution")
	}
	if customerRateBPS != 0 || agentRateBPS != service.AffiliateAgentPoolRateBPS {
		return usageBillingAffiliateSettlement{}, errors.New("invalid partner self source pool snapshot")
	}

	var agentStatus, agentRiskStatus string
	err := tx.QueryRowContext(ctx, `
		SELECT status, risk_status
		FROM agent_principals
		WHERE agent_id = $1
	`, agentID).Scan(&agentStatus, &agentRiskStatus)
	if errors.Is(err, sql.ErrNoRows) {
		// Missing principals are semantically terminated: historical
		// attribution stays auditable but can no longer earn cash.
		return usageBillingAffiliateSettlement{}, nil
	}
	if err != nil {
		return usageBillingAffiliateSettlement{}, err
	}
	if agentStatus == "terminated" {
		return usageBillingAffiliateSettlement{}, nil
	}

	postingStatus := "risk_hold"
	if agentStatus == "active" && agentRiskStatus == "clear" {
		postingStatus = "posted"
	}
	commissionMicros := usageBillingRateAmountMicros(
		sourceAmountMicros,
		service.AffiliateAgentPoolRateBPS,
	)
	if commissionMicros <= 0 {
		return usageBillingAffiliateSettlement{}, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, consumer_user_id, entry_type,
			amount_micros, source_amount_micros,
			customer_rebate_rate_bps, agent_commission_rate_bps,
			posting_status, source_type, source_id,
			idempotency_key, metadata, occurred_at
		)
		VALUES (
			$1, $1, 'earned',
			$2, $3,
			0, $4,
			$5, 'confirmed_consumption', $6,
			$7,
			jsonb_build_object(
				'attribution_policy', 'PARTNER_SELF_USAGE',
				'display_type', 'self_consumption_commission'
			),
			NOW()
		)
		ON CONFLICT (idempotency_key) DO NOTHING
	`,
		agentID,
		commissionMicros,
		sourceAmountMicros,
		agentRateBPS,
		postingStatus,
		performanceEventID,
		fmt.Sprintf("confirmed:%d:self-cash", performanceEventID),
	)
	if err != nil {
		return usageBillingAffiliateSettlement{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return usageBillingAffiliateSettlement{}, err
	}
	if affected == 1 && postingStatus == "posted" {
		return usageBillingAffiliateSettlement{AgentCommissionMicros: commissionMicros}, nil
	}
	return usageBillingAffiliateSettlement{}, nil
}

func usageBillingRateAmountMicros(sourceMicros int64, rateBPS int32) int64 {
	if sourceMicros <= 0 || rateBPS <= 0 {
		return 0
	}
	value := new(big.Int).Mul(big.NewInt(sourceMicros), big.NewInt(int64(rateBPS)))
	value.Quo(value, big.NewInt(10_000))
	if !value.IsInt64() {
		return 0
	}
	return value.Int64()
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
		if isUsageBillingLimitExceeded(err) {
			return capUsageBillingSingleSubscription(ctx, tx, target, costUSD)
		}
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
		return capUsageBillingSharedSubscriptions(ctx, tx, target, explicitNeedle, legacyNeedle, costUSD, true, false, false)
	}
	if usageBillingLimitExceeded(weeklyUsage, weeklyLimit, costUSD) {
		return capUsageBillingSharedSubscriptions(ctx, tx, target, explicitNeedle, legacyNeedle, costUSD, false, true, false)
	}
	if usageBillingLimitExceeded(monthlyUsage, monthlyLimit, costUSD) {
		return capUsageBillingSharedSubscriptions(ctx, tx, target, explicitNeedle, legacyNeedle, costUSD, false, false, true)
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

func isUsageBillingLimitExceeded(err error) bool {
	return errors.Is(err, service.ErrDailyLimitExceeded) ||
		errors.Is(err, service.ErrWeeklyLimitExceeded) ||
		errors.Is(err, service.ErrMonthlyLimitExceeded)
}

func capUsageBillingSingleSubscription(ctx context.Context, tx *sql.Tx, target *usageBillingSubscriptionTarget, costUSD float64) ([]service.SubscriptionUsageUpdate, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = CASE
				WHEN g.daily_limit_usd IS NOT NULL AND g.daily_limit_usd > 0 AND us.daily_usage_usd + $1 > g.daily_limit_usd
					THEN g.daily_limit_usd
				ELSE us.daily_usage_usd
			END,
			weekly_usage_usd = CASE
				WHEN g.weekly_limit_usd IS NOT NULL AND g.weekly_limit_usd > 0 AND us.weekly_usage_usd + $1 > g.weekly_limit_usd
					THEN g.weekly_limit_usd
				ELSE us.weekly_usage_usd
			END,
			monthly_usage_usd = CASE
				WHEN g.monthly_limit_usd IS NOT NULL AND g.monthly_limit_usd > 0 AND us.monthly_usage_usd + $1 > g.monthly_limit_usd
					THEN g.monthly_limit_usd
				ELSE us.monthly_usage_usd
			END,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`, costUSD, target.ID)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, service.ErrSubscriptionNotFound
	}
	return []service.SubscriptionUsageUpdate{{
		UserID:  target.UserID,
		GroupID: target.GroupID,
		CostUSD: costUSD,
	}}, nil
}

func capUsageBillingSharedSubscriptions(
	ctx context.Context,
	tx *sql.Tx,
	target *usageBillingSubscriptionTarget,
	explicitNeedle string,
	legacyNeedle string,
	costUSD float64,
	capDaily bool,
	capWeekly bool,
	capMonthly bool,
) ([]service.SubscriptionUsageUpdate, error) {
	rows, err := tx.QueryContext(ctx, `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = CASE
				WHEN $5 AND g.daily_limit_usd IS NOT NULL AND g.daily_limit_usd > 0
					THEN g.daily_limit_usd
				ELSE us.daily_usage_usd
			END,
			weekly_usage_usd = CASE
				WHEN $6 AND g.weekly_limit_usd IS NOT NULL AND g.weekly_limit_usd > 0
					THEN g.weekly_limit_usd
				ELSE us.weekly_usage_usd
			END,
			monthly_usage_usd = CASE
				WHEN $7 AND g.monthly_limit_usd IS NOT NULL AND g.monthly_limit_usd > 0
					THEN g.monthly_limit_usd
				ELSE us.monthly_usage_usd
			END,
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
				OR ($4 <> '' AND STRPOS(COALESCE(us.notes, ''), $4) > 0)
			)
		RETURNING us.user_id, us.group_id
	`, target.UserID, service.SubscriptionStatusActive, explicitNeedle, legacyNeedle, capDaily, capWeekly, capMonthly)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	updates := make([]service.SubscriptionUsageUpdate, 0, 2)
	for rows.Next() {
		var update service.SubscriptionUsageUpdate
		if err := rows.Scan(&update.UserID, &update.GroupID); err != nil {
			return nil, err
		}
		update.CostUSD = costUSD
		updates = append(updates, update)
	}
	if err := rows.Err(); err != nil {
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
