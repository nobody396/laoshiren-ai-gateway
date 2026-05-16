package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

const agentSettlementAmountEpsilon = 0.00000001

type sqlTransactor interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

func (r *commissionRepository) GetCommissionRates(ctx context.Context) (*service.CommissionRates, error) {
	if r.sql == nil {
		return defaultCommissionRatesForRepo(), nil
	}
	var rates service.CommissionRates
	err := scanSingleRow(ctx, r.sql, `
		SELECT consumption_rate, first_recharge_invitee_rate, first_recharge_referral_rate, updated_at
		FROM agent_commission_settings
		WHERE id = 1
	`, nil, &rates.ConsumptionRate, &rates.FirstRechargeInviteeRate, &rates.FirstRechargeReferralRate, &rates.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return defaultCommissionRatesForRepo(), nil
	}
	if err != nil {
		return nil, err
	}
	return &rates, nil
}

func (r *commissionRepository) UpdateCommissionRates(ctx context.Context, rates *service.CommissionRates) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	return scanSingleRow(ctx, r.sql, `
		INSERT INTO agent_commission_settings (
			id,
			consumption_rate,
			first_recharge_invitee_rate,
			first_recharge_referral_rate,
			updated_at
		) VALUES (1, $1, $2, $3, NOW())
		ON CONFLICT (id) DO UPDATE SET
			consumption_rate = EXCLUDED.consumption_rate,
			first_recharge_invitee_rate = EXCLUDED.first_recharge_invitee_rate,
			first_recharge_referral_rate = EXCLUDED.first_recharge_referral_rate,
			updated_at = NOW()
		RETURNING updated_at
	`, []any{
		rates.ConsumptionRate,
		rates.FirstRechargeInviteeRate,
		rates.FirstRechargeReferralRate,
	}, &rates.UpdatedAt)
}

func (r *commissionRepository) GetAgentRateConfig(ctx context.Context, agentID int64) (*service.AgentRateConfig, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	var config service.AgentRateConfig
	var createdAt, updatedAt time.Time
	err := scanSingleRow(ctx, r.sql, `
		SELECT agent_id, consumption_rate, enabled, created_at, updated_at
		FROM agent_rate_configs
		WHERE agent_id = $1
	`, []any{agentID}, &config.AgentID, &config.ConsumptionRate, &config.Enabled, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if isMissingAgentManagementRelation(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	config.CreatedAt = &createdAt
	config.UpdatedAt = &updatedAt
	rate, source, err := r.ResolveAgentConsumptionRate(ctx, agentID)
	if err != nil {
		return nil, err
	}
	config.EffectiveConsumptionRate = rate
	config.RateSource = source
	return &config, nil
}

func (r *commissionRepository) UpsertAgentRateConfig(ctx context.Context, config *service.AgentRateConfig) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	var updatedAt time.Time
	if err := scanSingleRow(ctx, r.sql, `
		INSERT INTO agent_rate_configs (agent_id, consumption_rate, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (agent_id) DO UPDATE SET
			consumption_rate = EXCLUDED.consumption_rate,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
		RETURNING updated_at
	`, []any{config.AgentID, config.ConsumptionRate, config.Enabled}, &updatedAt); err != nil {
		return err
	}
	config.UpdatedAt = &updatedAt
	return nil
}

func (r *commissionRepository) ResolveAgentConsumptionRate(ctx context.Context, agentID int64) (float64, string, error) {
	if r.sql == nil {
		return 0.06, "global", nil
	}
	var rate float64
	var source string
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			CASE
				WHEN als.agent_id IS NOT NULL THEN als.current_rate
				WHEN arc.enabled = true THEN arc.consumption_rate
				ELSE gs.consumption_rate
			END AS consumption_rate,
			CASE
				WHEN als.agent_id IS NOT NULL THEN als.rate_source
				WHEN arc.enabled = true THEN 'agent_override'
				ELSE 'global'
			END AS rate_source
		FROM agent_commission_settings gs
		LEFT JOIN agent_rate_configs arc ON arc.agent_id = $1
		LEFT JOIN agent_level_states als ON als.agent_id = $1
		WHERE gs.id = 1
	`, []any{agentID}, &rate, &source)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return 0.06, "global", nil
	}
	return rate, source, err
}

func (r *commissionRepository) ListAdminAgents(ctx context.Context, params pagination.PaginationParams, filters service.AdminAgentListFilters) ([]service.AdminAgentSummary, *pagination.PaginationResult, error) {
	return r.listAdminAgents(ctx, params, filters, nil)
}

func (r *commissionRepository) GetAdminAgent(ctx context.Context, agentID int64, start, end *time.Time) (*service.AdminAgentSummary, error) {
	items, _, err := r.listAdminAgents(ctx, pagination.PaginationParams{Page: 1, PageSize: 1}, service.AdminAgentListFilters{Start: start, End: end}, &agentID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, service.ErrUserNotFound
	}
	return &items[0], nil
}

func (r *commissionRepository) listAdminAgents(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.AdminAgentListFilters,
	agentID *int64,
) ([]service.AdminAgentSummary, *pagination.PaginationResult, error) {
	if r.sql == nil {
		return nil, nil, fmt.Errorf("sql executor is not configured")
	}

	where := []string{"u.role = 'agent'", "u.deleted_at IS NULL"}
	baseArgs := make([]any, 0)
	if agentID != nil {
		baseArgs = append(baseArgs, *agentID)
		where = append(where, fmt.Sprintf("u.id = $%d", len(baseArgs)))
	}
	search := strings.TrimSpace(strings.ToLower(filters.Search))
	if search != "" {
		baseArgs = append(baseArgs, "%"+search+"%")
		idx := len(baseArgs)
		where = append(where, fmt.Sprintf(`(
			LOWER(u.email) LIKE $%d OR
			LOWER(COALESCE(u.username, '')) LIKE $%d OR
			LOWER(COALESCE(u.invite_code, '')) LIKE $%d
		)`, idx, idx, idx))
	}
	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM users u WHERE %s`, whereSQL)
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, baseArgs, &total); err != nil {
		return nil, nil, fmt.Errorf("count admin agents: %w", err)
	}

	args := append([]any{}, baseArgs...)
	periodUsageCond, periodCommissionCond := buildAgentPeriodConditions(&args, "ul", "cr", filters.Start, filters.End)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)
	args = append(args, monthStart)
	monthStartIdx := len(args)
	args = append(args, monthEnd)
	monthEndIdx := len(args)

	orderBy := adminAgentOrderBy(params.SortBy)
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	query := fmt.Sprintf(`
		SELECT
			u.id,
			u.email,
			COALESCE(u.username, ''),
			u.status,
			u.invite_code,
			u.created_at,
			u.last_active_at,
			COALESCE(invited.total, 0) AS invited_user_count,
			COALESCE(total_usage.total, 0) AS total_consumption,
			COALESCE(period_usage.total, 0) AS period_consumption,
			COALESCE(total_comm.total, 0) AS total_commission,
			COALESCE(period_comm.total, 0) AS period_commission,
			COALESCE(month_comm.total, 0) AS this_month_commission,
			COALESCE(settled.total, 0) AS settled_commission,
			GREATEST(COALESCE(total_comm.total, 0) - COALESCE(settled.total, 0), 0) AS unsettled_commission,
			CASE
				WHEN als.agent_id IS NOT NULL THEN als.current_rate
				WHEN arc.enabled = true THEN arc.consumption_rate
				ELSE gs.consumption_rate
			END AS consumption_rate,
			CASE
				WHEN als.agent_id IS NOT NULL THEN als.rate_source
				WHEN arc.enabled = true THEN 'agent_override'
				ELSE 'global'
			END AS rate_source,
			COALESCE(arc.enabled, false) AS override_enabled,
			arc.consumption_rate AS override_consumption_rate,
			COALESCE(als.current_level_key, '') AS current_level_key,
			COALESCE(als.permanent_level_key, '') AS permanent_level_key,
			als.temporary_level_key,
			COALESCE(als.base_rate, 0) AS base_rate,
			COALESCE(als.last_evaluated_period, '') AS last_evaluated_period,
			COALESCE(als.last_month_consumption, 0) AS last_month_consumption,
			als.next_level_key,
			COALESCE(als.next_level_gap, 0) AS next_level_gap
		FROM users u
		CROSS JOIN agent_commission_settings gs
		LEFT JOIN (
			SELECT agent_id, COUNT(*) AS total
			FROM users
			WHERE agent_id IS NOT NULL AND deleted_at IS NULL
			GROUP BY agent_id
		) invited ON invited.agent_id = u.id
		LEFT JOIN (
			SELECT users.agent_id, SUM(ul.actual_cost) AS total
			FROM usage_logs ul
			JOIN users ON users.id = ul.user_id
			WHERE users.agent_id IS NOT NULL AND users.deleted_at IS NULL
			GROUP BY users.agent_id
		) total_usage ON total_usage.agent_id = u.id
		LEFT JOIN (
			SELECT users.agent_id, SUM(ul.actual_cost) AS total
			FROM usage_logs ul
			JOIN users ON users.id = ul.user_id
			WHERE users.agent_id IS NOT NULL AND users.deleted_at IS NULL %s
			GROUP BY users.agent_id
		) period_usage ON period_usage.agent_id = u.id
		LEFT JOIN (
			SELECT beneficiary_id, SUM(amount) AS total
			FROM commission_records
			GROUP BY beneficiary_id
		) total_comm ON total_comm.beneficiary_id = u.id
		LEFT JOIN (
			SELECT beneficiary_id, SUM(amount) AS total
			FROM commission_records cr
			WHERE true %s
			GROUP BY beneficiary_id
		) period_comm ON period_comm.beneficiary_id = u.id
		LEFT JOIN (
			SELECT beneficiary_id, SUM(amount) AS total
			FROM commission_records cr
			WHERE cr.created_at >= $%d AND cr.created_at <= $%d
			GROUP BY beneficiary_id
		) month_comm ON month_comm.beneficiary_id = u.id
		LEFT JOIN (
			SELECT agent_id, SUM(amount) AS total
			FROM agent_settlements
			WHERE status = 'completed'
			GROUP BY agent_id
		) settled ON settled.agent_id = u.id
		LEFT JOIN agent_rate_configs arc ON arc.agent_id = u.id
		LEFT JOIN agent_level_states als ON als.agent_id = u.id
		WHERE gs.id = 1 AND %s
		ORDER BY %s %s, u.id DESC
		LIMIT $%d OFFSET $%d
	`, periodUsageCond, periodCommissionCond, monthStartIdx, monthEndIdx, whereSQL, orderBy, sortOrder, len(args)+1, len(args)+2)

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin agents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminAgentSummary, 0)
	for rows.Next() {
		var item service.AdminAgentSummary
		var inviteCode sql.NullString
		var lastActiveAt sql.NullTime
		var overrideRate sql.NullFloat64
		var temporaryLevel, nextLevel sql.NullString
		if err := rows.Scan(
			&item.AgentID,
			&item.Email,
			&item.Username,
			&item.Status,
			&inviteCode,
			&item.CreatedAt,
			&lastActiveAt,
			&item.InvitedUserCount,
			&item.TotalConsumption,
			&item.PeriodConsumption,
			&item.TotalCommission,
			&item.PeriodCommission,
			&item.ThisMonthCommission,
			&item.SettledCommission,
			&item.UnsettledCommission,
			&item.ConsumptionRate,
			&item.RateSource,
			&item.OverrideEnabled,
			&overrideRate,
			&item.CurrentLevel,
			&item.PermanentLevel,
			&temporaryLevel,
			&item.BaseRate,
			&item.LastEvaluatedPeriod,
			&item.LastMonthConsumption,
			&nextLevel,
			&item.NextLevelGap,
		); err != nil {
			return nil, nil, fmt.Errorf("scan admin agent: %w", err)
		}
		if inviteCode.Valid {
			item.InviteCode = &inviteCode.String
		}
		if lastActiveAt.Valid {
			item.LastActiveAt = &lastActiveAt.Time
		}
		if overrideRate.Valid {
			item.OverrideConsumptionRate = &overrideRate.Float64
		}
		if temporaryLevel.Valid {
			item.TemporaryLevel = &temporaryLevel.String
		}
		if nextLevel.Valid {
			item.NextLevelKey = &nextLevel.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *commissionRepository) ListAdminAgentUsers(
	ctx context.Context,
	agentID int64,
	params pagination.PaginationParams,
	start, end *time.Time,
) ([]service.AdminAgentUserStat, *pagination.PaginationResult, error) {
	if r.sql == nil {
		return nil, nil, fmt.Errorf("sql executor is not configured")
	}
	var total int64
	if err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM users WHERE agent_id = $1 AND deleted_at IS NULL`, []any{agentID}, &total); err != nil {
		return nil, nil, fmt.Errorf("count agent users: %w", err)
	}

	args := []any{agentID}
	periodUsageCond, periodCommissionCond := buildAgentPeriodConditions(&args, "ul", "cr", start, end)
	orderBy := adminAgentUserOrderBy(params.SortBy)
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	query := fmt.Sprintf(`
		SELECT
			u.id,
			u.email,
			COALESCE(u.username, ''),
			u.created_at,
			u.first_invited_topup_at,
			u.total_recharged,
			COALESCE(total_usage.total, 0) AS total_consumption,
			COALESCE(period_usage.total, 0) AS period_consumption,
			COALESCE(total_comm.total, 0) AS total_commission,
			COALESCE(period_comm.total, 0) AS period_commission,
			COALESCE(total_comm.record_count, 0) AS commission_record_count,
			total_comm.last_commission_at
		FROM users u
		LEFT JOIN (
			SELECT user_id, SUM(actual_cost) AS total
			FROM usage_logs
			WHERE user_id IN (SELECT id FROM users WHERE agent_id = $1 AND deleted_at IS NULL)
			GROUP BY user_id
		) total_usage ON total_usage.user_id = u.id
		LEFT JOIN (
			SELECT user_id, SUM(actual_cost) AS total
			FROM usage_logs ul
			WHERE user_id IN (SELECT id FROM users WHERE agent_id = $1 AND deleted_at IS NULL) %s
			GROUP BY user_id
		) period_usage ON period_usage.user_id = u.id
		LEFT JOIN (
			SELECT user_id, SUM(amount) AS total, COUNT(*) AS record_count, MAX(created_at) AS last_commission_at
			FROM commission_records
			WHERE beneficiary_id = $1
			GROUP BY user_id
		) total_comm ON total_comm.user_id = u.id
		LEFT JOIN (
			SELECT user_id, SUM(amount) AS total
			FROM commission_records cr
			WHERE beneficiary_id = $1 %s
			GROUP BY user_id
		) period_comm ON period_comm.user_id = u.id
		WHERE u.agent_id = $1 AND u.deleted_at IS NULL
		ORDER BY %s %s, u.id DESC
		LIMIT $%d OFFSET $%d
	`, periodUsageCond, periodCommissionCond, orderBy, sortOrder, len(args)+1, len(args)+2)

	args = append(args, params.Limit(), params.Offset())
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list agent users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminAgentUserStat, 0)
	for rows.Next() {
		var item service.AdminAgentUserStat
		var firstTopupAt, lastCommissionAt sql.NullTime
		if err := rows.Scan(
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.JoinedAt,
			&firstTopupAt,
			&item.TotalRecharged,
			&item.TotalConsumption,
			&item.PeriodConsumption,
			&item.TotalCommission,
			&item.PeriodCommission,
			&item.CommissionRecordCount,
			&lastCommissionAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan agent user: %w", err)
		}
		if firstTopupAt.Valid {
			item.FirstInvitedTopupAt = &firstTopupAt.Time
		}
		if lastCommissionAt.Valid {
			item.LastCommissionAt = &lastCommissionAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *commissionRepository) ListAdminAgentCommissions(
	ctx context.Context,
	agentID int64,
	params pagination.PaginationParams,
	typeFilter string,
	start, end *time.Time,
) ([]service.AdminAgentCommissionRecord, *pagination.PaginationResult, error) {
	if r.sql == nil {
		return nil, nil, fmt.Errorf("sql executor is not configured")
	}
	clauses := []string{"cr.beneficiary_id = $1"}
	args := []any{agentID}
	argIdx := 2
	if typeFilter != "" {
		values := expandCommissionTypes(typeFilter)
		placeholders := make([]string, 0, len(values))
		for _, value := range values {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIdx))
			args = append(args, value)
			argIdx++
		}
		clauses = append(clauses, fmt.Sprintf("cr.type IN (%s)", strings.Join(placeholders, ", ")))
	}
	if start != nil {
		clauses = append(clauses, fmt.Sprintf("cr.created_at >= $%d", argIdx))
		args = append(args, *start)
		argIdx++
	}
	if end != nil {
		clauses = append(clauses, fmt.Sprintf("cr.created_at <= $%d", argIdx))
		args = append(args, *end)
		argIdx++
	}
	whereSQL := strings.Join(clauses, " AND ")

	var total int64
	if err := scanSingleRow(ctx, r.sql, fmt.Sprintf(`SELECT COUNT(*) FROM commission_records cr WHERE %s`, whereSQL), args, &total); err != nil {
		return nil, nil, fmt.Errorf("count agent commissions: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			cr.id,
			cr.beneficiary_id,
			cr.user_id,
			COALESCE(u.email, ''),
			COALESCE(u.username, ''),
			cr.amount,
			cr.source_amount,
			cr.type,
			cr.rate,
			cr.rate_source,
			cr.source_id,
			cr.note,
			cr.created_at
		FROM commission_records cr
		LEFT JOIN users u ON u.id = cr.user_id
		WHERE %s
		ORDER BY cr.created_at DESC, cr.id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)
	args = append(args, params.Limit(), params.Offset())
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list agent commissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminAgentCommissionRecord, 0)
	for rows.Next() {
		var item service.AdminAgentCommissionRecord
		var rate sql.NullFloat64
		var rateSource, note sql.NullString
		var sourceID sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.BeneficiaryID,
			&item.UserID,
			&item.UserEmail,
			&item.UserUsername,
			&item.Amount,
			&item.SourceAmount,
			&item.Type,
			&rate,
			&rateSource,
			&sourceID,
			&note,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan agent commission: %w", err)
		}
		if rate.Valid {
			item.Rate = rate.Float64
		}
		if rateSource.Valid {
			item.RateSource = rateSource.String
		}
		if sourceID.Valid {
			item.SourceID = &sourceID.Int64
		}
		if note.Valid {
			item.Note = &note.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *commissionRepository) ListAgentSettlements(ctx context.Context, agentID int64, params pagination.PaginationParams) ([]service.AgentSettlement, *pagination.PaginationResult, error) {
	if r.sql == nil {
		return nil, nil, fmt.Errorf("sql executor is not configured")
	}
	var total int64
	if err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM agent_settlements WHERE agent_id = $1`, []any{agentID}, &total); err != nil {
		if isMissingAgentManagementRelation(err) {
			return []service.AgentSettlement{}, paginationResultFromTotal(0, params), nil
		}
		return nil, nil, fmt.Errorf("count agent settlements: %w", err)
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, agent_id, amount, operator_id, note, status, created_at
		FROM agent_settlements
		WHERE agent_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, agentID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, fmt.Errorf("list agent settlements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AgentSettlement, 0)
	for rows.Next() {
		var item service.AgentSettlement
		if err := rows.Scan(&item.ID, &item.AgentID, &item.Amount, &item.OperatorID, &item.Note, &item.Status, &item.CreatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan agent settlement: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *commissionRepository) SumAgentSettlements(ctx context.Context, agentID int64) (float64, error) {
	if r.sql == nil {
		return 0, nil
	}
	var total float64
	err := scanSingleRow(ctx, r.sql, `
		SELECT COALESCE(SUM(amount), 0)
		FROM agent_settlements
		WHERE agent_id = $1 AND status = 'completed'
	`, []any{agentID}, &total)
	if isMissingAgentManagementRelation(err) {
		return 0, nil
	}
	return total, err
}

func (r *commissionRepository) CreateAgentSettlement(ctx context.Context, settlement *service.AgentSettlement) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	return insertAgentSettlement(ctx, r.sql, settlement)
}

func (r *commissionRepository) CreateAgentSettlementIfAvailable(ctx context.Context, settlement *service.AgentSettlement) (err error) {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	db, ok := r.sql.(sqlTransactor)
	if !ok {
		return fmt.Errorf("sql transactor is not configured")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, settlement.AgentID); err != nil {
		return fmt.Errorf("lock agent settlement: %w", err)
	}

	var totalCommission, settledCommission float64
	if err := scanSingleRow(ctx, tx, `
		SELECT
			COALESCE((SELECT SUM(amount) FROM commission_records WHERE beneficiary_id = $1), 0),
			COALESCE((SELECT SUM(amount) FROM agent_settlements WHERE agent_id = $1 AND status = 'completed'), 0)
	`, []any{settlement.AgentID}, &totalCommission, &settledCommission); err != nil {
		if isMissingAgentManagementRelation(err) {
			return infraerrors.BadRequest("SETTLEMENT_EXCEEDS_UNSETTLED", "settlement amount exceeds unsettled commission")
		}
		return fmt.Errorf("calculate agent settlement balance: %w", err)
	}

	available := totalCommission - settledCommission
	if settlement.Amount-available > agentSettlementAmountEpsilon {
		return infraerrors.BadRequest("SETTLEMENT_EXCEEDS_UNSETTLED", "settlement amount exceeds unsettled commission")
	}

	if settlement.Status == "" {
		settlement.Status = service.AgentSettlementStatusCompleted
	}
	if err := insertAgentSettlement(ctx, tx, settlement); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

func insertAgentSettlement(ctx context.Context, q sqlExecutor, settlement *service.AgentSettlement) error {
	return scanSingleRow(ctx, q, `
		INSERT INTO agent_settlements (agent_id, amount, operator_id, note, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, []any{
		settlement.AgentID,
		settlement.Amount,
		settlement.OperatorID,
		settlement.Note,
		settlement.Status,
	}, &settlement.ID, &settlement.CreatedAt)
}

func buildAgentPeriodConditions(args *[]any, usageAlias, commissionAlias string, start, end *time.Time) (usageCond, commissionCond string) {
	if start != nil {
		*args = append(*args, *start)
		idx := len(*args)
		usageCond += fmt.Sprintf(" AND %s.created_at >= $%d", usageAlias, idx)
		commissionCond += fmt.Sprintf(" AND %s.created_at >= $%d", commissionAlias, idx)
	}
	if end != nil {
		*args = append(*args, *end)
		idx := len(*args)
		usageCond += fmt.Sprintf(" AND %s.created_at <= $%d", usageAlias, idx)
		commissionCond += fmt.Sprintf(" AND %s.created_at <= $%d", commissionAlias, idx)
	}
	return usageCond, commissionCond
}

func adminAgentOrderBy(sortBy string) string {
	switch sortBy {
	case "email":
		return "LOWER(u.email)"
	case "invited_user_count":
		return "invited_user_count"
	case "total_consumption":
		return "total_consumption"
	case "period_commission":
		return "period_commission"
	case "unsettled_commission":
		return "unsettled_commission"
	case "created_at":
		return "u.created_at"
	default:
		return "total_commission"
	}
}

func adminAgentUserOrderBy(sortBy string) string {
	switch sortBy {
	case "joined_at":
		return "u.created_at"
	case "total_consumption":
		return "total_consumption"
	case "period_commission":
		return "period_commission"
	case "last_commission_at":
		return "last_commission_at"
	default:
		return "total_commission"
	}
}

func defaultCommissionRatesForRepo() *service.CommissionRates {
	return &service.CommissionRates{
		ConsumptionRate:           0.06,
		FirstRechargeInviteeRate:  0.10,
		FirstRechargeReferralRate: 0.05,
	}
}

func isMissingAgentManagementRelation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "no such table")
}
