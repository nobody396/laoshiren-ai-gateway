//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCustomerTierExactPaidSourcesConsumptionGraceOverrideAndIncidentSnapshot(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	prefix := "tier-" + uuid.NewString()
	financeLinkedRedeemID := seedCustomerTierPaidEvidence(t, user.ID, prefix, now)
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))

	evaluation, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(27_000), evaluation.VerifiedPaidValueCNYFen)
	require.Equal(t, service.CustomerTierPriority, evaluation.EffectiveTier)
	require.Equal(t, int64(1_000_000), evaluation.VerifiedPaidConsumptionMicros)
	require.Equal(t, int64(400_000), evaluation.Evidence.BalancePaidConsumptionMicros)
	require.Equal(t, int64(600_000), evaluation.Evidence.BuilderPassConsumptionMicros)
	replayed, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, evaluation.ID, replayed.ID, "an exact evaluation retry must be idempotent")
	exclusions := map[string]bool{}
	var financeLinkedSource *service.CustomerTierEvidenceItem
	for _, source := range evaluation.Evidence.PaidSources {
		if source.SourceType == "redeem_code" && source.SourceID == financeLinkedRedeemID {
			value := source
			financeLinkedSource = &value
		}
		if !source.Included {
			exclusions[source.ExclusionReason] = true
		}
	}
	require.NotNil(t, financeLinkedSource)
	require.True(t, financeLinkedSource.Included)
	require.Equal(t, int64(2_000), financeLinkedSource.GrossAmountCNYFen, "exact linked order gross must replace unresolved redeem paid_value")
	for _, reason := range []string{"status_not_completed", "native_checkout_duplicate", "non_sale_purpose", "unresolved_paid_value"} {
		require.True(t, exclusions[reason], reason)
	}

	strategicUser := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-grace-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,100000,'alipay','completed',$3,$3,$3)`, "TG"+uuid.NewString()[:20], strategicUser.ID, now)
	require.NoError(t, err)
	first, err := tiers.EvaluateUserAt(ctx, strategicUser.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierStrategic, first.EffectiveTier)
	downgradeAt := now.Add(91 * 24 * time.Hour)
	downgrade, err := tiers.EvaluateUserAt(ctx, strategicUser.ID, downgradeAt)
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierStrategic, downgrade.EffectiveTier)
	require.NotNil(t, downgrade.GraceExpiresAt)
	afterGrace, err := tiers.EvaluateUserAt(ctx, strategicUser.ID, downgradeAt.Add(30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierStandard, afterGrace.EffectiveTier)

	overrideStart := downgradeAt.Add(31 * 24 * time.Hour)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO customer_tier_overrides(user_id,tier,reason,starts_at,expires_at,created_by_user_id) VALUES($1,'priority','contractual recovery tier',$2,$3,42)`, strategicUser.ID, overrideStart, overrideStart.Add(24*time.Hour))
	require.NoError(t, err)
	withOverride, err := tiers.EvaluateUserAt(ctx, strategicUser.ID, overrideStart.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierPriority, withOverride.EffectiveTier)
	afterOverride, err := tiers.EvaluateUserAt(ctx, strategicUser.ID, overrideStart.Add(25*time.Hour))
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierStandard, afterOverride.EffectiveTier)
	require.Equal(t, "override_expired", afterOverride.ResolutionReason)

	incidentID := seedCustomerTierIncident(t, user.ID, prefix, now.Add(30*time.Second))
	require.NoError(t, tiers.EnsureIncidentSnapshots(ctx, incidentID))
	var snapTier string
	var snapPaid, snapCap int64
	var snapMultiplier float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT tier,verified_paid_value_cny_fen,multiplier,cap_cny_fen FROM customer_tier_incident_snapshots WHERE incident_id=$1 AND user_id=$2`, incidentID, user.ID).Scan(&snapTier, &snapPaid, &snapMultiplier, &snapCap))
	require.Equal(t, "priority", snapTier)
	require.Equal(t, int64(27_000), snapPaid)
	require.InDelta(t, 1.25, snapMultiplier, 1e-12)
	require.Equal(t, int64(5_000), snapCap)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,100000,'alipay','completed',$3,$3,$3)`, "TS"+uuid.NewString()[:20], user.ID, now.Add(2*time.Minute))
	require.NoError(t, err)
	upgraded, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(3*time.Minute))
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierStrategic, upgraded.EffectiveTier)
	require.NoError(t, tiers.EnsureIncidentSnapshots(ctx, incidentID))
	var frozenTier string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT tier FROM customer_tier_incident_snapshots WHERE incident_id=$1 AND user_id=$2`, incidentID, user.ID).Scan(&frozenTier))
	require.Equal(t, "priority", frozenTier, "later payments cannot mutate an Incident tier snapshot")
	_, err = integrationDB.ExecContext(ctx, `UPDATE customer_tier_incident_snapshots SET tier='strategic' WHERE incident_id=$1 AND user_id=$2`, incidentID, user.ID)
	requirePostgresConstraint(t, err, "customer_tier_evidence_immutable")
	_, err = integrationDB.ExecContext(ctx, `UPDATE customer_tier_policy_versions SET rolling_window_days=91 WHERE version=1`)
	requirePostgresConstraint(t, err, "customer_tier_evidence_immutable")
}

func TestCustomerTierRefundsAreStructuredPartialFullAndIdempotent(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-refund-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	var sourceID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,30000,'alipay','completed',$3,$3,$3) RETURNING id`, "RF"+uuid.NewString()[:20], user.ID, now).Scan(&sourceID))
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))
	partial, err := tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "partial-" + uuid.NewString(), UserID: user.ID, SourceType: "topup_order", SourceID: sourceID, AmountCNYFen: 5_000, Reason: "partial customer refund", RefundedAt: now.Add(time.Minute)})
	require.NoError(t, err)
	duplicate, err := tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: partial.IdempotencyKey, UserID: user.ID, SourceType: "topup_order", SourceID: sourceID, AmountCNYFen: 5_000, Reason: "partial customer refund", RefundedAt: now.Add(time.Minute)})
	require.NoError(t, err)
	require.Equal(t, partial.ID, duplicate.ID)
	_, err = tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: partial.IdempotencyKey, UserID: user.ID, SourceType: "topup_order", SourceID: sourceID, AmountCNYFen: 5_000, Reason: "partial customer refund", RefundedAt: now.Add(2 * time.Minute)})
	require.ErrorContains(t, err, "different input")
	evaluation, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(25_000), evaluation.VerifiedPaidValueCNYFen)
	require.Equal(t, int64(30_000), evaluation.Evidence.PaidSources[0].GrossAmountCNYFen)
	require.Equal(t, int64(5_000), evaluation.Evidence.PaidSources[0].RefundedAmountCNYFen)
	_, err = tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "full-" + uuid.NewString(), UserID: user.ID, SourceType: "topup_order", SourceID: sourceID, AmountCNYFen: 25_000, Reason: "remaining customer refund", RefundedAt: now.Add(3 * time.Minute)})
	require.NoError(t, err)
	full, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(4*time.Minute))
	require.NoError(t, err)
	require.Zero(t, full.VerifiedPaidValueCNYFen)
	require.False(t, full.Evidence.PaidSources[0].Included)
	require.Equal(t, "fully_refunded", full.Evidence.PaidSources[0].ExclusionReason)
	_, err = tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "over-" + uuid.NewString(), UserID: user.ID, SourceType: "topup_order", SourceID: sourceID, AmountCNYFen: 1, Reason: "invalid over refund", RefundedAt: now.Add(5 * time.Minute)})
	require.ErrorContains(t, err, "exceeds")
	override, err := tiers.CreateOverride(ctx, service.CustomerTierOverrideCommand{UserID: user.ID, Tier: service.CustomerTierPriority, Reason: "verified immediate override", ExpiresAt: time.Now().UTC().Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, "applied", override.RefreshStatus)
	explanation, err := tiers.ExplainUser(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierPriority, explanation.Current.EffectiveTier)
	require.Equal(t, &override.ID, explanation.Current.OverrideID)
	nativeUser := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-native-refund-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	financeLinkedRedeemID := seedCustomerTierPaidEvidence(t, nativeUser.ID, "native-refund-"+uuid.NewString(), now)
	financePartial, err := tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "finance-partial-" + uuid.NewString(), UserID: nativeUser.ID, SourceType: "redeem_code", SourceID: financeLinkedRedeemID, AmountCNYFen: 1_000, Reason: "partial linked-order refund", RefundedAt: now.Add(time.Minute)})
	require.NoError(t, err)
	require.Equal(t, "redeem_code", financePartial.SourceType)
	require.Equal(t, financeLinkedRedeemID, financePartial.SourceID)
	var nativeRedeemID, nativeOrderID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT r.id,n.id FROM redeem_codes r JOIN native_checkout_orders n ON n.redeem_code_id=r.id WHERE r.used_by=$1 AND n.status='completed' AND n.redeem_purpose='sale_recharge'`, nativeUser.ID).Scan(&nativeRedeemID, &nativeOrderID))
	canonical, err := tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "native-" + uuid.NewString(), UserID: nativeUser.ID, SourceType: "redeem_code", SourceID: nativeRedeemID, AmountCNYFen: 7_000, Reason: "native checkout refund", RefundedAt: now.Add(time.Minute)})
	require.NoError(t, err)
	require.Equal(t, "native_checkout_order", canonical.SourceType)
	require.Equal(t, nativeOrderID, canonical.SourceID)
	nativeEvaluation, err := tiers.EvaluateUserAt(ctx, nativeUser.ID, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(19_000), nativeEvaluation.VerifiedPaidValueCNYFen)
	var linkedSource *service.CustomerTierEvidenceItem
	for _, source := range nativeEvaluation.Evidence.PaidSources {
		if source.SourceType == "redeem_code" && source.SourceID == financeLinkedRedeemID {
			value := source
			linkedSource = &value
		}
	}
	require.NotNil(t, linkedSource)
	require.Equal(t, int64(2_000), linkedSource.GrossAmountCNYFen)
	require.Equal(t, int64(1_000), linkedSource.RefundedAmountCNYFen)
	require.Equal(t, int64(1_000), linkedSource.AmountCNYFen)
	var pendingSourceID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM topup_orders WHERE user_id=$1 AND status='pending' LIMIT 1`, nativeUser.ID).Scan(&pendingSourceID))
	_, err = tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "pending-" + uuid.NewString(), UserID: nativeUser.ID, SourceType: "topup_order", SourceID: pendingSourceID, AmountCNYFen: 100, Reason: "must reject excluded source", RefundedAt: now.Add(time.Minute)})
	require.ErrorContains(t, err, "not canonical included")
}

func TestCustomerTierFinanceFallbackRejectsAmbiguousOrderToMultipleRedeemCodes(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-ambiguous-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	orderNo := "LD" + strings.ToUpper(uuid.NewString()[:20])
	var firstRedeemID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,paid_value,status,used_by,used_at,purpose,sales_status,sold_at,external_order_no) VALUES($1,'balance',10,0,'used',$3,$4,'sale_recharge','sold',$4,$5),($2,'balance',10,0,'unused',NULL,NULL,'sale_recharge','sold',$4,$5) RETURNING id`, uuid.NewString()[:24], uuid.NewString()[:24], user.ID, now, orderNo).Scan(&firstRedeemID))
	note := "tier-ambiguous-finance: 链动小铺订单 " + orderNo + "；链动小铺结算：毛额 ¥20.00，手续费 3% ¥0.60，实收 ¥19.40"
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO finance_transactions(type,category,amount_fen,occurred_at,note,source,payment_channel) VALUES('income','sale_revenue',1940,$1,$2,'skill','liandong_shop')`, now, note)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM finance_transactions WHERE note=$1`, note)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE external_order_no=$1`, orderNo)
	})
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))
	evaluation, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Zero(t, evaluation.VerifiedPaidValueCNYFen)
	require.Len(t, evaluation.Evidence.PaidSources, 1)
	for _, source := range evaluation.Evidence.PaidSources {
		require.False(t, source.Included)
		require.Equal(t, "ambiguous_paid_value", source.ExclusionReason)
	}
	_, err = tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "ambiguous-" + uuid.NewString(), UserID: user.ID, SourceType: "redeem_code", SourceID: firstRedeemID, AmountCNYFen: 100, Reason: "must reject ambiguous paid source", RefundedAt: now.Add(time.Minute)})
	require.ErrorContains(t, err, "not canonical included")

	crossUser := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-cross-window-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	crossOrderNo := "LD" + strings.ToUpper(uuid.NewString()[:20])
	var crossRedeemID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,paid_value,status,used_by,used_at,purpose,sales_status,sold_at,external_order_no) VALUES($1,'balance',20,0,'used',$2,$3,'sale_recharge','sold',$3,$4) RETURNING id`, uuid.NewString()[:24], crossUser.ID, now, crossOrderNo).Scan(&crossRedeemID))
	crossPrefix := "tier-cross-window-finance-" + uuid.NewString()
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO finance_transactions(type,category,amount_fen,occurred_at,note,source,payment_channel) VALUES('income','sale_revenue',1940,$1,$3,'skill','liandong_shop'),('income','sale_revenue',1940,$2,$4,'skill','liandong_shop')`, now, now.Add(-91*24*time.Hour), crossPrefix+":recent 订单 "+crossOrderNo+"；毛额 ¥20.00", crossPrefix+":old 订单 "+crossOrderNo+"；毛额 ¥20.00")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM finance_transactions WHERE note LIKE $1`, crossPrefix+"%")
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE external_order_no=$1`, crossOrderNo)
	})
	crossEvaluation, err := tiers.EvaluateUserAt(ctx, crossUser.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Zero(t, crossEvaluation.VerifiedPaidValueCNYFen)
	require.Len(t, crossEvaluation.Evidence.PaidSources, 1)
	require.Equal(t, "ambiguous_paid_value", crossEvaluation.Evidence.PaidSources[0].ExclusionReason, "finance uniqueness must be global even when one duplicate is outside the 90-day amount window")
	_, err = tiers.RecordPaidValueRefund(ctx, service.CustomerPaidValueRefundCommand{IdempotencyKey: "cross-window-" + uuid.NewString(), UserID: crossUser.ID, SourceType: "redeem_code", SourceID: crossRedeemID, AmountCNYFen: 100, Reason: "must reject globally ambiguous finance source", RefundedAt: now.Add(time.Minute)})
	require.ErrorContains(t, err, "not canonical included")
}

func TestCustomerTierEvidencePagesBeyondOneThousandSources(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-page-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	prefix := "PG" + uuid.NewString()[:8]
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) SELECT $1||lpad(n::text,8,'0'),$2,100,'alipay','completed',$3,$3,$3 FROM generate_series(1,1001) n`, prefix, user.ID, now)
	require.NoError(t, err)
	evaluation, err := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient)).EvaluateUserAt(ctx, user.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, evaluation.Evidence.PaidSources, 1001)
	require.Equal(t, int64(100_100), evaluation.VerifiedPaidValueCNYFen)
}

func TestCustomerTierIncidentSnapshotRejectsRetroactiveOverrideAndImpactMovement(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-causal-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,25000,'alipay','completed',$3,$3,$3)`, "CA"+uuid.NewString()[:20], user.ID, now.Add(-2*time.Hour))
	require.NoError(t, err)
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))
	impactAt := now.Add(-time.Hour)
	incidentID := seedCustomerTierIncident(t, user.ID, "causal-"+uuid.NewString(), impactAt)
	_, err = tiers.CreateOverride(ctx, service.CustomerTierOverrideCommand{UserID: user.ID, Tier: service.CustomerTierStrategic, Reason: "invalid retroactive override", StartsAt: impactAt.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)})
	require.ErrorContains(t, err, "cannot start in the past")
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO customer_tier_overrides(user_id,tier,reason,starts_at,expires_at,created_at) VALUES($1,'strategic','late imported override',$2,$3,$4)`, user.ID, impactAt.Add(-time.Minute), now.Add(time.Hour), now)
	require.NoError(t, err)
	require.NoError(t, tiers.EnsureIncidentSnapshots(ctx, incidentID))
	var tier string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT tier FROM customer_tier_incident_snapshots WHERE incident_id=$1 AND user_id=$2`, incidentID, user.ID).Scan(&tier))
	require.Equal(t, "priority", tier, "an override created after impact must not rewrite history")
	_, err = integrationDB.ExecContext(ctx, `UPDATE reliability_incidents SET customer_impact_started_at=$2 WHERE id=$1`, incidentID, impactAt.Add(-time.Minute))
	require.NoError(t, err)
	require.ErrorContains(t, tiers.EnsureIncidentSnapshots(ctx, incidentID), "impact window changed")
	_, err = integrationDB.ExecContext(ctx, `UPDATE reliability_incidents SET customer_impact_started_at=$2 WHERE id=$1`, incidentID, impactAt)
	require.NoError(t, err)
}

func TestCustomerTierIncidentSnapshotDrainsMoreThanOneThousandCustomers(t *testing.T) {
	ctx := context.Background()
	_ = testEntClient(t)
	impactAt := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	prefix := "bulk-" + uuid.NewString()[:8]
	rows, err := integrationDB.QueryContext(ctx, `INSERT INTO users(email,password_hash) SELECT $1||n||'@example.com','hash' FROM generate_series(1,1001) n RETURNING id`, prefix)
	require.NoError(t, err)
	ids := make([]int64, 0, 1001)
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Close())
	require.Len(t, ids, 1001)
	var incidentID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,customer_impact_started_at) VALUES($1,'investigating','bulk tier snapshot',$2,$2) RETURNING id`, uuid.NewString(), impactAt).Scan(&incidentID))
	cleanupCustomerTierIncident(t, incidentID)
	_, err = integrationDB.ExecContext(ctx, `WITH targets AS (SELECT unnest($1::bigint[]) user_id), observations AS (INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,user_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at) SELECT $2||user_id,'customer_request','tier_bulk',$2||user_id,user_id,'openai','bulk-model','text','responses','failure',503,'provider',TRUE,$3 FROM targets RETURNING id) INSERT INTO reliability_incident_observation_links(incident_id,observation_id,relation) SELECT $4,id,'customer_impact' FROM observations`, pq.Array(ids), prefix, impactAt, incidentID)
	require.NoError(t, err)
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))
	errs := make(chan error, 2)
	go func() { errs <- tiers.EnsureIncidentSnapshots(ctx, incidentID) }()
	go func() { errs <- tiers.EnsureIncidentSnapshots(ctx, incidentID) }()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_incident_snapshots WHERE incident_id=$1`, incidentID).Scan(&count))
	require.Equal(t, 1001, count)
}

func TestCustomerTierEvaluatorContinuesPastOneCustomerFailure(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Microsecond)
	before := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-eval-before-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	bad := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-eval-bad-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	after := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-eval-after-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$3,100,'alipay','completed',$5,$5,$5),($2,$4,100,'alipay','completed',$5,$5,$5)`, `EB`+uuid.NewString()[:20], `EA`+uuid.NewString()[:20], before.ID, after.ID, now.Add(-time.Minute))
	require.NoError(t, err)
	var badLot1, badLot2 int64
	maxMicros := int64(^uint64(0) >> 1)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'paid_topup',$2,$3,0,$4) RETURNING id`, bad.ID, "overflow-a-"+uuid.NewString(), maxMicros, now.Add(-time.Minute)).Scan(&badLot1))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'paid_topup',$2,$3,0,$4) RETURNING id`, bad.ID, "overflow-b-"+uuid.NewString(), maxMicros, now.Add(-time.Minute)).Scan(&badLot2))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO balance_lot_consumptions(balance_lot_id,user_id,usage_event_key,amount_micros,created_at) VALUES($1,$3,$4,$6,$5),($2,$3,$7,$6,$5)`, badLot1, badLot2, bad.ID, "overflow-use-a-"+uuid.NewString(), now.Add(-time.Minute), maxMicros, "overflow-use-b-"+uuid.NewString())
	require.NoError(t, err)
	repo := NewSettingRepository(integrationEntClient)
	require.NoError(t, repo.Set(ctx, service.SettingKeyCustomerTierEvaluationEnabled, "true"))
	tiers := service.NewCustomerTierService(integrationDB, repo)
	require.NoError(t, tiers.EvaluateAll(ctx, now))
	var afterCurrent int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_current WHERE user_id=$1`, after.ID).Scan(&afterCurrent))
	require.Equal(t, 1, afterCurrent)
	var failureCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_evaluation_failures WHERE user_id=$1 AND resolved_at IS NULL`, bad.ID).Scan(&failureCount))
	require.Equal(t, 1, failureCount)
	require.NoError(t, repo.Set(ctx, service.SettingKeyCustomerTierEvaluationEnabled, "false"))
}

func TestCustomerTierEvaluatorResumesFromDurableCheckpoint(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	cutoff := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Microsecond)
	before := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-resume-before-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	after := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-resume-after-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	runID := uuid.New()
	_, err := integrationDB.ExecContext(ctx, `UPDATE customer_tier_evaluator_state SET run_id=$1,cutoff_at=$2,last_user_id=$3,updated_at=NOW() WHERE singleton=TRUE`, runID, cutoff, before.ID)
	require.NoError(t, err)
	repo := NewSettingRepository(integrationEntClient)
	require.NoError(t, repo.Set(ctx, service.SettingKeyCustomerTierEvaluationEnabled, "true"))
	tiers := service.NewCustomerTierService(integrationDB, repo)
	require.NoError(t, tiers.EvaluateAll(ctx, cutoff.Add(time.Hour)))
	var beforeCount, afterCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_evaluations WHERE user_id=$1 AND evaluated_at=$2`, before.ID, cutoff).Scan(&beforeCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_evaluations WHERE user_id=$1 AND evaluated_at=$2`, after.ID, cutoff).Scan(&afterCount))
	require.Zero(t, beforeCount)
	require.Equal(t, 1, afterCount)
	var cleared bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT run_id IS NULL AND last_user_id=0 FROM customer_tier_evaluator_state WHERE singleton=TRUE`).Scan(&cleared))
	require.True(t, cleared)
	require.NoError(t, repo.Set(ctx, service.SettingKeyCustomerTierEvaluationEnabled, "false"))
}

func TestCustomerTierOverrideRefreshFailureIsDurableAndRetryable(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-override-retry-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	maxMicros := int64(^uint64(0) >> 1)
	var lot1, lot2 int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'paid_topup',$2,$3,0,$4) RETURNING id`, user.ID, "retry-a-"+uuid.NewString(), maxMicros, now).Scan(&lot1))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'paid_topup',$2,$3,0,$4) RETURNING id`, user.ID, "retry-b-"+uuid.NewString(), maxMicros, now).Scan(&lot2))
	var consumption2 int64
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO balance_lot_consumptions(balance_lot_id,user_id,usage_event_key,amount_micros,created_at) VALUES($1,$2,$3,$5,$4)`, lot1, user.ID, "retry-use-a-"+uuid.NewString(), now, maxMicros)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lot_consumptions(balance_lot_id,user_id,usage_event_key,amount_micros,created_at) VALUES($1,$2,$3,$5,$4) RETURNING id`, lot2, user.ID, "retry-use-b-"+uuid.NewString(), now, maxMicros).Scan(&consumption2))
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))
	override, err := tiers.CreateOverride(ctx, service.CustomerTierOverrideCommand{UserID: user.ID, Tier: service.CustomerTierPriority, Reason: "contractual tier", StartsAt: now, ExpiresAt: now.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, "failed", override.RefreshStatus)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM balance_lot_consumptions WHERE id=$1`, consumption2)
	require.NoError(t, err)
	require.NoError(t, tiers.RefreshExpiredOverrides(ctx, now.Add(time.Second)))
	var status, tier string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT r.status,c.effective_tier FROM customer_tier_override_refreshes r JOIN customer_tier_current c ON c.override_id=r.override_id WHERE r.override_id=$1`, override.ID).Scan(&status, &tier))
	require.Equal(t, "applied", status)
	require.Equal(t, "priority", tier)
}

func TestCustomerTierReadProjectsExpiredOverrideWithoutWrites(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("tier-expiry-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	tiers := service.NewCustomerTierService(integrationDB, NewSettingRepository(integrationEntClient))
	var overrideID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO customer_tier_overrides(user_id,tier,reason,starts_at,expires_at,created_at) VALUES($1,'strategic','short override',$2,$3,$4) RETURNING id`, user.ID, now.Add(-4*time.Second), now.Add(-time.Second), now.Add(-5*time.Second)).Scan(&overrideID))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO customer_tier_override_refreshes(override_id,status) VALUES($1,'applied')`, overrideID)
	require.NoError(t, err)
	active, err := tiers.EvaluateUserAt(ctx, user.ID, now.Add(-2*time.Second))
	require.NoError(t, err)
	require.Equal(t, service.CustomerTierStrategic, active.EffectiveTier)
	var evaluationsBefore, historyBefore int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_evaluations WHERE user_id=$1`, user.ID).Scan(&evaluationsBefore))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_history WHERE user_id=$1`, user.ID).Scan(&historyBefore))
	explanation, err := tiers.ExplainUser(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, explanation.Current.OverrideID)
	require.Equal(t, service.CustomerTierStandard, explanation.Current.EffectiveTier)
	require.Equal(t, "override_expired_pending_evaluation", explanation.Current.ProjectionStatus)
	admin, err := tiers.AdminSnapshot(ctx, service.AdminCustomerTierFilter{Page: 1, PageSize: 200, Search: user.Email})
	require.NoError(t, err)
	require.Len(t, admin.Customers, 1)
	require.Equal(t, "override_expired_pending_evaluation", admin.Customers[0].ProjectionStatus)
	var evaluationsAfter, historyAfter int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_evaluations WHERE user_id=$1`, user.ID).Scan(&evaluationsAfter))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM customer_tier_history WHERE user_id=$1`, user.ID).Scan(&historyAfter))
	require.Equal(t, evaluationsBefore, evaluationsAfter)
	require.Equal(t, historyBefore, historyAfter)
}

func seedCustomerTierPaidEvidence(t *testing.T, userID int64, prefix string, at time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM finance_transactions WHERE note LIKE $1`, prefix+"-finance-order:%")
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_orders WHERE user_id=$1`, userID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE used_by=$1`, userID)
	})
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO topup_orders(order_no,user_id,amount_cny_fen,pay_type,status,completed_at,created_at,updated_at) VALUES($1,$2,10000,'alipay','completed',$3,$3,$3),($4,$2,5000,'wechat','pending',NULL,$3,$3)`, "TT"+uuid.NewString()[:20], userID, at, "TP"+uuid.NewString()[:20])
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO payment_orders(order_no,user_id,amount_cents,status,completed_at,created_at,updated_at) VALUES($1,$2,5000,'completed',$3,$3,$3)`, "PM"+uuid.NewString()[:20], userID, at)
	require.NoError(t, err)
	offerCode := "offer-" + uuid.NewString()[:12]
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO native_checkout_offers(code,provider,provider_goods_key,name,product_kind,pay_amount_cny_fen,benefit_amount_cny_fen,redeem_type,redeem_value,redeem_paid_value,redeem_purpose,redeem_sales_status) VALUES($1,'ldxp',$2,'tier offer','balance',7000,7000,'balance',70,50,'sale_recharge','sold')`, offerCode, "goods-"+uuid.NewString()[:12])
	require.NoError(t, err)
	var nativeRedeemID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,paid_value,status,used_by,used_at,purpose,sales_status,sold_at) VALUES($1,'balance',50,50,'used',$2,$3,'sale_recharge','sold',$3) RETURNING id`, uuid.NewString()[:24], userID, at).Scan(&nativeRedeemID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO native_checkout_orders(order_no,user_id,offer_code,provider,provider_goods_key,contact_hash,product_kind,pay_amount_cny_fen,benefit_amount_cny_fen,redeem_type,redeem_value,redeem_paid_value,redeem_purpose,redeem_sales_status,status,redeem_code_id,completed_at,created_at,updated_at) VALUES($1,$2,$3,'ldxp',$4,$5,'balance',7000,7000,'balance',70,50,'sale_recharge','sold','completed',$6,$7,$7,$7)`, "NO"+uuid.NewString()[:20], userID, offerCode, "goods-order-"+uuid.NewString()[:8], strings.Repeat("a", 64), nativeRedeemID, at)
	require.NoError(t, err)
	for _, purpose := range []string{"gift", "compensation", "internal_test", "migration"} {
		excludedOffer := "offer-" + uuid.NewString()[:12]
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO native_checkout_offers(code,provider,provider_goods_key,name,product_kind,pay_amount_cny_fen,benefit_amount_cny_fen,redeem_type,redeem_value,redeem_paid_value,redeem_purpose,redeem_sales_status) VALUES($1,'ldxp',$2,$3,'balance',100,100,'balance',1,0,$4,'gifted')`, excludedOffer, "goods-"+uuid.NewString()[:12], purpose+" offer", purpose)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO native_checkout_orders(order_no,user_id,offer_code,provider,provider_goods_key,contact_hash,product_kind,pay_amount_cny_fen,benefit_amount_cny_fen,redeem_type,redeem_value,redeem_paid_value,redeem_purpose,redeem_sales_status,status,completed_at,created_at,updated_at) VALUES($1,$2,$3,'ldxp',$4,$5,'balance',100,100,'balance',1,0,$6,'gifted','completed',$7,$7,$7)`, `NX`+uuid.NewString()[:20], userID, excludedOffer, "goods-order-"+uuid.NewString()[:8], strings.Repeat("b", 64), purpose, at)
		require.NoError(t, err)
	}
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,paid_value,status,used_by,used_at,purpose,sales_status,sold_at) VALUES
($1,'balance',30,30,'used',$4,$5,'sale_recharge','sold',$5),
($2,'balance',10,10,'used',$4,$5,'gift','gifted',$5),
($3,'balance',20,0,'used',$4,$5,'sale_recharge','sold',$5)`, uuid.NewString()[:24], uuid.NewString()[:24], uuid.NewString()[:24], userID, at)
	require.NoError(t, err)
	financeOrderNo := "LD" + strings.ToUpper(uuid.NewString()[:20])
	var financeLinkedRedeemID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,paid_value,status,used_by,used_at,purpose,sales_status,sold_at,external_order_no) VALUES($1,'balance',20,0,'used',$2,$3,'sale_recharge','sold',$3,$4) RETURNING id`, uuid.NewString()[:24], userID, at, financeOrderNo).Scan(&financeLinkedRedeemID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO finance_transactions(type,category,amount_fen,occurred_at,note,source,payment_channel) VALUES('income','sale_revenue',1940,$1,$2,'skill','liandong_shop')`, at, prefix+"-finance-order: 链动小铺订单 "+financeOrderNo+"；链动小铺结算：毛额 ¥20.00，手续费 3% ¥0.60，实收 ¥19.40")
	require.NoError(t, err)
	var paidLot, giftLot int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'paid_topup',$2,1000000,600000,$3) RETURNING id`, userID, prefix+"-paid-lot", at).Scan(&paidLot))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_key,original_amount_micros,remaining_amount_micros,occurred_at) VALUES($1,'gift',$2,1000000,100000,$3) RETURNING id`, userID, prefix+"-gift-lot", at).Scan(&giftLot))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO balance_lot_consumptions(balance_lot_id,user_id,usage_event_key,amount_micros,created_at) VALUES($1,$3,$4,400000,$5),($2,$3,$6,900000,$5)`, paidLot, giftLot, userID, prefix+"-paid-use", at, prefix+"-gift-use")
	require.NoError(t, err)
	var paidCycleID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO monthly_entitlement_cycles(user_id,source_type,source_key,product_code,sale_price_micros,credit_limit_micros,used_credit_micros,confirmed_consumption_micros,starts_at,ends_at) VALUES($1,'paid_redeem',$2,'builder-pass',1000000,2000000,600000,600000,$3,$4) RETURNING id`, userID, prefix+"-paid-cycle", at, at.Add(31*24*time.Hour)).Scan(&paidCycleID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO monthly_entitlement_consumptions(cycle_id,user_id,usage_event_key,confirmed_amount_micros,created_at) VALUES($1,$2,$3,600000,$4)`, paidCycleID, userID, prefix+"-paid-cycle-use", at)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO monthly_entitlement_cycles(user_id,source_type,source_key,product_code,sale_price_micros,credit_limit_micros,used_credit_micros,confirmed_consumption_micros,starts_at,ends_at) VALUES($1,'gift',$2,'gift-pass',1000000,2000000,800000,800000,$3,$4)`, userID, prefix+"-gift-cycle", at, at.Add(31*24*time.Hour))
	require.NoError(t, err)
	return financeLinkedRedeemID
}

func seedCustomerTierIncident(t *testing.T, userID int64, prefix string, impactAt time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	var incidentID, observationID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,customer_impact_started_at) VALUES($1,'investigating','tier snapshot incident',$2,$2) RETURNING id`, uuid.NewString(), impactAt).Scan(&incidentID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,user_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at) VALUES($1,'customer_request','tier_integration',$1,$2,'openai','tier-model','text','responses','failure',503,'provider',TRUE,$3) RETURNING id`, prefix+"-incident-observation", userID, impactAt).Scan(&observationID))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO reliability_incident_observation_links(incident_id,observation_id,relation) VALUES($1,$2,'customer_impact')`, incidentID, observationID)
	require.NoError(t, err)
	cleanupCustomerTierIncident(t, incidentID)
	return incidentID
}

func cleanupCustomerTierIncident(t *testing.T, incidentID int64) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		_, err = tx.ExecContext(ctx, `ALTER TABLE customer_tier_incident_snapshots DISABLE TRIGGER trg_customer_tier_incident_snapshots_immutable`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM customer_tier_incident_snapshots WHERE incident_id=$1`, incidentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `ALTER TABLE customer_tier_incident_snapshots ENABLE TRIGGER trg_customer_tier_incident_snapshots_immutable`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM reliability_incidents WHERE id=$1`, incidentID)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	})
}
