package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type compensationFailure struct {
	ObservationID, UserID, ProductID                          int64
	GroupID                                                   sql.NullInt64
	ObservedAt                                                time.Time
	RequestID, FactType, Outcome, ErrorOwner, ExclusionReason string
	CustomerImpact, Qualified                                 bool
	QualificationReason                                       string
}
type compensationItemBuild struct {
	item     CompensationDraftItem
	rateID   int64
	weightID any
	evidence []byte
}
type compensationUserBuild struct {
	user     CompensationDraftUser
	evidence []byte
	items    []compensationItemBuild
	paid30   CustomerTierEvidence
}
type compensationDraftBuild struct {
	policy                 CompensationPolicy
	incidentID, periodID   int64
	impactStart, impactEnd time.Time
	users                  []compensationUserBuild
	productRollingPaid     int64
	total                  int64
	hash                   string
	payload                []byte
}
type compensationUserEvidence = CompensationUserEvidenceSnapshot

func (s *CompensationControlService) DraftIncident(ctx context.Context, incidentID, actorUserID int64) (*CompensationDraft, error) {
	if s == nil || s.db == nil || s.tiers == nil || incidentID <= 0 {
		return nil, fmt.Errorf("compensation draft input is invalid")
	}
	if !s.enabled(ctx) {
		return nil, fmt.Errorf("compensation shadow draft generation is disabled")
	}
	if err := s.tiers.EnsureIncidentSnapshots(ctx, incidentID); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-draft'),$1)`, incidentID); err != nil {
		return nil, err
	}
	var existingID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM compensation_drafts WHERE incident_id=$1 AND revision_number=1`, incidentID).Scan(&existingID)
	if err == nil {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return s.GetDraft(ctx, existingID)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	build, err := s.buildCompensationDraft(ctx, tx, incidentID)
	if err != nil {
		return nil, err
	}
	draftID, err := persistCompensationDraftTx(ctx, tx, build, uuid.NewString(), 1, nil, "", actorUserID, false)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetDraft(ctx, draftID)
}

func (s *CompensationControlService) buildCompensationDraft(ctx context.Context, tx *sql.Tx, incidentID int64) (compensationDraftBuild, error) {
	var build compensationDraftBuild
	build.incidentID = incidentID
	if err := tx.QueryRowContext(ctx, `SELECT customer_impact_started_at,customer_impact_ended_at FROM reliability_incidents WHERE id=$1 AND customer_impact_started_at IS NOT NULL AND customer_impact_ended_at IS NOT NULL`, incidentID).Scan(&build.impactStart, &build.impactEnd); err != nil {
		if err == sql.ErrNoRows {
			return build, fmt.Errorf("incident customer impact window is incomplete")
		}
		return build, err
	}
	build.impactStart = build.impactStart.UTC()
	build.impactEnd = build.impactEnd.UTC()
	if err := tx.QueryRowContext(ctx, `SELECT id FROM compensation_shadow_periods WHERE ended_at IS NULL ORDER BY started_at DESC,id DESC LIMIT 1`).Scan(&build.periodID); err != nil {
		return build, fmt.Errorf("active compensation shadow period is required: %w", err)
	}
	policy, err := loadCompensationPolicy(ctx, tx, build.impactEnd)
	if err != nil {
		return build, err
	}
	build.policy = policy
	failures, err := loadCompensationFailures(ctx, tx, incidentID)
	if err != nil {
		return build, err
	}
	if len(failures) == 0 {
		return build, fmt.Errorf("incident has no attributed final customer failures")
	}
	segments, err := loadCompensationSegments(ctx, tx, incidentID, build.impactEnd)
	if err != nil {
		return build, err
	}
	byUser := map[int64][]compensationFailure{}
	affectedUsers := map[int64]bool{}
	for _, failure := range failures {
		byUser[failure.UserID] = append(byUser[failure.UserID], failure)
		if failure.FactType == "customer_request" && failure.Outcome == "failure" && failure.CustomerImpact {
			affectedUsers[failure.UserID] = true
		}
	}
	for userID := range byUser {
		if !affectedUsers[userID] {
			delete(byUser, userID)
		}
	}
	if len(byUser) == 0 {
		return build, fmt.Errorf("incident has no attributed final customer failures")
	}
	userIDs := make([]int64, 0, len(byUser))
	for id := range byUser {
		userIDs = append(userIDs, id)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	paid30 := map[int64]CustomerTierEvidence{}
	tierPolicy, err := loadCustomerTierPolicy(ctx, tx, build.impactEnd)
	if err != nil {
		return build, err
	}
	tierPolicy.RollingWindowDays = 30
	productUserSeen := map[string]bool{}
	for _, userID := range userIDs {
		evidence, err := s.tiers.loadEvidenceWithPolicyTx(ctx, tx, userID, build.impactEnd, tierPolicy)
		if err != nil {
			return build, err
		}
		paid30[userID] = evidence
		products := map[int64]bool{}
		for _, f := range byUser[userID] {
			if f.FactType == "customer_request" && f.Outcome == "failure" && f.CustomerImpact {
				products[f.ProductID] = true
			}
		}
		for productID := range products {
			key := fmt.Sprintf("%d:%d", productID, userID)
			if !productUserSeen[key] {
				build.productRollingPaid, err = checkedAddCompensation(build.productRollingPaid, evidence.VerifiedPaidValueCNYFen)
				if err != nil {
					return build, err
				}
				productUserSeen[key] = true
			}
		}
	}
	for _, userID := range userIDs {
		userBuild, err := s.buildCompensationUser(ctx, tx, incidentID, userID, byUser[userID], segments, build, paid30[userID])
		if err != nil {
			return build, err
		}
		build.total, err = checkedAddCompensation(build.total, userBuild.user.ProposedTotalCNYFen)
		if err != nil {
			return build, err
		}
		build.users = append(build.users, userBuild)
	}
	evidenceUsers := make([]compensationUserEvidence, 0, len(build.users))
	for _, user := range build.users {
		items := make([]CompensationDraftItem, len(user.items))
		for i := range user.items {
			items[i] = user.items[i].item
		}
		evidenceUsers = append(evidenceUsers, compensationUserEvidence{User: user.user, Items: items, Paid30: user.paid30, Policy: build.policy})
	}
	payload := struct {
		IncidentID         int64                      `json:"incident_id"`
		ImpactStart        time.Time                  `json:"impact_start"`
		ImpactEnd          time.Time                  `json:"impact_end"`
		Policy             CompensationPolicy         `json:"policy"`
		ProductRollingPaid int64                      `json:"affected_product_rolling_paid_value_cny_fen"`
		Users              []compensationUserEvidence `json:"users"`
	}{incidentID, build.impactStart, build.impactEnd, build.policy, build.productRollingPaid, evidenceUsers}
	build.payload, err = canonicalCompensationJSON(payload)
	if err != nil {
		return build, err
	}
	sum := sha256.Sum256(build.payload)
	build.hash = hex.EncodeToString(sum[:])
	return build, nil
}

func loadCompensationFailures(ctx context.Context, tx *sql.Tx, incidentID int64) ([]compensationFailure, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT o.id,o.user_id,l.product_id,o.group_id,o.observed_at,o.request_id,o.fact_type,o.outcome,o.error_owner,o.exclusion_reason,o.customer_impact FROM reliability_incident_observation_links l JOIN reliability_observations o ON o.id=l.observation_id WHERE l.incident_id=$1 AND l.relation='customer_impact' AND l.product_id IS NOT NULL AND o.user_id IS NOT NULL ORDER BY o.user_id,l.product_id,o.group_id,o.observed_at,o.id`, incidentID)
	if err != nil {
		return nil, err
	}
	var result []compensationFailure
	for rows.Next() {
		var item compensationFailure
		if err := rows.Scan(&item.ObservationID, &item.UserID, &item.ProductID, &item.GroupID, &item.ObservedAt, &item.RequestID, &item.FactType, &item.Outcome, &item.ErrorOwner, &item.ExclusionReason, &item.CustomerImpact); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.ObservedAt = item.ObservedAt.UTC()
		item.Qualified, item.QualificationReason = qualifyCompensationFailure(item)
		result = append(result, item)
	}
	return result, rows.Close()
}

func qualifyCompensationFailure(item compensationFailure) (bool, string) {
	switch {
	case item.FactType != "customer_request":
		return false, "not_final_customer_request"
	case item.Outcome != "failure":
		return false, "final_outcome_not_failure"
	case !item.CustomerImpact:
		return false, "not_customer_impacting"
	case item.ErrorOwner != "provider" && item.ErrorOwner != "platform":
		return false, "excluded_error_owner"
	case item.ExclusionReason != "":
		return false, item.ExclusionReason
	case strings.TrimSpace(item.RequestID) == "":
		return false, "missing_final_request_identity"
	default:
		return true, "qualified_final_failure"
	}
}

func compensationEvidenceFact(item compensationFailure) CompensationEvidenceFact {
	result := CompensationEvidenceFact{ObservationID: item.ObservationID, RequestID: item.RequestID, FactType: item.FactType, Outcome: item.Outcome, ErrorOwner: item.ErrorOwner, ExclusionReason: item.ExclusionReason, CustomerImpact: item.CustomerImpact, ProductID: item.ProductID, ObservedAt: item.ObservedAt, Qualified: item.Qualified, QualificationReason: item.QualificationReason}
	if item.GroupID.Valid {
		v := item.GroupID.Int64
		result.GroupID = &v
	}
	return result
}

func buildCompensationSegmentEvidence(productID int64, segments []CompensationImpactSegment, firstFailure, impactEnd time.Time) (int64, []CompensationEvidenceSegment, error) {
	var total int64
	result := []CompensationEvidenceSegment{}
	for _, segment := range segments {
		start := segment.StartedAt
		if firstFailure.After(start) {
			start = firstFailure
		}
		end := segment.EndedAt
		if end.IsZero() || impactEnd.Before(end) {
			end = impactEnd
		}
		if end.After(start) {
			duration := end.Sub(start).Milliseconds()
			var err error
			total, err = checkedAddCompensation(total, duration)
			if err != nil {
				return 0, nil, err
			}
			result = append(result, CompensationEvidenceSegment{SegmentID: segment.ID, ProductID: productID, StartedAt: segment.StartedAt, EndedAt: segment.EndedAt, ClippedStartedAt: start, ClippedEndedAt: end, DurationMS: duration})
		}
	}
	return total, result, nil
}
func loadCompensationSegments(ctx context.Context, tx *sql.Tx, incidentID int64, impactEnd time.Time) (map[int64][]CompensationImpactSegment, error) {
	rows, err := tx.QueryContext(ctx, `SELECT p.product_id,s.id,s.started_at,COALESCE(s.ended_at,$2) FROM reliability_incident_impact_segments s JOIN reliability_incident_products p ON p.id=s.incident_product_id WHERE s.incident_id=$1 ORDER BY p.product_id,s.started_at,s.id`, incidentID, impactEnd)
	if err != nil {
		return nil, err
	}
	result := map[int64][]CompensationImpactSegment{}
	for rows.Next() {
		var productID int64
		var item CompensationImpactSegment
		if err := rows.Scan(&productID, &item.ID, &item.StartedAt, &item.EndedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result[productID] = append(result[productID], item)
	}
	return result, rows.Close()
}

func (s *CompensationControlService) buildCompensationUser(ctx context.Context, tx *sql.Tx, incidentID, userID int64, failures []compensationFailure, segments map[int64][]CompensationImpactSegment, build compensationDraftBuild, paid30 CustomerTierEvidence) (compensationUserBuild, error) {
	var result compensationUserBuild
	result.user.UserID = userID
	result.user.EvidenceComplete = true
	uniqueRequests := map[string]bool{}
	qualified := make([]compensationFailure, 0, len(failures))
	for _, failure := range failures {
		if failure.Qualified && !uniqueRequests[failure.RequestID] {
			uniqueRequests[failure.RequestID] = true
			qualified = append(qualified, failure)
		}
	}
	result.user.FinalFailureCount = len(uniqueRequests)
	eligible := len(uniqueRequests) >= build.policy.FinalFailureThreshold
	result.user.Eligible = eligible
	result.user.Included = eligible
	first := failures[0].ObservedAt
	if len(qualified) > 0 {
		first = qualified[0].ObservedAt
	}
	for _, f := range qualified {
		if f.ObservedAt.Before(first) {
			first = f.ObservedAt
		}
	}
	result.user.FirstQualifyingFailureAt = &first
	var tier string
	var multiplierBPS int64
	var cap int64
	var snapshotID, paid90 int64
	err := tx.QueryRowContext(ctx, `SELECT id,tier,ROUND(multiplier*10000)::bigint,cap_cny_fen,verified_paid_value_cny_fen FROM customer_tier_incident_snapshots WHERE incident_id=$1 AND user_id=$2`, incidentID, userID).Scan(&snapshotID, &tier, &multiplierBPS, &cap, &paid90)
	if err == sql.ErrNoRows {
		result.user.Included = false
		result.user.ExclusionReason = "missing_tier_snapshot"
		result.user.EvidenceComplete = false
	} else if err != nil {
		return result, err
	} else {
		result.user.TierSnapshotID = &snapshotID
		result.user.Tier = CustomerTier(tier)
		result.user.TierCapCNYFen = cap
		result.user.VerifiedPaidValueCNYFen = paid90
	}
	if !eligible && result.user.ExclusionReason == "" {
		result.user.ExclusionReason = "fewer_than_four_final_failures"
		result.user.Included = false
	}
	var rollingMicros int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT SUM(original_amount_micros) FROM balance_lots WHERE user_id=$1 AND source_type='compensation' AND occurred_at>=$2 AND occurred_at<$3),0)::bigint+COALESCE((SELECT SUM(credit_limit_micros) FROM monthly_entitlement_cycles WHERE user_id=$1 AND source_type='compensation' AND starts_at>=$2 AND starts_at<$3),0)::bigint`, userID, build.impactEnd.Add(-time.Duration(build.policy.RollingWindowDays)*24*time.Hour), build.impactEnd).Scan(&rollingMicros); err != nil {
		return result, err
	}
	result.user.RollingGoodwillExecutedCNYFen = roundMicrosToFen(rollingMicros)
	type key struct {
		product  int64
		group    int64
		hasGroup bool
	}
	byKey := map[key][]compensationFailure{}
	for _, f := range failures {
		if f.QualificationReason == "missing_final_request_identity" {
			result.user.EvidenceComplete = false
		}
		k := key{product: f.ProductID}
		if f.GroupID.Valid {
			k.group = f.GroupID.Int64
			k.hasGroup = true
		}
		byKey[k] = append(byKey[k], f)
	}
	keys := make([]key, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].product != keys[j].product {
			return keys[i].product < keys[j].product
		}
		return keys[i].group < keys[j].group
	})
	for _, k := range keys {
		fs := byKey[k]
		qualifiedForItem := make([]compensationFailure, 0, len(fs))
		seenItemRequests := map[string]bool{}
		for _, f := range fs {
			if f.Qualified && !seenItemRequests[f.RequestID] {
				seenItemRequests[f.RequestID] = true
				qualifiedForItem = append(qualifiedForItem, f)
			}
		}
		firstForItem := fs[0].ObservedAt
		if len(qualifiedForItem) > 0 {
			firstForItem = qualifiedForItem[0].ObservedAt
		}
		item := CompensationDraftItem{UserID: userID, ProductID: k.product, FirstQualifyingFailureAt: firstForItem, TierMultiplierBPS: multiplierBPS, BenefitChannel: "balance"}
		for _, f := range fs {
			item.SourceObservationIDs = append(item.SourceObservationIDs, f.ObservationID)
			item.SourceFacts = append(item.SourceFacts, compensationEvidenceFact(f))
		}
		for _, f := range qualifiedForItem {
			if f.ObservedAt.Before(item.FirstQualifyingFailureAt) {
				item.FirstQualifyingFailureAt = f.ObservedAt
			}
		}
		if k.hasGroup {
			v := k.group
			item.GroupID = &v
		}
		if len(qualifiedForItem) > 0 {
			item.CompensableDurationMS, item.Segments, err = buildCompensationSegmentEvidence(k.product, segments[k.product], item.FirstQualifyingFailureAt, build.impactEnd)
			if err != nil {
				return result, err
			}
			for _, segment := range item.Segments {
				item.SegmentIDs = append(item.SegmentIDs, segment.SegmentID)
			}
		} else {
			item.ExclusionReason = "no_qualified_final_failure"
		}
		rateID, rate, err := loadProductCompensationRate(ctx, tx, k.product, build.impactStart)
		if err != nil {
			item.ExclusionReason = "missing_product_rate"
		} else {
			item.ProductRateVersionID = rateID
			item.ProductRateCNYFenPerHour = rate
		}
		var weightID any
		if !k.hasGroup {
			item.ExclusionReason = "missing_group_attribution"
			item.GroupWeightBPS = 10_000
		} else {
			weightIDValue, weight, channel, name, err := ensureGroupCompensationWeight(ctx, tx, k.group, build.impactStart)
			if err != nil {
				item.ExclusionReason = "missing_group_weight"
			} else {
				weightID = weightIDValue
				item.GroupWeightVersionID = &weightIDValue
				item.GroupWeightBPS = weight
				item.BenefitChannel = channel
				item.GroupName = name
			}
		}
		if item.CompensableDurationMS == 0 {
			item.ExclusionReason = "no_compensable_segment"
		}
		if item.ExclusionReason == "missing_product_rate" || item.ExclusionReason == "missing_group_weight" || item.ExclusionReason == "missing_group_attribution" || item.ExclusionReason == "no_compensable_segment" {
			result.user.EvidenceComplete = false
		}
		if !result.user.Included && item.ExclusionReason == "" {
			item.ExclusionReason = result.user.ExclusionReason
		}
		if item.ExclusionReason == "" {
			raw, err := CalculateCompensationRawMicros(item.CompensableDurationMS, item.ProductRateCNYFenPerHour, item.GroupWeightBPS, item.TierMultiplierBPS)
			if err != nil {
				return result, err
			}
			item.RawValueCNYMicros = raw
			result.user.RawValueCNYMicros, err = checkedAddCompensation(result.user.RawValueCNYMicros, raw)
			if err != nil {
				return result, err
			}
		}
		evidence, _ := json.Marshal(item)
		result.items = append(result.items, compensationItemBuild{item: item, rateID: item.ProductRateVersionID, weightID: weightID, evidence: evidence})
	}
	capResult := ApplyCompensationCaps(CompensationCapInput{RawValueMicros: result.user.RawValueCNYMicros, Tier: result.user.Tier, TierCapCNYFen: result.user.TierCapCNYFen, VerifiedPaidValueCNYFen: result.user.VerifiedPaidValueCNYFen, RollingExecutedCNYFen: result.user.RollingGoodwillExecutedCNYFen}, build.policy)
	if !result.user.Included {
		capResult.FinalCNYFen = 0
	}
	result.user.RelationshipCapCNYFen = capResult.RelationshipCapCNYFen
	result.user.RollingCapCNYFen = capResult.RollingCapCNYFen
	result.user.RollingRemainingCNYFen = capResult.RollingRemainingCNYFen
	result.user.LimitingCap = capResult.LimitingCap
	result.user.ProposedTotalCNYFen = capResult.FinalCNYFen
	raws := make([]int64, len(result.items))
	for i := range result.items {
		raws[i] = result.items[i].item.RawValueCNYMicros
	}
	allocated := AllocateCompensationFen(capResult.FinalCNYFen, raws)
	for i := range result.items {
		result.items[i].item.ProposedCNYFen = allocated[i]
		result.items[i].evidence, _ = json.Marshal(result.items[i].item)
		if result.items[i].item.BenefitChannel == "builder_pass" {
			result.user.BuilderPassBenefitCNYFen, err = checkedAddCompensation(result.user.BuilderPassBenefitCNYFen, allocated[i])
			if err != nil {
				return result, err
			}
		} else {
			result.user.BalanceBenefitCNYFen, err = checkedAddCompensation(result.user.BalanceBenefitCNYFen, allocated[i])
			if err != nil {
				return result, err
			}
		}
	}
	evidenceItems := make([]CompensationDraftItem, len(result.items))
	for i := range result.items {
		evidenceItems[i] = result.items[i].item
	}
	payload := compensationUserEvidence{User: result.user, Items: evidenceItems, Paid30: paid30, Policy: build.policy}
	result.evidence, _ = canonicalCompensationJSON(payload)
	hash := sha256.Sum256(result.evidence)
	result.user.EvidenceHash = hex.EncodeToString(hash[:])
	result.paid30 = paid30
	return result, nil
}

func loadProductCompensationRate(ctx context.Context, tx *sql.Tx, productID int64, at time.Time) (int64, int64, error) {
	var id, rate int64
	err := tx.QueryRowContext(ctx, `SELECT id,rate_cny_fen_per_hour FROM compensation_product_rate_versions WHERE product_id=$1 AND effective_from<=$2 ORDER BY effective_from DESC,version DESC LIMIT 1`, productID, at).Scan(&id, &rate)
	return id, rate, err
}
func ensureGroupCompensationWeight(ctx context.Context, tx *sql.Tx, groupID int64, at time.Time) (int64, int64, string, string, error) {
	var id, weight int64
	var name, channel string
	err := tx.QueryRowContext(ctx, `SELECT id,weight_bps,group_display_name,benefit_channel FROM compensation_group_weight_versions WHERE group_id=$1 AND effective_from<=$2 ORDER BY effective_from DESC,version DESC LIMIT 1`, groupID, at).Scan(&id, &weight, &name, &channel)
	if err != nil {
		return 0, 0, "", "", err
	}
	return id, weight, channel, name, nil
}

func loadCompensationPolicy(ctx context.Context, q customerTierQueryer, at time.Time) (CompensationPolicy, error) {
	var p CompensationPolicy
	err := q.QueryRowContext(ctx, `SELECT version,final_failure_threshold,customer_relationship_cap_bps,high_value_threshold_bps,rolling_window_days,standard_rolling_fixed_cap_cny_fen,standard_rolling_paid_value_bps,priority_rolling_fixed_cap_cny_fen,priority_rolling_paid_value_bps,shadow_minimum_days,shadow_minimum_incidents FROM compensation_policy_versions WHERE effective_from<=$1 ORDER BY effective_from DESC,version DESC LIMIT 1`, at).Scan(&p.Version, &p.FinalFailureThreshold, &p.CustomerRelationshipCapBPS, &p.HighValueThresholdBPS, &p.RollingWindowDays, &p.StandardRollingFixedCapCNYFen, &p.StandardRollingPaidValueBPS, &p.PriorityRollingFixedCapCNYFen, &p.PriorityRollingPaidValueBPS, &p.ShadowMinimumDays, &p.ShadowMinimumIncidents)
	if err != nil {
		return p, err
	}
	if !p.Valid() {
		return p, fmt.Errorf("compensation policy version %d is invalid", p.Version)
	}
	return p, nil
}

func loadCompensationPolicyVersion(ctx context.Context, q customerTierQueryer, version int) (CompensationPolicy, error) {
	var p CompensationPolicy
	err := q.QueryRowContext(ctx, `SELECT version,final_failure_threshold,customer_relationship_cap_bps,high_value_threshold_bps,rolling_window_days,standard_rolling_fixed_cap_cny_fen,standard_rolling_paid_value_bps,priority_rolling_fixed_cap_cny_fen,priority_rolling_paid_value_bps,shadow_minimum_days,shadow_minimum_incidents FROM compensation_policy_versions WHERE version=$1`, version).Scan(&p.Version, &p.FinalFailureThreshold, &p.CustomerRelationshipCapBPS, &p.HighValueThresholdBPS, &p.RollingWindowDays, &p.StandardRollingFixedCapCNYFen, &p.StandardRollingPaidValueBPS, &p.PriorityRollingFixedCapCNYFen, &p.PriorityRollingPaidValueBPS, &p.ShadowMinimumDays, &p.ShadowMinimumIncidents)
	if err != nil {
		return p, err
	}
	if !p.Valid() {
		return p, fmt.Errorf("compensation policy version %d is invalid", version)
	}
	return p, nil
}

func persistCompensationDraftTx(ctx context.Context, tx *sql.Tx, build compensationDraftBuild, series string, revision int, replaces *int64, reason string, actor int64, redesigned bool) (int64, error) {
	highThreshold := mulBPS(build.productRollingPaid, build.policy.HighValueThresholdBPS)
	high := IsHighValueCompensationDraft(build.total, build.productRollingPaid, build.policy.HighValueThresholdBPS)
	state := "shadow"
	if high {
		state = "redesign_required"
	}
	var actorValue, replacesValue any
	if actor > 0 {
		actorValue = actor
	}
	if replaces != nil {
		replacesValue = *replaces
	}
	var draftID int64
	err := tx.QueryRowContext(ctx, `INSERT INTO compensation_drafts(series_id,incident_id,shadow_period_id,policy_version,revision_number,replaces_draft_id,state,revision_reason,created_by_user_id,redesigned,eligible_user_count,affected_user_count,proposed_total_cny_fen,affected_product_rolling_paid_value_cny_fen,high_value_threshold_cny_fen,high_value,evidence_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`, series, build.incidentID, build.periodID, build.policy.Version, revision, replacesValue, state, reason, actorValue, redesigned, countEligible(build.users), len(build.users), build.total, build.productRollingPaid, highThreshold, high, build.hash).Scan(&draftID)
	if err != nil {
		return 0, err
	}
	for _, userBuild := range build.users {
		u := userBuild.user
		var tierSnapshot any
		if u.TierSnapshotID != nil {
			tierSnapshot = *u.TierSnapshotID
		}
		var first any
		if u.FirstQualifyingFailureAt != nil {
			first = *u.FirstQualifyingFailureAt
		}
		var rollingCap, rollingRemaining any
		if u.RollingCapCNYFen != nil {
			rollingCap = *u.RollingCapCNYFen
		}
		if u.RollingRemainingCNYFen != nil {
			rollingRemaining = *u.RollingRemainingCNYFen
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO compensation_draft_users(draft_id,user_id,tier_snapshot_id,eligible,included,evidence_complete,final_failure_count,first_qualifying_failure_at,verified_paid_value_cny_fen,rolling_goodwill_executed_cny_fen,raw_value_cny_micros,tier_cap_cny_fen,relationship_cap_cny_fen,rolling_cap_cny_fen,rolling_remaining_cny_fen,proposed_total_cny_fen,balance_benefit_cny_fen,builder_pass_benefit_cny_fen,limiting_cap,exclusion_reason,evidence,evidence_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`, draftID, u.UserID, tierSnapshot, u.Eligible, u.Included, u.EvidenceComplete, u.FinalFailureCount, first, u.VerifiedPaidValueCNYFen, u.RollingGoodwillExecutedCNYFen, u.RawValueCNYMicros, u.TierCapCNYFen, u.RelationshipCapCNYFen, rollingCap, rollingRemaining, u.ProposedTotalCNYFen, u.BalanceBenefitCNYFen, u.BuilderPassBenefitCNYFen, u.LimitingCap, u.ExclusionReason, userBuild.evidence, u.EvidenceHash)
		if err != nil {
			return 0, err
		}
		for _, itemBuild := range userBuild.items {
			item := itemBuild.item
			var group, rate any
			if item.GroupID != nil {
				group = *item.GroupID
			}
			if item.ProductRateVersionID > 0 {
				rate = item.ProductRateVersionID
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO compensation_draft_items(draft_id,user_id,product_id,group_id,benefit_channel,first_qualifying_failure_at,compensable_duration_ms,product_rate_version_id,group_weight_version_id,tier_multiplier_bps,raw_value_cny_micros,proposed_cny_fen,exclusion_reason,evidence) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, draftID, item.UserID, item.ProductID, group, item.BenefitChannel, item.FirstQualifyingFailureAt, item.CompensableDurationMS, rate, itemBuild.weightID, item.TierMultiplierBPS, item.RawValueCNYMicros, item.ProposedCNYFen, item.ExclusionReason, itemBuild.evidence)
			if err != nil {
				return 0, err
			}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO compensation_evidence_snapshots(draft_id,user_id,snapshot_kind,source_identifier_count,payload,payload_hash,retention_until) VALUES($1,$2,'user',$3,$4,$5,$6)`, draftID, u.UserID, countUserSources(userBuild), userBuild.evidence, u.EvidenceHash, time.Now().UTC().AddDate(3, 0, 1))
		if err != nil {
			return 0, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO compensation_evidence_snapshots(draft_id,user_id,snapshot_kind,source_identifier_count,payload,payload_hash,retention_until) VALUES($1,NULL,'draft',$2,$3,$4,$5)`, draftID, countDraftSources(build), build.payload, build.hash, time.Now().UTC().AddDate(3, 0, 1))
	if err != nil {
		return 0, err
	}
	return draftID, nil
}
func countEligible(users []compensationUserBuild) int {
	n := 0
	for _, u := range users {
		if u.user.Eligible {
			n++
		}
	}
	return n
}
func countUserSources(user compensationUserBuild) int {
	n := 0
	for _, item := range user.items {
		n += len(item.item.SourceObservationIDs) + len(item.item.SegmentIDs)
	}
	return n
}
func countDraftSources(build compensationDraftBuild) int {
	n := 0
	for _, u := range build.users {
		n += countUserSources(u)
	}
	return n
}

func canonicalCompensationJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}
