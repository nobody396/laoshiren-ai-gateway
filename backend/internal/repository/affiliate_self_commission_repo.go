package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

type affiliateSelfCommissionPolicyRepository struct {
	db *sql.DB
}

func NewAffiliateSelfCommissionPolicyRepository(
	db *sql.DB,
) service.AffiliateSelfCommissionPolicyRepository {
	return &affiliateSelfCommissionPolicyRepository{db: db}
}

type affiliateSelfCommissionEligibility struct {
	agentStatus string
	riskStatus  string
	hasUpstream bool
}

func (e affiliateSelfCommissionEligibility) eligible() bool {
	return !e.hasUpstream &&
		e.agentStatus == "active" &&
		e.riskStatus == service.AffiliateRiskStatusClear
}

func (e affiliateSelfCommissionEligibility) blockReason() string {
	switch {
	case e.hasUpstream:
		return service.AffiliateSelfCommissionBlockHasUpstream
	case e.agentStatus != "active":
		return service.AffiliateSelfCommissionBlockAgentNotActive
	case e.riskStatus != service.AffiliateRiskStatusClear:
		return service.AffiliateSelfCommissionBlockRiskNotClear
	default:
		return service.AffiliateSelfCommissionBlockNone
	}
}

func (r *affiliateSelfCommissionPolicyRepository) SetAffiliateSelfCommissionPolicy(
	ctx context.Context,
	agentID int64,
	enabled bool,
	expectedRevision int64,
	reason string,
	operatorID int64,
) (_ *service.AffiliateSelfCommissionPolicy, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate self-commission policy repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	eligibility, err := lockAffiliateSelfCommissionPartner(ctx, tx, agentID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateSelfCommissionAgentNotFound
	}
	if err != nil {
		return nil, err
	}

	var (
		currentEnabled     bool
		currentRevision    int64
		currentRateBPS     int
		currentEffectiveAt sql.NullTime
		currentUpdatedBy   sql.NullInt64
		currentReason      string
		currentCreatedAt   time.Time
		currentUpdatedAt   time.Time
		hasCurrent         bool
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			enabled,
			rate_bps,
			effective_at,
			revision,
			updated_by,
			reason,
			created_at,
			updated_at
		FROM affiliate_agent_self_commission_policies
		WHERE agent_id = $1
		FOR UPDATE
	`, agentID).Scan(
		&currentEnabled,
		&currentRateBPS,
		&currentEffectiveAt,
		&currentRevision,
		&currentUpdatedBy,
		&currentReason,
		&currentCreatedAt,
		&currentUpdatedAt,
	)
	switch {
	case err == nil:
		hasCurrent = true
	case errors.Is(err, sql.ErrNoRows):
		currentRateBPS = service.AffiliateSelfCommissionRateBPS
		currentRevision = 0
	default:
		return nil, err
	}

	if expectedRevision != currentRevision {
		return nil, service.ErrAffiliateSelfCommissionRevisionConflict
	}
	if enabled && !eligibility.eligible() {
		return nil, service.ErrAffiliateSelfCommissionNotEligible.WithMetadata(map[string]string{
			"block_reason_code": eligibility.blockReason(),
		})
	}

	// Setting the already-current state is an intentional no-op. This avoids
	// manufacturing audit revisions for repeated clicks while still enforcing
	// the caller's optimistic revision.
	if enabled == currentEnabled {
		result := &service.AffiliateSelfCommissionPolicy{
			AgentID:         agentID,
			Enabled:         currentEnabled,
			RateBPS:         currentRateBPS,
			Revision:        currentRevision,
			Reason:          currentReason,
			HasUpstream:     eligibility.hasUpstream,
			Eligible:        eligibility.eligible(),
			BlockReasonCode: eligibility.blockReason(),
		}
		if currentEffectiveAt.Valid {
			result.EffectiveAt = &currentEffectiveAt.Time
		}
		if currentUpdatedBy.Valid {
			result.UpdatedBy = &currentUpdatedBy.Int64
		}
		if hasCurrent {
			result.CreatedAt = &currentCreatedAt
			result.UpdatedAt = &currentUpdatedAt
		}
		return result, nil
	}

	nextRevision := currentRevision + 1

	if hasCurrent {
		_, err = tx.ExecContext(ctx, `
			UPDATE affiliate_agent_self_commission_policies
			SET enabled = $1,
				rate_bps = 1000,
				effective_at = CASE WHEN $1 THEN NOW() ELSE NULL END,
				revision = $2,
				updated_by = $3,
				reason = $4,
				updated_at = NOW()
			WHERE agent_id = $5
				AND revision = $6
		`, enabled, nextRevision, operatorID, reason, agentID, currentRevision)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO affiliate_agent_self_commission_policies (
				agent_id,
				enabled,
				rate_bps,
				effective_at,
				revision,
				updated_by,
				reason
			)
			VALUES (
				$1, $2, 1000,
				CASE WHEN $2 THEN NOW() ELSE NULL END,
				$3, $4, $5
			)
		`, agentID, enabled, nextRevision, operatorID, reason)
	}
	if err != nil {
		return nil, translateAffiliateSelfCommissionPolicyError(err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_agent_self_commission_events (
			agent_id,
			previous_enabled,
			next_enabled,
			rate_bps,
			effective_at,
			revision,
			reason,
			operator_id,
			metadata
		)
		SELECT
			agent_id,
			$1,
			enabled,
			rate_bps,
			effective_at,
			revision,
			reason,
			updated_by,
			jsonb_build_object('source', 'admin_api')
		FROM affiliate_agent_self_commission_policies
		WHERE agent_id = $2
	`, currentEnabled, agentID); err != nil {
		return nil, err
	}

	result, err := scanAffiliateSelfCommissionPolicy(
		tx.QueryRowContext(ctx, `
			SELECT
				p.agent_id,
				p.enabled,
				p.rate_bps,
				p.effective_at,
				p.revision,
				p.updated_by,
				p.reason,
				p.created_at,
				p.updated_at
			FROM affiliate_agent_self_commission_policies p
			WHERE p.agent_id = $1
		`, agentID),
		eligibility,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, translateAffiliateSelfCommissionPolicyError(err)
	}
	return result, nil
}

func lockAffiliateSelfCommissionPartner(
	ctx context.Context,
	tx *sql.Tx,
	agentID int64,
) (affiliateSelfCommissionEligibility, error) {
	var (
		out       affiliateSelfCommissionEligibility
		inviterID sql.NullInt64
		legacyID  sql.NullInt64
		bound     bool
	)
	err := tx.QueryRowContext(ctx, `
		SELECT
			ap.status,
			ap.risk_status,
			u.inviter_id,
			u.agent_id,
			EXISTS (
				SELECT 1
				FROM affiliate_bindings binding
				WHERE binding.customer_user_id = u.id
			)
		FROM users u
		JOIN agent_principals ap ON ap.agent_id = u.id
		WHERE u.id = $1
			AND u.deleted_at IS NULL
		FOR UPDATE OF u, ap
	`, agentID).Scan(
		&out.agentStatus,
		&out.riskStatus,
		&inviterID,
		&legacyID,
		&bound,
	)
	out.hasUpstream = inviterID.Valid || legacyID.Valid || bound
	return out, err
}

type affiliateSelfCommissionPolicyRow interface {
	Scan(dest ...any) error
}

func scanAffiliateSelfCommissionPolicy(
	row affiliateSelfCommissionPolicyRow,
	eligibility affiliateSelfCommissionEligibility,
) (*service.AffiliateSelfCommissionPolicy, error) {
	out := &service.AffiliateSelfCommissionPolicy{
		HasUpstream:     eligibility.hasUpstream,
		Eligible:        eligibility.eligible(),
		BlockReasonCode: eligibility.blockReason(),
	}
	var (
		effectiveAt sql.NullTime
		updatedBy   sql.NullInt64
		createdAt   time.Time
		updatedAt   time.Time
	)
	err := row.Scan(
		&out.AgentID,
		&out.Enabled,
		&out.RateBPS,
		&effectiveAt,
		&out.Revision,
		&updatedBy,
		&out.Reason,
		&createdAt,
		&updatedAt,
	)
	if effectiveAt.Valid {
		out.EffectiveAt = &effectiveAt.Time
	}
	if updatedBy.Valid {
		out.UpdatedBy = &updatedBy.Int64
	}
	out.CreatedAt = &createdAt
	out.UpdatedAt = &updatedAt
	return out, err
}

func translateAffiliateSelfCommissionPolicyError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}
	switch pqErr.Constraint {
	case "affiliate_self_commission_partner_exists":
		return service.ErrAffiliateSelfCommissionAgentNotFound
	case "affiliate_self_commission_partner_eligible":
		return service.ErrAffiliateSelfCommissionNotEligible
	case "affiliate_self_commission_no_upstream",
		"affiliate_binding_self_commission_exclusion",
		"users_upstream_self_commission_exclusion":
		return service.ErrAffiliateSelfCommissionNotEligible.WithMetadata(map[string]string{
			"block_reason_code": service.AffiliateSelfCommissionBlockHasUpstream,
		})
	case "uq_affiliate_self_commission_event_revision":
		return service.ErrAffiliateSelfCommissionRevisionConflict
	default:
		return err
	}
}
