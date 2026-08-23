package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *CustomerTierService) CreateOverride(ctx context.Context, command CustomerTierOverrideCommand) (*CustomerTierOverride, error) {
	command.Reason = strings.TrimSpace(command.Reason)
	if s == nil || s.db == nil || command.UserID <= 0 || !command.Tier.Valid() || command.Reason == "" || len(command.Reason) > 500 {
		return nil, fmt.Errorf("customer, tier, and reason are required")
	}
	if command.StartsAt.IsZero() {
		command.StartsAt = time.Now().UTC()
	}
	now := time.Now().UTC()
	command.StartsAt = command.StartsAt.UTC()
	command.ExpiresAt = command.ExpiresAt.UTC()
	if !command.ExpiresAt.After(command.StartsAt) || command.ExpiresAt.Sub(command.StartsAt) > 180*24*time.Hour {
		return nil, fmt.Errorf("customer tier override must be time-bounded within 180 days")
	}
	if command.StartsAt.Before(now.Add(-time.Minute)) {
		return nil, fmt.Errorf("customer tier override cannot start in the past")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, command.UserID); err != nil {
		return nil, err
	}
	var overlap bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM customer_tier_overrides WHERE user_id=$1 AND starts_at<$3 AND expires_at>$2)`, command.UserID, command.StartsAt, command.ExpiresAt).Scan(&overlap); err != nil {
		return nil, err
	}
	if overlap {
		return nil, fmt.Errorf("customer tier override overlaps an existing override")
	}
	item := &CustomerTierOverride{UserID: command.UserID, Tier: command.Tier, Reason: command.Reason, StartsAt: command.StartsAt, ExpiresAt: command.ExpiresAt, CreatedAt: time.Now().UTC()}
	var actor any
	if command.ActorUserID > 0 {
		actor = command.ActorUserID
		value := command.ActorUserID
		item.CreatedByUserID = &value
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO customer_tier_overrides(user_id,tier,reason,starts_at,expires_at,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,created_at`, command.UserID, string(command.Tier), command.Reason, command.StartsAt, command.ExpiresAt, actor).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO customer_tier_override_refreshes(override_id,status) VALUES($1,'pending')`, item.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	item.RefreshStatus = "pending"
	if !now.Before(command.StartsAt) && now.Before(command.ExpiresAt) {
		refreshAt := time.Now().UTC()
		if refreshAt.Before(item.CreatedAt) {
			refreshAt = item.CreatedAt
		}
		s.refreshOverride(ctx, item, refreshAt)
	}
	return item, nil
}

func (s *CustomerTierService) refreshOverride(ctx context.Context, item *CustomerTierOverride, now time.Time) {
	evaluation, err := s.EvaluateUserAt(ctx, item.UserID, now)
	if err != nil {
		item.RefreshStatus = "failed"
		item.RefreshError = truncateCustomerTierError(err)
		_, _ = s.db.ExecContext(context.Background(), `UPDATE customer_tier_override_refreshes SET status='failed',attempts=attempts+1,last_error=$2,updated_at=NOW() WHERE override_id=$1`, item.ID, item.RefreshError)
		return
	}
	item.RefreshStatus = "applied"
	item.RefreshError = ""
	_, _ = s.db.ExecContext(context.Background(), `UPDATE customer_tier_override_refreshes SET status='applied',attempts=attempts+1,last_error=NULL,applied_evaluation_id=$2,updated_at=NOW() WHERE override_id=$1`, item.ID, evaluation.ID)
}

func (s *CustomerTierService) RefreshExpiredOverrides(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT c.user_id FROM customer_tier_current c JOIN customer_tier_overrides o ON o.id=c.override_id WHERE o.expires_at<=$1 ORDER BY c.user_id LIMIT 500`, now)
	if err != nil {
		return err
	}
	var expired []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		expired = append(expired, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range expired {
		if _, err := s.EvaluateUserAt(ctx, id, now); err != nil {
			return fmt.Errorf("refresh expired customer tier override for user %d: %w", id, err)
		}
	}
	rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT o.user_id FROM customer_tier_overrides o JOIN customer_tier_override_refreshes r ON r.override_id=o.id WHERE r.status IN('pending','failed') AND o.starts_at<=$1 AND o.expires_at>$1 ORDER BY o.user_id LIMIT 500`, now)
	if err != nil {
		return err
	}
	var pending []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		pending = append(pending, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range pending {
		evaluation, err := s.EvaluateUserAt(ctx, id, now)
		if err != nil {
			continue
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE customer_tier_override_refreshes r SET status='applied',attempts=attempts+1,last_error=NULL,applied_evaluation_id=$2,updated_at=NOW() FROM customer_tier_overrides o WHERE r.override_id=o.id AND o.user_id=$1 AND o.starts_at<=$3 AND o.expires_at>$3`, id, evaluation.ID, now)
	}
	return nil
}

func (s *CustomerTierService) EnsureIncidentSnapshots(ctx context.Context, incidentID int64) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("customer tier snapshots are unavailable")
	}
	mismatchQuery := `SELECT COUNT(*) FROM customer_tier_incident_snapshots s JOIN reliability_incidents i ON i.id=s.incident_id WHERE i.customer_impact_started_at IS DISTINCT FROM s.customer_impact_started_at`
	args := []any{}
	if incidentID > 0 {
		mismatchQuery += ` AND i.id=$1`
		args = append(args, incidentID)
	}
	var mismatches int64
	if err := s.db.QueryRowContext(ctx, mismatchQuery, args...).Scan(&mismatches); err != nil {
		return err
	}
	if mismatches > 0 {
		return fmt.Errorf("customer tier snapshot impact window changed after freeze")
	}
	for {
		query := `
SELECT DISTINCT i.id,i.customer_impact_started_at,o.user_id
FROM reliability_incidents i
JOIN reliability_incident_observation_links l ON l.incident_id=i.id AND l.relation='customer_impact'
JOIN reliability_observations o ON o.id=l.observation_id AND o.user_id IS NOT NULL
LEFT JOIN customer_tier_incident_snapshots s ON s.incident_id=i.id AND s.user_id=o.user_id
WHERE i.customer_impact_started_at IS NOT NULL AND s.id IS NULL`
		pageArgs := []any{}
		if incidentID > 0 {
			query += ` AND i.id=$1`
			pageArgs = append(pageArgs, incidentID)
		}
		query += ` ORDER BY i.id,o.user_id LIMIT 500`
		rows, err := s.db.QueryContext(ctx, query, pageArgs...)
		if err != nil {
			return err
		}
		type target struct {
			incidentID, userID int64
			impactAt           time.Time
		}
		targets := make([]target, 0, 500)
		for rows.Next() {
			var item target
			if err := rows.Scan(&item.incidentID, &item.impactAt, &item.userID); err != nil {
				_ = rows.Close()
				return err
			}
			targets = append(targets, item)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		for _, target := range targets {
			if err := s.snapshotIncidentTier(ctx, target.incidentID, target.userID, target.impactAt.UTC()); err != nil {
				return err
			}
		}
	}
}

func (s *CustomerTierService) snapshotIncidentTier(ctx context.Context, incidentID, userID int64, impactAt time.Time) error {
	policy, err := loadCustomerTierPolicy(ctx, s.db, impactAt)
	if err != nil {
		return err
	}
	evidence, err := s.loadEvidenceWithPolicy(ctx, userID, impactAt, policy)
	if err != nil {
		return err
	}
	calculated := policy.Classify(evidence.VerifiedPaidValueCNYFen)
	input := CustomerTierResolutionInput{Now: impactAt, CurrentTier: CustomerTierStandard, CalculatedTier: calculated, DowngradeGrace: policy.Grace()}
	var previousTier, previousOverride sql.NullString
	var grace sql.NullTime
	err = s.db.QueryRowContext(ctx, `SELECT effective_tier,grace_expires_at,COALESCE(override_id::text,'') FROM customer_tier_evaluations WHERE user_id=$1 AND evaluated_at<=$2 ORDER BY evaluated_at DESC,id DESC LIMIT 1`, userID, impactAt).Scan(&previousTier, &grace, &previousOverride)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if previousTier.Valid {
		input.CurrentTier = CustomerTier(previousTier.String)
	}
	if grace.Valid {
		value := grace.Time.UTC()
		input.GraceExpiresAt = &value
	}
	var override CustomerTierOverride
	var overrideTier sql.NullString
	err = s.db.QueryRowContext(ctx, `SELECT id,tier FROM customer_tier_overrides WHERE user_id=$1 AND starts_at<=$2 AND expires_at>$2 AND created_at<=$2 ORDER BY created_at DESC,id DESC LIMIT 1`, userID, impactAt).Scan(&override.ID, &overrideTier)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if overrideTier.Valid {
		override.Tier = CustomerTier(overrideTier.String)
		input.OverrideActive = true
		input.OverrideTier = override.Tier
	}
	input.PreviousOverrideActive = previousOverride.Valid && previousOverride.String != "" && !input.OverrideActive
	resolution := ResolveCustomerTier(input)
	benefit := policy.Benefit(resolution.EffectiveTier)
	payload := struct {
		Evidence   CustomerTierEvidence   `json:"evidence"`
		Resolution CustomerTierResolution `json:"resolution"`
		Policy     CustomerTierPolicy     `json:"policy"`
	}{Evidence: evidence, Resolution: resolution, Policy: policy}
	evidenceJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(evidenceJSON)
	var overrideID any
	if input.OverrideActive {
		overrideID = override.ID
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO customer_tier_incident_snapshots(incident_id,user_id,policy_version,tier,multiplier,cap_cny_fen,
 verified_paid_value_cny_fen,verified_paid_consumption_micros,override_id,customer_impact_started_at,evidence,policy_snapshot,evidence_hash)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(incident_id,user_id) DO NOTHING`, incidentID, userID, policy.Version, string(resolution.EffectiveTier), benefit.Multiplier, benefit.CapCNYFen, evidence.VerifiedPaidValueCNYFen, evidence.VerifiedPaidConsumptionMicros, overrideID, impactAt, evidenceJSON, policyJSON, hex.EncodeToString(hash[:]))
	if err != nil {
		return err
	}
	var frozen time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT customer_impact_started_at FROM customer_tier_incident_snapshots WHERE incident_id=$1 AND user_id=$2`, incidentID, userID).Scan(&frozen); err != nil {
		return err
	}
	if !frozen.Equal(impactAt) {
		return fmt.Errorf("customer tier snapshot impact window changed after freeze")
	}
	return nil
}
