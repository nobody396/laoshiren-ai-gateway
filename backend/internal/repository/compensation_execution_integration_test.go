//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCompensationExecutionApprovalPartialResumeExactReadbackAndNotice(t *testing.T) {
	ctx := service.ContextWithRBACActorSuperAdmin(context.Background(), true)
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("comp-exec-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	var paygGroup, builderGroup int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,1,'standard') RETURNING id`, `exec-payg-`+uuid.NewString()).Scan(&paygGroup))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,1,'credit') RETURNING id`, `exec-builder-`+uuid.NewString()).Scan(&builderGroup))
	var paygWeight, builderWeight, productID, rateID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM compensation_group_weight_versions WHERE group_id=$1 ORDER BY version DESC LIMIT 1`, paygGroup).Scan(&paygWeight))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM compensation_group_weight_versions WHERE group_id=$1 ORDER BY version DESC LIMIT 1`, builderGroup).Scan(&builderWeight))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM service_status_products WHERE code='openai-codex-api'`).Scan(&productID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM compensation_product_rate_versions WHERE product_id=$1 ORDER BY version DESC LIMIT 1`, productID).Scan(&rateID))
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "true"))
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationExecutionEnabled, "false"))
	var periodID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at) VALUES($1,1,$2) RETURNING id`, uuid.New(), now.Add(-31*24*time.Hour)).Scan(&periodID))
	hashBytes := sha256.Sum256([]byte(`{}`))
	hash := hex.EncodeToString(hashBytes[:])
	incidentIDs := []int64{}
	draftIDs := []int64{}
	createBase := func(index int, target bool) (int64, int64) {
		start := now.Add(time.Duration(index-4) * time.Hour)
		var incidentID int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,observation_ended_at,customer_impact_started_at,customer_impact_ended_at,resolved_at) VALUES($1,'resolved','execution gate incident',$2,$3,$2,$3,$3) RETURNING id`, uuid.NewString(), start, start.Add(30*time.Minute)).Scan(&incidentID))
		cleanupCustomerTierIncident(t, incidentID)
		var draftID int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO compensation_drafts(series_id,incident_id,shadow_period_id,policy_version,revision_number,state,eligible_user_count,affected_user_count,proposed_total_cny_fen,affected_product_rolling_paid_value_cny_fen,high_value_threshold_cny_fen,high_value,evidence_hash) VALUES($1,$2,$3,1,1,'shadow',$4,$4,$5,100000,10000,FALSE,$6) RETURNING id`, uuid.New(), incidentID, periodID, map[bool]int{true: 1, false: 0}[target], map[bool]int64{true: 3000, false: 0}[target], hash).Scan(&draftID))
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO compensation_evidence_snapshots(draft_id,snapshot_kind,source_identifier_count,payload,payload_hash,retention_until) VALUES($1,'draft',0,'{}',$2,$3)`, draftID, hash, now.AddDate(3, 0, 1))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_shadow_reviews(draft_id,owner_judgement_total_cny_fen,variance_cny_fen,result,notes,created_by_user_id) VALUES($1,$2,0,'aligned','owner aligned execution fixture',42)`, draftID, map[bool]int64{true: 3000, false: 0}[target])
		require.NoError(t, err)
		incidentIDs = append(incidentIDs, incidentID)
		draftIDs = append(draftIDs, draftID)
		return incidentID, draftID
	}
	_, _ = createBase(0, false)
	_, _ = createBase(1, false)
	targetIncident, targetDraft := createBase(2, true)
	var draftUserID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO compensation_draft_users(draft_id,user_id,eligible,included,evidence_complete,final_failure_count,verified_paid_value_cny_fen,rolling_goodwill_executed_cny_fen,raw_value_cny_micros,tier_cap_cny_fen,relationship_cap_cny_fen,proposed_total_cny_fen,balance_benefit_cny_fen,builder_pass_benefit_cny_fen,limiting_cap,evidence,evidence_hash) VALUES($1,$2,TRUE,TRUE,TRUE,4,30000,0,30000000,5000,3000,3000,600,2400,'relationship','{}',$3) RETURNING id`, targetDraft, user.ID, hash).Scan(&draftUserID))
	excludedChargedRequest, qualifiedRequest := uuid.NewString(), uuid.NewString()
	itemEvidence, err := json.Marshal(map[string]any{"source_facts": []map[string]any{
		{"request_id": excludedChargedRequest, "fact_type": "customer_request", "outcome": "success", "customer_impact": false, "error_owner": "provider", "qualified": false, "qualification_reason": "final_outcome_not_failure"},
		{"request_id": qualifiedRequest, "fact_type": "customer_request", "outcome": "failure", "customer_impact": true, "error_owner": "provider", "qualified": true, "qualification_reason": "qualified_final_failure"},
	}})
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_draft_items(draft_id,user_id,product_id,group_id,benefit_channel,first_qualifying_failure_at,compensable_duration_ms,product_rate_version_id,group_weight_version_id,tier_multiplier_bps,raw_value_cny_micros,proposed_cny_fen,evidence) VALUES($1,$2,$3,$4,'balance',$6,1800000,$7,$8,10000,6000000,600,$10),($1,$2,$3,$5,'builder_pass',$6,3600000,$7,$9,10000,24000000,2400,$10)`, targetDraft, user.ID, productID, paygGroup, builderGroup, now.Add(-time.Hour), rateID, paygWeight, builderWeight, itemEvidence)
	require.NoError(t, err)
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-compensation-" + uuid.NewString(), Name: "compensation notice facts"})
	account := mustCreateAccount(t, client, &service.Account{Name: "compensation-notice-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM accounts WHERE id=$1`, account.ID)
	})
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost) VALUES($1,$2,$3,$4,'excluded-recovered',1),($1,$2,$3,$5,'qualified-final',0)`, user.ID, apiKey.ID, account.ID, excludedChargedRequest, qualifiedRequest)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO compensation_evidence_snapshots(draft_id,user_id,snapshot_kind,source_identifier_count,payload,payload_hash,retention_until) VALUES($1,$2,'user',0,'{}',$3,$4)`, targetDraft, user.ID, hash, now.AddDate(3, 0, 1))
	require.NoError(t, err)
	cleanupCompensationExecutionFixture(t, periodID, draftIDs, incidentIDs, user.ID, []int64{paygGroup, builderGroup}, settings)
	draftControl := service.NewCompensationControlService(integrationDB, settings, nil)
	execution := service.NewCompensationExecutionService(integrationDB, settings, draftControl, nil)
	assessment, err := draftControl.Assessment(ctx)
	require.NoError(t, err)
	require.True(t, assessment.EligibleForOwnerReview)
	approvalPreview, err := execution.Preview(ctx, targetDraft)
	require.NoError(t, err)
	require.Len(t, approvalPreview.NoticePreviews, 1)
	require.False(t, approvalPreview.NoticePreviews[0].FailedRequestsCharged, "charged excluded/recovered facts must not be presented as a charged final failure")
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET actual_cost=1 WHERE user_id=$1 AND request_id=$2`, user.ID, qualifiedRequest)
	require.NoError(t, err)
	chargedPreview, err := execution.Preview(ctx, targetDraft)
	require.NoError(t, err)
	require.True(t, chargedPreview.NoticePreviews[0].FailedRequestsCharged, "a charged frozen qualified final failure must be surfaced")
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET actual_cost=0 WHERE user_id=$1 AND request_id=$2`, user.ID, qualifiedRequest)
	require.NoError(t, err)
	approvalPreview, err = execution.Preview(ctx, targetDraft)
	require.NoError(t, err)
	require.False(t, approvalPreview.NoticePreviews[0].FailedRequestsCharged)
	require.Len(t, approvalPreview.ApprovalPreviewHash, 64)
	_, err = execution.ApproveDraft(context.Background(), service.CompensationApprovalCommand{DraftID: targetDraft, ApprovalKey: "non-owner-" + uuid.NewString(), Reason: "must fail", Confirmation: service.CompensationDraftApprovalConfirmation, PreviewHash: approvalPreview.ApprovalPreviewHash, ActorUserID: 42})
	require.ErrorContains(t, err, "required")
	err = execution.UpdateEnabled(ctx, service.CompensationExecutionSettingsCommand{Enabled: true, Confirmation: "wrong", ActorUserID: 42})
	require.ErrorContains(t, err, "exact execution confirmation")
	_, err = execution.ApproveDraft(ctx, service.CompensationApprovalCommand{DraftID: targetDraft, ApprovalKey: "approval-" + uuid.NewString(), Reason: "explicit owner approved recovery", Confirmation: "wrong", PreviewHash: approvalPreview.ApprovalPreviewHash, ActorUserID: 42})
	require.Error(t, err)
	_, err = execution.ApproveDraft(ctx, service.CompensationApprovalCommand{DraftID: targetDraft, ApprovalKey: "stale-preview-" + uuid.NewString(), Reason: "explicit owner approved recovery", Confirmation: service.CompensationDraftApprovalConfirmation, PreviewHash: strings.Repeat("0", 64), ActorUserID: 42})
	require.ErrorContains(t, err, "preview changed")
	approval, err := execution.ApproveDraft(ctx, service.CompensationApprovalCommand{DraftID: targetDraft, ApprovalKey: "approval-" + uuid.NewString(), Reason: "explicit owner approved recovery", Confirmation: service.CompensationDraftApprovalConfirmation, PreviewHash: approvalPreview.ApprovalPreviewHash, ActorUserID: 42})
	require.NoError(t, err)
	require.Equal(t, targetDraft, approval.DraftID)
	_, err = draftControl.ReviseDraft(ctx, service.CompensationRevisionCommand{DraftID: targetDraft, Reason: "must not fork an approved series", ActorUserID: 42, Adjustments: []service.CompensationRevisionAdjustment{{UserID: user.ID, Included: true, ProposedCNYFen: 2999}}})
	require.ErrorContains(t, err, "approved compensation series")
	require.NoError(t, execution.UpdateEnabled(ctx, service.CompensationExecutionSettingsCommand{Enabled: true, Confirmation: service.CompensationExecutionEnableConfirmation, ActorUserID: 42}))
	permissionTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = permissionTx.Rollback() }()
	_, err = permissionTx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-execution-permission'))`)
	require.NoError(t, err)
	type executionResult struct {
		batch service.CompensationExecutionBatch
		err   error
	}
	executionDone := make(chan executionResult, 1)
	go func() {
		batch, executeErr := execution.ExecuteApprovedDraft(ctx, targetDraft)
		executionDone <- executionResult{batch: batch, err: executeErr}
	}()
	deadline := time.Now().Add(3 * time.Second)
	var executionWaiting bool
	for time.Now().Before(deadline) {
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query LIKE $1)`, `%compensation-execution-permission%`).Scan(&executionWaiting))
		if executionWaiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, executionWaiting, "execution must wait on the permission lock after its optimistic preview")
	_, err = permissionTx.ExecContext(ctx, `UPDATE settings SET value='false',updated_at=NOW() WHERE key=$1`, service.SettingKeyCompensationExecutionEnabled)
	require.NoError(t, err)
	_, err = permissionTx.ExecContext(ctx, `INSERT INTO compensation_execution_permission_audit(enabled,actor_user_id,confirmation_hash,assessment) VALUES(FALSE,42,$1,'{"eligible_for_owner_review":true}')`, strings.Repeat("0", 64))
	require.NoError(t, err)
	require.NoError(t, permissionTx.Commit())
	blockedExecution := <-executionDone
	require.ErrorContains(t, blockedExecution.err, "permission is disabled")
	var executionCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM compensation_executions WHERE approval_id=$1`, approval.ID).Scan(&executionCount))
	require.Zero(t, executionCount)
	require.NoError(t, execution.UpdateEnabled(ctx, service.CompensationExecutionSettingsCommand{Enabled: true, Confirmation: service.CompensationExecutionEnableConfirmation, ActorUserID: 42}))
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "false"))
	preview, err := execution.Preview(ctx, targetDraft)
	require.NoError(t, err)
	require.True(t, preview.Executable)
	require.Equal(t, int64(600), preview.BalanceCNYFen)
	require.Equal(t, int64(2400), preview.BuilderPassCNYFen)
	require.Len(t, preview.NoticePreviews, 1)
	require.Equal(t, user.ID, preview.NoticePreviews[0].UserID)
	require.Contains(t, preview.NoticePreviews[0].Body, "北京时间")
	expectedNotificationBody := preview.NoticePreviews[0].Body
	first, err := execution.ExecuteApprovedDraft(ctx, targetDraft)
	require.ErrorContains(t, err, "partially completed")
	require.Equal(t, 1, first.VerifiedCount)
	require.GreaterOrEqual(t, first.FailedCount, 1)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance::double precision FROM users WHERE id=$1`, user.ID).Scan(&balance))
	require.InDelta(t, 6, balance, 1e-9)
	var lotCount, noticeCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM balance_lots WHERE source_type='compensation' AND user_id=$1`, user.ID).Scan(&lotCount))
	require.Equal(t, 1, lotCount)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM compensation_notices WHERE incident_id=$1 AND user_id=$2`, targetIncident, user.ID).Scan(&noticeCount))
	require.Zero(t, noticeCount)
	require.NoError(t, execution.UpdateEnabled(ctx, service.CompensationExecutionSettingsCommand{Enabled: false, ActorUserID: 42}))
	expiresAt := now.Add(30 * 24 * time.Hour)
	var subscriptionID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,status,assigned_by) VALUES($1,$2,$3,$4,'active',$1) RETURNING id`, user.ID, builderGroup, now.Add(-time.Hour), expiresAt).Scan(&subscriptionID))
	_, err = execution.ExecuteApprovedDraft(ctx, targetDraft)
	require.ErrorContains(t, err, "blocked")
	var blockedCycleCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM monthly_entitlement_cycles WHERE source_type='compensation' AND user_id=$1`, user.ID).Scan(&blockedCycleCount))
	require.Zero(t, blockedCycleCount, "disabled permission must block a resumed channel")
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "true"))
	require.NoError(t, execution.UpdateEnabled(ctx, service.CompensationExecutionSettingsCommand{Enabled: true, Confirmation: service.CompensationExecutionEnableConfirmation, ActorUserID: 42}))
	require.NoError(t, settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "false"))
	second, err := execution.ExecuteApprovedDraft(ctx, targetDraft)
	if err != nil {
		rows, queryErr := integrationDB.QueryContext(ctx, `SELECT benefit_channel,state,last_error FROM compensation_executions WHERE approval_id=$1 ORDER BY id`, approval.ID)
		require.NoError(t, queryErr)
		var failures []string
		for rows.Next() {
			var channel, state, message string
			require.NoError(t, rows.Scan(&channel, &state, &message))
			failures = append(failures, fmt.Sprintf("%s=%s:%s", channel, state, message))
		}
		_ = rows.Close()
		t.Fatalf("second execution failed: %v (%v)", err, failures)
	}
	require.Equal(t, "verified", second.State)
	require.Equal(t, 2, second.VerifiedCount)
	var cycleID, cycleMicros int64
	var cycleEnd, subscriptionEnd time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT c.id,c.credit_limit_micros,c.ends_at,s.expires_at FROM monthly_entitlement_cycles c JOIN monthly_entitlement_cycle_subscriptions cs ON cs.cycle_id=c.id JOIN user_subscriptions s ON s.id=cs.user_subscription_id WHERE c.source_type='compensation' AND c.user_id=$1`, user.ID).Scan(&cycleID, &cycleMicros, &cycleEnd, &subscriptionEnd))
	require.Equal(t, int64(24_000_000), cycleMicros)
	require.True(t, cycleEnd.Equal(subscriptionEnd), "Builder Pass compensation must not extend validity")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM compensation_notices WHERE incident_id=$1 AND user_id=$2`, targetIncident, user.ID).Scan(&noticeCount))
	require.Equal(t, 1, noticeCount)
	var notificationBody string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT n.body FROM compensation_notices c JOIN user_notifications n ON n.id=c.notification_id WHERE c.incident_id=$1 AND c.user_id=$2`, targetIncident, user.ID).Scan(&notificationBody))
	require.NotContains(t, notificationBody, "group")
	require.NotContains(t, notificationBody, "account")
	require.Contains(t, notificationBody, "余额 ¥6.00")
	require.Contains(t, notificationBody, "Builder Pass ¥24.00")
	require.Equal(t, expectedNotificationBody, notificationBody, "the owner-approved preview must equal the delivered copy")
	var notificationID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT notification_id FROM compensation_notices WHERE incident_id=$1 AND user_id=$2`, targetIncident, user.ID).Scan(&notificationID))
	_, err = integrationDB.ExecContext(ctx, `UPDATE user_notifications SET body='tampered' WHERE id=$1`, notificationID)
	requirePostgresConstraint(t, err, "compensation_notification_immutable")
	_, err = integrationDB.ExecContext(ctx, `UPDATE user_notifications SET read_at=NOW() WHERE id=$1`, notificationID)
	require.NoError(t, err, "customers must still be able to mark an immutable notice as read")
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM user_notifications WHERE id=$1`, notificationID)
	requirePostgresConstraint(t, err, "compensation_notification_immutable")
	var ordinaryNotificationID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO user_notifications(user_id,type,title,body,dedupe_key) VALUES($1,'ordinary','ordinary','ordinary',$2) RETURNING id`, user.ID, "ordinary:"+uuid.NewString()).Scan(&ordinaryNotificationID))
	deletedOrdinary, err := integrationDB.ExecContext(ctx, `DELETE FROM user_notifications WHERE id=$1`, ordinaryNotificationID)
	require.NoError(t, err)
	deletedRows, err := deletedOrdinary.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), deletedRows, "the compensation trigger must not change ordinary notification lifecycle")
	third, err := execution.ExecuteApprovedDraft(ctx, targetDraft)
	require.NoError(t, err)
	require.Equal(t, 2, third.VerifiedCount)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM balance_lots WHERE source_type='compensation' AND user_id=$1`, user.ID).Scan(&lotCount))
	require.Equal(t, 1, lotCount)
	var cycleCount, notificationCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM monthly_entitlement_cycles WHERE source_type='compensation' AND user_id=$1`, user.ID).Scan(&cycleCount))
	require.Equal(t, 1, cycleCount)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notifications WHERE dedupe_key=$1`, fmt.Sprintf("compensation:%d:%d", targetIncident, user.ID)).Scan(&notificationCount))
	require.Equal(t, 1, notificationCount)
	var verifiedExecutionID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM compensation_executions WHERE draft_user_id=$1 AND benefit_channel='balance'`, draftUserID).Scan(&verifiedExecutionID))
	_, err = integrationDB.ExecContext(ctx, `UPDATE compensation_executions SET amount_cny_fen=1 WHERE id=$1`, verifiedExecutionID)
	requirePostgresConstraint(t, err, "verified_compensation_execution_immutable")
}

func cleanupCompensationExecutionFixture(t *testing.T, periodID int64, draftIDs, incidentIDs []int64, userID int64, groupIDs []int64, settings service.SettingRepository) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_ = settings.Set(ctx, service.SettingKeyCompensationExecutionEnabled, "false")
		_ = settings.Set(ctx, service.SettingKeyCompensationShadowDraftEnabled, "false")
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		tables := []string{"compensation_notices", "compensation_execution_receipts", "compensation_execution_events", "compensation_execution_assets", "compensation_executions", "compensation_approval_notices", "compensation_draft_approvals", "compensation_execution_permission_audit", "compensation_shadow_reviews", "compensation_evidence_snapshots", "compensation_draft_items", "compensation_draft_users", "compensation_drafts", "compensation_group_weight_versions"}
		for _, table := range tables {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DISABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_notices WHERE incident_id=ANY($1)`, pq.Array(incidentIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM user_notifications WHERE dedupe_key LIKE 'compensation:%' AND user_id=$1`, userID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_execution_receipts`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_execution_events`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_execution_assets`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_executions`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_approval_notices`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_draft_approvals`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_execution_permission_audit`)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_shadow_reviews WHERE draft_id=ANY($1)`, pq.Array(draftIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_evidence_snapshots WHERE draft_id=ANY($1)`, pq.Array(draftIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_draft_items WHERE draft_id=ANY($1)`, pq.Array(draftIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_draft_users WHERE draft_id=ANY($1)`, pq.Array(draftIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_drafts WHERE id=ANY($1)`, pq.Array(draftIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_shadow_periods WHERE id=$1`, periodID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_group_weight_versions WHERE group_id=ANY($1)`, pq.Array(groupIDs))
		for _, table := range tables {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ENABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		require.NoError(t, tx.Commit())
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM account_change_records WHERE user_id=$1 AND reason='compensation'`, userID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM monthly_entitlement_cycle_subscriptions WHERE cycle_id IN(SELECT id FROM monthly_entitlement_cycles WHERE user_id=$1 AND source_type='compensation')`, userID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM monthly_entitlement_cycles WHERE user_id=$1 AND source_type='compensation'`, userID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM user_subscriptions WHERE user_id=$1`, userID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM balance_lots WHERE user_id=$1 AND source_type='compensation'`, userID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
		result, err := integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=ANY($1)`, pq.Array(groupIDs))
		require.NoError(t, err)
		deleted, err := result.RowsAffected()
		require.NoError(t, err)
		require.Equal(t, int64(len(groupIDs)), deleted)
	})
}
