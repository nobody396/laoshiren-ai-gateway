package repository

import (
	"context"
	"errors"
)

const monthlyCompensationDisplayScale = 10

// SumPublicCompensationCNY returns the cumulative customer-visible value of
// executed compensation. Balance compensation is already stored in display
// CNY. Legacy Builder Pass compensation stores one raw-credit row per linked
// subscription, so it is netted by run, de-duplicated across groups and scaled
// back to the customer-visible credit value. Reversals reduce the same run.
func (r *dashboardAggregationRepository) SumPublicCompensationCNY(ctx context.Context) (float64, error) {
	if r == nil || r.sql == nil {
		return 0, errors.New("public compensation repository is not configured")
	}

	rows, err := r.sql.QueryContext(ctx, `
WITH compensation_rows AS (
    SELECT
        id,
        user_id,
        asset_type,
        delta,
        reference_no,
        dedupe_key,
        group_id
    FROM account_change_records
    WHERE user_id <> 2
      AND (
          reason = 'compensation'
          OR (
              reason = 'admin_adjustment'
              AND (
                  (asset_type = 'subscription' AND COALESCE(reference_no, '') LIKE 'monthly-comp-%')
                  OR (
                      asset_type <> 'subscription'
                      AND COALESCE(notes, '') ~* '(赔付|补偿|compensation|goodwill)'
                  )
              )
          )
      )
),
direct_compensation AS (
    SELECT COALESCE(SUM(delta), 0)::numeric AS amount_cny
    FROM compensation_rows
    WHERE asset_type <> 'subscription'
),
monthly_runs AS (
    SELECT
        user_id,
        REGEXP_REPLACE(
            COALESCE(reference_no, dedupe_key, id::text),
            '-reversal$',
            ''
        ) AS run_key,
        SUM(delta)::numeric
            / GREATEST(COUNT(DISTINCT group_id), 1)
            * $1::numeric AS amount_cny
    FROM compensation_rows
    WHERE asset_type = 'subscription'
    GROUP BY
        user_id,
        REGEXP_REPLACE(
            COALESCE(reference_no, dedupe_key, id::text),
            '-reversal$',
            ''
        )
),
monthly_compensation AS (
    SELECT COALESCE(SUM(amount_cny), 0)::numeric AS amount_cny
    FROM monthly_runs
)
SELECT GREATEST(direct_compensation.amount_cny + monthly_compensation.amount_cny, 0)::double precision
FROM direct_compensation, monthly_compensation
`, monthlyCompensationDisplayScale)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, errors.New("public compensation query returned no row")
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return total, nil
}
