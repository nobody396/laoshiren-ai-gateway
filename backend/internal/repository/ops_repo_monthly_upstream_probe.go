package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func (r *opsRepository) InsertMonthlyUpstreamProbeResult(ctx context.Context, input *service.MonthlyUpstreamProbePoint) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return fmt.Errorf("nil input")
	}
	probePath := input.ProbePath
	if probePath == "" {
		probePath = service.MonthlyUpstreamProbePathDirectUpstream
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO monthly_upstream_probe_results (
  account_id, account_name, platform, model, probe_path, status, http_status,
  latency_ms, error_code, error_message, checked_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		opsNullInt64(nonZeroInt64Ptr(input.AccountID)),
		input.AccountName,
		input.Platform,
		input.Model,
		probePath,
		input.Status,
		opsNullInt(input.HTTPStatus),
		input.LatencyMs,
		input.ErrorCode,
		input.ErrorMessage,
		input.CheckedAt,
	)
	return err
}

func (r *opsRepository) ListMonthlyUpstreamProbeResults(ctx context.Context, since time.Time) ([]service.MonthlyUpstreamProbePoint, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT
  COALESCE(account_id, 0),
  account_name,
  platform,
  model,
  probe_path,
  status,
  http_status,
  latency_ms,
  error_code,
  error_message,
  checked_at
FROM monthly_upstream_probe_results
WHERE checked_at >= $1
ORDER BY checked_at ASC, account_name ASC`, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.MonthlyUpstreamProbePoint, 0, 256)
	for rows.Next() {
		var point service.MonthlyUpstreamProbePoint
		var httpStatus sql.NullInt64
		if err := rows.Scan(
			&point.AccountID,
			&point.AccountName,
			&point.Platform,
			&point.Model,
			&point.ProbePath,
			&point.Status,
			&httpStatus,
			&point.LatencyMs,
			&point.ErrorCode,
			&point.ErrorMessage,
			&point.CheckedAt,
		); err != nil {
			return nil, err
		}
		if httpStatus.Valid {
			status := int(httpStatus.Int64)
			point.HTTPStatus = &status
		}
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func nonZeroInt64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
