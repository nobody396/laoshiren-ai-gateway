package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/commissionrecord"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type commissionRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

// NewCommissionRepository 创建分佣记录仓储实例
func NewCommissionRepository(client *dbent.Client, sqlDB *sql.DB) service.CommissionRepository {
	return &commissionRepository{client: client, sql: sqlDB}
}

// Create 创建分佣记录。
// 对于 consumption 类型的记录，数据库上存在 (type, source_id) 部分唯一索引作为幂等兜底；
// 若因重复触发命中该唯一约束，这里直接视为幂等成功并返回 nil。
func (r *commissionRepository) Create(ctx context.Context, record *service.CommissionRecord) error {
	if r.sql != nil && (record.Rate > 0 || record.RateSource != "") {
		if err := r.createWithRateSnapshot(ctx, record); err != nil {
			if isUniqueConstraintViolation(err) {
				return nil
			}
			if !isMissingCommissionRateColumn(err) {
				return fmt.Errorf("create commission record: %w", err)
			}
			// Older test schemas may not have the rate snapshot columns. Fall back
			// to the generated ent insert so existing tests keep exercising the
			// commission path without needing a full migration run.
		} else {
			return nil
		}
	}

	client := clientFromContext(ctx, r.client)
	builder := client.CommissionRecord.Create().
		SetBeneficiaryID(record.BeneficiaryID).
		SetUserID(record.UserID).
		SetAmount(record.Amount).
		SetSourceAmount(record.SourceAmount).
		SetType(record.Type)

	if record.SourceID != nil {
		builder = builder.SetSourceID(*record.SourceID)
	}
	if record.Note != nil {
		builder = builder.SetNote(*record.Note)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil
		}
		return fmt.Errorf("create commission record: %w", err)
	}
	record.ID = created.ID
	record.CreatedAt = created.CreatedAt
	return nil
}

func (r *commissionRepository) createWithRateSnapshot(ctx context.Context, record *service.CommissionRecord) error {
	var sourceID any
	if record.SourceID != nil {
		sourceID = *record.SourceID
	}
	var note any
	if record.Note != nil {
		note = *record.Note
	}
	var rate any
	if record.Rate > 0 {
		rate = record.Rate
	}
	var rateSource any
	if record.RateSource != "" {
		rateSource = record.RateSource
	}

	err := scanSingleRow(ctx, r.sql, `
		INSERT INTO commission_records (
			beneficiary_id,
			user_id,
			amount,
			source_amount,
			type,
			rate,
			rate_source,
			source_id,
			note
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`, []any{
		record.BeneficiaryID,
		record.UserID,
		record.Amount,
		record.SourceAmount,
		record.Type,
		rate,
		rateSource,
		sourceID,
		note,
	}, &record.ID, &record.CreatedAt)
	return err
}

func isMissingCommissionRateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "commission_records") &&
		(strings.Contains(msg, "rate") || strings.Contains(msg, "rate_source")) &&
		(strings.Contains(msg, "does not exist") || strings.Contains(msg, "no such column"))
}

// ListByBeneficiary 查询某收益人的分佣记录，支持类型过滤和日期范围
func (r *commissionRepository) ListByBeneficiary(
	ctx context.Context,
	beneficiaryID int64,
	params pagination.PaginationParams,
	typeFilter string,
	start, end *time.Time,
) ([]service.CommissionRecord, *pagination.PaginationResult, error) {
	q := r.client.CommissionRecord.Query().
		Where(commissionrecord.BeneficiaryIDEQ(beneficiaryID))

	if typeFilter != "" {
		q = q.Where(commissionrecord.TypeIn(expandCommissionTypes(typeFilter)...))
	}
	if start != nil {
		q = q.Where(commissionrecord.CreatedAtGTE(*start))
	}
	if end != nil {
		q = q.Where(commissionrecord.CreatedAtLTE(*end))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("count commission records: %w", err)
	}

	rows, err := q.
		Order(dbent.Desc(commissionrecord.FieldCreatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list commission records: %w", err)
	}

	result := make([]service.CommissionRecord, 0, len(rows))
	for _, m := range rows {
		result = append(result, commissionRecordEntityToService(m))
	}
	return result, paginationResultFromTotal(int64(total), params), nil
}

// SumByBeneficiaryAndPeriod 统计收益人在指定日期范围内的分佣总额
func (r *commissionRepository) SumByBeneficiaryAndPeriod(
	ctx context.Context,
	beneficiaryID int64,
	start, end *time.Time,
) (float64, error) {
	if r.sql == nil {
		return 0, fmt.Errorf("sql executor is not configured")
	}

	clauses := []string{"beneficiary_id = $1"}
	args := []any{beneficiaryID}
	argIdx := 2

	if start != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *start)
		argIdx++
	}
	if end != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *end)
	}

	query := fmt.Sprintf(
		`SELECT COALESCE(SUM(amount), 0) FROM commission_records WHERE %s`,
		strings.Join(clauses, " AND "),
	)

	var total float64
	err := scanSingleRow(ctx, r.sql, query, args, &total)
	if err != nil {
		return 0, fmt.Errorf("sum commission: %w", err)
	}
	return total, nil
}

// SumByBeneficiaryTypeAndPeriod 统计收益人在指定类型和日期范围内的分佣总额
func (r *commissionRepository) SumByBeneficiaryTypeAndPeriod(
	ctx context.Context,
	beneficiaryID int64,
	commType string,
	start, end *time.Time,
) (float64, error) {
	if r.sql == nil {
		return 0, fmt.Errorf("sql executor is not configured")
	}

	typeValues := expandCommissionTypes(commType)
	clauses := []string{"beneficiary_id = $1"}
	args := []any{beneficiaryID}
	argIdx := 2

	typePlaceholders := make([]string, 0, len(typeValues))
	for _, value := range typeValues {
		typePlaceholders = append(typePlaceholders, fmt.Sprintf("$%d", argIdx))
		args = append(args, value)
		argIdx++
	}
	clauses = append(clauses, fmt.Sprintf("type IN (%s)", strings.Join(typePlaceholders, ", ")))

	if start != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *start)
		argIdx++
	}
	if end != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *end)
	}

	query := fmt.Sprintf(
		`SELECT COALESCE(SUM(amount), 0) FROM commission_records WHERE %s`,
		strings.Join(clauses, " AND "),
	)

	var total float64
	err := scanSingleRow(ctx, r.sql, query, args, &total)
	if err != nil {
		return 0, fmt.Errorf("sum commission by type: %w", err)
	}
	return total, nil
}

// ListInvitedUsersWithStats 查询代理商旗下用户列表及其消费/分佣统计
func (r *commissionRepository) ListInvitedUsersWithStats(
	ctx context.Context,
	agentID int64,
	params pagination.PaginationParams,
	start, end *time.Time,
) ([]service.InvitedUserStat, *pagination.PaginationResult, error) {
	if r.sql == nil {
		return nil, nil, fmt.Errorf("sql executor is not configured")
	}

	// 时间条件（用于 usage_logs 和 commission_records 的子查询）
	timeCondConsume := ""
	timeCondComm := ""
	args := []any{agentID}
	argIdx := 2

	if start != nil {
		timeCondConsume += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		timeCondComm += fmt.Sprintf(" AND cr.created_at >= $%d", argIdx)
		args = append(args, *start)
		argIdx++
	}
	if end != nil {
		timeCondConsume += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		timeCondComm += fmt.Sprintf(" AND cr.created_at <= $%d", argIdx)
		args = append(args, *end)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) FROM users WHERE agent_id = $1 AND deleted_at IS NULL`
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, []any{agentID}, &total); err != nil {
		return nil, nil, fmt.Errorf("count invited users: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			u.id,
			u.email,
			u.username,
			u.created_at,
			COALESCE(consume.total, 0) AS consumed_amount,
			COALESCE(comm.total, 0)    AS commission_amount
		FROM users u
		LEFT JOIN (
			SELECT user_id, SUM(actual_cost) AS total
			FROM usage_logs ul
			WHERE user_id IN (SELECT id FROM users WHERE agent_id = $1 AND deleted_at IS NULL)
			%s
			GROUP BY user_id
		) consume ON consume.user_id = u.id
		LEFT JOIN (
			SELECT user_id, SUM(amount) AS total
			FROM commission_records cr
			WHERE cr.beneficiary_id = $1
			  AND cr.type IN ('consumption', 'consumption_commission')
			%s
			GROUP BY cr.user_id
		) comm ON comm.user_id = u.id
		WHERE u.agent_id = $1 AND u.deleted_at IS NULL
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d
	`, timeCondConsume, timeCondComm, argIdx, argIdx+1)

	args = append(args, params.Limit(), params.Offset())

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list invited users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.InvitedUserStat, 0)
	for rows.Next() {
		var stat service.InvitedUserStat
		if err := rows.Scan(
			&stat.UserID,
			&stat.Email,
			&stat.Username,
			&stat.RegisteredAt,
			&stat.ConsumedAmount,
			&stat.CommissionAmount,
		); err != nil {
			return nil, nil, fmt.Errorf("scan invited user stat: %w", err)
		}
		result = append(result, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return result, paginationResultFromTotal(total, params), nil
}

func expandCommissionTypes(typeFilter string) []string {
	switch typeFilter {
	case service.CommissionTypeConsumption, "consumption_commission":
		return []string{service.CommissionTypeConsumption, "consumption_commission"}
	case service.CommissionTypeFirstRechargeInvitee, "first_recharge_invitee_bonus":
		return []string{service.CommissionTypeFirstRechargeInvitee, "first_recharge_invitee_bonus"}
	case service.CommissionTypeFirstRechargeFriendInvitee, "first_recharge_friend_invitee_bonus":
		return []string{service.CommissionTypeFirstRechargeFriendInvitee, "first_recharge_friend_invitee_bonus"}
	case service.CommissionTypeFirstRechargeReferral, "first_recharge_referral_bonus":
		return []string{service.CommissionTypeFirstRechargeReferral, "first_recharge_referral_bonus"}
	default:
		return []string{typeFilter}
	}
}

func commissionRecordEntityToService(m *dbent.CommissionRecord) service.CommissionRecord {
	r := service.CommissionRecord{
		ID:            m.ID,
		BeneficiaryID: m.BeneficiaryID,
		UserID:        m.UserID,
		Amount:        m.Amount,
		SourceAmount:  m.SourceAmount,
		Type:          m.Type,
		SourceID:      m.SourceID,
		CreatedAt:     m.CreatedAt,
	}
	if m.Note != nil {
		note := *m.Note
		r.Note = &note
	}
	return r
}
