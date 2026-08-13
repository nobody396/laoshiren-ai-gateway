package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// GetCostAccountingRealUsage sums, per group, the amount charged to the user
// and the true upstream cost incurred by the business. Credit groups must use
// subscription billing rows (billing_type=1); public pay-as-you-go groups must
// use standard billing rows (billing_type=0). The upstream cost expression is
// shared with usage statistics and daily billing reconciliation, including
// fixed-price image cost captured in account_stats_cost.
func (r *opsRepository) GetCostAccountingRealUsage(ctx context.Context, creditGroupIDs, payAsYouGoGroupIDs []int64, start, end time.Time) (map[int64]service.CostAccountingUsageRow, error) {
	result := make(map[int64]service.CostAccountingUsageRow)
	if r == nil || r.db == nil || (len(creditGroupIDs) == 0 && len(payAsYouGoGroupIDs) == 0) {
		return result, nil
	}

	const q = `
SELECT group_id,
       COUNT(*) AS request_count,
       COALESCE(SUM(actual_cost), 0) AS raw_credits,
       COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) AS real_cost
FROM usage_logs
WHERE created_at >= $3
  AND created_at < $4
  AND (
    (billing_type = 1 AND group_id = ANY($1))
    OR
    (billing_type = 0 AND group_id = ANY($2))
  )
  AND (actual_cost > 0 OR COALESCE(account_stats_cost, total_cost) > 0)
GROUP BY group_id`

	rows, err := r.db.QueryContext(ctx, q, pq.Array(creditGroupIDs), pq.Array(payAsYouGoGroupIDs), start, end)
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
