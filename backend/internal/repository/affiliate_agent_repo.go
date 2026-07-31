package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateAgentRepository struct {
	db *sql.DB
}

func NewAffiliateAgentRepository(db *sql.DB) service.AffiliateAgentRepository {
	return &affiliateAgentRepository{db: db}
}

func (r *affiliateAgentRepository) GetAgentQualification(
	ctx context.Context,
	userID int64,
) (*service.AffiliateAgentQualification, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	return queryAffiliateAgentQualification(ctx, r.db, userID)
}

func (r *affiliateAgentRepository) ListQualifiedCandidates(
	ctx context.Context,
	limit int,
) ([]service.AffiliateQualifiedCandidate, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	// Keep this set-based for the admin queue: candidate discovery must not run
	// one qualification query per user as the customer base grows.
	rows, err := r.db.QueryContext(ctx, `
		WITH settings AS (
			SELECT
				mode,
				started_at,
				qualification_direct_user_count,
				qualification_min_user_consumption_micros,
				qualification_direct_team_consumption_micros,
				qualification_self_consumption_micros
			FROM affiliate_program_settings
			WHERE id = 1
		),
		baseline_net AS (
			SELECT
				e.user_id,
				SUM(e.confirmed_consumption_micros)::bigint AS amount_micros
			FROM affiliate_qualification_baseline_entries e
			CROSS JOIN settings s
			WHERE s.started_at IS NOT NULL
				AND e.cutoff_at <= s.started_at
			GROUP BY e.user_id
		),
		live_net AS (
			SELECT
				e.user_id,
				SUM(
					CASE e.event_type
						WHEN 'confirmed_consumption' THEN e.amount_micros
						WHEN 'consumption_reversal' THEN -e.amount_micros
						ELSE 0
					END
				)::bigint AS amount_micros
			FROM affiliate_performance_events e
			CROSS JOIN settings s
			WHERE e.event_type IN ('confirmed_consumption', 'consumption_reversal')
				AND s.started_at IS NOT NULL
				AND e.occurred_at >= s.started_at
				AND COALESCE(e.metadata ->> 'program_mode', '') = 'live'
			GROUP BY e.user_id
		),
		user_net AS (
			SELECT
				user_id,
				GREATEST(SUM(amount_micros), 0)::bigint AS amount_micros
			FROM (
				SELECT user_id, amount_micros FROM baseline_net
				UNION ALL
				SELECT user_id, amount_micros FROM live_net
			) amounts
			GROUP BY user_id
		),
		direct_totals AS (
			SELECT
				b.inviter_user_id AS user_id,
				COUNT(*) FILTER (
					WHERE n.amount_micros >= s.qualification_min_user_consumption_micros
				)::integer AS valid_direct_count,
				COALESCE(SUM(n.amount_micros) FILTER (
					WHERE n.amount_micros >= s.qualification_min_user_consumption_micros
				), 0)::bigint AS direct_micros
			FROM affiliate_bindings b
			JOIN user_net n ON n.user_id = b.customer_user_id
			CROSS JOIN settings s
			WHERE b.customer_user_id <> b.inviter_user_id
			GROUP BY b.inviter_user_id
		),
		candidates AS (
			SELECT
				u.id AS user_id,
				COALESCE(u.email, '') AS email,
				COALESCE(u.username, '') AS username,
				COALESCE(n.amount_micros, 0)::bigint AS self_micros,
				COALESCE(d.direct_micros, 0)::bigint AS direct_micros,
				COALESCE(d.valid_direct_count, 0)::integer AS valid_direct_count,
				(
					COALESCE(d.valid_direct_count, 0) >= s.qualification_direct_user_count
					AND COALESCE(d.direct_micros, 0) >= s.qualification_direct_team_consumption_micros
				) AS direct_qualified,
				(
					COALESCE(n.amount_micros, 0) >= s.qualification_self_consumption_micros
				) AS self_qualified
			FROM users u
			CROSS JOIN settings s
			LEFT JOIN user_net n ON n.user_id = u.id
			LEFT JOIN direct_totals d ON d.user_id = u.id
			LEFT JOIN agent_principals ap ON ap.agent_id = u.id
			WHERE s.mode = 'live'
				AND s.started_at IS NOT NULL
				AND u.deleted_at IS NULL
				AND u.status = 'active'
				AND COALESCE(ap.status, 'candidate') NOT IN (
					'pending_review', 'active', 'suspended', 'terminated'
				)
				AND COALESCE(ap.risk_status, 'clear') = 'clear'
				AND NOT EXISTS (
					SELECT 1
					FROM affiliate_agent_applications a
					WHERE a.user_id = u.id
						AND a.status = 'pending_review'
				)
		)
		SELECT
			user_id,
			email,
			username,
			CASE
				WHEN direct_qualified THEN 'direct_team'
				ELSE 'self_consumption'
			END AS qualification_route,
			valid_direct_count,
			self_micros,
			direct_micros,
			(self_micros + direct_micros)::bigint AS combined_micros
		FROM candidates
		WHERE direct_qualified OR self_qualified
		ORDER BY
			CASE WHEN direct_qualified THEN 0 ELSE 1 END,
			(self_micros + direct_micros) DESC,
			user_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateQualifiedCandidate, 0, limit)
	for rows.Next() {
		var item service.AffiliateQualifiedCandidate
		if err := rows.Scan(
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.QualificationRoute,
			&item.ValidDirectUserCount,
			&item.SelfConsumptionMicros,
			&item.DirectTeamConsumptionMicros,
			&item.CombinedConsumptionMicros,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *affiliateAgentRepository) GetOperationsSummary(
	ctx context.Context,
) (*service.AffiliateOperationsSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	var out service.AffiliateOperationsSummary
	err := r.db.QueryRowContext(ctx, `
		WITH settings AS (
			SELECT mode, started_at, qualification_direct_user_count,
				qualification_min_user_consumption_micros,
				qualification_direct_team_consumption_micros,
				qualification_self_consumption_micros
			FROM affiliate_program_settings WHERE id = 1
		), baseline_net AS (
			SELECT e.user_id, SUM(e.confirmed_consumption_micros)::bigint AS amount_micros
			FROM affiliate_qualification_baseline_entries e CROSS JOIN settings s
			WHERE s.started_at IS NOT NULL AND e.cutoff_at <= s.started_at GROUP BY e.user_id
		), live_net AS (
			SELECT e.user_id, SUM(CASE e.event_type
				WHEN 'confirmed_consumption' THEN e.amount_micros
				WHEN 'consumption_reversal' THEN -e.amount_micros ELSE 0 END)::bigint AS amount_micros
			FROM affiliate_performance_events e CROSS JOIN settings s
			WHERE e.event_type IN ('confirmed_consumption', 'consumption_reversal')
				AND s.started_at IS NOT NULL AND e.occurred_at >= s.started_at
				AND COALESCE(e.metadata ->> 'program_mode', '') = 'live'
			GROUP BY e.user_id
		), user_net AS (
			SELECT user_id, GREATEST(SUM(amount_micros), 0)::bigint AS amount_micros
			FROM (SELECT * FROM baseline_net UNION ALL SELECT * FROM live_net) amounts GROUP BY user_id
		), direct_totals AS (
			SELECT b.inviter_user_id AS user_id,
				COUNT(*) FILTER (WHERE n.amount_micros >= s.qualification_min_user_consumption_micros)::integer AS valid_direct_count,
				COALESCE(SUM(n.amount_micros) FILTER (WHERE n.amount_micros >= s.qualification_min_user_consumption_micros), 0)::bigint AS direct_micros
			FROM affiliate_bindings b JOIN user_net n ON n.user_id = b.customer_user_id CROSS JOIN settings s
			WHERE b.customer_user_id <> b.inviter_user_id GROUP BY b.inviter_user_id
		), qualified AS (
			SELECT u.id
			FROM users u CROSS JOIN settings s
			LEFT JOIN user_net n ON n.user_id = u.id
			LEFT JOIN direct_totals d ON d.user_id = u.id
			LEFT JOIN agent_principals ap ON ap.agent_id = u.id
			WHERE s.mode = 'live' AND s.started_at IS NOT NULL AND u.deleted_at IS NULL AND u.status = 'active'
				AND COALESCE(ap.status, 'candidate') NOT IN ('pending_review', 'active', 'suspended', 'terminated')
				AND COALESCE(ap.risk_status, 'clear') = 'clear'
				AND NOT EXISTS (SELECT 1 FROM affiliate_agent_applications a WHERE a.user_id = u.id AND a.status = 'pending_review')
				AND ((COALESCE(d.valid_direct_count, 0) >= s.qualification_direct_user_count
					AND COALESCE(d.direct_micros, 0) >= s.qualification_direct_team_consumption_micros)
					OR COALESCE(n.amount_micros, 0) >= s.qualification_self_consumption_micros)
		), counts AS (
			SELECT
				(SELECT COUNT(*) FROM qualified) AS qualified_followup,
				(SELECT COUNT(*) FROM affiliate_agent_applications WHERE status = 'pending_review') AS pending_applications,
				(SELECT COUNT(*) FROM agent_payment_profiles WHERE verification_status = 'pending_review') AS pending_profiles,
				(SELECT COUNT(*) FROM agent_withdrawal_requests WHERE status = 'processing') AS processing_withdrawals,
				(SELECT COUNT(*) FROM agent_withdrawal_requests WHERE status = 'processing' AND due_at < NOW()) AS overdue_withdrawals,
				(SELECT COUNT(*) FROM agent_principals WHERE status IN ('active', 'suspended') AND risk_status <> 'clear') AS abnormal_partners
		)
		SELECT qualified_followup, pending_applications, pending_profiles,
			processing_withdrawals, overdue_withdrawals, abnormal_partners,
			(pending_applications + pending_profiles + processing_withdrawals + abnormal_partners)
		FROM counts
	`).Scan(
		&out.QualifiedFollowup,
		&out.PendingApplications,
		&out.PendingPaymentProfiles,
		&out.ProcessingWithdrawals,
		&out.OverdueWithdrawals,
		&out.AbnormalPartners,
		&out.ActionableTotal,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *affiliateAgentRepository) ListPartnerPerformance(
	ctx context.Context,
	limit int,
) ([]service.AffiliatePartnerPerformance, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH partners AS (
			SELECT ap.agent_id, COALESCE(u.email, '') AS email, COALESCE(u.username, '') AS username,
				COALESCE(ap.activated_at, ap.updated_at) AS activated_at
			FROM agent_principals ap JOIN users u ON u.id = ap.agent_id
			WHERE ap.status IN ('active', 'suspended') AND u.deleted_at IS NULL
			ORDER BY ap.activated_at DESC NULLS LAST, ap.agent_id DESC LIMIT $1
		), direct_users AS (
			SELECT b.inviter_user_id AS agent_id, b.customer_user_id, b.bound_at
			FROM affiliate_bindings b JOIN partners p ON p.agent_id = b.inviter_user_id
			WHERE b.customer_user_id <> b.inviter_user_id
		), raw_paid AS (
			SELECT x.user_id, x.amount_micros, x.occurred_at
			FROM (
				SELECT user_id, original_amount_micros AS amount_micros, occurred_at
				FROM balance_lots WHERE source_type IN ('paid_redeem', 'paid_topup')
				UNION ALL
				SELECT user_id, sale_price_micros AS amount_micros, starts_at AS occurred_at
				FROM monthly_entitlement_cycles WHERE source_type IN ('paid_redeem', 'paid_topup')
			) x
		), self_paid AS (
			SELECT p.agent_id, COALESCE(SUM(r.amount_micros), 0)::bigint AS amount_micros
			FROM partners p LEFT JOIN raw_paid r ON r.user_id = p.agent_id AND r.occurred_at >= p.activated_at
			GROUP BY p.agent_id
		), team_paid AS (
			SELECT p.agent_id, COUNT(DISTINCT d.customer_user_id) AS direct_user_count,
				COUNT(DISTINCT d.customer_user_id) FILTER (WHERE r.user_id IS NOT NULL) AS paid_user_count,
				COALESCE(SUM(r.amount_micros), 0)::bigint AS amount_micros
			FROM partners p LEFT JOIN direct_users d ON d.agent_id = p.agent_id
			LEFT JOIN raw_paid r ON r.user_id = d.customer_user_id AND r.occurred_at >= p.activated_at
			GROUP BY p.agent_id
		), consumption AS (
			SELECT p.agent_id, e.user_id,
				SUM(CASE e.event_type WHEN 'confirmed_consumption' THEN e.amount_micros ELSE -e.amount_micros END)::bigint AS amount_micros,
				SUM(CASE WHEN e.occurred_at >= NOW() - INTERVAL '30 days'
					THEN CASE e.event_type WHEN 'confirmed_consumption' THEN e.amount_micros ELSE -e.amount_micros END ELSE 0 END)::bigint AS recent_micros
			FROM partners p JOIN affiliate_performance_events e ON e.direct_agent_id = p.agent_id
			WHERE e.event_type IN ('confirmed_consumption', 'consumption_reversal') AND e.occurred_at >= p.activated_at
			GROUP BY p.agent_id, e.user_id
		), consumption_stats AS (
			SELECT agent_id,
				COALESCE(SUM(amount_micros) FILTER (WHERE user_id = agent_id), 0)::bigint AS self_micros,
				COALESCE(SUM(amount_micros) FILTER (WHERE user_id <> agent_id), 0)::bigint AS team_micros,
				COALESCE(SUM(recent_micros), 0)::bigint AS recent_micros
			FROM consumption GROUP BY agent_id
		), cash AS (
			SELECT p.agent_id,
				COALESCE(SUM(e.amount_micros) FILTER (WHERE e.posting_status = 'posted' AND e.entry_type IN ('earned','risk_release','reversal')), 0)::bigint AS earned,
				COALESCE(SUM(e.amount_micros) FILTER (WHERE e.posting_status = 'posted'), 0)::bigint AS available
			FROM partners p LEFT JOIN agent_cash_commission_entries e ON e.agent_id = p.agent_id AND e.occurred_at >= p.activated_at
			GROUP BY p.agent_id
		), withdrawals AS (
			SELECT p.agent_id,
				COALESCE(SUM(w.amount_micros) FILTER (WHERE w.status = 'processing'), 0)::bigint AS processing,
				COALESCE(SUM(w.amount_micros) FILTER (WHERE w.status = 'paid'), 0)::bigint AS paid
			FROM partners p LEFT JOIN agent_withdrawal_requests w ON w.agent_id = p.agent_id AND w.requested_at >= p.activated_at
			GROUP BY p.agent_id
		)
		SELECT p.agent_id, p.email, p.username, p.activated_at,
			COALESCE(tp.direct_user_count, 0), COALESCE(tp.paid_user_count, 0),
			COALESCE(sp.amount_micros, 0), COALESCE(tp.amount_micros, 0),
			COALESCE(cs.self_micros, 0), COALESCE(cs.team_micros, 0),
			COALESCE(cs.recent_micros, 0),
			COALESCE(cash.earned, 0), COALESCE(cash.available, 0),
			COALESCE(withdrawals.processing, 0), COALESCE(withdrawals.paid, 0)
		FROM partners p
		LEFT JOIN self_paid sp ON sp.agent_id = p.agent_id
		LEFT JOIN team_paid tp ON tp.agent_id = p.agent_id
		LEFT JOIN consumption_stats cs ON cs.agent_id = p.agent_id
		LEFT JOIN cash ON cash.agent_id = p.agent_id
		LEFT JOIN withdrawals ON withdrawals.agent_id = p.agent_id
		ORDER BY p.activated_at DESC, p.agent_id DESC
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliatePartnerPerformance, 0, limit)
	for rows.Next() {
		var item service.AffiliatePartnerPerformance
		if err := rows.Scan(
			&item.AgentID, &item.Email, &item.Username, &item.ActivatedAt,
			&item.DirectUserCount, &item.PaidDirectUserCount,
			&item.SelfRechargeMicros, &item.DirectTeamRechargeMicros,
			&item.SelfConsumptionMicros, &item.DirectTeamConsumptionMicros,
			&item.Recent30dConsumptionMicros, &item.LifetimeEarnedMicros,
			&item.AvailableCommissionMicros, &item.ProcessingWithdrawalMicros,
			&item.PaidCommissionMicros,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *affiliateAgentRepository) GetPartnerPerformance(
	ctx context.Context,
	agentID int64,
	start time.Time,
	end time.Time,
) (*service.AffiliatePartnerPerformanceDetail, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	var summary service.AffiliatePartnerPerformance
	var activatedAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT ap.agent_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
			COALESCE(ap.activated_at, ap.updated_at)
		FROM agent_principals ap JOIN users u ON u.id = ap.agent_id
		WHERE ap.agent_id = $1 AND ap.status IN ('active', 'suspended') AND u.deleted_at IS NULL
	`, agentID).Scan(&summary.AgentID, &summary.Email, &summary.Username, &activatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliatePartnerPerformanceNotFound
	}
	if err != nil {
		return nil, err
	}
	summary.ActivatedAt = activatedAt
	if start.IsZero() || start.Before(activatedAt) {
		start = activatedAt
	}
	if !start.Before(end) {
		return nil, service.ErrInvalidInput
	}
	err = r.db.QueryRowContext(ctx, `
		WITH direct_users AS (
			SELECT customer_user_id FROM affiliate_bindings
			WHERE inviter_user_id = $1 AND customer_user_id <> inviter_user_id
		), raw_paid AS (
			SELECT user_id, original_amount_micros AS amount_micros, occurred_at
			FROM balance_lots WHERE source_type IN ('paid_redeem', 'paid_topup')
			UNION ALL
			SELECT user_id, sale_price_micros AS amount_micros, starts_at AS occurred_at
			FROM monthly_entitlement_cycles WHERE source_type IN ('paid_redeem', 'paid_topup')
		), paid AS (
			SELECT
				COALESCE(SUM(amount_micros) FILTER (WHERE user_id = $1), 0)::bigint AS self_paid,
				COALESCE(SUM(amount_micros) FILTER (WHERE user_id IN (SELECT customer_user_id FROM direct_users)), 0)::bigint AS team_paid,
				COUNT(DISTINCT user_id) FILTER (WHERE user_id IN (SELECT customer_user_id FROM direct_users)) AS paid_users
			FROM raw_paid WHERE occurred_at >= $2 AND occurred_at < $3
		), consumption AS (
			SELECT
				COALESCE(SUM(CASE event_type WHEN 'confirmed_consumption' THEN amount_micros ELSE -amount_micros END)
					FILTER (WHERE user_id = $1), 0)::bigint AS self_consumption,
				COALESCE(SUM(CASE event_type WHEN 'confirmed_consumption' THEN amount_micros ELSE -amount_micros END)
					FILTER (WHERE user_id <> $1), 0)::bigint AS team_consumption,
				COALESCE(SUM(CASE event_type WHEN 'confirmed_consumption' THEN amount_micros ELSE -amount_micros END)
					FILTER (WHERE occurred_at >= GREATEST($2::timestamptz, NOW() - INTERVAL '30 days')), 0)::bigint AS recent_consumption
			FROM affiliate_performance_events
			WHERE direct_agent_id = $1 AND event_type IN ('confirmed_consumption', 'consumption_reversal')
				AND occurred_at >= $2 AND occurred_at < $3
		), cash AS (
			SELECT
				COALESCE(SUM(amount_micros) FILTER (WHERE posting_status = 'posted'
					AND entry_type IN ('earned','risk_release','reversal') AND occurred_at >= $2 AND occurred_at < $3), 0)::bigint AS earned,
				COALESCE(SUM(amount_micros) FILTER (WHERE posting_status = 'posted'), 0)::bigint AS available
			FROM agent_cash_commission_entries WHERE agent_id = $1
		), withdrawals AS (
			SELECT
				COALESCE(SUM(amount_micros) FILTER (WHERE status = 'processing'), 0)::bigint AS processing,
				COALESCE(SUM(amount_micros) FILTER (WHERE status = 'paid' AND requested_at >= $2 AND requested_at < $3), 0)::bigint AS paid
			FROM agent_withdrawal_requests WHERE agent_id = $1
		)
		SELECT (SELECT COUNT(*) FROM direct_users), paid.paid_users,
			paid.self_paid, paid.team_paid, consumption.self_consumption,
			consumption.team_consumption, consumption.recent_consumption,
			cash.earned, cash.available, withdrawals.processing, withdrawals.paid
		FROM paid CROSS JOIN consumption CROSS JOIN cash CROSS JOIN withdrawals
	`, agentID, start, end).Scan(
		&summary.DirectUserCount, &summary.PaidDirectUserCount,
		&summary.SelfRechargeMicros, &summary.DirectTeamRechargeMicros,
		&summary.SelfConsumptionMicros, &summary.DirectTeamConsumptionMicros,
		&summary.Recent30dConsumptionMicros, &summary.LifetimeEarnedMicros,
		&summary.AvailableCommissionMicros, &summary.ProcessingWithdrawalMicros,
		&summary.PaidCommissionMicros,
	)
	if err != nil {
		return nil, err
	}

	users, err := r.listPartnerUserPerformance(ctx, agentID, start, end)
	if err != nil {
		return nil, err
	}
	ledger, err := r.listPartnerCommissionEntries(ctx, agentID, start, end)
	if err != nil {
		return nil, err
	}
	withdrawals, err := r.listPartnerWithdrawals(ctx, agentID, start, end)
	if err != nil {
		return nil, err
	}
	return &service.AffiliatePartnerPerformanceDetail{
		Summary: summary, PeriodStart: start, PeriodEnd: end,
		DirectUsers: users, CommissionLedger: ledger, Withdrawals: withdrawals,
	}, nil
}

func (r *affiliateAgentRepository) listPartnerUserPerformance(
	ctx context.Context, agentID int64, start time.Time, end time.Time,
) ([]service.AffiliatePartnerUserPerformance, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), b.bound_at,
			COALESCE((
				SELECT SUM(x.amount_micros) FROM (
					SELECT original_amount_micros AS amount_micros, occurred_at FROM balance_lots
					WHERE user_id = u.id AND source_type IN ('paid_redeem', 'paid_topup')
					UNION ALL
					SELECT sale_price_micros, starts_at FROM monthly_entitlement_cycles
					WHERE user_id = u.id AND source_type IN ('paid_redeem', 'paid_topup')
				) x WHERE x.occurred_at >= $2 AND x.occurred_at < $3
			), 0)::bigint,
			COALESCE((SELECT SUM(CASE event_type WHEN 'confirmed_consumption' THEN amount_micros ELSE -amount_micros END)
				FROM affiliate_performance_events WHERE direct_agent_id = $1 AND user_id = u.id
					AND event_type IN ('confirmed_consumption', 'consumption_reversal') AND occurred_at >= $2 AND occurred_at < $3), 0)::bigint,
			COALESCE((SELECT SUM(amount_micros) FROM agent_cash_commission_entries
				WHERE agent_id = $1 AND consumer_user_id = u.id AND posting_status <> 'reversed'
					AND entry_type IN ('earned','risk_release','reversal') AND occurred_at >= $2 AND occurred_at < $3), 0)::bigint
		FROM affiliate_bindings b JOIN users u ON u.id = b.customer_user_id
		WHERE b.inviter_user_id = $1 AND b.customer_user_id <> b.inviter_user_id AND u.deleted_at IS NULL
		ORDER BY b.bound_at DESC, u.id DESC LIMIT 500
	`, agentID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliatePartnerUserPerformance, 0)
	for rows.Next() {
		var item service.AffiliatePartnerUserPerformance
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.JoinedAt,
			&item.RechargeMicros, &item.ConsumptionMicros, &item.GeneratedCommissionMicros); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *affiliateAgentRepository) listPartnerCommissionEntries(
	ctx context.Context, agentID int64, start time.Time, end time.Time,
) ([]service.AffiliatePartnerCommissionEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, COALESCE(consumer_user_id, 0), entry_type, posting_status, amount_micros, occurred_at
		FROM agent_cash_commission_entries
		WHERE agent_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		ORDER BY occurred_at DESC, id DESC LIMIT 100
	`, agentID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliatePartnerCommissionEntry, 0)
	for rows.Next() {
		var item service.AffiliatePartnerCommissionEntry
		if err := rows.Scan(&item.ID, &item.ConsumerUserID, &item.EntryType, &item.PostingStatus, &item.AmountMicros, &item.OccurredAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *affiliateAgentRepository) listPartnerWithdrawals(
	ctx context.Context, agentID int64, start time.Time, end time.Time,
) ([]service.AffiliatePartnerWithdrawal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, amount_micros, status, requested_at, paid_at,
			COALESCE(payment_reference, ''), COALESCE(failure_reason, '')
		FROM agent_withdrawal_requests
		WHERE agent_id = $1 AND requested_at >= $2 AND requested_at < $3
		ORDER BY requested_at DESC, id DESC LIMIT 100
	`, agentID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliatePartnerWithdrawal, 0)
	for rows.Next() {
		var item service.AffiliatePartnerWithdrawal
		if err := rows.Scan(&item.ID, &item.AmountMicros, &item.Status, &item.RequestedAt,
			&item.PaidAt, &item.PaymentReference, &item.FailureReason); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *affiliateAgentRepository) ReviewAgentApplication(
	ctx context.Context,
	applicationID int64,
	approve bool,
	decisionNote string,
	operatorID int64,
	defaultCode string,
	defaultCustomerRateBPS int32,
) (_ *service.AffiliateAgentReviewResult, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		userID            int64
		applicationStatus string
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, status
		FROM affiliate_agent_applications
		WHERE id = $1
		FOR UPDATE
	`, applicationID).Scan(&userID, &applicationStatus); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateApplicationNotFound
	} else if err != nil {
		return nil, err
	}
	if applicationStatus != "pending_review" {
		return nil, service.ErrAffiliateAgentActivationBlocked
	}
	if !approve {
		if _, err := tx.ExecContext(ctx, `
			UPDATE affiliate_agent_applications
			SET status = 'rejected',
				decision_note = $2,
				reviewed_at = NOW(),
				reviewed_by = $3,
				updated_at = NOW()
			WHERE id = $1
				AND status = 'pending_review'
		`, applicationID, decisionNote, operatorID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO agent_principals (
				agent_id, status, risk_status,
				applied_at, reviewed_at, reviewed_by, decision_note
			)
			VALUES ($1, 'rejected', 'clear', NOW(), NOW(), $2, $3)
			ON CONFLICT (agent_id) DO UPDATE SET
				status = 'rejected',
				reviewed_at = NOW(),
				reviewed_by = $2,
				decision_note = $3,
				updated_at = NOW()
		`, userID, operatorID, decisionNote); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO affiliate_agent_status_events (
				agent_id, previous_status, next_status, reason,
				application_id, operator_id
			)
			VALUES ($1, 'pending_review', 'rejected', $2, $3, $4)
		`, userID, decisionNote, applicationID, operatorID); err != nil {
			return nil, err
		}
		application, err := queryAffiliateAgentApplication(ctx, tx, applicationID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.AffiliateAgentReviewResult{Application: *application}, nil
	}

	var userStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM users
		WHERE id = $1
			AND deleted_at IS NULL
		FOR UPDATE
	`, userID).Scan(&userStatus); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	} else if err != nil {
		return nil, err
	}
	if userStatus != service.StatusActive {
		return nil, service.ErrAffiliateAgentActivationBlocked
	}

	qualification, err := queryAffiliateAgentQualification(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if qualification.ProgramMode != service.AffiliateProgramModeLive ||
		qualification.ProgramStartedAt == nil {
		return nil, service.ErrAffiliateProgramNotLive
	}
	switch qualification.AgentStatus {
	case "active":
		// Idempotent repair below ensures legacy/partial activation has a default link.
	case "suspended", "rejected":
		return nil, service.ErrAffiliateAgentActivationBlocked
	default:
		if !qualification.Qualified {
			return nil, service.ErrAffiliateQualificationNotMet
		}
	}
	if qualification.RiskStatus != "clear" {
		return nil, service.ErrAffiliateAgentActivationBlocked
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET role = 'agent',
			updated_at = NOW()
		WHERE id = $1
			AND deleted_at IS NULL
	`, userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status,
			qualified_at, activated_at,
			applied_at, reviewed_at, reviewed_by, decision_note
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW(), NOW(), NOW(), $2, $3)
		ON CONFLICT (agent_id) DO UPDATE SET
			status = 'active',
			qualified_at = COALESCE(agent_principals.qualified_at, NOW()),
			activated_at = COALESCE(agent_principals.activated_at, NOW()),
			reviewed_at = NOW(),
			reviewed_by = $2,
			decision_note = $3,
			updated_at = NOW()
		WHERE agent_principals.status NOT IN ('suspended', 'rejected')
	`, userID, operatorID, decisionNote); err != nil {
		return nil, err
	}

	defaultLink, err := getDefaultAffiliateLink(ctx, tx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		var codeCollision bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM users WHERE invite_code = $1
			)
		`, defaultCode).Scan(&codeCollision); err != nil {
			return nil, err
		}
		if codeCollision {
			return nil, service.ErrAffiliateLinkConflict
		}
		agentRateBPS := service.AffiliateAgentPoolRateBPS - defaultCustomerRateBPS
		var linkID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO affiliate_links (
				agent_id, code, name, channel,
				is_default, status, current_rate_version
			)
			VALUES ($1, $2, '默认推广链接', 'default', TRUE, 'active', 1)
			RETURNING id
		`, userID, defaultCode).Scan(&linkID)
		if err != nil {
			if isPostgresUniqueViolation(err) {
				return nil, service.ErrAffiliateLinkConflict
			}
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO affiliate_link_rate_versions (
				link_id, version,
				customer_rebate_rate_bps,
				agent_commission_rate_bps,
				created_by, effective_at
			)
			VALUES ($1, 1, $2, $3, $4, NOW())
		`, linkID, defaultCustomerRateBPS, agentRateBPS, userID); err != nil {
			return nil, err
		}
		defaultLink, err = getDefaultAffiliateLink(ctx, tx, userID)
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, NULL, 'agent_activated', 0,
			'qualification', NULL, $2, NOW(),
			jsonb_build_object(
				'qualification_route', $3::text,
				'program_mode', 'live'
			)
		)
		ON CONFLICT (event_key) DO NOTHING
	`, userID, fmt.Sprintf("agent-activated:user:%d", userID), qualification.QualificationRoute); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_qualification_states (
			user_id,
			direct_valid_consumer_count,
			direct_team_consumption_micros,
			self_consumption_micros,
			combined_consumption_micros,
			qualifying_route,
			status,
			qualified_at,
			evaluated_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW(), NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			direct_valid_consumer_count = EXCLUDED.direct_valid_consumer_count,
			direct_team_consumption_micros = EXCLUDED.direct_team_consumption_micros,
			self_consumption_micros = EXCLUDED.self_consumption_micros,
			combined_consumption_micros = EXCLUDED.combined_consumption_micros,
			qualifying_route = EXCLUDED.qualifying_route,
			status = 'active',
			qualified_at = COALESCE(affiliate_qualification_states.qualified_at, NOW()),
			evaluated_at = NOW(),
			updated_at = NOW()
	`, userID,
		qualification.ValidDirectUserCount,
		qualification.DirectTeamConsumptionMicros,
		qualification.SelfConsumptionMicros,
		qualification.CombinedConsumptionMicros,
		qualification.QualificationRoute,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_agent_notices (
			agent_id, notice_type, title, message,
			source_type, source_id, idempotency_key,
			metadata
		)
		SELECT
			$1, 'community_invite', s.title, s.message,
			'agent_activation', $1, $2,
			jsonb_build_object('has_qr_code', BTRIM(s.qr_object_key) <> '')
		FROM affiliate_community_settings s
		WHERE s.id = 1
			AND s.enabled = TRUE
		ON CONFLICT (idempotency_key) DO NOTHING
	`, userID, fmt.Sprintf("agent-activated:user:%d:community", userID)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE affiliate_agent_applications
		SET status = 'approved',
			decision_note = $2,
			reviewed_at = NOW(),
			reviewed_by = $3,
			updated_at = NOW()
		WHERE id = $1
			AND status = 'pending_review'
	`, applicationID, decisionNote, operatorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_agent_status_events (
			agent_id, previous_status, next_status, reason,
			application_id, operator_id
		)
		VALUES ($1, 'pending_review', 'active', $2, $3, $4)
	`, userID, decisionNote, applicationID, operatorID); err != nil {
		return nil, err
	}
	application, err := queryAffiliateAgentApplication(ctx, tx, applicationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	finalQualification, err := r.GetAgentQualification(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &service.AffiliateAgentReviewResult{
		Application: *application,
		Activation: &service.AffiliateAgentActivation{
			Qualification: *finalQualification,
			DefaultLink:   *defaultLink,
		},
	}, nil
}

type affiliateQualificationQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func queryAffiliateAgentQualification(
	ctx context.Context,
	q affiliateQualificationQuerier,
	userID int64,
) (*service.AffiliateAgentQualification, error) {
	out := &service.AffiliateAgentQualification{UserID: userID}
	var (
		startedAt      sql.NullTime
		principalState sql.NullString
		riskState      sql.NullString
		activatedAt    sql.NullTime
	)
	err := q.QueryRowContext(ctx, `
		WITH settings AS (
			SELECT
				mode,
				started_at,
				qualification_direct_user_count,
				qualification_min_user_consumption_micros,
				qualification_direct_team_consumption_micros,
				qualification_self_consumption_micros
			FROM affiliate_program_settings
			WHERE id = 1
		),
		baseline_net AS (
			SELECT
				e.user_id,
				SUM(e.confirmed_consumption_micros)::bigint AS amount_micros
			FROM affiliate_qualification_baseline_entries e
			CROSS JOIN settings s
			WHERE s.started_at IS NOT NULL
				AND e.cutoff_at <= s.started_at
			GROUP BY e.user_id
		),
		live_net AS (
			SELECT
				e.user_id,
				SUM(
					CASE e.event_type
						WHEN 'confirmed_consumption' THEN e.amount_micros
						WHEN 'consumption_reversal' THEN -e.amount_micros
						ELSE 0
					END
				)::bigint AS amount_micros
			FROM affiliate_performance_events e
			CROSS JOIN settings s
			WHERE e.event_type IN ('confirmed_consumption', 'consumption_reversal')
				AND s.started_at IS NOT NULL
				AND e.occurred_at >= s.started_at
				AND COALESCE(e.metadata ->> 'program_mode', '') = 'live'
			GROUP BY e.user_id
		),
		user_net AS (
			SELECT
				user_id,
				GREATEST(SUM(amount_micros), 0)::bigint AS amount_micros
			FROM (
				SELECT user_id, amount_micros FROM baseline_net
				UNION ALL
				SELECT user_id, amount_micros FROM live_net
			) amounts
			GROUP BY user_id
		),
		direct_users AS (
			SELECT
				b.customer_user_id AS user_id,
				COALESCE(n.amount_micros, 0)::bigint AS amount_micros
			FROM affiliate_bindings b
			LEFT JOIN user_net n ON n.user_id = b.customer_user_id
			WHERE b.inviter_user_id = $1
				AND b.customer_user_id <> $1
		),
		totals AS (
			SELECT
				COALESCE((
					SELECT amount_micros
					FROM user_net
					WHERE user_id = $1
				), 0)::bigint AS self_micros,
				COALESCE((
					SELECT SUM(direct_users.amount_micros)
					FROM direct_users, settings
					WHERE direct_users.amount_micros >= settings.qualification_min_user_consumption_micros
				), 0)::bigint AS direct_micros,
				COALESCE((
					SELECT COUNT(*)
					FROM direct_users, settings
					WHERE direct_users.amount_micros >= settings.qualification_min_user_consumption_micros
				), 0)::integer AS valid_direct_count
		)
		SELECT
			s.mode,
			s.started_at,
			ap.status,
			ap.risk_status,
			ap.activated_at,
			t.self_micros,
			t.direct_micros,
			(t.self_micros + t.direct_micros)::bigint,
			t.valid_direct_count,
			s.qualification_direct_user_count,
			s.qualification_min_user_consumption_micros,
			s.qualification_direct_team_consumption_micros,
			s.qualification_self_consumption_micros
		FROM settings s
		CROSS JOIN totals t
		LEFT JOIN agent_principals ap ON ap.agent_id = $1
		JOIN users u ON u.id = $1 AND u.deleted_at IS NULL
	`, userID).Scan(
		&out.ProgramMode,
		&startedAt,
		&principalState,
		&riskState,
		&activatedAt,
		&out.SelfConsumptionMicros,
		&out.DirectTeamConsumptionMicros,
		&out.CombinedConsumptionMicros,
		&out.ValidDirectUserCount,
		&out.RequiredDirectUserCount,
		&out.RequiredPerUserMicros,
		&out.RequiredDirectTeamMicros,
		&out.RequiredSelfMicros,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		out.ProgramStartedAt = &startedAt.Time
	}
	out.AgentStatus = "not_qualified"
	if principalState.Valid {
		out.AgentStatus = principalState.String
	}
	out.RiskStatus = "clear"
	if riskState.Valid {
		out.RiskStatus = riskState.String
	}
	if activatedAt.Valid {
		out.ActivatedAt = &activatedAt.Time
	}
	out.DirectRouteQualified =
		out.ValidDirectUserCount >= out.RequiredDirectUserCount &&
			out.DirectTeamConsumptionMicros >= out.RequiredDirectTeamMicros
	out.SelfRouteQualified =
		out.SelfConsumptionMicros >= out.RequiredSelfMicros
	out.Qualified = out.DirectRouteQualified || out.SelfRouteQualified
	switch {
	case out.DirectRouteQualified:
		out.QualificationRoute = "direct_team"
	case out.SelfRouteQualified:
		out.QualificationRoute = "self_consumption"
	}
	if out.AgentStatus == "active" {
		out.Qualified = true
		out.CanActivate = false
	} else if out.Qualified && out.AgentStatus == "not_qualified" {
		out.AgentStatus = "qualified"
	}
	out.CanActivate = false
	out.CanApply =
		out.ProgramMode == service.AffiliateProgramModeLive &&
			out.ProgramStartedAt != nil &&
			out.Qualified &&
			out.AgentStatus != "active" &&
			out.AgentStatus != "pending_review" &&
			out.AgentStatus != "suspended" &&
			out.AgentStatus != "terminated" &&
			out.RiskStatus == "clear"
	return out, nil
}

func getDefaultAffiliateLink(
	ctx context.Context,
	q affiliateQualificationQuerier,
	agentID int64,
) (*service.AffiliateLink, error) {
	out := &service.AffiliateLink{}
	err := scanAffiliateLink(q.QueryRowContext(ctx, `
		SELECT
			l.id, l.agent_id, l.code, l.name, l.channel,
			l.is_default, l.status, l.current_rate_version,
			rv.customer_rebate_rate_bps,
			rv.agent_commission_rate_bps,
			l.created_at, l.updated_at
		FROM affiliate_links l
		JOIN affiliate_link_rate_versions rv
			ON rv.link_id = l.id
			AND rv.version = l.current_rate_version
		WHERE l.agent_id = $1
			AND l.is_default = TRUE
	`, agentID), out)
	return out, err
}
