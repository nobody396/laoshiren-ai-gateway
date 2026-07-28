package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func (r *affiliateAgentRepository) SubmitAgentApplication(
	ctx context.Context,
	userID int64,
	note string,
) (_ *service.AffiliateAgentApplication, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
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
	if !qualification.Qualified {
		return nil, service.ErrAffiliateQualificationNotMet
	}
	if qualification.AgentStatus == "active" ||
		qualification.AgentStatus == "suspended" ||
		qualification.AgentStatus == "terminated" {
		return nil, service.ErrAffiliateAgentActivationBlocked
	}
	if qualification.AgentStatus == "pending_review" {
		return nil, service.ErrAffiliateApplicationPending
	}
	if qualification.RiskStatus != "clear" {
		return nil, service.ErrAffiliateAgentActivationBlocked
	}

	route := qualification.QualificationRoute
	if route == "combined" {
		route = "direct_volume"
	}
	var applicationID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_agent_applications (
			user_id, status, qualifying_route,
			direct_valid_consumer_count,
			direct_team_consumption_micros,
			application_note
		)
		VALUES ($1, 'pending_review', $2, $3, $4, $5)
		RETURNING id
	`,
		userID,
		route,
		qualification.ValidDirectUserCount,
		qualification.DirectTeamConsumptionMicros,
		note,
	).Scan(&applicationID)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAffiliateApplicationPending
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status,
			qualified_at, applied_at, application_note
		)
		VALUES ($1, 'pending_review', 'clear', NOW(), NOW(), $2)
		ON CONFLICT (agent_id) DO UPDATE SET
			status = 'pending_review',
			qualified_at = COALESCE(agent_principals.qualified_at, NOW()),
			applied_at = NOW(),
			application_note = $2,
			decision_note = '',
			updated_at = NOW()
		WHERE agent_principals.status NOT IN ('active', 'suspended', 'terminated')
	`, userID, note); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_agent_status_events (
			agent_id, previous_status, next_status, reason, application_id
		)
		VALUES ($1, $2, 'pending_review', '用户提交合伙人申请', $3)
	`, userID, affiliatePersistedAgentStatus(qualification.AgentStatus), applicationID); err != nil {
		return nil, err
	}

	application, err := queryAffiliateAgentApplication(ctx, tx, applicationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return application, nil
}

func affiliatePersistedAgentStatus(status string) string {
	switch status {
	case "candidate", "pending_review", "active", "rejected", "suspended", "terminated":
		return status
	default:
		return "candidate"
	}
}

func (r *affiliateAgentRepository) ListAgentApplications(
	ctx context.Context,
	status string,
	limit int,
) ([]service.AffiliateAgentApplication, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			a.id, a.user_id,
			COALESCE(u.email, ''), COALESCE(u.username, ''),
			a.status, a.qualifying_route,
			a.direct_valid_consumer_count,
			a.direct_team_consumption_micros,
			a.application_note, a.decision_note,
			a.submitted_at, a.reviewed_at, a.reviewed_by
		FROM affiliate_agent_applications a
		JOIN users u ON u.id = a.user_id AND u.deleted_at IS NULL
		WHERE ($1 = 'all' OR a.status = $1)
		ORDER BY
			CASE WHEN a.status = 'pending_review' THEN 0 ELSE 1 END,
			a.submitted_at DESC,
			a.id DESC
		LIMIT $2
	`, status, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliateAgentApplication, 0, limit)
	for rows.Next() {
		var item service.AffiliateAgentApplication
		var reviewedAt sql.NullTime
		var reviewedBy sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.Status,
			&item.QualifyingRoute,
			&item.ValidDirectUserCount,
			&item.DirectTeamConsumptionMicros,
			&item.ApplicationNote,
			&item.DecisionNote,
			&item.SubmittedAt,
			&reviewedAt,
			&reviewedBy,
		); err != nil {
			return nil, err
		}
		if reviewedAt.Valid {
			item.ReviewedAt = &reviewedAt.Time
		}
		if reviewedBy.Valid {
			item.ReviewedBy = &reviewedBy.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func queryAffiliateAgentApplication(
	ctx context.Context,
	q affiliateQualificationQuerier,
	applicationID int64,
) (*service.AffiliateAgentApplication, error) {
	item := &service.AffiliateAgentApplication{}
	var reviewedAt sql.NullTime
	var reviewedBy sql.NullInt64
	err := q.QueryRowContext(ctx, `
		SELECT
			a.id, a.user_id,
			COALESCE(u.email, ''), COALESCE(u.username, ''),
			a.status, a.qualifying_route,
			a.direct_valid_consumer_count,
			a.direct_team_consumption_micros,
			a.application_note, a.decision_note,
			a.submitted_at, a.reviewed_at, a.reviewed_by
		FROM affiliate_agent_applications a
		JOIN users u ON u.id = a.user_id AND u.deleted_at IS NULL
		WHERE a.id = $1
	`, applicationID).Scan(
		&item.ID,
		&item.UserID,
		&item.Email,
		&item.Username,
		&item.Status,
		&item.QualifyingRoute,
		&item.ValidDirectUserCount,
		&item.DirectTeamConsumptionMicros,
		&item.ApplicationNote,
		&item.DecisionNote,
		&item.SubmittedAt,
		&reviewedAt,
		&reviewedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateApplicationNotFound
	}
	if err != nil {
		return nil, err
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	if reviewedBy.Valid {
		item.ReviewedBy = &reviewedBy.Int64
	}
	return item, nil
}
