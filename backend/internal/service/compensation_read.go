package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (s *CompensationControlService) GetDraft(ctx context.Context, draftID int64) (*CompensationDraft, error) {
	if s == nil || s.db == nil || draftID <= 0 {
		return nil, fmt.Errorf("compensation draft input is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	draft := &CompensationDraft{Users: []CompensationDraftUser{}}
	var series, state, hash string
	var replaces, actor sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT id,series_id::text,incident_id,shadow_period_id,policy_version,revision_number,replaces_draft_id,state,revision_reason,created_by_user_id,redesigned,eligible_user_count,affected_user_count,proposed_total_cny_fen,affected_product_rolling_paid_value_cny_fen,high_value_threshold_cny_fen,high_value,evidence_hash,created_at FROM compensation_drafts WHERE id=$1`, draftID).Scan(&draft.ID, &series, &draft.IncidentID, &draft.ShadowPeriodID, &draft.PolicyVersion, &draft.RevisionNumber, &replaces, &state, &draft.RevisionReason, &actor, &draft.Redesigned, &draft.EligibleUserCount, &draft.AffectedUserCount, &draft.ProposedTotalCNYFen, &draft.AffectedProductRollingPaidValueCNYFen, &draft.HighValueThresholdCNYFen, &draft.HighValue, &hash, &draft.CreatedAt)
	if err != nil {
		return nil, err
	}
	draft.SeriesID = series
	draft.State = state
	draft.EvidenceHash = hash
	if replaces.Valid {
		v := replaces.Int64
		draft.ReplacesDraftID = &v
	}
	if actor.Valid {
		v := actor.Int64
		draft.CreatedByUserID = &v
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.id,u.user_id,usr.email,u.tier_snapshot_id,COALESCE(s.tier,'standard'),u.eligible,u.included,u.evidence_complete,u.final_failure_count,u.first_qualifying_failure_at,u.verified_paid_value_cny_fen,u.rolling_goodwill_executed_cny_fen,u.raw_value_cny_micros,u.tier_cap_cny_fen,u.relationship_cap_cny_fen,u.rolling_cap_cny_fen,u.rolling_remaining_cny_fen,u.proposed_total_cny_fen,u.balance_benefit_cny_fen,u.builder_pass_benefit_cny_fen,u.limiting_cap,u.exclusion_reason,u.evidence_hash FROM compensation_draft_users u JOIN users usr ON usr.id=u.user_id LEFT JOIN customer_tier_incident_snapshots s ON s.id=u.tier_snapshot_id WHERE u.draft_id=$1 ORDER BY u.user_id`, draftID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u CompensationDraftUser
		var tier string
		var snapshot sql.NullInt64
		var firstTime sql.NullTime
		var rollingCapValue, rollingRemainingValue sql.NullInt64
		if err := rows.Scan(&u.ID, &u.UserID, &u.Email, &snapshot, &tier, &u.Eligible, &u.Included, &u.EvidenceComplete, &u.FinalFailureCount, &firstTime, &u.VerifiedPaidValueCNYFen, &u.RollingGoodwillExecutedCNYFen, &u.RawValueCNYMicros, &u.TierCapCNYFen, &u.RelationshipCapCNYFen, &rollingCapValue, &rollingRemainingValue, &u.ProposedTotalCNYFen, &u.BalanceBenefitCNYFen, &u.BuilderPassBenefitCNYFen, &u.LimitingCap, &u.ExclusionReason, &u.EvidenceHash); err != nil {
			_ = rows.Close()
			return nil, err
		}
		u.Tier = CustomerTier(tier)
		if snapshot.Valid {
			v := snapshot.Int64
			u.TierSnapshotID = &v
		}
		if firstTime.Valid {
			v := firstTime.Time.UTC()
			u.FirstQualifyingFailureAt = &v
		}
		if rollingCapValue.Valid {
			v := rollingCapValue.Int64
			u.RollingCapCNYFen = &v
		}
		if rollingRemainingValue.Valid {
			v := rollingRemainingValue.Int64
			u.RollingRemainingCNYFen = &v
		}
		u.Items = []CompensationDraftItem{}
		draft.Users = append(draft.Users, u)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	index := map[int64]*CompensationDraftUser{}
	for i := range draft.Users {
		index[draft.Users[i].UserID] = &draft.Users[i]
	}
	itemRows, err := s.db.QueryContext(ctx, `SELECT i.id,i.user_id,i.product_id,p.display_name,i.group_id,COALESCE(w.group_display_name,''),i.benefit_channel,i.first_qualifying_failure_at,i.compensable_duration_ms,i.product_rate_version_id,COALESCE(r.rate_cny_fen_per_hour,0),i.group_weight_version_id,COALESCE(w.weight_bps,0),i.tier_multiplier_bps,i.raw_value_cny_micros,i.proposed_cny_fen,i.exclusion_reason,i.evidence FROM compensation_draft_items i JOIN service_status_products p ON p.id=i.product_id LEFT JOIN compensation_product_rate_versions r ON r.id=i.product_rate_version_id LEFT JOIN compensation_group_weight_versions w ON w.id=i.group_weight_version_id WHERE i.draft_id=$1 ORDER BY i.user_id,i.product_id,i.group_id`, draftID)
	if err != nil {
		return nil, err
	}
	for itemRows.Next() {
		var item CompensationDraftItem
		var group, weight, rateVersion sql.NullInt64
		var evidence []byte
		if err := itemRows.Scan(&item.ID, &item.UserID, &item.ProductID, &item.ProductName, &group, &item.GroupName, &item.BenefitChannel, &item.FirstQualifyingFailureAt, &item.CompensableDurationMS, &rateVersion, &item.ProductRateCNYFenPerHour, &weight, &item.GroupWeightBPS, &item.TierMultiplierBPS, &item.RawValueCNYMicros, &item.ProposedCNYFen, &item.ExclusionReason, &evidence); err != nil {
			_ = itemRows.Close()
			return nil, err
		}
		if group.Valid {
			v := group.Int64
			item.GroupID = &v
		}
		if rateVersion.Valid {
			item.ProductRateVersionID = rateVersion.Int64
		}
		if weight.Valid {
			v := weight.Int64
			item.GroupWeightVersionID = &v
		}
		var frozen CompensationDraftItem
		if json.Unmarshal(evidence, &frozen) == nil {
			item.SourceObservationIDs = frozen.SourceObservationIDs
			item.SegmentIDs = frozen.SegmentIDs
			item.SourceFacts = frozen.SourceFacts
			item.Segments = frozen.Segments
		}
		if user := index[item.UserID]; user != nil {
			user.Items = append(user.Items, item)
		}
	}
	if err := itemRows.Close(); err != nil {
		return nil, err
	}
	var review CompensationShadowReview
	var reviewActor sql.NullInt64
	err = s.db.QueryRowContext(ctx, `SELECT id,draft_id,owner_judgement_total_cny_fen,variance_cny_fen,result,notes,created_by_user_id,created_at FROM compensation_shadow_reviews WHERE draft_id=$1`, draftID).Scan(&review.ID, &review.DraftID, &review.OwnerJudgementTotalCNYFen, &review.VarianceCNYFen, &review.Result, &review.Notes, &reviewActor, &review.CreatedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		if reviewActor.Valid {
			v := reviewActor.Int64
			review.CreatedByUserID = &v
		}
		draft.Review = &review
	}
	return draft, nil
}

func (s *CompensationControlService) AdminSnapshot(ctx context.Context) (*AdminCompensationSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("compensation shadow service is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := &AdminCompensationSnapshot{Enabled: s.enabled(ctx), GeneratedAt: time.Now().UTC(), Drafts: []CompensationDraft{}}
	assessment, err := s.Assessment(ctx)
	if err != nil {
		return nil, err
	}
	result.Assessment = assessment
	rows, err := s.db.QueryContext(ctx, `WITH latest AS (SELECT DISTINCT ON(series_id) id,series_id::text,incident_id,shadow_period_id,policy_version,revision_number,replaces_draft_id,state,revision_reason,created_by_user_id,redesigned,eligible_user_count,affected_user_count,proposed_total_cny_fen,affected_product_rolling_paid_value_cny_fen,high_value_threshold_cny_fen,high_value,evidence_hash,created_at FROM compensation_drafts ORDER BY series_id,revision_number DESC,id DESC) SELECT id,series_id,incident_id,shadow_period_id,policy_version,revision_number,replaces_draft_id,state,revision_reason,created_by_user_id,redesigned,eligible_user_count,affected_user_count,proposed_total_cny_fen,affected_product_rolling_paid_value_cny_fen,high_value_threshold_cny_fen,high_value,evidence_hash,created_at FROM latest ORDER BY created_at DESC,id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var draft CompensationDraft
		var replaces, actor sql.NullInt64
		if err := rows.Scan(&draft.ID, &draft.SeriesID, &draft.IncidentID, &draft.ShadowPeriodID, &draft.PolicyVersion, &draft.RevisionNumber, &replaces, &draft.State, &draft.RevisionReason, &actor, &draft.Redesigned, &draft.EligibleUserCount, &draft.AffectedUserCount, &draft.ProposedTotalCNYFen, &draft.AffectedProductRollingPaidValueCNYFen, &draft.HighValueThresholdCNYFen, &draft.HighValue, &draft.EvidenceHash, &draft.CreatedAt); err != nil {
			return nil, err
		}
		if replaces.Valid {
			v := replaces.Int64
			draft.ReplacesDraftID = &v
		}
		if actor.Valid {
			v := actor.Int64
			draft.CreatedByUserID = &v
		}
		result.Drafts = append(result.Drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *CompensationControlService) Assessment(ctx context.Context) (CompensationShadowAssessment, error) {
	result := CompensationShadowAssessment{Enabled: s.enabled(ctx), ExecutionAvailable: false}
	var periodID int64
	var started time.Time
	var policyVersion int
	err := s.db.QueryRowContext(ctx, `SELECT id,started_at,policy_version FROM compensation_shadow_periods ORDER BY id DESC LIMIT 1`).Scan(&periodID, &started, &policyVersion)
	if err == sql.ErrNoRows {
		policy, policyErr := loadCompensationPolicy(ctx, s.db, time.Now().UTC())
		if policyErr != nil {
			return result, policyErr
		}
		result.MinimumDays = policy.ShadowMinimumDays
		result.MinimumIncidents = policy.ShadowMinimumIncidents
		return result, nil
	}
	if err != nil {
		return result, err
	}
	policy, err := loadCompensationPolicyVersion(ctx, s.db, policyVersion)
	if err != nil {
		return result, err
	}
	result.MinimumDays = policy.ShadowMinimumDays
	result.MinimumIncidents = policy.ShadowMinimumIncidents
	v := started.UTC()
	result.StartedAt = &v
	if days := int(time.Since(v).Hours() / 24); days > 0 {
		result.ElapsedDays = days
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT incident_id) FROM compensation_drafts WHERE shadow_period_id=$1 AND revision_number=1`, periodID).Scan(&result.EvaluatedIncidentCount); err != nil {
		return result, err
	}
	if err = s.db.QueryRowContext(ctx, `WITH latest AS (SELECT DISTINCT ON(series_id) id,incident_id,state,high_value FROM compensation_drafts WHERE shadow_period_id=$1 ORDER BY series_id,revision_number DESC,id DESC) SELECT COUNT(DISTINCT l.incident_id) FROM latest l JOIN compensation_shadow_reviews r ON r.draft_id=l.id WHERE r.result='aligned' AND l.high_value=FALSE AND l.state='shadow'`, periodID).Scan(&result.ReviewedIncidentCount); err != nil {
		return result, err
	}
	if err = s.db.QueryRowContext(ctx, `WITH latest AS (SELECT DISTINCT ON(series_id) id,incident_id,state,high_value FROM compensation_drafts WHERE shadow_period_id=$1 ORDER BY series_id,revision_number DESC,id DESC) SELECT NOT EXISTS(SELECT 1 FROM compensation_draft_failures WHERE shadow_period_id=$1 AND resolved_at IS NULL) AND NOT EXISTS(SELECT 1 FROM reliability_incidents i WHERE i.customer_impact_ended_at>=$2 AND NOT EXISTS(SELECT 1 FROM compensation_drafts d WHERE d.shadow_period_id=$1 AND d.incident_id=i.id)) AND NOT EXISTS(SELECT 1 FROM latest d JOIN reliability_incidents i ON i.id=d.incident_id WHERE d.high_value=TRUE OR d.state<>'shadow' OR i.evidence_gap=TRUE OR NOT EXISTS(SELECT 1 FROM compensation_evidence_snapshots e WHERE e.draft_id=d.id AND e.snapshot_kind='draft') OR EXISTS(SELECT 1 FROM compensation_draft_users u WHERE u.draft_id=d.id AND u.evidence_complete=FALSE) OR EXISTS(SELECT 1 FROM compensation_draft_items item WHERE item.draft_id=d.id AND item.exclusion_reason IN('missing_product_rate','missing_group_weight','missing_group_attribution','no_compensable_segment')))`, periodID, started).Scan(&result.EvidenceComplete); err != nil {
		return result, err
	}
	result.EligibleForOwnerReview = result.Enabled && result.ElapsedDays >= result.MinimumDays && result.EvaluatedIncidentCount >= result.MinimumIncidents && result.ReviewedIncidentCount >= result.MinimumIncidents && result.EvidenceComplete
	return result, nil
}
