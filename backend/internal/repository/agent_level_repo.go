package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func (r *commissionRepository) GetAgentLevelRules(ctx context.Context) ([]service.AgentLevelRule, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			level_key,
			level_name,
			rate,
			monthly_consumption_threshold,
			cumulative_consumption_threshold,
			sort_order,
			enabled,
			updated_at
		FROM agent_level_rules
		ORDER BY sort_order ASC, rate ASC
	`)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AgentLevelRule, 0)
	for rows.Next() {
		var item service.AgentLevelRule
		var monthly, cumulative sql.NullFloat64
		var updatedAt time.Time
		if err := rows.Scan(
			&item.LevelKey,
			&item.LevelName,
			&item.Rate,
			&monthly,
			&cumulative,
			&item.SortOrder,
			&item.Enabled,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if monthly.Valid {
			item.MonthlyConsumptionThreshold = &monthly.Float64
		}
		if cumulative.Valid {
			item.CumulativeConsumptionThreshold = &cumulative.Float64
		}
		item.UpdatedAt = &updatedAt
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *commissionRepository) UpdateAgentLevelRules(ctx context.Context, rules []service.AgentLevelRule) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	tx, err := beginAgentLevelTx(ctx, r.sql)
	if err != nil {
		return err
	}
	if tx != nil {
		defer func() { _ = tx.Rollback() }()
	}
	exec := sqlExecutor(r.sql)
	if tx != nil {
		exec = tx
	}

	for _, rule := range rules {
		var monthly any
		if rule.MonthlyConsumptionThreshold != nil {
			monthly = *rule.MonthlyConsumptionThreshold
		}
		var cumulative any
		if rule.CumulativeConsumptionThreshold != nil {
			cumulative = *rule.CumulativeConsumptionThreshold
		}
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO agent_level_rules (
				level_key,
				level_name,
				rate,
				monthly_consumption_threshold,
				cumulative_consumption_threshold,
				sort_order,
				enabled,
				updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (level_key) DO UPDATE SET
				level_name = EXCLUDED.level_name,
				rate = EXCLUDED.rate,
				monthly_consumption_threshold = EXCLUDED.monthly_consumption_threshold,
				cumulative_consumption_threshold = EXCLUDED.cumulative_consumption_threshold,
				sort_order = EXCLUDED.sort_order,
				enabled = EXCLUDED.enabled,
				updated_at = NOW()
		`, rule.LevelKey, rule.LevelName, rule.Rate, monthly, cumulative, rule.SortOrder, rule.Enabled); err != nil {
			return err
		}
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (r *commissionRepository) GetAgentLevelState(ctx context.Context, agentID int64) (*service.AgentLevelState, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	var state service.AgentLevelState
	var temporary, nextLevel sql.NullString
	var evaluatedAt, createdAt, updatedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			agent_id,
			base_level_key,
			base_rate,
			permanent_level_key,
			temporary_level_key,
			current_level_key,
			current_rate,
			rate_source,
			last_evaluated_period,
			last_month_consumption,
			total_consumption,
			next_level_key,
			next_level_gap,
			evaluated_at,
			created_at,
			updated_at
		FROM agent_level_states
		WHERE agent_id = $1
	`, []any{agentID},
		&state.AgentID,
		&state.BaseLevelKey,
		&state.BaseRate,
		&state.PermanentLevelKey,
		&temporary,
		&state.CurrentLevelKey,
		&state.CurrentRate,
		&state.RateSource,
		&state.LastEvaluatedPeriod,
		&state.LastMonthConsumption,
		&state.TotalConsumption,
		&nextLevel,
		&state.NextLevelGap,
		&evaluatedAt,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if temporary.Valid {
		state.TemporaryLevelKey = &temporary.String
	}
	if nextLevel.Valid {
		state.NextLevelKey = &nextLevel.String
	}
	if evaluatedAt.Valid {
		state.EvaluatedAt = &evaluatedAt.Time
	}
	if createdAt.Valid {
		state.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		state.UpdatedAt = &updatedAt.Time
	}
	return &state, nil
}

func (r *commissionRepository) ListAgentIDs(ctx context.Context) ([]int64, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id
		FROM users
		WHERE role = 'agent' AND deleted_at IS NULL
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *commissionRepository) GetAgentLevelUsageStats(ctx context.Context, agentID int64, periodStart, periodEnd time.Time) (*service.AgentLevelUsageStats, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	var stats service.AgentLevelUsageStats
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $2 AND ul.created_at < $3 AND ul.actual_cost > 0), 0) AS last_month_consumption,
			COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.actual_cost > 0), 0) AS total_consumption
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		WHERE u.agent_id = $1 AND u.deleted_at IS NULL
	`, []any{agentID, periodStart, periodEnd}, &stats.LastMonthConsumption, &stats.TotalConsumption)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *commissionRepository) GetAgentManualBaseRate(ctx context.Context, agentID int64) (float64, string, error) {
	if r.sql == nil {
		return 0.05, service.AgentLevelRateSourceLevel, nil
	}
	var rate float64
	var source string
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			CASE
				WHEN arc.enabled = true THEN arc.consumption_rate
				ELSE COALESCE(light.rate, 0.050000)
			END AS base_rate,
			CASE
				WHEN arc.enabled = true THEN 'agent_manual_base'
				ELSE 'agent_level'
			END AS base_source
		FROM users u
		LEFT JOIN agent_rate_configs arc ON arc.agent_id = u.id
		LEFT JOIN agent_level_rules light ON light.level_key = 'light'
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`, []any{agentID}, &rate, &source)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return 0.05, service.AgentLevelRateSourceLevel, nil
	}
	return rate, source, err
}

func (r *commissionRepository) UpsertAgentLevelState(ctx context.Context, state *service.AgentLevelState) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	if state == nil {
		return nil
	}
	var temporary, nextLevel any
	if state.TemporaryLevelKey != nil {
		temporary = *state.TemporaryLevelKey
	}
	if state.NextLevelKey != nil {
		nextLevel = *state.NextLevelKey
	}
	var evaluatedAt any
	if state.EvaluatedAt != nil {
		evaluatedAt = *state.EvaluatedAt
	}
	var updatedAt time.Time
	err := scanSingleRow(ctx, r.sql, `
		INSERT INTO agent_level_states (
			agent_id,
			base_level_key,
			base_rate,
			permanent_level_key,
			temporary_level_key,
			current_level_key,
			current_rate,
			rate_source,
			last_evaluated_period,
			last_month_consumption,
			total_consumption,
			next_level_key,
			next_level_gap,
			evaluated_at,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())
		ON CONFLICT (agent_id) DO UPDATE SET
			base_level_key = EXCLUDED.base_level_key,
			base_rate = EXCLUDED.base_rate,
			permanent_level_key = EXCLUDED.permanent_level_key,
			temporary_level_key = EXCLUDED.temporary_level_key,
			current_level_key = EXCLUDED.current_level_key,
			current_rate = EXCLUDED.current_rate,
			rate_source = EXCLUDED.rate_source,
			last_evaluated_period = EXCLUDED.last_evaluated_period,
			last_month_consumption = EXCLUDED.last_month_consumption,
			total_consumption = EXCLUDED.total_consumption,
			next_level_key = EXCLUDED.next_level_key,
			next_level_gap = EXCLUDED.next_level_gap,
			evaluated_at = EXCLUDED.evaluated_at,
			updated_at = NOW()
		RETURNING updated_at
	`, []any{
		state.AgentID,
		state.BaseLevelKey,
		state.BaseRate,
		state.PermanentLevelKey,
		temporary,
		state.CurrentLevelKey,
		state.CurrentRate,
		state.RateSource,
		state.LastEvaluatedPeriod,
		state.LastMonthConsumption,
		state.TotalConsumption,
		nextLevel,
		state.NextLevelGap,
		evaluatedAt,
	}, &updatedAt)
	if err != nil {
		return err
	}
	state.UpdatedAt = &updatedAt
	return nil
}

func beginAgentLevelTx(ctx context.Context, exec sqlExecutor) (*sql.Tx, error) {
	transactor, ok := exec.(sqlTransactor)
	if !ok {
		return nil, nil
	}
	return transactor.BeginTx(ctx, nil)
}
