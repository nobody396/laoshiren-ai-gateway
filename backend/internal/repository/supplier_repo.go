package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

type supplierRepository struct {
	db *sql.DB
}

// NewSupplierRepository creates a supplier repository backed by SQL.
func NewSupplierRepository(db *sql.DB) service.SupplierRepository {
	return &supplierRepository{db: db}
}

func (r *supplierRepository) runInTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin supplier tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *supplierRepository) Create(ctx context.Context, supplier *service.Supplier) error {
	return r.runInTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO suppliers (
				name, website_url, base_url, api_key, upstream_group,
				contact_platform, contact_value, status, cost_rmb_per_usd, notes,
				probe_enabled, probe_model, probe_interval_minutes, last_probe_status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING id, created_at, updated_at`,
			supplier.Name,
			supplier.WebsiteURL,
			supplier.BaseURL,
			supplier.APIKey,
			supplier.UpstreamGroup,
			supplier.ContactPlatform,
			supplier.ContactValue,
			supplier.Status,
			nullableFloatArg(supplier.CostRMBPerUSD),
			supplier.Notes,
			supplier.ProbeEnabled,
			supplier.ProbeModel,
			supplier.ProbeIntervalMinutes,
			supplier.LastProbeStatus,
		).Scan(&supplier.ID, &supplier.CreatedAt, &supplier.UpdatedAt)
		if err != nil {
			if isUniqueViolation(err) {
				return service.ErrSupplierExists
			}
			return fmt.Errorf("insert supplier: %w", err)
		}

		if err := replaceSupplierTargetGroupsTx(ctx, tx, supplier.ID, supplier.TargetGroupIDs); err != nil {
			return err
		}
		supplier.TargetGroupCount = len(supplier.TargetGroupIDs)
		return nil
	})
}

func (r *supplierRepository) GetByID(ctx context.Context, id int64) (*service.Supplier, error) {
	supplier, err := scanSupplier(r.db.QueryRowContext(ctx, supplierSelectSQL()+` WHERE s.id = $1 AND s.deleted_at IS NULL`, id))
	if err == sql.ErrNoRows {
		return nil, service.ErrSupplierNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get supplier: %w", err)
	}

	if supplier.TargetGroupIDs, err = r.loadTargetGroupIDs(ctx, id); err != nil {
		return nil, err
	}
	return supplier, nil
}

func (r *supplierRepository) Update(ctx context.Context, supplier *service.Supplier, replaceTargetGroups bool, _ bool) error {
	return r.runInTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE suppliers SET
				name = $1,
				website_url = $2,
				base_url = $3,
				api_key = $4,
				upstream_group = $5,
				contact_platform = $6,
				contact_value = $7,
				status = $8,
				cost_rmb_per_usd = $9,
				notes = $10,
				probe_enabled = $11,
				probe_model = $12,
				probe_interval_minutes = $13,
				updated_at = NOW()
			WHERE id = $14 AND deleted_at IS NULL`,
			supplier.Name,
			supplier.WebsiteURL,
			supplier.BaseURL,
			supplier.APIKey,
			supplier.UpstreamGroup,
			supplier.ContactPlatform,
			supplier.ContactValue,
			supplier.Status,
			nullableFloatArg(supplier.CostRMBPerUSD),
			supplier.Notes,
			supplier.ProbeEnabled,
			supplier.ProbeModel,
			supplier.ProbeIntervalMinutes,
			supplier.ID,
		)
		if err != nil {
			if isUniqueViolation(err) {
				return service.ErrSupplierExists
			}
			return fmt.Errorf("update supplier: %w", err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return service.ErrSupplierNotFound
		}

		if replaceTargetGroups {
			if err := replaceSupplierTargetGroupsTx(ctx, tx, supplier.ID, supplier.TargetGroupIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *supplierRepository) RecordProbeResult(ctx context.Context, supplierID int64, result *service.SupplierProbeResult) error {
	if result == nil {
		return fmt.Errorf("supplier probe result is required")
	}
	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now()
	}
	successIncrement := 0
	if result.Status == service.SupplierProbeStatusSuccess {
		successIncrement = 1
	}

	return r.runInTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO supplier_probe_results (
				supplier_id, status, sub_status, http_code, model, latency_ms,
				accuracy_ok, response_text, error_message, checked_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, created_at`,
			supplierID,
			result.Status,
			result.SubStatus,
			result.HTTPCode,
			result.Model,
			result.LatencyMs,
			result.AccuracyOK,
			result.ResponseText,
			result.ErrorMessage,
			result.CheckedAt,
		).Scan(&result.ID, &result.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert supplier probe result: %w", err)
		}

		updateResult, err := tx.ExecContext(ctx,
			`UPDATE suppliers SET
				last_probe_status = $1,
				last_probe_sub_status = $2,
				last_probe_http_code = $3,
				last_probe_latency_ms = $4,
				last_probe_error = $5,
				last_probe_at = $6,
				next_probe_at = $6::timestamptz + (probe_interval_minutes * INTERVAL '1 minute'),
				probe_success_count = probe_success_count + $7,
				probe_total_count = probe_total_count + 1,
				probe_success_rate = ROUND(((probe_success_count + $7)::numeric / (probe_total_count + 1)) * 100, 2),
				updated_at = NOW()
			WHERE id = $8 AND deleted_at IS NULL`,
			result.Status,
			result.SubStatus,
			result.HTTPCode,
			result.LatencyMs,
			result.ErrorMessage,
			result.CheckedAt,
			successIncrement,
			supplierID,
		)
		if err != nil {
			return fmt.Errorf("update supplier probe stats: %w", err)
		}
		rows, _ := updateResult.RowsAffected()
		if rows == 0 {
			return service.ErrSupplierNotFound
		}
		return nil
	})
}

func (r *supplierRepository) Delete(ctx context.Context, id int64) error {
	return r.runInTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE suppliers SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
			id,
		)
		if err != nil {
			return fmt.Errorf("delete supplier: %w", err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return service.ErrSupplierNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM supplier_groups WHERE supplier_id = $1`, id); err != nil {
			return fmt.Errorf("delete supplier groups: %w", err)
		}
		return nil
	})
}

func (r *supplierRepository) List(ctx context.Context, params pagination.PaginationParams, filter service.SupplierListFilter) ([]service.Supplier, *pagination.PaginationResult, error) {
	where, args := buildSupplierListWhere(filter)
	whereClause := strings.Join(where, " AND ")

	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM suppliers s WHERE %s`, whereClause)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count suppliers: %w", err)
	}

	pageSize := params.Limit()
	page := params.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	argIdx := len(args) + 1
	dataQuery := fmt.Sprintf(
		`%s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		supplierSelectSQL(),
		whereClause,
		supplierListOrderBy(params),
		argIdx,
		argIdx+1,
	)
	queryArgs := append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("query suppliers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	suppliers := make([]service.Supplier, 0, pageSize)
	supplierIDs := make([]int64, 0, pageSize)
	for rows.Next() {
		supplier, err := scanSupplier(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan supplier: %w", err)
		}
		suppliers = append(suppliers, *supplier)
		supplierIDs = append(supplierIDs, supplier.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate suppliers: %w", err)
	}

	if len(supplierIDs) > 0 {
		groupMap, err := r.batchLoadTargetGroupIDs(ctx, supplierIDs)
		if err != nil {
			return nil, nil, err
		}
		for i := range suppliers {
			suppliers[i].TargetGroupIDs = groupMap[suppliers[i].ID]
		}
	}

	pages := 0
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return suppliers, &pagination.PaginationResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
	}, nil
}

func scanSupplier(scanner rowScanner) (*service.Supplier, error) {
	var supplier service.Supplier
	var costRMBPerUSD sql.NullFloat64
	var lastProbeHTTPCode sql.NullInt64
	var lastProbeLatency sql.NullInt64
	var lastProbeAt sql.NullTime
	var nextProbeAt sql.NullTime
	var probeSuccessRate sql.NullFloat64
	err := scanner.Scan(
		&supplier.ID,
		&supplier.Name,
		&supplier.WebsiteURL,
		&supplier.BaseURL,
		&supplier.APIKey,
		&supplier.UpstreamGroup,
		&supplier.ContactPlatform,
		&supplier.ContactValue,
		&supplier.Status,
		&costRMBPerUSD,
		&supplier.Notes,
		&supplier.ProbeEnabled,
		&supplier.ProbeModel,
		&supplier.ProbeIntervalMinutes,
		&supplier.LastProbeStatus,
		&supplier.LastProbeSubStatus,
		&lastProbeHTTPCode,
		&lastProbeLatency,
		&supplier.LastProbeError,
		&lastProbeAt,
		&nextProbeAt,
		&probeSuccessRate,
		&supplier.ProbeSuccessCount,
		&supplier.ProbeTotalCount,
		&supplier.CreatedAt,
		&supplier.UpdatedAt,
		&supplier.TargetGroupCount,
	)
	if err != nil {
		return nil, err
	}
	if costRMBPerUSD.Valid {
		supplier.CostRMBPerUSD = &costRMBPerUSD.Float64
	}
	if lastProbeHTTPCode.Valid {
		httpCode := int(lastProbeHTTPCode.Int64)
		supplier.LastProbeHTTPCode = &httpCode
	}
	if lastProbeLatency.Valid {
		supplier.LastProbeLatencyMs = &lastProbeLatency.Int64
	}
	if lastProbeAt.Valid {
		supplier.LastProbeAt = &lastProbeAt.Time
	}
	if nextProbeAt.Valid {
		supplier.NextProbeAt = &nextProbeAt.Time
	}
	if probeSuccessRate.Valid {
		supplier.ProbeSuccessRate = probeSuccessRate.Float64
	}
	if supplier.TargetGroupIDs == nil {
		supplier.TargetGroupIDs = []int64{}
	}
	return &supplier, nil
}

func supplierSelectSQL() string {
	return `SELECT
		s.id,
		s.name,
		s.website_url,
		s.base_url,
		s.api_key,
		s.upstream_group,
		s.contact_platform,
		s.contact_value,
		s.status,
		s.cost_rmb_per_usd,
		s.notes,
		s.probe_enabled,
		s.probe_model,
		s.probe_interval_minutes,
		s.last_probe_status,
		s.last_probe_sub_status,
		s.last_probe_http_code,
		s.last_probe_latency_ms,
		s.last_probe_error,
		s.last_probe_at,
		s.next_probe_at,
		s.probe_success_rate,
		s.probe_success_count,
		s.probe_total_count,
		s.created_at,
		s.updated_at,
		(SELECT COUNT(*) FROM supplier_groups sg WHERE sg.supplier_id = s.id) AS target_group_count
	FROM suppliers s`
}

func buildSupplierListWhere(filter service.SupplierListFilter) ([]string, []any) {
	where := []string{"s.deleted_at IS NULL"}
	args := []any{}
	argIdx := 1

	if filter.Status != "" {
		where = append(where, fmt.Sprintf("s.status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.ProbeStatus != "" {
		where = append(where, fmt.Sprintf("s.last_probe_status = $%d", argIdx))
		args = append(args, filter.ProbeStatus)
		argIdx++
	}
	if filter.Search != "" {
		where = append(where, fmt.Sprintf(`(
			s.name ILIKE $%d ESCAPE '\' OR
			s.website_url ILIKE $%d ESCAPE '\' OR
			s.base_url ILIKE $%d ESCAPE '\' OR
			s.upstream_group ILIKE $%d ESCAPE '\' OR
			s.contact_value ILIKE $%d ESCAPE '\' OR
			s.notes ILIKE $%d ESCAPE '\'
		)`, argIdx, argIdx, argIdx, argIdx, argIdx, argIdx))
		args = append(args, "%"+escapeLike(filter.Search)+"%")
	}

	return where, args
}

func supplierListOrderBy(params pagination.PaginationParams) string {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := strings.ToUpper(params.NormalizedSortOrder(pagination.SortOrderDesc))

	var column string
	switch sortBy {
	case "id":
		column = "s.id"
	case "name":
		column = "s.name"
	case "status":
		column = "s.status"
	case "probe_status":
		column = "s.last_probe_status"
	case "probe_success_rate":
		column = "s.probe_success_rate"
	case "last_probe_at":
		column = "s.last_probe_at"
	case "created_at":
		column = "s.created_at"
	case "updated_at", "":
		column = "s.updated_at"
	default:
		column = "s.updated_at"
		sortOrder = "DESC"
	}

	return fmt.Sprintf("%s %s, s.id %s", column, sortOrder, sortOrder)
}

func (r *supplierRepository) loadTargetGroupIDs(ctx context.Context, supplierID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT group_id FROM supplier_groups WHERE supplier_id = $1 ORDER BY group_id`,
		supplierID,
	)
	if err != nil {
		return nil, fmt.Errorf("load supplier groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan supplier target group id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate supplier target group ids: %w", err)
	}
	if ids == nil {
		return []int64{}, nil
	}
	return ids, nil
}

func (r *supplierRepository) batchLoadTargetGroupIDs(ctx context.Context, supplierIDs []int64) (map[int64][]int64, error) {
	groupMap := make(map[int64][]int64, len(supplierIDs))
	for _, id := range supplierIDs {
		groupMap[id] = []int64{}
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT supplier_id, group_id FROM supplier_groups
		 WHERE supplier_id = ANY($1) ORDER BY supplier_id, group_id`,
		pq.Array(supplierIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("batch load supplier target groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var supplierID, groupID int64
		if err := rows.Scan(&supplierID, &groupID); err != nil {
			return nil, fmt.Errorf("scan supplier target group map: %w", err)
		}
		groupMap[supplierID] = append(groupMap[supplierID], groupID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate supplier target group map: %w", err)
	}
	return groupMap, nil
}

func replaceSupplierTargetGroupsTx(ctx context.Context, exec dbExec, supplierID int64, groupIDs []int64) error {
	if _, err := exec.ExecContext(ctx, `DELETE FROM supplier_groups WHERE supplier_id = $1`, supplierID); err != nil {
		return fmt.Errorf("delete supplier groups: %w", err)
	}
	if len(groupIDs) == 0 {
		return nil
	}
	if _, err := exec.ExecContext(ctx,
		`INSERT INTO supplier_groups (supplier_id, group_id)
		 SELECT $1, unnest($2::bigint[])`,
		supplierID,
		pq.Array(groupIDs),
	); err != nil {
		return fmt.Errorf("insert supplier groups: %w", err)
	}
	return nil
}

func nullableFloatArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
