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
	"github.com/lib/pq"
)

type commissionRepository struct {
	client *dbent.Client
	sql    sqlExecutor
	db     *sql.DB
}

// NewCommissionRepository 创建分佣记录仓储实例
func NewCommissionRepository(client *dbent.Client, sqlDB *sql.DB) service.CommissionRepository {
	return &commissionRepository{client: client, sql: sqlDB, db: sqlDB}
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
	if r.sql != nil && r.affiliateV2ReportingTablesAvailable(ctx) {
		return r.listByBeneficiaryAffiliateAware(ctx, beneficiaryID, params, typeFilter, start, end)
	}
	return r.listByBeneficiaryLegacy(ctx, beneficiaryID, params, typeFilter, start, end)
}

func (r *commissionRepository) listByBeneficiaryLegacy(
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

func (r *commissionRepository) listByBeneficiaryAffiliateAware(
	ctx context.Context,
	beneficiaryID int64,
	params pagination.PaginationParams,
	typeFilter string,
	start, end *time.Time,
) ([]service.CommissionRecord, *pagination.PaginationResult, error) {
	legacyTypes := expandCommissionTypes(typeFilter)
	affiliateTypeAllowed := typeFilter == "" ||
		typeFilter == service.CommissionTypeConsumption ||
		typeFilter == "consumption_commission" ||
		typeFilter == "self_consumption_commission" ||
		typeFilter == "consumption_reversal"
	args := []any{
		beneficiaryID,
		pq.Array(legacyTypes),
		typeFilter,
		affiliateTypeAllowed,
		nullableTime(start),
		nullableTime(end),
	}

	const recordsCTE = `
		WITH records AS (
			SELECT
				cr.id::bigint AS id,
				cr.beneficiary_id::bigint AS beneficiary_id,
				cr.user_id::bigint AS user_id,
				COALESCE(u.email, '') AS user_email,
				COALESCE(u.username, '') AS username,
				cr.amount::double precision AS amount,
				cr.source_amount::double precision AS source_amount,
				cr.type::text AS type,
				COALESCE(cr.rate, 0)::double precision AS rate,
				COALESCE(cr.rate_source, '')::text AS rate_source,
				cr.source_id::bigint AS source_id,
				cr.note::text AS note,
				cr.created_at AS created_at
			FROM commission_records cr
			LEFT JOIN users u ON u.id = cr.user_id
			WHERE cr.beneficiary_id = $1
			  AND ($3 = '' OR cr.type = ANY($2::text[]))
			  AND ($5::timestamptz IS NULL OR cr.created_at >= $5::timestamptz)
			  AND ($6::timestamptz IS NULL OR cr.created_at <= $6::timestamptz)

			UNION ALL

			SELECT
				(1000000000000 + MIN(cash.id))::bigint AS id,
				cash.agent_id::bigint AS beneficiary_id,
				COALESCE(cash.consumer_user_id, 0)::bigint AS user_id,
				COALESCE(u.email, '') AS user_email,
				COALESCE(u.username, '') AS username,
				(SUM(cash.amount_micros)::numeric / 1000000)::double precision AS amount,
				(
					SUM(
						CASE
							WHEN cash.entry_type = 'reversal'
								THEN -COALESCE(cash.source_amount_micros, 0)
							ELSE COALESCE(cash.source_amount_micros, 0)
						END
					)::numeric / 1000000
				)::double precision AS source_amount,
				CASE
					WHEN cash.consumer_user_id = cash.agent_id
						THEN 'self_consumption_commission'
					WHEN BOOL_AND(cash.entry_type = 'reversal') THEN 'consumption_reversal'
					ELSE 'consumption_commission'
				END AS type,
				CASE
					WHEN SUM(
						CASE
							WHEN cash.entry_type = 'reversal'
								THEN -COALESCE(cash.source_amount_micros, 0)
							ELSE COALESCE(cash.source_amount_micros, 0)
						END
					) = 0 THEN 0
					ELSE (
						SUM(cash.amount_micros)::numeric
						/ SUM(
							CASE
								WHEN cash.entry_type = 'reversal'
									THEN -COALESCE(cash.source_amount_micros, 0)
								ELSE COALESCE(cash.source_amount_micros, 0)
							END
						)::numeric
					)::double precision
				END AS rate,
				'affiliate_v3_daily'::text AS rate_source,
				NULL::bigint AS source_id,
				CASE
					WHEN BOOL_OR(cash.posting_status = 'risk_hold') THEN '含待确认分润'
					ELSE NULL
				END AS note,
				(
					(cash.occurred_at AT TIME ZONE 'Asia/Shanghai')::date::timestamp
					AT TIME ZONE 'Asia/Shanghai'
				) AS created_at
			FROM agent_cash_commission_entries cash
			LEFT JOIN users u ON u.id = cash.consumer_user_id
			WHERE cash.agent_id = $1
			  AND $4::boolean
			  AND cash.consumer_user_id IS NOT NULL
			  AND cash.entry_type IN ('earned', 'risk_release', 'reversal')
			  AND cash.posting_status <> 'reversed'
			  AND ($5::timestamptz IS NULL OR cash.occurred_at >= $5::timestamptz)
			  AND ($6::timestamptz IS NULL OR cash.occurred_at <= $6::timestamptz)
			GROUP BY
				cash.agent_id,
				cash.consumer_user_id,
				u.email,
				u.username,
				(cash.occurred_at AT TIME ZONE 'Asia/Shanghai')::date
			HAVING
				$3 = ''
				OR $3 = 'consumption'
				OR (
					$3 = 'self_consumption_commission'
					AND cash.consumer_user_id = cash.agent_id
				)
				OR (
					$3 = 'consumption_commission'
					AND cash.consumer_user_id <> cash.agent_id
					AND NOT BOOL_AND(cash.entry_type = 'reversal')
				)
				OR (
					$3 = 'consumption_reversal'
					AND cash.consumer_user_id <> cash.agent_id
					AND BOOL_AND(cash.entry_type = 'reversal')
				)
		)
	`

	var total int64
	if err := scanSingleRow(ctx, r.sql, recordsCTE+`SELECT COUNT(*) FROM records`, args, &total); err != nil {
		return nil, nil, fmt.Errorf("count affiliate-aware commission records: %w", err)
	}

	rows, err := r.sql.QueryContext(
		ctx,
		recordsCTE+`
			SELECT
				id, beneficiary_id, user_id, user_email, username,
				amount, source_amount, type, rate, rate_source,
				source_id, note, created_at
			FROM records
			ORDER BY created_at DESC, id DESC
			LIMIT $7 OFFSET $8
		`,
		append(args, params.Limit(), params.Offset())...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list affiliate-aware commission records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.CommissionRecord, 0)
	for rows.Next() {
		var item service.CommissionRecord
		var sourceID sql.NullInt64
		var note sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.BeneficiaryID,
			&item.UserID,
			&item.UserEmail,
			&item.Username,
			&item.Amount,
			&item.SourceAmount,
			&item.Type,
			&item.Rate,
			&item.RateSource,
			&sourceID,
			&note,
			&item.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("scan affiliate-aware commission record: %w", err)
		}
		if sourceID.Valid {
			item.SourceID = &sourceID.Int64
		}
		if note.Valid && strings.TrimSpace(note.String) != "" {
			value := note.String
			item.Note = &value
		}
		item.UserEmail = service.MaskEmail(item.UserEmail)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return result, paginationResultFromTotal(total, params), nil
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
	if r.affiliateV2ReportingTablesAvailable(ctx) {
		return r.sumAffiliateAwareCommission(ctx, beneficiaryID, "all", true, nil, start, end)
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
	if r.affiliateV2ReportingTablesAvailable(ctx) {
		legacyTypes := expandCommissionTypes(commType)
		affiliateTypeFilter := ""
		switch commType {
		case service.CommissionTypeConsumption:
			affiliateTypeFilter = "all"
		case "consumption_commission":
			affiliateTypeFilter = "regular"
		case "self_consumption_commission":
			affiliateTypeFilter = "self"
		case "consumption_reversal":
			affiliateTypeFilter = "reversal"
		}
		includeOrdinaryRewards := commType == service.CommissionTypeFirstRechargeReferral
		return r.sumAffiliateAwareCommission(ctx, beneficiaryID, affiliateTypeFilter, includeOrdinaryRewards, legacyTypes, start, end)
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

func (r *commissionRepository) sumAffiliateAwareCommission(
	ctx context.Context,
	beneficiaryID int64,
	affiliateTypeFilter string,
	includeOrdinaryRewards bool,
	legacyTypes []string,
	start, end *time.Time,
) (float64, error) {
	typeFilterEnabled := len(legacyTypes) > 0
	if !typeFilterEnabled {
		legacyTypes = []string{""}
	}
	args := []any{
		beneficiaryID,
		affiliateTypeFilter,
		includeOrdinaryRewards,
		typeFilterEnabled,
		pq.Array(legacyTypes),
		nullableTime(start),
		nullableTime(end),
	}
	var total float64
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			COALESCE((
				SELECT SUM(cr.amount)
				FROM commission_records cr
				WHERE cr.beneficiary_id = $1
				  AND (NOT $4::boolean OR cr.type = ANY($5::text[]))
				  AND ($6::timestamptz IS NULL OR cr.created_at >= $6::timestamptz)
				  AND ($7::timestamptz IS NULL OR cr.created_at <= $7::timestamptz)
			), 0)::double precision
			+
			COALESCE((
				SELECT SUM(cash.amount_micros::numeric / 1000000)
				FROM agent_cash_commission_entries cash
				WHERE cash.agent_id = $1
				  AND $2::text <> ''
				  AND (
						$2::text = 'all'
						OR ($2::text = 'self' AND cash.consumer_user_id = cash.agent_id)
						OR (
							$2::text = 'regular'
							AND cash.consumer_user_id IS DISTINCT FROM cash.agent_id
							AND cash.entry_type <> 'reversal'
						)
						OR (
							$2::text = 'reversal'
							AND cash.consumer_user_id IS DISTINCT FROM cash.agent_id
							AND cash.entry_type = 'reversal'
						)
				  )
				  AND cash.entry_type IN ('earned', 'risk_release', 'reversal')
				  AND cash.posting_status <> 'reversed'
				  AND ($6::timestamptz IS NULL OR cash.occurred_at >= $6::timestamptz)
				  AND ($7::timestamptz IS NULL OR cash.occurred_at <= $7::timestamptz)
			), 0)::double precision
			+
			COALESCE((
				SELECT SUM(reward.amount_micros::numeric / 1000000)
				FROM affiliate_reward_entries reward
				WHERE reward.beneficiary_user_id = $1
				  AND $3::boolean
				  AND reward.reward_type = 'ordinary_referral'
				  AND reward.status = 'posted'
				  AND ($6::timestamptz IS NULL OR reward.posted_at >= $6::timestamptz)
				  AND ($7::timestamptz IS NULL OR reward.posted_at <= $7::timestamptz)
			), 0)::double precision
	`, args, &total)
	if err != nil {
		return 0, fmt.Errorf("sum affiliate-aware commission: %w", err)
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
	if r.affiliateV2ReportingTablesAvailable(ctx) {
		return r.listInvitedUsersWithAffiliateStats(ctx, agentID, params, start, end)
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
			COALESCE(u.total_recharged, 0) AS recharged_amount,
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
			&stat.RechargedAmount,
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

func (r *commissionRepository) listInvitedUsersWithAffiliateStats(
	ctx context.Context,
	agentID int64,
	params pagination.PaginationParams,
	start, end *time.Time,
) ([]service.InvitedUserStat, *pagination.PaginationResult, error) {
	args := []any{agentID, nullableTime(start), nullableTime(end)}
	const directUsersCTE = `
		WITH direct_users AS (
			SELECT DISTINCT ON (u.id)
				u.id,
				COALESCE(u.email, '') AS email,
				COALESCE(u.username, '') AS username,
				u.created_at
			FROM users u
			LEFT JOIN affiliate_bindings ab
				ON ab.customer_user_id = u.id
				AND (ab.agent_id = $1 OR ab.inviter_user_id = $1)
			WHERE u.deleted_at IS NULL
			  AND (u.agent_id = $1 OR ab.agent_id = $1 OR ab.inviter_user_id = $1)
			ORDER BY u.id, u.created_at DESC
		)
	`

	var total int64
	if err := scanSingleRow(ctx, r.sql, directUsersCTE+`SELECT COUNT(*) FROM direct_users`, []any{agentID}, &total); err != nil {
		return nil, nil, fmt.Errorf("count affiliate invited users: %w", err)
	}

	query := directUsersCTE + `
		, recharge_totals AS (
			-- Period totals come from immutable paid entitlement sources. The
			-- users.total_recharged cache is neither period-scoped nor complete
			-- for redeemed balance cards and monthly plans.
			SELECT user_id, SUM(amount)::double precision AS total
			FROM (
				SELECT
					user_id,
					(original_amount_micros::numeric / 1000000)::double precision AS amount
				FROM balance_lots
				WHERE source_type IN ('paid_redeem', 'paid_topup')
				  AND user_id IN (SELECT id FROM direct_users)
				  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
				  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)

				UNION ALL

				SELECT
					user_id,
					(sale_price_micros::numeric / 1000000)::double precision AS amount
				FROM monthly_entitlement_cycles
				WHERE source_type IN ('paid_redeem', 'paid_topup')
				  AND user_id IN (SELECT id FROM direct_users)
				  AND ($2::timestamptz IS NULL OR starts_at >= $2::timestamptz)
				  AND ($3::timestamptz IS NULL OR starts_at <= $3::timestamptz)
			) paid
			GROUP BY user_id
		),
		newcomer_recharge_totals AS (
			SELECT claim.user_id,
				SUM(offer.pay_amount_cny_fen::numeric / 100)::double precision AS total
			FROM native_checkout_manual_claims claim
			JOIN native_checkout_offers offer ON offer.code = claim.offer_code
			WHERE claim.offer_code = 'newcomer-balance-5-to-10'
			  AND claim.user_id IN (SELECT id FROM direct_users)
			  AND ($2::timestamptz IS NULL OR claim.created_at >= $2::timestamptz)
			  AND ($3::timestamptz IS NULL OR claim.created_at <= $3::timestamptz)
			GROUP BY claim.user_id
		),
		usage_totals AS (
			SELECT user_id, SUM(actual_cost)::double precision AS total
			FROM usage_logs
			WHERE user_id IN (SELECT id FROM direct_users)
			  AND ($2::timestamptz IS NULL OR created_at >= $2::timestamptz)
			  AND ($3::timestamptz IS NULL OR created_at <= $3::timestamptz)
			GROUP BY user_id
		),
		newcomer_consumption_totals AS (
			SELECT claim.user_id,
				SUM(consumed.amount_micros::numeric / 1000000)::double precision AS total
			FROM native_checkout_manual_claims claim
			JOIN balance_lots lot ON lot.user_id = claim.user_id
				AND lot.source_id = claim.redeem_code_id
				AND lot.source_key = 'redeem:balance:' || claim.redeem_code_id::text
				AND lot.source_type = 'gift'
				AND lot.affiliate_policy = 'NONE'
			JOIN balance_lot_consumptions consumed ON consumed.balance_lot_id = lot.id
			WHERE claim.offer_code = 'newcomer-balance-5-to-10'
			  AND claim.user_id IN (SELECT id FROM direct_users)
			  AND ($2::timestamptz IS NULL OR consumed.created_at >= $2::timestamptz)
			  AND ($3::timestamptz IS NULL OR consumed.created_at <= $3::timestamptz)
			GROUP BY claim.user_id
		),
		performance_totals AS (
			SELECT user_id,
				SUM(CASE event_type
					WHEN 'confirmed_consumption' THEN amount_micros
					ELSE -amount_micros
				END)::bigint AS total_micros
			FROM affiliate_performance_events
			WHERE direct_agent_id = $1
			  AND user_id IN (SELECT id FROM direct_users)
			  AND event_type IN ('confirmed_consumption', 'consumption_reversal')
			  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
			  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
			GROUP BY user_id
		),
		cash_totals AS (
			SELECT consumer_user_id AS user_id, SUM(amount_micros)::bigint AS total_micros
			FROM agent_cash_commission_entries
			WHERE agent_id = $1
			  AND consumer_user_id IN (SELECT id FROM direct_users)
			  AND entry_type IN ('earned', 'risk_release', 'reversal')
			  AND posting_status <> 'reversed'
			  AND ($2::timestamptz IS NULL OR occurred_at >= $2::timestamptz)
			  AND ($3::timestamptz IS NULL OR occurred_at <= $3::timestamptz)
			GROUP BY consumer_user_id
		),
		legacy_commission_totals AS (
			SELECT user_id, SUM(amount)::double precision AS total
			FROM commission_records cr
			WHERE cr.beneficiary_id = $1
			  AND cr.user_id IN (SELECT id FROM direct_users)
			  AND cr.type IN ('consumption', 'consumption_commission')
			  AND ($2::timestamptz IS NULL OR cr.created_at >= $2::timestamptz)
			  AND ($3::timestamptz IS NULL OR cr.created_at <= $3::timestamptz)
			GROUP BY user_id
		)
		SELECT
			d.id,
			d.email,
			d.username,
			d.created_at,
			(
				COALESCE(recharge_totals.total, 0)
				+ COALESCE(newcomer_recharge_totals.total, 0)
			)::double precision AS recharged_amount,
			(CASE
				WHEN performance_totals.user_id IS NOT NULL THEN
					(performance_totals.total_micros::numeric / 1000000)::double precision
					+ COALESCE(newcomer_consumption_totals.total, 0)
				ELSE COALESCE(usage_totals.total, newcomer_consumption_totals.total, 0)
			END)::double precision AS consumed_amount,
			COALESCE(
				(cash_totals.total_micros::numeric / 1000000)::double precision,
				legacy_commission_totals.total,
				0
			)::double precision AS commission_amount
		FROM direct_users d
		LEFT JOIN recharge_totals ON recharge_totals.user_id = d.id
		LEFT JOIN newcomer_recharge_totals ON newcomer_recharge_totals.user_id = d.id
		LEFT JOIN usage_totals ON usage_totals.user_id = d.id
		LEFT JOIN newcomer_consumption_totals ON newcomer_consumption_totals.user_id = d.id
		LEFT JOIN performance_totals ON performance_totals.user_id = d.id
		LEFT JOIN cash_totals ON cash_totals.user_id = d.id
		LEFT JOIN legacy_commission_totals ON legacy_commission_totals.user_id = d.id
		ORDER BY d.created_at DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := r.sql.QueryContext(ctx, query, append(args, params.Limit(), params.Offset())...)
	if err != nil {
		return nil, nil, fmt.Errorf("list affiliate invited users: %w", err)
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
			&stat.RechargedAmount,
			&stat.ConsumedAmount,
			&stat.CommissionAmount,
		); err != nil {
			return nil, nil, fmt.Errorf("scan affiliate invited user stat: %w", err)
		}
		result = append(result, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return result, paginationResultFromTotal(total, params), nil
}

func (r *commissionRepository) affiliateV2ReportingTablesAvailable(ctx context.Context) bool {
	if r == nil || r.sql == nil {
		return false
	}
	var exists bool
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			COALESCE(to_regclass('public.affiliate_bindings') IS NOT NULL, FALSE)
			AND COALESCE(to_regclass('public.affiliate_performance_events') IS NOT NULL, FALSE)
			AND COALESCE(to_regclass('public.agent_cash_commission_entries') IS NOT NULL, FALSE)
	`, nil, &exists)
	return err == nil && exists
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
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
	case service.CommissionTypeInviteActivityRegistrationBonus:
		return []string{service.CommissionTypeInviteActivityRegistrationBonus}
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
