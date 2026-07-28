package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateRiskRepository struct {
	db *sql.DB
}

func NewAffiliateRiskRepository(db *sql.DB) service.AffiliateRiskRepository {
	return &affiliateRiskRepository{db: db}
}

func (r *affiliateRiskRepository) ListAffiliateRiskPrincipals(
	ctx context.Context,
	limit int,
) ([]service.AffiliateRiskPrincipal, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate risk repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			ap.agent_id,
			u.email,
			COALESCE(u.username, ''),
			ap.status,
			ap.risk_status,
			ap.risk_note,
			COALESCE((
				SELECT COUNT(*)
				FROM affiliate_reward_entries reward
				JOIN affiliate_performance_events event
					ON reward.source_type = 'confirmed_consumption'
					AND reward.source_id = event.id
				WHERE event.direct_agent_id = ap.agent_id
					AND reward.status = 'risk_hold'
			), 0)::integer,
			COALESCE((
				SELECT SUM(reward.amount_micros)
				FROM affiliate_reward_entries reward
				JOIN affiliate_performance_events event
					ON reward.source_type = 'confirmed_consumption'
					AND reward.source_id = event.id
				WHERE event.direct_agent_id = ap.agent_id
					AND reward.status = 'risk_hold'
			), 0)::bigint,
			COALESCE((
				SELECT COUNT(*)
				FROM agent_cash_commission_entries cash
				WHERE cash.agent_id = ap.agent_id
					AND cash.entry_type = 'earned'
					AND cash.posting_status = 'risk_hold'
			), 0)::integer,
			COALESCE((
				SELECT SUM(cash.amount_micros)
				FROM agent_cash_commission_entries cash
				WHERE cash.agent_id = ap.agent_id
					AND cash.entry_type = 'earned'
					AND cash.posting_status = 'risk_hold'
			), 0)::bigint,
			ap.updated_at
		FROM agent_principals ap
		JOIN users u
			ON u.id = ap.agent_id
			AND u.deleted_at IS NULL
		WHERE ap.status IN ('active', 'suspended')
		ORDER BY
			CASE ap.risk_status
				WHEN 'blocked' THEN 0
				WHEN 'review' THEN 1
				ELSE 2
			END,
			ap.updated_at DESC,
			ap.agent_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateRiskPrincipal, 0)
	for rows.Next() {
		var item service.AffiliateRiskPrincipal
		if err := rows.Scan(
			&item.AgentID,
			&item.Email,
			&item.Username,
			&item.AgentStatus,
			&item.RiskStatus,
			&item.RiskNote,
			&item.HeldRewardCount,
			&item.HeldRewardMicros,
			&item.HeldCashCount,
			&item.HeldCashMicros,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *affiliateRiskRepository) SetAffiliateAgentRisk(
	ctx context.Context,
	agentID int64,
	nextStatus string,
	reason string,
	operatorID int64,
) (_ *service.AffiliateRiskActionResult, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate risk repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return nil, err
	}
	var previousStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT risk_status
		FROM agent_principals
		WHERE agent_id = $1
		FOR UPDATE
	`, agentID).Scan(&previousStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateRiskAgentNotFound
	}
	if err != nil {
		return nil, err
	}

	result := &service.AffiliateRiskActionResult{
		AgentID:            agentID,
		PreviousRiskStatus: previousStatus,
		NextRiskStatus:     nextStatus,
		Reason:             reason,
	}
	if nextStatus == service.AffiliateRiskStatusClear {
		rewardCount, rewardMicros, userIDs, err := releaseAffiliateRiskRewards(ctx, tx, agentID)
		if err != nil {
			return nil, err
		}
		result.ReleasedRewardCount = rewardCount
		result.ReleasedRewardMicros = rewardMicros
		result.AffectedCreditUsers = userIDs

		rows, err := tx.QueryContext(ctx, `
			UPDATE agent_cash_commission_entries
			SET posting_status = 'posted',
				metadata = metadata || jsonb_build_object(
					'risk_released_at', NOW(),
					'risk_released_by', $2::bigint
				)
			WHERE agent_id = $1
				AND entry_type = 'earned'
				AND posting_status = 'risk_hold'
			RETURNING amount_micros
		`, agentID, operatorID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var amount int64
			if err := rows.Scan(&amount); err != nil {
				_ = rows.Close()
				return nil, err
			}
			result.ReleasedCashCount++
			result.ReleasedCashMicros += amount
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_principals
		SET risk_status = $1,
			risk_note = $2,
			reviewed_at = NOW(),
			reviewed_by = $3,
			updated_at = NOW()
		WHERE agent_id = $4
	`, nextStatus, reason, operatorID, agentID); err != nil {
		return nil, err
	}

	actionType := nextStatus
	if nextStatus == service.AffiliateRiskStatusBlocked {
		actionType = "block"
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_risk_actions (
			agent_id, action_type,
			previous_risk_status, next_risk_status,
			reason,
			released_reward_count, released_reward_micros,
			released_cash_count, released_cash_micros,
			operator_id,
			metadata
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			jsonb_build_object('timezone', 'Asia/Shanghai')
		)
		RETURNING id, created_at
	`,
		agentID,
		actionType,
		previousStatus,
		nextStatus,
		reason,
		result.ReleasedRewardCount,
		result.ReleasedRewardMicros,
		result.ReleasedCashCount,
		result.ReleasedCashMicros,
		operatorID,
	).Scan(&result.ID, &result.CreatedAt)
	if err != nil {
		return nil, err
	}

	noticeType, title, message := affiliateRiskNotice(nextStatus)
	if err := insertAffiliateAgentNotice(
		ctx,
		tx,
		agentID,
		noticeType,
		title,
		message,
		"risk_action",
		result.ID,
		fmt.Sprintf("risk-action:%d:notice", result.ID),
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func releaseAffiliateRiskRewards(
	ctx context.Context,
	tx *sql.Tx,
	agentID int64,
) (int32, int64, []int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT
			reward.id,
			reward.beneficiary_user_id,
			reward.reward_type,
			reward.amount_micros
		FROM affiliate_reward_entries reward
		JOIN affiliate_performance_events event
			ON reward.source_type = 'confirmed_consumption'
			AND reward.source_id = event.id
		WHERE event.direct_agent_id = $1
			AND reward.status = 'risk_hold'
		ORDER BY reward.id
		FOR UPDATE OF reward
	`, agentID)
	if err != nil {
		return 0, 0, nil, err
	}
	type heldReward struct {
		id            int64
		beneficiaryID int64
		rewardType    string
		amountMicros  int64
	}
	held := make([]heldReward, 0)
	for rows.Next() {
		var reward heldReward
		if err := rows.Scan(
			&reward.id,
			&reward.beneficiaryID,
			&reward.rewardType,
			&reward.amountMicros,
		); err != nil {
			_ = rows.Close()
			return 0, 0, nil, err
		}
		held = append(held, reward)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, nil, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, nil, err
	}

	var (
		count   int32
		total   int64
		userIDs = make([]int64, 0, len(held))
	)
	for _, reward := range held {
		var lotID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO balance_lots (
				user_id, source_type, source_id, source_key,
				original_amount_micros, remaining_amount_micros,
				affiliate_eligible, occurred_at
			)
			VALUES ($1, $2, $3, $4, $5, $5, FALSE, NOW())
			ON CONFLICT (source_key) DO NOTHING
			RETURNING id
		`,
			reward.beneficiaryID,
			affiliateRewardBalanceLotSource(reward.rewardType),
			reward.id,
			fmt.Sprintf("affiliate_reward:%d", reward.id),
			reward.amountMicros,
		).Scan(&lotID)
		switch {
		case err == nil:
			result, err := tx.ExecContext(ctx, `
				UPDATE users
				SET balance = balance + ($1::numeric / 1000000),
					updated_at = NOW()
				WHERE id = $2
					AND deleted_at IS NULL
			`, reward.amountMicros, reward.beneficiaryID)
			if err != nil {
				return 0, 0, nil, err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return 0, 0, nil, err
			}
			if affected != 1 {
				return 0, 0, nil, service.ErrUserNotFound
			}
		case errors.Is(err, sql.ErrNoRows):
			// Existing balance lot proves a previous idempotent credit.
		default:
			return 0, 0, nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE affiliate_reward_entries
			SET status = 'posted',
				posted_at = COALESCE(posted_at, NOW()),
				metadata = metadata || jsonb_build_object('risk_released_at', NOW())
			WHERE id = $1
				AND status = 'risk_hold'
		`, reward.id); err != nil {
			return 0, 0, nil, err
		}
		count++
		total += reward.amountMicros
		userIDs = append(userIDs, reward.beneficiaryID)
	}
	return count, total, userIDs, nil
}

func (r *affiliateRiskRepository) ReverseAffiliatePerformanceEvent(
	ctx context.Context,
	eventID int64,
	reason string,
	operatorID int64,
) (_ *service.AffiliatePerformanceReversal, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate risk repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if existing, err := getAffiliatePerformanceReversal(ctx, tx, eventID); err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var (
		consumerID int64
		agentID    sql.NullInt64
		eventType  string
		amount     int64
		program    string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			user_id,
			direct_agent_id,
			event_type,
			amount_micros,
			COALESCE(metadata ->> 'program_mode', '')
		FROM affiliate_performance_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&consumerID, &agentID, &eventType, &amount, &program)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliatePerformanceEventNotFound
	}
	if err != nil {
		return nil, err
	}
	if eventType != "confirmed_consumption" || amount <= 0 || program != service.AffiliateProgramModeLive {
		return nil, service.ErrAffiliatePerformanceEventNotReversible
	}
	if agentID.Valid {
		if err := lockAffiliateAgent(ctx, tx, agentID.Int64); err != nil {
			return nil, err
		}
	}

	eventKey := fmt.Sprintf("reversal:event:%d", eventID)
	var reversalEventID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, $2, 'consumption_reversal', $3,
			'performance_reversal', $4, $5, NOW(),
			jsonb_build_object(
				'program_mode', 'live',
				'reason', $6::text,
				'operator_id', $7::bigint
			)
		)
		ON CONFLICT (event_key) DO NOTHING
		RETURNING id
	`, consumerID, nullableInt64Value(agentID), amount, eventID, eventKey, reason, operatorID).Scan(&reversalEventID)
	if errors.Is(err, sql.ErrNoRows) {
		existing, queryErr := getAffiliatePerformanceReversal(ctx, tx, eventID)
		if queryErr != nil {
			return nil, queryErr
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return nil, commitErr
		}
		return existing, nil
	}
	if err != nil {
		return nil, err
	}
	reversedRewardMicros, affectedUsers, err := reverseAffiliateEventRewards(
		ctx,
		tx,
		eventID,
		reversalEventID,
		reason,
		operatorID,
	)
	if err != nil {
		return nil, err
	}
	reversedCashMicros, err := reverseAffiliateEventCash(
		ctx,
		tx,
		eventID,
		reversalEventID,
		reason,
		operatorID,
	)
	if err != nil {
		return nil, err
	}
	if agentID.Valid && reversedCashMicros > 0 {
		if err := holdAffiliateAgentAfterReversalIfNeeded(
			ctx,
			tx,
			agentID.Int64,
			reversalEventID,
			reason,
			operatorID,
		); err != nil {
			return nil, err
		}
	}

	out := &service.AffiliatePerformanceReversal{
		OriginalEventID:      eventID,
		ReversalEventID:      reversalEventID,
		ConsumerUserID:       consumerID,
		AmountMicros:         amount,
		ReversedRewardMicros: reversedRewardMicros,
		ReversedCashMicros:   reversedCashMicros,
		Reason:               reason,
		OperatorID:           operatorID,
		AffectedCreditUsers:  affectedUsers,
	}
	if agentID.Valid {
		agent := agentID.Int64
		out.DirectAgentID = &agent
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_performance_reversals (
			original_event_id, reversal_event_id,
			consumer_user_id, direct_agent_id,
			amount_micros,
			reversed_reward_micros, reversed_cash_micros,
			reason, operator_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`,
		out.OriginalEventID,
		out.ReversalEventID,
		out.ConsumerUserID,
		nullableInt64Value(agentID),
		out.AmountMicros,
		out.ReversedRewardMicros,
		out.ReversedCashMicros,
		out.Reason,
		out.OperatorID,
	).Scan(&out.ID, &out.CreatedAt)
	if err != nil {
		return nil, err
	}

	if agentID.Valid && reversedCashMicros > 0 {
		if err := insertAffiliateAgentNotice(
			ctx,
			tx,
			agentID.Int64,
			"commission_reversed",
			"佣金冲正通知",
			"一笔关联消费已被冲正，对应佣金已从可提现余额中扣回。",
			"performance_reversal",
			out.ID,
			fmt.Sprintf("performance-reversal:%d:notice", out.ID),
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func reverseAffiliateEventRewards(
	ctx context.Context,
	tx *sql.Tx,
	eventID int64,
	reversalEventID int64,
	reason string,
	operatorID int64,
) (int64, []int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT
			id,
			beneficiary_user_id,
			consumer_user_id,
			amount_micros,
			source_amount_micros,
			status
		FROM affiliate_reward_entries
		WHERE source_type = 'confirmed_consumption'
			AND source_id = $1
			AND reward_type <> 'reversal'
			AND status <> 'reversed'
		ORDER BY id
		FOR UPDATE
	`, eventID)
	if err != nil {
		return 0, nil, err
	}
	type rewardRow struct {
		id           int64
		beneficiary  int64
		consumer     int64
		amount       int64
		sourceAmount int64
		status       string
	}
	rewards := make([]rewardRow, 0)
	for rows.Next() {
		var item rewardRow
		if err := rows.Scan(
			&item.id,
			&item.beneficiary,
			&item.consumer,
			&item.amount,
			&item.sourceAmount,
			&item.status,
		); err != nil {
			_ = rows.Close()
			return 0, nil, err
		}
		rewards = append(rewards, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, nil, err
	}
	if err := rows.Close(); err != nil {
		return 0, nil, err
	}

	var (
		total   int64
		userIDs = make([]int64, 0, len(rewards))
	)
	for _, reward := range rewards {
		reversalStatus := "reversed"
		if reward.status == "posted" {
			result, err := tx.ExecContext(ctx, `
				UPDATE users
				SET balance = balance - ($1::numeric / 1000000),
					updated_at = NOW()
				WHERE id = $2
					AND deleted_at IS NULL
			`, reward.amount, reward.beneficiary)
			if err != nil {
				return 0, nil, err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return 0, nil, err
			}
			if affected != 1 {
				return 0, nil, service.ErrUserNotFound
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE balance_lots
				SET remaining_amount_micros = 0,
					updated_at = NOW()
				WHERE source_key = $1
			`, fmt.Sprintf("affiliate_reward:%d", reward.id)); err != nil {
				return 0, nil, err
			}
			reversalStatus = "posted"
			userIDs = append(userIDs, reward.beneficiary)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE affiliate_reward_entries
			SET status = 'reversed',
				metadata = metadata || jsonb_build_object(
					'reversed_at', NOW(),
					'reversed_by', $2::bigint,
					'reversal_reason', $3::text
				)
			WHERE id = $1
		`, reward.id, operatorID, reason); err != nil {
			return 0, nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO affiliate_reward_entries (
				beneficiary_user_id, consumer_user_id,
				reward_type, asset_type,
				amount_micros, source_amount_micros,
				status, available_at, posted_at,
				source_type, source_id, idempotency_key,
				reversal_of_id, metadata
			)
			VALUES (
				$1, $2,
				'reversal', 'platform_credit',
				$3, $4,
				$5::varchar, NOW(), CASE WHEN $5::varchar = 'posted' THEN NOW() ELSE NULL END,
				'performance_reversal', $6, $7,
				$8, jsonb_build_object(
					'direction', 'debit',
					'reason', $9::text,
					'operator_id', $10::bigint
				)
			)
			ON CONFLICT (idempotency_key) DO NOTHING
		`,
			reward.beneficiary,
			reward.consumer,
			reward.amount,
			reward.sourceAmount,
			reversalStatus,
			reversalEventID,
			fmt.Sprintf("reversal:event:%d:reward:%d", eventID, reward.id),
			reward.id,
			reason,
			operatorID,
		); err != nil {
			return 0, nil, err
		}
		total += reward.amount
	}
	return total, userIDs, nil
}

func reverseAffiliateEventCash(
	ctx context.Context,
	tx *sql.Tx,
	eventID int64,
	reversalEventID int64,
	reason string,
	operatorID int64,
) (int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT
			id,
			agent_id,
			consumer_user_id,
			amount_micros,
			source_amount_micros,
			customer_rebate_rate_bps,
			agent_commission_rate_bps,
			posting_status
		FROM agent_cash_commission_entries
		WHERE source_type = 'confirmed_consumption'
			AND source_id = $1
			AND entry_type = 'earned'
			AND posting_status IN ('posted', 'risk_hold')
		ORDER BY id
		FOR UPDATE
	`, eventID)
	if err != nil {
		return 0, err
	}
	type cashRow struct {
		id           int64
		agentID      int64
		consumerID   sql.NullInt64
		amount       int64
		sourceAmount sql.NullInt64
		customerRate sql.NullInt32
		agentRate    sql.NullInt32
		status       string
	}
	entries := make([]cashRow, 0)
	for rows.Next() {
		var item cashRow
		if err := rows.Scan(
			&item.id,
			&item.agentID,
			&item.consumerID,
			&item.amount,
			&item.sourceAmount,
			&item.customerRate,
			&item.agentRate,
			&item.status,
		); err != nil {
			_ = rows.Close()
			return 0, err
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	var total int64
	for _, entry := range entries {
		reversalStatus := "reversed"
		switch entry.status {
		case "risk_hold":
			if _, err := tx.ExecContext(ctx, `
				UPDATE agent_cash_commission_entries
				SET posting_status = 'reversed',
					metadata = metadata || jsonb_build_object(
						'reversed_at', NOW(),
						'reversed_by', $2::bigint,
						'reversal_reason', $3::text
					)
				WHERE id = $1
			`, entry.id, operatorID, reason); err != nil {
				return 0, err
			}
		case "posted":
			reversalStatus = "posted"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO agent_cash_commission_entries (
				agent_id, consumer_user_id, entry_type,
				amount_micros, source_amount_micros,
				customer_rebate_rate_bps, agent_commission_rate_bps,
				posting_status, source_type, source_id,
				idempotency_key, related_entry_id,
				metadata, occurred_at
			)
			VALUES (
				$1, $2, 'reversal',
				-($3::bigint), $4,
				$5, $6,
				$7, 'performance_reversal', $8,
				$9, $10,
				jsonb_build_object(
					'reason', $11::text,
					'operator_id', $12::bigint
				), NOW()
			)
			ON CONFLICT (idempotency_key) DO NOTHING
		`,
			entry.agentID,
			nullableInt64Value(entry.consumerID),
			entry.amount,
			nullableInt64Value(entry.sourceAmount),
			nullableInt32Value(entry.customerRate),
			nullableInt32Value(entry.agentRate),
			reversalStatus,
			reversalEventID,
			fmt.Sprintf("reversal:event:%d:cash:%d", eventID, entry.id),
			entry.id,
			reason,
			operatorID,
		); err != nil {
			return 0, err
		}
		total += entry.amount
	}
	return total, nil
}

func holdAffiliateAgentAfterReversalIfNeeded(
	ctx context.Context,
	tx *sql.Tx,
	agentID int64,
	reversalEventID int64,
	reason string,
	operatorID int64,
) error {
	var (
		availableMicros    int64
		processingCount    int32
		previousRiskStatus string
	)
	err := tx.QueryRowContext(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(amount_micros)
				FROM agent_cash_commission_entries
				WHERE agent_id = ap.agent_id
					AND posting_status = 'posted'
			), 0)::bigint,
			COALESCE((
				SELECT COUNT(*)
				FROM agent_withdrawal_requests
				WHERE agent_id = ap.agent_id
					AND status = 'processing'
			), 0)::integer,
			ap.risk_status
		FROM agent_principals ap
		WHERE ap.agent_id = $1
		FOR UPDATE
	`, agentID).Scan(&availableMicros, &processingCount, &previousRiskStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrAffiliateRiskAgentNotFound
	}
	if err != nil {
		return err
	}
	if availableMicros >= 0 && processingCount == 0 {
		return nil
	}

	riskReason := fmt.Sprintf(
		"消费冲正自动复核：%s（可提现余额微单位=%d，处理中提现=%d）",
		reason,
		availableMicros,
		processingCount,
	)
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_principals
		SET risk_status = 'review',
			risk_note = $1,
			reviewed_at = NOW(),
			reviewed_by = $2,
			updated_at = NOW()
		WHERE agent_id = $3
	`, riskReason, operatorID, agentID); err != nil {
		return err
	}
	var actionID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_risk_actions (
			agent_id, action_type,
			previous_risk_status, next_risk_status,
			reason, operator_id, metadata
		)
		VALUES (
			$1, 'review',
			$2, 'review',
			$3, $4,
			jsonb_build_object(
				'trigger', 'performance_reversal',
				'reversal_event_id', $5::bigint,
				'available_cash_micros', $6::bigint,
				'processing_withdrawal_count', $7::integer
			)
		)
		RETURNING id
	`,
		agentID,
		previousRiskStatus,
		riskReason,
		operatorID,
		reversalEventID,
		availableMicros,
		processingCount,
	).Scan(&actionID); err != nil {
		return err
	}
	return insertAffiliateAgentNotice(
		ctx,
		tx,
		agentID,
		"risk_review",
		"联盟账户复核中",
		"关联消费冲正后账户已自动进入风险复核，处理中提现不会在复核完成前打款。",
		"risk_action",
		actionID,
		fmt.Sprintf("performance-reversal:%d:risk-notice", reversalEventID),
	)
}

type affiliatePerformanceReversalQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getAffiliatePerformanceReversal(
	ctx context.Context,
	q affiliatePerformanceReversalQuerier,
	originalEventID int64,
) (*service.AffiliatePerformanceReversal, error) {
	out := &service.AffiliatePerformanceReversal{}
	var agentID sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT
			id,
			original_event_id,
			reversal_event_id,
			consumer_user_id,
			direct_agent_id,
			amount_micros,
			reversed_reward_micros,
			reversed_cash_micros,
			reason,
			operator_id,
			created_at
		FROM affiliate_performance_reversals
		WHERE original_event_id = $1
	`, originalEventID).Scan(
		&out.ID,
		&out.OriginalEventID,
		&out.ReversalEventID,
		&out.ConsumerUserID,
		&agentID,
		&out.AmountMicros,
		&out.ReversedRewardMicros,
		&out.ReversedCashMicros,
		&out.Reason,
		&out.OperatorID,
		&out.CreatedAt,
	)
	if agentID.Valid {
		agent := agentID.Int64
		out.DirectAgentID = &agent
	}
	return out, err
}

func affiliateRiskNotice(status string) (noticeType string, title string, message string) {
	switch status {
	case service.AffiliateRiskStatusClear:
		return "risk_cleared", "风控状态已解除", "风控复核已完成，冻结奖励已释放，提现与额度转换功能已恢复。"
	case service.AffiliateRiskStatusBlocked:
		return "risk_blocked", "联盟账户已暂停", "联盟账户因风险原因已暂停，提现、转换和新增推广关系暂不可用。"
	default:
		return "risk_review", "联盟账户复核中", "联盟账户正在进行风险复核，期间提现、转换和新增推广关系暂不可用。"
	}
}

func nullableInt64Value(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullableInt32Value(value sql.NullInt32) any {
	if value.Valid {
		return value.Int32
	}
	return nil
}
