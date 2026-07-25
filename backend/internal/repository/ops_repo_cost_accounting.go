package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// GetCostAccountingRealUsage sums, per group, the raw credits consumed
// (usage_logs.actual_cost, already the amount debited from a credit group's
// pool) and the true CNY-equivalent cost this business incurred
// (actual_cost * account_rate_multiplier / rate_multiplier), using each
// row's own historical multiplier snapshot rather than current live rates.
// billing_type = 1 matches the subscription/credit billing path used
// elsewhere in this codebase (see monthly_compensation.py's usage query).
func (r *opsRepository) GetCostAccountingRealUsage(ctx context.Context, groupIDs []int64, start, end time.Time) (map[int64]service.CostAccountingUsageRow, error) {
	result := make(map[int64]service.CostAccountingUsageRow)
	if r == nil || r.db == nil || len(groupIDs) == 0 {
		return result, nil
	}

	const q = `
SELECT group_id,
       COUNT(*) AS request_count,
       COALESCE(SUM(actual_cost), 0) AS raw_credits,
       COALESCE(SUM(actual_cost * COALESCE(account_rate_multiplier, 1) / NULLIF(rate_multiplier, 0)), 0) AS real_cost
FROM usage_logs
WHERE group_id = ANY($1)
  AND created_at >= $2
  AND created_at < $3
  AND billing_type = 1
  AND actual_cost > 0
GROUP BY group_id`

	rows, err := r.db.QueryContext(ctx, q, pq.Array(groupIDs), start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var row service.CostAccountingUsageRow
		var rawCredits, realCost sql.NullFloat64
		if err := rows.Scan(&row.GroupID, &row.RequestCount, &rawCredits, &realCost); err != nil {
			return nil, err
		}
		row.RawCredits = rawCredits.Float64
		row.RealCostCNY = realCost.Float64
		result[row.GroupID] = row
	}
	return result, rows.Err()
}
