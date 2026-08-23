package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *CustomerTierService) AdminSnapshot(ctx context.Context, filter AdminCustomerTierFilter) (*AdminCustomerTierSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("customer tier service is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	generatedAt := time.Now().UTC()
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 50
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	filter.Search = strings.TrimSpace(filter.Search)
	tierFilter := ""
	if filter.Tier.Valid() {
		tierFilter = string(filter.Tier)
	}
	projectionCTE := `WITH projected AS (SELECT c.user_id,c.calculated_tier,CASE WHEN o.expires_at<=$3 THEN c.calculated_tier ELSE c.effective_tier END effective_tier,c.verified_paid_value_cny_fen,c.verified_paid_consumption_micros,CASE WHEN o.expires_at<=$3 THEN NULL ELSE c.grace_expires_at END grace_expires_at,CASE WHEN o.expires_at<=$3 THEN NULL ELSE c.override_id END override_id,c.last_evaluation_id,c.evaluated_at,CASE WHEN o.expires_at<=$3 THEN 'override_expired_pending_evaluation' ELSE '' END projection_status FROM customer_tier_current c LEFT JOIN customer_tier_overrides o ON o.id=c.override_id) `
	var total int64
	if err := s.db.QueryRowContext(ctx, projectionCTE+`SELECT COUNT(*) FROM projected c JOIN users u ON u.id=c.user_id WHERE ($1='' OR u.email ILIKE '%'||$1||'%' OR c.user_id::text=$1) AND ($2='' OR c.effective_tier=$2)`, filter.Search, tierFilter, generatedAt).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, projectionCTE+`SELECT c.user_id,u.email,c.calculated_tier,c.effective_tier,c.verified_paid_value_cny_fen,c.verified_paid_consumption_micros,c.grace_expires_at,c.override_id,c.last_evaluation_id,c.evaluated_at,c.projection_status FROM projected c JOIN users u ON u.id=c.user_id WHERE ($1='' OR u.email ILIKE '%'||$1||'%' OR c.user_id::text=$1) AND ($2='' OR c.effective_tier=$2) ORDER BY CASE c.effective_tier WHEN 'strategic' THEN 3 WHEN 'priority' THEN 2 ELSE 1 END DESC,c.verified_paid_value_cny_fen DESC,c.user_id LIMIT $4 OFFSET $5`, filter.Search, tierFilter, generatedAt, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := &AdminCustomerTierSnapshot{Enabled: s.enabled(ctx), GeneratedAt: generatedAt, Customers: []CustomerTierCurrent{}, Page: filter.Page, PageSize: filter.PageSize, Total: total}
	for rows.Next() {
		var item CustomerTierCurrent
		var calculated, effective string
		var grace sql.NullTime
		var override sql.NullInt64
		if err := rows.Scan(&item.UserID, &item.Email, &calculated, &effective, &item.VerifiedPaidValueCNYFen, &item.VerifiedPaidConsumptionMicros, &grace, &override, &item.LastEvaluationID, &item.EvaluatedAt, &item.ProjectionStatus); err != nil {
			return nil, err
		}
		item.CalculatedTier, item.EffectiveTier = CustomerTier(calculated), CustomerTier(effective)
		if grace.Valid {
			v := grace.Time.UTC()
			item.GraceExpiresAt = &v
		}
		if override.Valid {
			v := override.Int64
			item.OverrideID = &v
		}
		result.Customers = append(result.Customers, item)
	}
	return result, rows.Err()
}

func (s *CustomerTierService) ExplainUser(ctx context.Context, userID int64) (*CustomerTierExplanation, error) {
	if s == nil || s.db == nil || userID <= 0 {
		return nil, fmt.Errorf("customer tier explanation input is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	now := time.Now().UTC()
	result := &CustomerTierExplanation{History: []CustomerTierHistoryItem{}, Overrides: []CustomerTierOverride{}}
	var calculated, effective string
	var grace sql.NullTime
	var overrideID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT c.user_id,u.email,c.calculated_tier,CASE WHEN o.expires_at<=$2 THEN c.calculated_tier ELSE c.effective_tier END,
 c.verified_paid_value_cny_fen,c.verified_paid_consumption_micros,
 CASE WHEN o.expires_at<=$2 THEN NULL ELSE c.grace_expires_at END,
 CASE WHEN o.expires_at<=$2 THEN NULL ELSE c.override_id END,c.last_evaluation_id,c.evaluated_at,
 CASE WHEN o.expires_at<=$2 THEN 'override_expired_pending_evaluation' ELSE '' END
FROM customer_tier_current c JOIN users u ON u.id=c.user_id LEFT JOIN customer_tier_overrides o ON o.id=c.override_id WHERE c.user_id=$1`, userID, now).Scan(
		&result.Current.UserID, &result.Current.Email, &calculated, &effective, &result.Current.VerifiedPaidValueCNYFen,
		&result.Current.VerifiedPaidConsumptionMicros, &grace, &overrideID, &result.Current.LastEvaluationID, &result.Current.EvaluatedAt, &result.Current.ProjectionStatus)
	if err != nil {
		return nil, err
	}
	result.Current.CalculatedTier, result.Current.EffectiveTier = CustomerTier(calculated), CustomerTier(effective)
	if grace.Valid {
		value := grace.Time.UTC()
		result.Current.GraceExpiresAt = &value
	}
	if overrideID.Valid {
		value := overrideID.Int64
		result.Current.OverrideID = &value
	}
	var evidenceRaw []byte
	var evaluationCalculated, evaluationEffective string
	var evaluationGrace sql.NullTime
	var evaluationOverride sql.NullInt64
	err = s.db.QueryRowContext(ctx, `
SELECT id,user_id,policy_version,calculated_tier,effective_tier,resolution_reason,verified_paid_value_cny_fen,
 verified_paid_consumption_micros,grace_expires_at,override_id,evidence,evidence_hash,evaluated_at
FROM customer_tier_evaluations WHERE id=$1`, result.Current.LastEvaluationID).Scan(
		&result.Evaluation.ID, &result.Evaluation.UserID, &result.Evaluation.PolicyVersion, &evaluationCalculated, &evaluationEffective,
		&result.Evaluation.ResolutionReason, &result.Evaluation.VerifiedPaidValueCNYFen, &result.Evaluation.VerifiedPaidConsumptionMicros,
		&evaluationGrace, &evaluationOverride, &evidenceRaw, &result.Evaluation.EvidenceHash, &result.Evaluation.EvaluatedAt)
	if err != nil {
		return nil, err
	}
	result.Evaluation.CalculatedTier, result.Evaluation.EffectiveTier = CustomerTier(evaluationCalculated), CustomerTier(evaluationEffective)
	if evaluationGrace.Valid {
		value := evaluationGrace.Time.UTC()
		result.Evaluation.GraceExpiresAt = &value
	}
	if evaluationOverride.Valid {
		value := evaluationOverride.Int64
		result.Evaluation.OverrideID = &value
	}
	if err := json.Unmarshal(evidenceRaw, &result.Evaluation.Evidence); err != nil {
		return nil, err
	}
	historyRows, err := s.db.QueryContext(ctx, `SELECT id,previous_tier,new_tier,change_reason,changed_at FROM customer_tier_history WHERE user_id=$1 ORDER BY changed_at DESC,id DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	for historyRows.Next() {
		var item CustomerTierHistoryItem
		var previous sql.NullString
		var next string
		if err := historyRows.Scan(&item.ID, &previous, &next, &item.ChangeReason, &item.ChangedAt); err != nil {
			_ = historyRows.Close()
			return nil, err
		}
		item.NewTier = CustomerTier(next)
		if previous.Valid {
			value := CustomerTier(previous.String)
			item.PreviousTier = &value
		}
		result.History = append(result.History, item)
	}
	if err := historyRows.Close(); err != nil {
		return nil, err
	}
	overrideRows, err := s.db.QueryContext(ctx, `SELECT o.id,o.user_id,o.tier,o.reason,o.starts_at,o.expires_at,o.created_by_user_id,o.created_at,COALESCE(r.status,'pending'),COALESCE(r.last_error,'') FROM customer_tier_overrides o LEFT JOIN customer_tier_override_refreshes r ON r.override_id=o.id WHERE o.user_id=$1 ORDER BY o.created_at DESC,o.id DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	for overrideRows.Next() {
		var item CustomerTierOverride
		var tier string
		var actor sql.NullInt64
		if err := overrideRows.Scan(&item.ID, &item.UserID, &tier, &item.Reason, &item.StartsAt, &item.ExpiresAt, &actor, &item.CreatedAt, &item.RefreshStatus, &item.RefreshError); err != nil {
			_ = overrideRows.Close()
			return nil, err
		}
		item.Tier = CustomerTier(tier)
		if actor.Valid {
			value := actor.Int64
			item.CreatedByUserID = &value
		}
		result.Overrides = append(result.Overrides, item)
	}
	if err := overrideRows.Close(); err != nil {
		return nil, err
	}
	result.Benefit = result.Evaluation.Evidence.Policy.Benefit(result.Current.EffectiveTier)
	return result, nil
}
