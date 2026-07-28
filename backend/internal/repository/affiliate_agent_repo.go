package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
				qualification_combined_consumption_micros
			FROM affiliate_program_settings
			WHERE id = 1
		),
		live_net AS (
			SELECT
				e.user_id,
				e.direct_agent_id,
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
			GROUP BY e.user_id, e.direct_agent_id
		),
		direct_users AS (
			SELECT
				user_id,
				GREATEST(SUM(amount_micros), 0)::bigint AS amount_micros
			FROM live_net
			WHERE direct_agent_id = $1
				AND user_id <> $1
			GROUP BY user_id
		),
		totals AS (
			SELECT
				COALESCE((
					SELECT GREATEST(SUM(amount_micros), 0)::bigint
					FROM live_net
					WHERE user_id = $1
				), 0)::bigint AS self_micros,
				COALESCE((SELECT SUM(amount_micros) FROM direct_users), 0)::bigint AS direct_micros,
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
			t.direct_micros,
			t.valid_direct_count,
			s.qualification_direct_user_count,
			s.qualification_min_user_consumption_micros,
			s.qualification_direct_team_consumption_micros,
			s.qualification_combined_consumption_micros
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
		&out.RequiredCombinedMicros,
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
	out.CombinedRouteQualified =
		out.DirectTeamConsumptionMicros >= out.RequiredCombinedMicros
	out.Qualified = out.DirectRouteQualified || out.CombinedRouteQualified
	switch {
	case out.DirectRouteQualified:
		out.QualificationRoute = "direct_team"
	case out.CombinedRouteQualified:
		out.QualificationRoute = "direct_volume"
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
