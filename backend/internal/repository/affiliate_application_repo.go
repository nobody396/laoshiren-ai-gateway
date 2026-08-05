package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

// isPostgresRetryableTransactionError reports serialization failures and
// deadlocks (SQLSTATE 40001/40P01) that disappear on a whole-transaction retry.
func isPostgresRetryableTransactionError(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return string(pqErr.Code) == "40001" || string(pqErr.Code) == "40P01"
}

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
	var applicationID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_agent_applications (
			user_id, status, qualifying_route,
			direct_valid_consumer_count,
			self_consumption_micros,
			direct_team_consumption_micros,
			combined_consumption_micros,
			application_note
		)
		VALUES ($1, 'pending_review', $2, $3, $4, $5, $6, $7)
		RETURNING id
	`,
		userID,
		route,
		qualification.ValidDirectUserCount,
		qualification.SelfConsumptionMicros,
		qualification.DirectTeamConsumptionMicros,
		qualification.CombinedConsumptionMicros,
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

// AutoActivateAgent activates a qualified user without manual review. It is
// idempotent: an already-active agent only gets the default-link repair, a
// legacy pending application is converged to approved in place, and every
// event/notice insert carries a deterministic conflict key. The decision note
// is fixed to service.AffiliateAgentAutoActivationNote and reviewed_by stays
// NULL so the record is distinguishable from a manual approval.
func (r *affiliateAgentRepository) AutoActivateAgent(
	ctx context.Context,
	userID int64,
	applicationNote string,
	defaultCode string,
	defaultCustomerRateBPS int32,
) (_ *service.AffiliateAgentReviewResult, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate agent repository db is nil")
	}
	defer func() {
		if err != nil && isPostgresRetryableTransactionError(err) {
			err = service.ErrAffiliateActivationConflict
		}
	}()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

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
	case "suspended", "rejected", "terminated":
		return nil, service.ErrAffiliateAgentActivationBlocked
	default:
		if !qualification.Qualified {
			return nil, service.ErrAffiliateQualificationNotMet
		}
	}
	if qualification.RiskStatus != "clear" {
		return nil, service.ErrAffiliateAgentActivationBlocked
	}

	var pendingApplicationID int64
	hasPendingApplication := true
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM affiliate_agent_applications
		WHERE user_id = $1
			AND status = 'pending_review'
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE
	`, userID).Scan(&pendingApplicationID); errors.Is(err, sql.ErrNoRows) {
		hasPendingApplication = false
	} else if err != nil {
		return nil, err
	}

	defaultLink, newlyActivated, err := activateAffiliateAgentInTx(
		ctx,
		tx,
		qualification,
		nil,
		service.AffiliateAgentAutoActivationNote,
		defaultCode,
		defaultCustomerRateBPS,
	)
	if err != nil {
		return nil, err
	}

	var application *service.AffiliateAgentApplication
	switch {
	case hasPendingApplication:
		// Converge the legacy pending application instead of leaving it queued
		// for a manual review that will never come.
		if _, err := tx.ExecContext(ctx, `
			UPDATE affiliate_agent_applications
			SET status = 'approved',
				decision_note = $2,
				reviewed_at = NOW(),
				reviewed_by = NULL,
				updated_at = NOW()
			WHERE id = $1
				AND status = 'pending_review'
		`, pendingApplicationID, service.AffiliateAgentAutoActivationNote); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO affiliate_agent_status_events (
				agent_id, previous_status, next_status, reason,
				application_id
			)
			VALUES ($1, 'pending_review', 'active', $2, $3)
		`, userID, service.AffiliateAgentAutoActivationNote, pendingApplicationID); err != nil {
			return nil, err
		}
		application, err = queryAffiliateAgentApplication(ctx, tx, pendingApplicationID)
		if err != nil {
			return nil, err
		}
	case qualification.AgentStatus != "active":
		var applicationID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO affiliate_agent_applications (
				user_id, status, qualifying_route,
				direct_valid_consumer_count,
				self_consumption_micros,
				direct_team_consumption_micros,
				combined_consumption_micros,
				application_note, decision_note,
				reviewed_at
			)
			VALUES ($1, 'approved', $2, $3, $4, $5, $6, $7, $8, NOW())
			RETURNING id
		`,
			userID,
			qualification.QualificationRoute,
			qualification.ValidDirectUserCount,
			qualification.SelfConsumptionMicros,
			qualification.DirectTeamConsumptionMicros,
			qualification.CombinedConsumptionMicros,
			applicationNote,
			service.AffiliateAgentAutoActivationNote,
		).Scan(&applicationID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO affiliate_agent_status_events (
				agent_id, previous_status, next_status, reason,
				application_id
			)
			VALUES ($1, $2, 'active', $3, $4)
		`, userID, affiliatePersistedAgentStatus(qualification.AgentStatus), service.AffiliateAgentAutoActivationNote, applicationID); err != nil {
			return nil, err
		}
		application, err = queryAffiliateAgentApplication(ctx, tx, applicationID)
		if err != nil {
			return nil, err
		}
	default:
		// Already active (a concurrent activation won): return the latest
		// application for reference when one exists.
		var latestApplicationID int64
		if err := tx.QueryRowContext(ctx, `
			SELECT id
			FROM affiliate_agent_applications
			WHERE user_id = $1
			ORDER BY id DESC
			LIMIT 1
		`, userID).Scan(&latestApplicationID); err == nil {
			application, err = queryAffiliateAgentApplication(ctx, tx, latestApplicationID)
			if err != nil {
				return nil, err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	finalQualification, err := r.GetAgentQualification(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := &service.AffiliateAgentReviewResult{
		Activation: &service.AffiliateAgentActivation{
			Qualification: *finalQualification,
			DefaultLink:   *defaultLink,
		},
		NewlyActivated: newlyActivated,
	}
	if application != nil {
		result.Application = *application
	}
	return result, nil
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
			a.self_consumption_micros,
			a.direct_team_consumption_micros,
			a.combined_consumption_micros,
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
			&item.SelfConsumptionMicros,
			&item.DirectTeamConsumptionMicros,
			&item.CombinedConsumptionMicros,
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
			a.self_consumption_micros,
			a.direct_team_consumption_micros,
			a.combined_consumption_micros,
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
		&item.SelfConsumptionMicros,
		&item.DirectTeamConsumptionMicros,
		&item.CombinedConsumptionMicros,
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
