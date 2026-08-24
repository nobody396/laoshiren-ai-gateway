//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCompensationShadowDraftGoldenRevisionRetentionAndNoDelivery(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	impactStart := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Microsecond)
	impactEnd := impactStart.Add(2 * time.Hour)
	payg := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("comp-payg-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	excluded := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("comp-excluded-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	var paygGroup, builderGroup int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,1,'standard') RETURNING id`, `comp-payg-`+uuid.NewString()).Scan(&paygGroup))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,2,'credit') RETURNING id`, `comp-builder-`+uuid.NewString()).Scan(&builderGroup))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO compensation_group_weight_versions(group_id,version,weight_bps,source_rate_multiplier,benefit_channel,group_display_name,effective_from) VALUES($1,2,10000,1,'balance','Frozen Payg',$3),($2,2,20000,2,'builder_pass','Frozen Builder',$3)`, paygGroup, builderGroup, impactStart)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE groups SET rate_multiplier=9,subscription_type=CASE WHEN id=$1 THEN 'credit' ELSE 'standard' END,name=name||' mutated' WHERE id IN($1,$2)`, paygGroup, builderGroup)
	require.NoError(t, err)
	cleanupCompensationPrincipals(t, []int64{payg.ID, excluded.ID}, []int64{paygGroup, builderGroup})
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,30000,'alipay','completed',$4,$4,$4),($3,$5,10000,'alipay','completed',$4,$4,$4)`, `CP`+uuid.NewString()[:20], payg.ID, `CE`+uuid.NewString()[:20], impactStart.Add(-24*time.Hour), excluded.ID)
	require.NoError(t, err)
	var product1, product2 int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM service_status_products WHERE code='openai-codex-api'`).Scan(&product1))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM service_status_products WHERE code='builder-pass-gpt'`).Scan(&product2))
	var incidentID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,observation_ended_at,customer_impact_started_at,customer_impact_ended_at,monitoring_since,resolved_at) VALUES($1,'resolved','compensation golden incident',$2,$3,$2,$3,$3,$3) RETURNING id`, uuid.NewString(), impactStart, impactEnd).Scan(&incidentID))
	cleanupCustomerTierIncident(t, incidentID)
	var incidentProduct1, incidentProduct2 int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incident_products(incident_id,product_id,affected_at,current_status,recovered_at) VALUES($1,$2,$3,'monitoring',$4) RETURNING id`, incidentID, product1, impactStart, impactEnd).Scan(&incidentProduct1))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incident_products(incident_id,product_id,affected_at,current_status,recovered_at) VALUES($1,$2,$3,'monitoring',$4) RETURNING id`, incidentID, product2, impactStart.Add(30*time.Minute), impactEnd).Scan(&incidentProduct2))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO reliability_incident_impact_segments(incident_id,incident_product_id,started_at,ended_at,close_reason) VALUES($1,$2,$4,$5,'monitoring'),($1,$3,$6,$7,'monitoring')`, incidentID, incidentProduct1, incidentProduct2, impactStart, impactStart.Add(time.Hour), impactStart.Add(30*time.Minute), impactEnd)
	require.NoError(t, err)
	seedCompensationFailures(t, incidentID, payg.ID, product1, paygGroup, impactStart.Add(30*time.Minute), 2, "payg")
	seedCompensationFailures(t, incidentID, payg.ID, product2, builderGroup, impactStart.Add(time.Hour), 2, "builder")
	seedCompensationFailures(t, incidentID, excluded.ID, product1, paygGroup, impactStart.Add(15*time.Minute), 3, "excluded")
	seedCompensationExcludedAndDuplicateFacts(t, incidentID, payg.ID, product1, paygGroup, impactStart.Add(40*time.Minute))
	evidenceHash := hex.EncodeToString(make([]byte, 32))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO customer_tier_incident_snapshots(incident_id,user_id,policy_version,tier,multiplier,cap_cny_fen,verified_paid_value_cny_fen,verified_paid_consumption_micros,customer_impact_started_at,evidence,policy_snapshot,evidence_hash) VALUES($1,$2,1,'priority',1.25,5000,30000,0,$4,'{}','{}',$5),($1,$3,1,'standard',1,1000,10000,0,$4,'{}','{}',$5)`, incidentID, payg.ID, excluded.ID, impactStart, evidenceHash)
	require.NoError(t, err)
	settingRepo := NewSettingRepository(integrationEntClient)
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "true"))
	var periodID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at) VALUES($1,1,$2) RETURNING id`, uuid.New(), impactStart).Scan(&periodID))
	cleanupCompensationFixture(t, incidentID, []int64{paygGroup, builderGroup}, settingRepo)
	var beforeChanges, beforeLots, beforeCycles, beforeNotices int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_change_records`).Scan(&beforeChanges))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM balance_lots`).Scan(&beforeLots))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM monthly_entitlement_cycles`).Scan(&beforeCycles))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notifications`).Scan(&beforeNotices))
	tiers := service.NewCustomerTierService(integrationDB, settingRepo)
	compensation := service.NewCompensationControlService(integrationDB, settingRepo, tiers)
	draft, err := compensation.DraftIncident(ctx, incidentID, 42)
	require.NoError(t, err)
	require.Equal(t, 1, draft.RevisionNumber)
	require.Len(t, draft.Users, 2)
	require.Equal(t, int64(3000), draft.ProposedTotalCNYFen)
	var eligible, notEligible *service.CompensationDraftUser
	for i := range draft.Users {
		if draft.Users[i].UserID == payg.ID {
			eligible = &draft.Users[i]
		} else {
			notEligible = &draft.Users[i]
		}
	}
	require.NotNil(t, eligible)
	require.True(t, eligible.Eligible)
	require.Equal(t, 4, eligible.FinalFailureCount)
	require.Equal(t, int64(600), eligible.BalanceBenefitCNYFen)
	require.Equal(t, int64(2400), eligible.BuilderPassBenefitCNYFen)
	frozenChannels := map[string]string{}
	for _, item := range eligible.Items {
		frozenChannels[item.GroupName] = item.BenefitChannel
	}
	require.Equal(t, "balance", frozenChannels["Frozen Payg"])
	require.Equal(t, "builder_pass", frozenChannels["Frozen Builder"])
	require.Equal(t, "relationship", eligible.LimitingCap)
	reasons := map[string]bool{}
	for _, item := range eligible.Items {
		for _, fact := range item.SourceFacts {
			if !fact.Qualified {
				reasons[fact.QualificationReason] = true
			}
		}
	}
	for _, reason := range []string{"not_final_customer_request", "final_outcome_not_failure", "not_customer_impacting", "missing_final_request_identity"} {
		require.True(t, reasons[reason], reason)
	}
	require.NotNil(t, notEligible)
	require.False(t, notEligible.Eligible)
	require.Equal(t, "fewer_than_four_final_failures", notEligible.ExclusionReason)
	replay, err := compensation.DraftIncident(ctx, incidentID, 99)
	require.NoError(t, err)
	require.Equal(t, draft.ID, replay.ID)
	lockConn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = lockConn.Close() }()
	_, err = lockConn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext('compensation-draft'),$1)`, incidentID)
	require.NoError(t, err)
	revisionResult := make(chan *service.CompensationDraft, 1)
	revisionErrors := make(chan error, 1)
	go func() {
		item, revErr := compensation.ReviseDraft(ctx, service.CompensationRevisionCommand{DraftID: draft.ID, Reason: "owner-requested conservative redesign", ActorUserID: 42, Adjustments: []service.CompensationRevisionAdjustment{{UserID: payg.ID, Included: true, ProposedCNYFen: 2000}}})
		revisionResult <- item
		revisionErrors <- revErr
	}()
	time.Sleep(100 * time.Millisecond)
	reviewErrors := make(chan error, 1)
	go func() {
		_, reviewErr := compensation.ReviewShadowDraft(ctx, service.CompensationShadowReviewCommand{DraftID: draft.ID, OwnerJudgementTotalCNYFen: draft.ProposedTotalCNYFen, Result: "aligned", Notes: "must become stale behind revision", ActorUserID: 42})
		reviewErrors <- reviewErr
	}()
	time.Sleep(100 * time.Millisecond)
	_, err = lockConn.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext('compensation-draft'),$1)`, incidentID)
	require.NoError(t, err)
	revision := <-revisionResult
	require.NoError(t, <-revisionErrors)
	require.ErrorContains(t, <-reviewErrors, "latest draft revision")
	require.Equal(t, 2, revision.RevisionNumber)
	require.Equal(t, &draft.ID, revision.ReplacesDraftID)
	require.Equal(t, int64(2000), revision.ProposedTotalCNYFen)
	var revisionPayload []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload FROM compensation_evidence_snapshots WHERE draft_id=$1 AND user_id=$2 AND snapshot_kind='user'`, revision.ID, payg.ID).Scan(&revisionPayload))
	var revisionSnapshot service.CompensationUserEvidenceSnapshot
	require.NoError(t, json.Unmarshal(revisionPayload, &revisionSnapshot))
	require.Zero(t, revisionSnapshot.User.ID)
	require.Empty(t, revisionSnapshot.User.Email)
	require.Empty(t, revisionSnapshot.User.Items)
	var frozenRevisionTotal int64
	for _, item := range revisionSnapshot.Items {
		require.Zero(t, item.ID)
		frozenRevisionTotal += item.ProposedCNYFen
	}
	require.Equal(t, int64(2000), frozenRevisionTotal)
	reproducedRevision, err := service.ReproduceCompensationUserFromEvidence(revisionPayload)
	require.NoError(t, err)
	require.Equal(t, int64(2000), reproducedRevision.ProposedTotalCNYFen)
	var revisionPayg *service.CompensationDraftUser
	for i := range revision.Users {
		if revision.Users[i].UserID == payg.ID {
			revisionPayg = &revision.Users[i]
		}
	}
	require.NotNil(t, revisionPayg)
	require.Equal(t, revisionPayg.BalanceBenefitCNYFen, reproducedRevision.BalanceBenefitCNYFen)
	require.Equal(t, revisionPayg.BuilderPassBenefitCNYFen, reproducedRevision.BuilderPassBenefitCNYFen)
	review, err := compensation.ReviewShadowDraft(ctx, service.CompensationShadowReviewCommand{DraftID: revision.ID, OwnerJudgementTotalCNYFen: 1800, Result: "aligned", Notes: "compared against owner incident judgement", ActorUserID: 42})
	require.NoError(t, err)
	require.Equal(t, int64(200), review.VarianceCNYFen)
	assessment, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, assessment.ReviewedIncidentCount)
	require.False(t, assessment.ExecutionAvailable)
	require.False(t, assessment.EligibleForOwnerReview)
	excludedRevision, err := compensation.ReviseDraft(ctx, service.CompensationRevisionCommand{DraftID: revision.ID, Reason: "temporarily exclude while comparing owner judgement", ActorUserID: 42, Adjustments: []service.CompensationRevisionAdjustment{{UserID: payg.ID, Included: false, ProposedCNYFen: 0}}})
	require.NoError(t, err)
	var excludedUser *service.CompensationDraftUser
	for i := range excludedRevision.Users {
		if excludedRevision.Users[i].UserID == payg.ID {
			excludedUser = &excludedRevision.Users[i]
		}
	}
	require.NotNil(t, excludedUser)
	require.True(t, excludedUser.Eligible)
	require.False(t, excludedUser.Included)
	require.Equal(t, 1, excludedRevision.EligibleUserCount)
	reincludedRevision, err := compensation.ReviseDraft(ctx, service.CompensationRevisionCommand{DraftID: excludedRevision.ID, Reason: "restore inclusion after corrected comparison", ActorUserID: 42, Adjustments: []service.CompensationRevisionAdjustment{{UserID: payg.ID, Included: true, ProposedCNYFen: 1000}}})
	require.NoError(t, err)
	var reincludedUser *service.CompensationDraftUser
	for i := range reincludedRevision.Users {
		if reincludedRevision.Users[i].UserID == payg.ID {
			reincludedUser = &reincludedRevision.Users[i]
		}
	}
	require.NotNil(t, reincludedUser)
	require.True(t, reincludedUser.Eligible)
	require.True(t, reincludedUser.Included)
	latestAssessment, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.Zero(t, latestAssessment.ReviewedIncidentCount, "a review on an old revision cannot satisfy the latest-revision gate")
	var payload []byte
	var hash string
	var retention time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload,payload_hash,retention_until FROM compensation_evidence_snapshots WHERE draft_id=$1 AND snapshot_kind='draft'`, draft.ID).Scan(&payload, &hash, &retention))
	var decoded any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Contains(t, string(payload), `"user_id":`)
	require.Contains(t, string(payload), `"source_observation_ids":`)
	require.Contains(t, string(payload), `"segment_ids":`)
	var userPayload []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload FROM compensation_evidence_snapshots WHERE draft_id=$1 AND user_id=$2 AND snapshot_kind='user'`, draft.ID, payg.ID).Scan(&userPayload))
	reproduced, err := service.ReproduceCompensationUserFromEvidence(userPayload)
	require.NoError(t, err)
	require.True(t, reproduced.PolicyEligible)
	require.Equal(t, 4, reproduced.DistinctFinalFailures)
	require.Equal(t, eligible.RawValueCNYMicros, reproduced.RawValueCNYMicros)
	require.Equal(t, eligible.ProposedTotalCNYFen, reproduced.ProposedTotalCNYFen)
	require.Equal(t, eligible.BalanceBenefitCNYFen, reproduced.BalanceBenefitCNYFen)
	require.Equal(t, eligible.BuilderPassBenefitCNYFen, reproduced.BuilderPassBenefitCNYFen)
	canonicalPayload, err := json.Marshal(decoded)
	require.NoError(t, err)
	sum := sha256.Sum256(canonicalPayload)
	require.Equal(t, hex.EncodeToString(sum[:]), hash)
	require.True(t, retention.After(time.Now().UTC().AddDate(3, 0, 0)))
	_, err = integrationDB.ExecContext(ctx, `UPDATE compensation_drafts SET proposed_total_cny_fen=1 WHERE id=$1`, draft.ID)
	requirePostgresConstraint(t, err, "compensation_shadow_evidence_immutable")
	var afterChanges, afterLots, afterCycles, afterNotices int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_change_records`).Scan(&afterChanges))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM balance_lots`).Scan(&afterLots))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM monthly_entitlement_cycles`).Scan(&afterCycles))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notifications`).Scan(&afterNotices))
	require.Equal(t, beforeChanges, afterChanges)
	require.Equal(t, beforeLots, afterLots)
	require.Equal(t, beforeCycles, afterCycles)
	require.Equal(t, beforeNotices, afterNotices)
	_, err = integrationDB.ExecContext(ctx, `UPDATE compensation_shadow_periods SET ended_at=NOW() WHERE id=$1`, periodID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at) VALUES($1,1,$2)`, uuid.New(), time.Now().UTC().Add(-40*24*time.Hour))
	require.NoError(t, err)
	newPeriodAssessment, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.Zero(t, newPeriodAssessment.EvaluatedIncidentCount)
	require.Zero(t, newPeriodAssessment.ReviewedIncidentCount)
	require.False(t, newPeriodAssessment.EvidenceComplete)
	require.False(t, newPeriodAssessment.EligibleForOwnerReview)
}

func TestCompensationHighValueDraftRequiresRedesignBeforeReview(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	impactStart := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	impactEnd := impactStart.Add(time.Hour)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("comp-high-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	var groupID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,1,'standard') RETURNING id`, `comp-high-`+uuid.NewString()).Scan(&groupID))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO compensation_group_weight_versions(group_id,version,weight_bps,source_rate_multiplier,benefit_channel,group_display_name,effective_from) VALUES($1,2,10000,1,'balance','Frozen Group',$2)`, groupID, impactStart)
	require.NoError(t, err)
	cleanupCompensationPrincipals(t, []int64{user.ID}, []int64{groupID})
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,30000,'alipay','completed',$3,$3,$3)`, `CH`+uuid.NewString()[:20], user.ID, impactStart.Add(-60*24*time.Hour))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'compensation',$2,40000000,40000000,$4),($1,'compensation',$3,900000000,900000000,$5)`, user.ID, "rolling-in-"+uuid.NewString(), "rolling-out-"+uuid.NewString(), impactEnd.Add(-10*24*time.Hour), impactEnd.Add(-31*24*time.Hour))
	require.NoError(t, err)
	insideStart := impactEnd.Add(-10 * 24 * time.Hour)
	outsideStart := impactEnd.Add(-31 * 24 * time.Hour)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO monthly_entitlement_cycles(user_id,source_type,source_key,product_code,credit_limit_micros,starts_at,ends_at) VALUES($1,'compensation',$2,'builder-pass',40000000,$4,$5),($1,'compensation',$3,'builder-pass',900000000,$6,$7)`, user.ID, "rolling-cycle-in-"+uuid.NewString(), "rolling-cycle-out-"+uuid.NewString(), insideStart, insideStart.Add(31*24*time.Hour), outsideStart, outsideStart.Add(31*24*time.Hour))
	require.NoError(t, err)
	var productID, incidentID, incidentProductID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM service_status_products WHERE code='openai-codex-api'`).Scan(&productID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,observation_ended_at,customer_impact_started_at,customer_impact_ended_at,resolved_at) VALUES($1,'resolved','high value shadow',$2,$3,$2,$3,$3) RETURNING id`, uuid.NewString(), impactStart, impactEnd).Scan(&incidentID))
	cleanupCustomerTierIncident(t, incidentID)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incident_products(incident_id,product_id,affected_at,current_status,recovered_at) VALUES($1,$2,$3,'monitoring',$4) RETURNING id`, incidentID, productID, impactStart, impactEnd).Scan(&incidentProductID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO reliability_incident_impact_segments(incident_id,incident_product_id,started_at,ended_at,close_reason) VALUES($1,$2,$3,$4,'monitoring')`, incidentID, incidentProductID, impactStart, impactEnd)
	require.NoError(t, err)
	seedCompensationFailures(t, incidentID, user.ID, productID, groupID, impactStart, 4, "high")
	hash := hex.EncodeToString(make([]byte, 32))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO customer_tier_incident_snapshots(incident_id,user_id,policy_version,tier,multiplier,cap_cny_fen,verified_paid_value_cny_fen,verified_paid_consumption_micros,customer_impact_started_at,evidence,policy_snapshot,evidence_hash) VALUES($1,$2,1,'priority',1.25,5000,30000,0,$3,'{}','{}',$4)`, incidentID, user.ID, impactStart, hash)
	require.NoError(t, err)
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "true"))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at) VALUES($1,1,$2)`, uuid.New(), impactStart)
	require.NoError(t, err)
	cleanupCompensationFixture(t, incidentID, []int64{groupID}, settings)
	compensation := service.NewCompensationControlService(integrationDB, settings, service.NewCustomerTierService(integrationDB, settings))
	draft, err := compensation.DraftIncident(ctx, incidentID, 42)
	require.NoError(t, err)
	require.True(t, draft.HighValue)
	require.Equal(t, "redesign_required", draft.State)
	require.Equal(t, int64(8000), draft.Users[0].RollingGoodwillExecutedCNYFen)
	require.NotNil(t, draft.Users[0].RollingCapCNYFen)
	require.Equal(t, int64(9000), *draft.Users[0].RollingCapCNYFen)
	require.NotNil(t, draft.Users[0].RollingRemainingCNYFen)
	require.Equal(t, int64(1000), *draft.Users[0].RollingRemainingCNYFen)
	require.Equal(t, int64(1000), draft.Users[0].ProposedTotalCNYFen)
	_, err = compensation.ReviewShadowDraft(ctx, service.CompensationShadowReviewCommand{DraftID: draft.ID, OwnerJudgementTotalCNYFen: draft.ProposedTotalCNYFen, Result: "aligned", Notes: "must fail", ActorUserID: 42})
	require.ErrorContains(t, err, "must be redesigned")
	revision, err := compensation.ReviseDraft(ctx, service.CompensationRevisionCommand{DraftID: draft.ID, Reason: "redesign to zero after high-value gate", ActorUserID: 42, Adjustments: []service.CompensationRevisionAdjustment{{UserID: user.ID, Included: true, ProposedCNYFen: 0}}})
	require.NoError(t, err)
	require.False(t, revision.HighValue)
	require.True(t, revision.Redesigned)
	review, err := compensation.ReviewShadowDraft(ctx, service.CompensationShadowReviewCommand{DraftID: revision.ID, OwnerJudgementTotalCNYFen: 0, Result: "aligned", Notes: "redesigned below gate", ActorUserID: 42})
	require.NoError(t, err)
	require.Equal(t, "aligned", review.Result)
}

func TestCompensationMissingFrozenRatePersistsEvidenceGap(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	impactStart := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	impactEnd := impactStart.Add(time.Hour)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("comp-gap-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	var groupID, familyID, productID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,1,'standard') RETURNING id`, `comp-gap-`+uuid.NewString()).Scan(&groupID))
	cleanupCompensationPrincipals(t, []int64{user.ID}, []int64{groupID})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO compensation_group_weight_versions(group_id,version,weight_bps,source_rate_multiplier,benefit_channel,group_display_name,effective_from) VALUES($1,2,10000,1,'balance','Frozen Group',$2)`, groupID, impactStart)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM service_status_families WHERE code='other'`).Scan(&familyID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO service_status_products(family_id,code,display_name) VALUES($1,$2,'Missing Rate Product') RETURNING id`, familyID, "comp-gap-"+uuid.NewString()).Scan(&productID))
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(context.Background(), `DELETE FROM service_status_products WHERE id=$1`, productID)
		require.NoError(t, cleanupErr)
	})
	var incidentID, incidentProductID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,observation_ended_at,customer_impact_started_at,customer_impact_ended_at,resolved_at) VALUES($1,'resolved','missing rate gap',$2,$3,$2,$3,$3) RETURNING id`, uuid.NewString(), impactStart, impactEnd).Scan(&incidentID))
	cleanupCustomerTierIncident(t, incidentID)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incident_products(incident_id,product_id,affected_at,current_status,recovered_at) VALUES($1,$2,$3,'monitoring',$4) RETURNING id`, incidentID, productID, impactStart, impactEnd).Scan(&incidentProductID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO reliability_incident_impact_segments(incident_id,incident_product_id,started_at,ended_at,close_reason) VALUES($1,$2,$3,$4,'monitoring')`, incidentID, incidentProductID, impactStart, impactEnd)
	require.NoError(t, err)
	seedCompensationFailures(t, incidentID, user.ID, productID, groupID, impactStart, 4, "gap")
	hash := hex.EncodeToString(make([]byte, 32))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO customer_tier_incident_snapshots(incident_id,user_id,policy_version,tier,multiplier,cap_cny_fen,verified_paid_value_cny_fen,verified_paid_consumption_micros,customer_impact_started_at,evidence,policy_snapshot,evidence_hash) VALUES($1,$2,1,'standard',1,1000,10000,0,$3,'{}','{}',$4)`, incidentID, user.ID, impactStart, hash)
	require.NoError(t, err)
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "true"))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at) VALUES($1,1,$2)`, uuid.New(), impactStart)
	require.NoError(t, err)
	cleanupCompensationFixture(t, incidentID, []int64{groupID}, settings)
	compensation := service.NewCompensationControlService(integrationDB, settings, service.NewCustomerTierService(integrationDB, settings))
	draft, err := compensation.DraftIncident(ctx, incidentID, 42)
	require.NoError(t, err)
	require.Len(t, draft.Users, 1)
	require.False(t, draft.Users[0].EvidenceComplete)
	require.Zero(t, draft.Users[0].ProposedTotalCNYFen)
	require.Equal(t, "missing_product_rate", draft.Users[0].Items[0].ExclusionReason)
	require.Zero(t, draft.Users[0].Items[0].ProductRateVersionID)
	assessment, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.False(t, assessment.EvidenceComplete)
}

func TestCompensationShadowAssessmentRequiresCurrentPeriodLatestAlignedEvidence(t *testing.T) {
	ctx := context.Background()
	_ = testEntClient(t)
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "true"))
	var periodID int64
	started := time.Now().UTC().Add(-31 * 24 * time.Hour)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at) VALUES($1,1,$2) RETURNING id`, uuid.New(), started).Scan(&periodID))
	incidentIDs := []int64{}
	draftIDs := []int64{}
	create := func(index int) (int64, int64) {
		impact := started.Add(time.Duration(index+1) * time.Hour)
		var incidentID int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,observation_ended_at,customer_impact_started_at,customer_impact_ended_at,resolved_at) VALUES($1,'resolved','shadow gate',$2,$3,$2,$3,$3) RETURNING id`, uuid.NewString(), impact, impact.Add(time.Minute)).Scan(&incidentID))
		cleanupCustomerTierIncident(t, incidentID)
		series := uuid.New()
		hashBytes := sha256.Sum256([]byte(`{}`))
		hash := hex.EncodeToString(hashBytes[:])
		var draftID int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO compensation_drafts(series_id,incident_id,shadow_period_id,policy_version,revision_number,state,eligible_user_count,affected_user_count,proposed_total_cny_fen,affected_product_rolling_paid_value_cny_fen,high_value_threshold_cny_fen,high_value,evidence_hash) VALUES($1,$2,$3,1,1,'shadow',0,0,0,10000,1000,FALSE,$4) RETURNING id`, series, incidentID, periodID, hash).Scan(&draftID))
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO compensation_evidence_snapshots(draft_id,snapshot_kind,source_identifier_count,payload,payload_hash,retention_until) VALUES($1,'draft',0,'{}',$2,$3)`, draftID, hash, time.Now().UTC().AddDate(3, 0, 1))
		require.NoError(t, err)
		incidentIDs = append(incidentIDs, incidentID)
		draftIDs = append(draftIDs, draftID)
		return incidentID, draftID
	}
	compensation := service.NewCompensationControlService(integrationDB, settings, nil)
	for i := 0; i < 3; i++ {
		_, draftID := create(i)
		_, err := compensation.ReviewShadowDraft(ctx, service.CompensationShadowReviewCommand{DraftID: draftID, OwnerJudgementTotalCNYFen: 0, Result: "aligned", Notes: "aligned current-period review", ActorUserID: 42})
		require.NoError(t, err)
	}
	assessment, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, assessment.EvaluatedIncidentCount)
	require.Equal(t, 3, assessment.ReviewedIncidentCount)
	require.True(t, assessment.EvidenceComplete)
	require.True(t, assessment.EligibleForOwnerReview)
	fourthIncident, _ := create(3)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM compensation_shadow_reviews WHERE draft_id=$1`, draftIDs[3])
	if err != nil {
		requirePostgresConstraint(t, err, "compensation_shadow_evidence_immutable")
	}
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_draft_failures(shadow_period_id,incident_id,error_message,attempted_at) VALUES($1,$2,'temporary evidence failure',NOW())`, periodID, fourthIncident)
	require.NoError(t, err)
	blocked, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.False(t, blocked.EvidenceComplete)
	require.False(t, blocked.EligibleForOwnerReview)
	_, err = integrationDB.ExecContext(ctx, `UPDATE compensation_draft_failures SET resolved_at=NOW() WHERE shadow_period_id=$1 AND incident_id=$2`, periodID, fourthIncident)
	require.NoError(t, err)
	_, err = compensation.ReviewShadowDraft(ctx, service.CompensationShadowReviewCommand{DraftID: draftIDs[3], OwnerJudgementTotalCNYFen: 0, Result: "aligned", Notes: "recovered current-period review", ActorUserID: 42})
	require.NoError(t, err)
	recovered, err := compensation.Assessment(ctx)
	require.NoError(t, err)
	require.True(t, recovered.EvidenceComplete)
	require.True(t, recovered.EligibleForOwnerReview)
	t.Cleanup(func() {
		_ = settings.Set(context.Background(), service.SettingKeyCompensationShadowDraftEnabled, "false")
		tx, beginErr := integrationDB.BeginTx(context.Background(), nil)
		require.NoError(t, beginErr)
		defer func() { _ = tx.Rollback() }()
		for _, table := range []string{"compensation_shadow_reviews", "compensation_evidence_snapshots", "compensation_drafts"} {
			_, err := tx.ExecContext(context.Background(), fmt.Sprintf(`ALTER TABLE %s DISABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		_, err := tx.ExecContext(context.Background(), `DELETE FROM compensation_shadow_reviews WHERE draft_id=ANY($1)`, pq.Array(draftIDs))
		require.NoError(t, err)
		_, err = tx.ExecContext(context.Background(), `DELETE FROM compensation_evidence_snapshots WHERE draft_id=ANY($1)`, pq.Array(draftIDs))
		require.NoError(t, err)
		_, err = tx.ExecContext(context.Background(), `DELETE FROM compensation_drafts WHERE id=ANY($1)`, pq.Array(draftIDs))
		require.NoError(t, err)
		_, err = tx.ExecContext(context.Background(), `DELETE FROM compensation_shadow_periods WHERE id=$1`, periodID)
		require.NoError(t, err)
		for _, table := range []string{"compensation_shadow_reviews", "compensation_evidence_snapshots", "compensation_drafts"} {
			_, err := tx.ExecContext(context.Background(), fmt.Sprintf(`ALTER TABLE %s ENABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		require.NoError(t, tx.Commit())
	})
}

func seedCompensationFailures(t *testing.T, incidentID, userID, productID, groupID int64, first time.Time, count int, prefix string) {
	t.Helper()
	for i := 0; i < count; i++ {
		at := first.Add(time.Duration(i) * time.Minute)
		var observationID int64
		key := fmt.Sprintf("comp-%s-%s-%d", prefix, uuid.NewString(), i)
		require.NoError(t, integrationDB.QueryRowContext(context.Background(), `INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,request_id,user_id,group_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at) VALUES($1,'customer_request','compensation_test',$1,$1,$2,$3,'openai','golden-model','text','responses','failure',503,'provider',TRUE,$4) RETURNING id`, key, userID, groupID, at).Scan(&observationID))
		_, err := integrationDB.ExecContext(context.Background(), `INSERT INTO reliability_incident_observation_links(incident_id,observation_id,product_id,relation) VALUES($1,$2,$3,'customer_impact')`, incidentID, observationID, productID)
		require.NoError(t, err)
	}
}

func seedCompensationExcludedAndDuplicateFacts(t *testing.T, incidentID, userID, productID, groupID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	var duplicateRequest string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT o.request_id FROM reliability_incident_observation_links l JOIN reliability_observations o ON o.id=l.observation_id WHERE l.incident_id=$1 AND o.user_id=$2 AND l.product_id=$3 AND o.fact_type='customer_request' LIMIT 1`, incidentID, userID, productID).Scan(&duplicateRequest))
	fixtures := []struct {
		fact, outcome, owner string
		impact               bool
		request              string
	}{{"customer_request", "failure", "provider", true, duplicateRequest}, {"upstream_attempt", "failure", "provider", false, "attempt-" + uuid.NewString()}, {"active_probe", "failure", "provider", false, "probe-" + uuid.NewString()}, {"customer_request", "recovered", "provider", false, "recovered-" + uuid.NewString()}, {"customer_request", "excluded", "client", false, "client-" + uuid.NewString()}, {"customer_request", "failure", "business", false, "business-" + uuid.NewString()}, {"customer_request", "failure", "provider", true, ""}}
	for i, fixture := range fixtures {
		var observationID int64
		key := fmt.Sprintf("comp-exclusion-%s-%d", uuid.NewString(), i)
		status := 503
		if fixture.owner == "client" {
			status = 400
		}
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,request_id,user_id,group_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at) VALUES($1,$2,'compensation_exclusion_test',$1,$3,$4,$5,'openai','golden-model','text','responses',$6,$7,$8,$9,$10) RETURNING id`, key, fixture.fact, fixture.request, userID, groupID, fixture.outcome, status, fixture.owner, fixture.impact, at.Add(time.Duration(i)*time.Second)).Scan(&observationID))
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO reliability_incident_observation_links(incident_id,observation_id,product_id,relation) VALUES($1,$2,$3,'customer_impact')`, incidentID, observationID, productID)
		require.NoError(t, err)
	}
}

func cleanupCompensationFixture(t *testing.T, incidentID int64, groupIDs []int64, settings service.SettingRepository) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_ = settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "false")
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		for _, table := range []string{"compensation_shadow_reviews", "compensation_evidence_snapshots", "compensation_draft_items", "compensation_draft_users", "compensation_drafts", "compensation_group_weight_versions"} {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DISABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_evidence_snapshots WHERE draft_id IN(SELECT id FROM compensation_drafts WHERE incident_id=$1)`, incidentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_shadow_reviews WHERE draft_id IN(SELECT id FROM compensation_drafts WHERE incident_id=$1)`, incidentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_draft_items WHERE draft_id IN(SELECT id FROM compensation_drafts WHERE incident_id=$1)`, incidentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_draft_users WHERE draft_id IN(SELECT id FROM compensation_drafts WHERE incident_id=$1)`, incidentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_drafts WHERE incident_id=$1`, incidentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_group_weight_versions WHERE group_id=ANY($1)`, pq.Array(groupIDs))
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM compensation_shadow_periods`)
		require.NoError(t, err)
		for _, table := range []string{"compensation_shadow_reviews", "compensation_evidence_snapshots", "compensation_draft_items", "compensation_draft_users", "compensation_drafts", "compensation_group_weight_versions"} {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ENABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		require.NoError(t, tx.Commit())
	})
}

func cleanupCompensationPrincipals(t *testing.T, userIDs, groupIDs []int64) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM monthly_entitlement_cycles WHERE user_id=ANY($1)`, pq.Array(userIDs))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM balance_lots WHERE user_id=ANY($1)`, pq.Array(userIDs))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM topup_orders WHERE user_id=ANY($1)`, pq.Array(userIDs))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=ANY($1)`, pq.Array(userIDs))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=ANY($1)`, pq.Array(groupIDs))
		require.NoError(t, err)
	})
}
