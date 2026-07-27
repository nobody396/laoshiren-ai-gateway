package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

type affiliateLinkRepository struct {
	db *sql.DB
}

func NewAffiliateLinkRepository(db *sql.DB) service.AffiliateLinkRepository {
	return &affiliateLinkRepository{db: db}
}

func (r *affiliateLinkRepository) ResolveActiveLink(ctx context.Context, code string) (*service.AffiliateLinkReferral, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate link repository db is nil")
	}
	out := &service.AffiliateLinkReferral{}
	err := r.db.QueryRowContext(ctx, `
		SELECT
			l.agent_id,
			l.id,
			l.current_rate_version,
			rv.customer_rebate_rate_bps,
			rv.agent_commission_rate_bps
		FROM affiliate_links l
		JOIN agent_principals ap
			ON ap.agent_id = l.agent_id
		JOIN affiliate_link_rate_versions rv
			ON rv.link_id = l.id
			AND rv.version = l.current_rate_version
		WHERE l.code = $1
			AND l.status = 'active'
			AND ap.status = 'active'
			AND ap.risk_status = 'clear'
	`, strings.TrimSpace(code)).Scan(
		&out.AgentID,
		&out.LinkID,
		&out.RateVersion,
		&out.CustomerRebateRateBPS,
		&out.AgentCommissionRateBPS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *affiliateLinkRepository) BindAgentReferral(
	ctx context.Context,
	customerUserID int64,
	referral service.AffiliateLinkReferral,
) (_ error) {
	if r == nil || r.db == nil {
		return errors.New("affiliate link repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	current := service.AffiliateLinkReferral{}
	err = tx.QueryRowContext(ctx, `
		SELECT
			l.agent_id,
			l.id,
			l.current_rate_version,
			rv.customer_rebate_rate_bps,
			rv.agent_commission_rate_bps
		FROM affiliate_links l
		JOIN agent_principals ap
			ON ap.agent_id = l.agent_id
		JOIN affiliate_link_rate_versions rv
			ON rv.link_id = l.id
			AND rv.version = l.current_rate_version
		WHERE l.id = $1
			AND l.agent_id = $2
			AND l.status = 'active'
			AND ap.status = 'active'
			AND ap.risk_status = 'clear'
		FOR SHARE OF l, ap, rv
	`, referral.LinkID, referral.AgentID).Scan(
		&current.AgentID,
		&current.LinkID,
		&current.RateVersion,
		&current.CustomerRebateRateBPS,
		&current.AgentCommissionRateBPS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrAffiliateLinkNotFound
	}
	if err != nil {
		return err
	}
	if current.AgentID == customerUserID {
		return errors.New("affiliate self-binding is not allowed")
	}

	update, err := tx.ExecContext(ctx, `
		UPDATE users
		SET inviter_id = $1,
			agent_id = $1,
			updated_at = NOW()
		WHERE id = $2
			AND deleted_at IS NULL
			AND inviter_id IS NULL
	`, current.AgentID, customerUserID)
	if err != nil {
		return err
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		var existingAgentID sql.NullInt64
		err := tx.QueryRowContext(ctx, `
			SELECT agent_id
			FROM affiliate_bindings
			WHERE customer_user_id = $1
		`, customerUserID).Scan(&existingAgentID)
		if err == nil && existingAgentID.Valid && existingAgentID.Int64 == current.AgentID {
			return tx.Commit()
		}
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrUserNotFound
		}
		if err != nil {
			return err
		}
		return errors.New("customer already has a permanent affiliate binding")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id,
			inviter_user_id,
			binding_kind,
			agent_id,
			affiliate_link_id,
			link_rate_version,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps,
			bound_at
		)
		VALUES ($1, $2, 'agent', $2, $3, $4, $5, $6, NOW())
	`, customerUserID, current.AgentID, current.LinkID, current.RateVersion, current.CustomerRebateRateBPS, current.AgentCommissionRateBPS); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, $2, 'binding_created', 0,
			'agent_link_registration', $3::bigint, $4, NOW(),
			jsonb_build_object('link_id', $3::bigint, 'rate_version', $5::integer)
		)
		ON CONFLICT (event_key) DO NOTHING
	`, customerUserID, current.AgentID, current.LinkID, fmt.Sprintf("binding:user:%d", customerUserID), current.RateVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *affiliateLinkRepository) ListLinks(ctx context.Context, agentID int64) ([]service.AffiliateLink, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate link repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
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
		ORDER BY l.is_default DESC, l.id
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.AffiliateLink, 0, 6)
	for rows.Next() {
		var link service.AffiliateLink
		if err := scanAffiliateLink(rows, &link); err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, rows.Err()
}

func (r *affiliateLinkRepository) CreateLink(
	ctx context.Context,
	input service.CreateAffiliateLinkInput,
) (_ *service.AffiliateLink, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate link repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status, riskStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT status, risk_status
		FROM agent_principals
		WHERE agent_id = $1
		FOR UPDATE
	`, input.AgentID).Scan(&status, &riskStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAffiliateAgentNotActive
		}
		return nil, err
	}
	if status != "active" || riskStatus != service.AffiliateRiskStatusClear {
		return nil, service.ErrAffiliateAgentNotActive
	}
	if !input.IsDefault {
		var count, maxCampaigns int
		if err := tx.QueryRowContext(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE l.is_default = FALSE),
				s.max_campaign_links
			FROM affiliate_program_settings s
			LEFT JOIN affiliate_links l
				ON l.agent_id = $1
			WHERE s.id = 1
			GROUP BY s.max_campaign_links
		`, input.AgentID).Scan(&count, &maxCampaigns); err != nil {
			return nil, err
		}
		if count >= maxCampaigns {
			return nil, service.ErrAffiliateLinkLimit
		}
	}
	var collides bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE invite_code = $1
		)
	`, input.Code).Scan(&collides); err != nil {
		return nil, err
	}
	if collides {
		return nil, service.ErrAffiliateLinkConflict
	}

	agentRate := service.AffiliateAgentPoolRateBPS - input.CustomerRebateRateBPS
	var linkID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO affiliate_links (
			agent_id, code, name, channel, is_default,
			status, current_rate_version
		)
		VALUES ($1, $2, $3, $4, $5, 'active', 1)
		RETURNING id
	`, input.AgentID, input.Code, input.Name, input.Channel, input.IsDefault).Scan(&linkID)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAffiliateLinkConflict
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_link_rate_versions (
			link_id, version, customer_rebate_rate_bps,
			agent_commission_rate_bps, created_by, effective_at
		)
		VALUES ($1, 1, $2, $3, NULLIF($4, 0), NOW())
	`, linkID, input.CustomerRebateRateBPS, agentRate, input.CreatedBy); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getLink(ctx, input.AgentID, linkID)
}

func (r *affiliateLinkRepository) UpdateLinkRate(
	ctx context.Context,
	agentID, linkID int64,
	customerRate int32,
	changedBy int64,
) (_ *service.AffiliateLink, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var currentVersion int32
	err = tx.QueryRowContext(ctx, `
		SELECT l.current_rate_version
		FROM affiliate_links l
		JOIN agent_principals ap ON ap.agent_id = l.agent_id
		WHERE l.id = $1
			AND l.agent_id = $2
			AND ap.status = 'active'
			AND ap.risk_status = 'clear'
		FOR UPDATE OF l
	`, linkID, agentID).Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	nextVersion := currentVersion + 1
	agentRate := service.AffiliateAgentPoolRateBPS - customerRate
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_link_rate_versions (
			link_id, version, customer_rebate_rate_bps,
			agent_commission_rate_bps, created_by, effective_at
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), NOW())
	`, linkID, nextVersion, customerRate, agentRate, changedBy); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE affiliate_links
		SET current_rate_version = $1,
			updated_at = NOW()
		WHERE id = $2
	`, nextVersion, linkID); err != nil {
		return nil, err
	}
	// Existing customers have a rebate floor: lower link rates only affect
	// future bindings, while higher rates upgrade all current direct customers.
	if _, err := tx.ExecContext(ctx, `
		UPDATE affiliate_bindings
		SET
			link_rate_version = $1,
			customer_rebate_rate_snapshot_bps = $2,
			agent_commission_rate_snapshot_bps = $3,
			updated_at = NOW()
		WHERE affiliate_link_id = $4
			AND binding_kind = 'agent'
			AND customer_rebate_rate_snapshot_bps < $2
	`, nextVersion, customerRate, agentRate, linkID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getLink(ctx, agentID, linkID)
}

func (r *affiliateLinkRepository) SetLinkStatus(
	ctx context.Context,
	agentID, linkID int64,
	status string,
) (*service.AffiliateLink, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE affiliate_links l
		SET status = $1,
			updated_at = NOW()
		FROM agent_principals ap
		WHERE l.id = $2
			AND l.agent_id = $3
			AND ap.agent_id = l.agent_id
			AND ap.status = 'active'
			AND ap.risk_status = 'clear'
	`, status, linkID, agentID)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrAffiliateLinkNotFound
	}
	return r.getLink(ctx, agentID, linkID)
}

func (r *affiliateLinkRepository) getLink(ctx context.Context, agentID, linkID int64) (*service.AffiliateLink, error) {
	out := &service.AffiliateLink{}
	err := scanAffiliateLink(r.db.QueryRowContext(ctx, `
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
		WHERE l.id = $1
			AND l.agent_id = $2
	`, linkID, agentID), out)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateLinkNotFound
	}
	return out, err
}

type affiliateLinkScanner interface {
	Scan(dest ...any) error
}

func scanAffiliateLink(scanner affiliateLinkScanner, out *service.AffiliateLink) error {
	return scanner.Scan(
		&out.ID,
		&out.AgentID,
		&out.Code,
		&out.Name,
		&out.Channel,
		&out.IsDefault,
		&out.Status,
		&out.RateVersion,
		&out.CustomerRebateRateBPS,
		&out.AgentCommissionRateBPS,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
}

func isPostgresUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && string(pqErr.Code) == "23505"
}
