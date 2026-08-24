package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

type compensationNoticeQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *CompensationExecutionService) applyBalanceExecutionTx(ctx context.Context, tx *sql.Tx, executionID, userID, amountFen int64) (map[string]any, error) {
	micros, err := fenToMicros(amountFen)
	if err != nil {
		return nil, err
	}
	var existingAssetID, lotID sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id,asset_id FROM compensation_execution_assets WHERE execution_id=$1 AND asset_type='balance_lot'`, executionID).Scan(&existingAssetID, &lotID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	var before, after string
	var ledgerID int64
	if existingAssetID.Valid {
		var lotMicros, ledgerFen int64
		if err = tx.QueryRowContext(ctx, `SELECT original_amount_micros FROM balance_lots WHERE id=$1`, lotID.Int64).Scan(&lotMicros); err != nil {
			return nil, err
		}
		if err = tx.QueryRowContext(ctx, `SELECT id,ROUND(delta*100)::bigint FROM account_change_records WHERE dedupe_key=$1`, `compensation-execution:`+fmt.Sprint(executionID)).Scan(&ledgerID, &ledgerFen); err != nil {
			return nil, err
		}
		if lotMicros != micros || ledgerFen != amountFen {
			return nil, fmt.Errorf("existing balance compensation asset failed exact readback")
		}
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(asset_before::text,''),COALESCE(asset_after::text,'') FROM compensation_executions WHERE id=$1`, executionID).Scan(&before, &after)
	} else {
		if err = tx.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&before); err != nil {
			return nil, err
		}
		if err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+($2::numeric/100),updated_at=NOW() WHERE id=$1 RETURNING balance::text`, userID, amountFen).Scan(&after); err != nil {
			return nil, err
		}
		if err = tx.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_id,source_key,original_amount_micros,remaining_amount_micros,affiliate_eligible,occurred_at) VALUES($1,'compensation',$2,$3,$4,$4,FALSE,NOW()) RETURNING id`, userID, executionID, `compensation-execution:`+fmt.Sprint(executionID), micros).Scan(&lotID); err != nil {
			return nil, err
		}
		if err = tx.QueryRowContext(ctx, `INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,source_id,reference_no,notes,operator_user_id,validity_days,dedupe_key) SELECT $1,'balance','compensation',$2::numeric/100,'compensation_execution',$3,$4,'Approved service recovery benefit',a.approved_by_user_id,0,$4 FROM compensation_executions e JOIN compensation_draft_approvals a ON a.id=e.approval_id WHERE e.id=$3 RETURNING id`, userID, amountFen, executionID, `compensation-execution:`+fmt.Sprint(executionID)).Scan(&ledgerID); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_execution_assets(execution_id,amount_cny_fen,asset_type,asset_id) VALUES($1,$2,'balance_lot',$3)`, executionID, amountFen, lotID.Int64); err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE compensation_executions SET asset_before=$2::numeric,asset_after=$3::numeric,state='applied',applied_at=NOW(),updated_at=NOW() WHERE id=$1`, executionID, before, after); err != nil {
			return nil, err
		}
	}
	var appliedFen int64
	if err = tx.QueryRowContext(ctx, `SELECT ROUND(($1::numeric-$2::numeric)*100)::bigint`, after, before).Scan(&appliedFen); err != nil {
		return nil, err
	}
	if appliedFen != amountFen {
		return nil, fmt.Errorf("balance compensation failed exact delta readback")
	}
	sourceKey := "compensation-execution:" + fmt.Sprint(executionID)
	var assetExecutionID, assetAmountFen, assetID, lotUserID, lotSourceID, lotOriginal, lotRemaining, readLedgerID, ledgerUserID, ledgerSourceID, ledgerFen, ledgerOperatorID, approvedBy int64
	var assetGroupID sql.NullInt64
	var assetExpiry sql.NullTime
	var assetType, lotSourceType, lotSourceKey, ledgerAssetType, ledgerReason, ledgerSourceType, ledgerReference, ledgerNotes, ledgerDedupe string
	var lotAffiliateEligible bool
	err = tx.QueryRowContext(ctx, `SELECT a.execution_id,a.group_id,a.amount_cny_fen,a.asset_type,a.asset_id,a.benefit_expires_at,l.user_id,l.source_type,l.source_id,l.source_key,l.original_amount_micros,l.remaining_amount_micros,l.affiliate_eligible,r.id,r.user_id,r.asset_type,r.reason,ROUND(r.delta*100)::bigint,r.source_type,r.source_id,r.reference_no,r.notes,r.operator_user_id,r.dedupe_key,approval.approved_by_user_id FROM compensation_execution_assets a JOIN balance_lots l ON l.id=a.asset_id JOIN compensation_executions e ON e.id=a.execution_id JOIN compensation_draft_approvals approval ON approval.id=e.approval_id JOIN account_change_records r ON r.dedupe_key=$2 WHERE a.execution_id=$1 AND a.asset_type='balance_lot'`, executionID, sourceKey).Scan(&assetExecutionID, &assetGroupID, &assetAmountFen, &assetType, &assetID, &assetExpiry, &lotUserID, &lotSourceType, &lotSourceID, &lotSourceKey, &lotOriginal, &lotRemaining, &lotAffiliateEligible, &readLedgerID, &ledgerUserID, &ledgerAssetType, &ledgerReason, &ledgerFen, &ledgerSourceType, &ledgerSourceID, &ledgerReference, &ledgerNotes, &ledgerOperatorID, &ledgerDedupe, &approvedBy)
	if err != nil {
		return nil, err
	}
	if assetExecutionID != executionID || assetGroupID.Valid || assetAmountFen != amountFen || assetType != "balance_lot" || assetID != lotID.Int64 || assetExpiry.Valid || lotUserID != userID || lotSourceType != "compensation" || lotSourceID != executionID || lotSourceKey != sourceKey || lotOriginal != micros || lotRemaining != micros || lotAffiliateEligible || readLedgerID != ledgerID || ledgerUserID != userID || ledgerAssetType != "balance" || ledgerReason != "compensation" || ledgerFen != amountFen || ledgerSourceType != "compensation_execution" || ledgerSourceID != executionID || ledgerReference != sourceKey || ledgerNotes != "Approved service recovery benefit" || ledgerOperatorID != approvedBy || ledgerDedupe != sourceKey {
		return nil, fmt.Errorf("balance compensation asset or ledger failed exact readback")
	}
	return map[string]any{"channel": "balance", "amount_cny_fen": amountFen, "balance_lot_id": lotID.Int64, "account_change_record_id": ledgerID, "balance_before": before, "balance_after": after}, nil
}

func (s *CompensationExecutionService) applyBuilderPassExecutionTx(ctx context.Context, tx *sql.Tx, executionID, userID, amountFen int64) (map[string]any, error) {
	rows, err := tx.QueryContext(ctx, `SELECT item.group_id,SUM(item.proposed_cny_fen)::bigint FROM compensation_executions e JOIN compensation_draft_items item ON item.draft_id=(SELECT draft_id FROM compensation_draft_users WHERE id=e.draft_user_id) AND item.user_id=e.user_id WHERE e.id=$1 AND item.benefit_channel='builder_pass' AND item.group_id IS NOT NULL AND item.proposed_cny_fen>0 GROUP BY item.group_id ORDER BY item.group_id`, executionID)
	if err != nil {
		return nil, err
	}
	allocations := map[int64]int64{}
	var total int64
	for rows.Next() {
		var groupID, fen int64
		if err := rows.Scan(&groupID, &fen); err != nil {
			_ = rows.Close()
			return nil, err
		}
		allocations[groupID] = fen
		total, err = checkedAddCompensation(total, fen)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if total != amountFen || len(allocations) == 0 {
		return nil, fmt.Errorf("builder pass allocation does not match approved total")
	}
	groupIDs := make([]int64, 0, len(allocations))
	for groupID := range allocations {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })
	assets := []map[string]any{}
	for _, groupID := range groupIDs {
		fen := allocations[groupID]
		sourceKey := fmt.Sprintf("compensation-execution:%d:group:%d", executionID, groupID)
		micros, err := fenToMicros(fen)
		if err != nil {
			return nil, err
		}
		var existingAssetID, cycleID sql.NullInt64
		var existingExpiry sql.NullTime
		err = tx.QueryRowContext(ctx, `SELECT id,asset_id,benefit_expires_at FROM compensation_execution_assets WHERE execution_id=$1 AND group_id=$2`, executionID, groupID).Scan(&existingAssetID, &cycleID, &existingExpiry)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		var subscriptionID int64
		var expiresAt time.Time
		if existingAssetID.Valid {
			var cycleMicros int64
			if err = tx.QueryRowContext(ctx, `SELECT credit_limit_micros,ends_at FROM monthly_entitlement_cycles WHERE id=$1`, cycleID.Int64).Scan(&cycleMicros, &expiresAt); err != nil {
				return nil, err
			}
			if cycleMicros != micros || !existingExpiry.Valid || !existingExpiry.Time.Equal(expiresAt) {
				return nil, fmt.Errorf("existing Builder Pass compensation asset failed exact readback")
			}
			var linkedGroupID int64
			if err = tx.QueryRowContext(ctx, `SELECT user_subscription_id,group_id FROM monthly_entitlement_cycle_subscriptions WHERE cycle_id=$1 LIMIT 1`, cycleID.Int64).Scan(&subscriptionID, &linkedGroupID); err != nil {
				return nil, err
			}
			if linkedGroupID != groupID {
				return nil, fmt.Errorf("existing Builder Pass compensation group failed exact readback")
			}
		} else {
			if err = tx.QueryRowContext(ctx, `SELECT id,expires_at FROM user_subscriptions WHERE user_id=$1 AND group_id=$2 AND status='active' AND expires_at>NOW() AND deleted_at IS NULL ORDER BY expires_at DESC,id DESC LIMIT 1 FOR UPDATE`, userID, groupID).Scan(&subscriptionID, &expiresAt); err != nil {
				return nil, fmt.Errorf("active Builder Pass entitlement is required for group %d: %w", groupID, err)
			}
			if err = tx.QueryRowContext(ctx, `INSERT INTO monthly_entitlement_cycles(user_id,source_type,source_id,source_key,product_code,sale_price_micros,credit_limit_micros,used_credit_micros,confirmed_consumption_micros,affiliate_eligible,starts_at,ends_at) VALUES($1,'compensation',$2,$3,'compensation-builder-pass',0,$4,0,0,FALSE,NOW(),$5) RETURNING id`, userID, executionID, sourceKey, micros, expiresAt).Scan(&cycleID); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO monthly_entitlement_cycle_subscriptions(cycle_id,user_subscription_id,group_id) VALUES($1,$2,$3)`, cycleID.Int64, subscriptionID, groupID); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,source_id,reference_no,notes,operator_user_id,group_id,validity_days,dedupe_key) SELECT $1,'builder_pass_credit','compensation',$2::numeric/100,'compensation_execution',$3,$4,'Approved service recovery Builder Pass credit',a.approved_by_user_id,$5,0,$4 FROM compensation_executions e JOIN compensation_draft_approvals a ON a.id=e.approval_id WHERE e.id=$3`, userID, fen, executionID, sourceKey, groupID); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_execution_assets(execution_id,group_id,amount_cny_fen,asset_type,asset_id,benefit_expires_at) VALUES($1,$2,$3,'monthly_entitlement_cycle',$4,$5)`, executionID, groupID, fen, cycleID.Int64, expiresAt); err != nil {
				return nil, err
			}
		}
		var ledgerID, ledgerFen int64
		if err = tx.QueryRowContext(ctx, `SELECT id,ROUND(delta*100)::bigint FROM account_change_records WHERE dedupe_key=$1`, sourceKey).Scan(&ledgerID, &ledgerFen); err != nil {
			return nil, err
		}
		if ledgerFen != fen {
			return nil, fmt.Errorf("builder pass compensation ledger failed exact readback")
		}
		var assetExecutionID, assetGroupID, assetAmountFen, assetID, cycleUserID, cycleSourceID, saleMicros, creditMicros, usedMicros, confirmedMicros, linkCycleID, linkSubscriptionID, linkGroupID, linkCount, subscriptionUserID, subscriptionGroupID, readLedgerID, ledgerUserID, ledgerSourceID, ledgerOperatorID, ledgerGroupID, ledgerValidityDays, approvedBy int64
		var assetType, cycleSourceType, cycleSourceKey, productCode, subscriptionStatus, ledgerAssetType, ledgerReason, ledgerSourceType, ledgerReference, ledgerNotes, ledgerDedupe string
		var assetExpiry, cycleEndsAt, subscriptionExpiresAt time.Time
		var subscriptionDeletedAt sql.NullTime
		var affiliateEligible bool
		err = tx.QueryRowContext(ctx, `SELECT a.execution_id,a.group_id,a.amount_cny_fen,a.asset_type,a.asset_id,a.benefit_expires_at,c.user_id,c.source_type,c.source_id,c.source_key,c.product_code,c.sale_price_micros,c.credit_limit_micros,c.used_credit_micros,c.confirmed_consumption_micros,c.affiliate_eligible,c.ends_at,link.cycle_id,link.user_subscription_id,link.group_id,(SELECT COUNT(*) FROM monthly_entitlement_cycle_subscriptions links WHERE links.cycle_id=c.id),subscription.user_id,subscription.group_id,subscription.status,subscription.expires_at,subscription.deleted_at,ledger.id,ledger.user_id,ledger.asset_type,ledger.reason,ROUND(ledger.delta*100)::bigint,ledger.source_type,ledger.source_id,ledger.reference_no,ledger.notes,ledger.operator_user_id,ledger.group_id,ledger.validity_days,ledger.dedupe_key,approval.approved_by_user_id FROM compensation_execution_assets a JOIN monthly_entitlement_cycles c ON c.id=a.asset_id JOIN monthly_entitlement_cycle_subscriptions link ON link.cycle_id=c.id JOIN user_subscriptions subscription ON subscription.id=link.user_subscription_id JOIN compensation_executions execution ON execution.id=a.execution_id JOIN compensation_draft_approvals approval ON approval.id=execution.approval_id JOIN account_change_records ledger ON ledger.dedupe_key=$3 WHERE a.execution_id=$1 AND a.group_id=$2 AND a.asset_type='monthly_entitlement_cycle'`, executionID, groupID, sourceKey).Scan(&assetExecutionID, &assetGroupID, &assetAmountFen, &assetType, &assetID, &assetExpiry, &cycleUserID, &cycleSourceType, &cycleSourceID, &cycleSourceKey, &productCode, &saleMicros, &creditMicros, &usedMicros, &confirmedMicros, &affiliateEligible, &cycleEndsAt, &linkCycleID, &linkSubscriptionID, &linkGroupID, &linkCount, &subscriptionUserID, &subscriptionGroupID, &subscriptionStatus, &subscriptionExpiresAt, &subscriptionDeletedAt, &readLedgerID, &ledgerUserID, &ledgerAssetType, &ledgerReason, &ledgerFen, &ledgerSourceType, &ledgerSourceID, &ledgerReference, &ledgerNotes, &ledgerOperatorID, &ledgerGroupID, &ledgerValidityDays, &ledgerDedupe, &approvedBy)
		if err != nil {
			return nil, err
		}
		if assetExecutionID != executionID || assetGroupID != groupID || assetAmountFen != fen || assetType != "monthly_entitlement_cycle" || assetID != cycleID.Int64 || !assetExpiry.Equal(expiresAt) || cycleUserID != userID || cycleSourceType != "compensation" || cycleSourceID != executionID || cycleSourceKey != sourceKey || productCode != "compensation-builder-pass" || saleMicros != 0 || creditMicros != micros || usedMicros != 0 || confirmedMicros != 0 || affiliateEligible || !cycleEndsAt.Equal(expiresAt) || linkCycleID != cycleID.Int64 || linkSubscriptionID != subscriptionID || linkGroupID != groupID || linkCount != 1 || subscriptionUserID != userID || subscriptionGroupID != groupID || subscriptionStatus != "active" || !subscriptionExpiresAt.Equal(expiresAt) || subscriptionDeletedAt.Valid || readLedgerID != ledgerID || ledgerUserID != userID || ledgerAssetType != "builder_pass_credit" || ledgerReason != "compensation" || ledgerFen != fen || ledgerSourceType != "compensation_execution" || ledgerSourceID != executionID || ledgerReference != sourceKey || ledgerNotes != "Approved service recovery Builder Pass credit" || ledgerOperatorID != approvedBy || ledgerGroupID != groupID || ledgerValidityDays != 0 || ledgerDedupe != sourceKey {
			return nil, fmt.Errorf("builder pass compensation asset or ledger failed exact readback")
		}
		assets = append(assets, map[string]any{"group_id": groupID, "amount_cny_fen": fen, "monthly_entitlement_cycle_id": cycleID.Int64, "subscription_id": subscriptionID, "account_change_record_id": ledgerID, "expires_at": expiresAt.UTC().Format(time.RFC3339Nano)})
	}
	if _, err = tx.ExecContext(ctx, `UPDATE compensation_executions SET state='applied',applied_at=NOW(),updated_at=NOW() WHERE id=$1`, executionID); err != nil {
		return nil, err
	}
	return map[string]any{"channel": "builder_pass", "amount_cny_fen": amountFen, "assets": assets, "validity_extended": false}, nil
}

func (s *CompensationExecutionService) ensureCompensationNotice(ctx context.Context, draftUserID int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var draftID int64
	if err = tx.QueryRowContext(ctx, `SELECT draft_id FROM compensation_draft_users WHERE id=$1`, draftUserID).Scan(&draftID); err != nil {
		return false, err
	}
	approvalID, incidentID, err := s.lockAndValidateApprovedDraftTx(ctx, tx, draftID)
	if err != nil {
		return false, err
	}
	var approvalNoticeID, userID, frozenBalanceFen, frozenBuilderFen int64
	var frozenBody, frozenHash string
	var frozenPayload []byte
	if err = tx.QueryRowContext(ctx, `SELECT id,user_id,balance_cny_fen,builder_pass_cny_fen,body,payload,payload_hash FROM compensation_approval_notices WHERE approval_id=$1 AND draft_user_id=$2`, approvalID, draftUserID).Scan(&approvalNoticeID, &userID, &frozenBalanceFen, &frozenBuilderFen, &frozenBody, &frozenPayload, &frozenHash); err != nil {
		return false, err
	}
	readFrozenHash, hashErr := canonicalCompensationJSONHash(frozenPayload)
	if hashErr != nil || readFrozenHash != frozenHash {
		return false, fmt.Errorf("frozen compensation notice failed hash readback")
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1,$2)`, incidentID, userID); err != nil {
		return false, err
	}
	var executionCount, pending, balanceFen, builderFen int64
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE state<>'verified'),COALESCE(SUM(amount_cny_fen) FILTER(WHERE benefit_channel='balance' AND state='verified'),0),COALESCE(SUM(amount_cny_fen) FILTER(WHERE benefit_channel='builder_pass' AND state='verified'),0) FROM compensation_executions WHERE approval_id=$1 AND draft_user_id=$2`, approvalID, draftUserID).Scan(&executionCount, &pending, &balanceFen, &builderFen); err != nil {
		return false, err
	}
	if executionCount == 0 {
		return false, fmt.Errorf("compensation notice cannot be created without execution rows")
	}
	if pending > 0 {
		return false, tx.Commit()
	}
	if balanceFen != frozenBalanceFen || builderFen != frozenBuilderFen || balanceFen+builderFen <= 0 {
		return false, fmt.Errorf("verified compensation amounts differ from approved notice snapshot")
	}
	var existingNotificationID sql.NullInt64
	var existingBody, existingHash sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT c.notification_id,n.body,c.payload_hash FROM compensation_notices c JOIN user_notifications n ON n.id=c.notification_id WHERE c.incident_id=$1 AND c.user_id=$2`, incidentID, userID).Scan(&existingNotificationID, &existingBody, &existingHash)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	if err == nil {
		if existingBody.String != frozenBody || existingHash.String != frozenHash {
			return false, fmt.Errorf("existing compensation notice failed exact readback")
		}
		return false, tx.Commit()
	}
	dedupe := fmt.Sprintf("compensation:%d:%d", incidentID, userID)
	var notificationID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO user_notifications(user_id,type,title,body,action_url,dedupe_key) VALUES($1,'compensation','服务恢复补偿已发放',$2,'/usage',$3) ON CONFLICT(dedupe_key) DO NOTHING RETURNING id`, userID, frozenBody, dedupe).Scan(&notificationID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `SELECT id,body FROM user_notifications WHERE dedupe_key=$1`, dedupe).Scan(&notificationID, &existingBody)
		if err == nil && existingBody.String != frozenBody {
			return false, fmt.Errorf("notification dedupe key exists with different body")
		}
	}
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_notices(incident_id,user_id,notification_id,approval_notice_id,payload,payload_hash) VALUES($1,$2,$3,$4,$5,$6)`, incidentID, userID, notificationID, approvalNoticeID, frozenPayload, frozenHash); err != nil {
		return false, err
	}
	var readBody, readHash string
	var readApprovalNoticeID int64
	if err = tx.QueryRowContext(ctx, `SELECT n.body,c.payload_hash,c.approval_notice_id FROM compensation_notices c JOIN user_notifications n ON n.id=c.notification_id WHERE c.incident_id=$1 AND c.user_id=$2`, incidentID, userID).Scan(&readBody, &readHash, &readApprovalNoticeID); err != nil {
		return false, err
	}
	if readBody != frozenBody || readHash != frozenHash || readApprovalNoticeID != approvalNoticeID {
		return false, fmt.Errorf("compensation notice failed exact write readback")
	}
	return true, tx.Commit()
}

func (s *CompensationExecutionService) previewCompensationNotices(ctx context.Context, draftID int64) ([]CompensationNoticePreview, error) {
	snapshots, err := buildCompensationNoticePreviews(ctx, s.db, draftID)
	if err != nil {
		return nil, err
	}
	return compensationNoticePreviews(snapshots), nil
}

type compensationNoticeSnapshot struct {
	DraftUserID int64
	Preview     CompensationNoticePreview
}

func buildCompensationNoticePreviews(ctx context.Context, queryer compensationNoticeQueryer, draftID int64) ([]compensationNoticeSnapshot, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT u.id,u.user_id,u.balance_benefit_cny_fen,u.builder_pass_benefit_cny_fen,i.customer_impact_started_at,i.customer_impact_ended_at,COALESCE(array_agg(DISTINCT product.display_name ORDER BY product.display_name) FILTER(WHERE product.id IS NOT NULL),ARRAY[]::text[]),COALESCE(bool_or(charged.id IS NOT NULL),FALSE) FROM compensation_draft_users u JOIN compensation_drafts d ON d.id=u.draft_id JOIN reliability_incidents i ON i.id=d.incident_id LEFT JOIN compensation_draft_items item ON item.draft_id=u.draft_id AND item.user_id=u.user_id AND item.proposed_cny_fen>0 LEFT JOIN service_status_products product ON product.id=item.product_id LEFT JOIN LATERAL jsonb_array_elements(COALESCE(item.evidence->'source_facts','[]'::jsonb)) fact ON fact->>'qualified'='true' AND COALESCE(fact->>'request_id','')<>'' AND fact->>'fact_type'='customer_request' AND fact->>'outcome'='failure' AND fact->>'customer_impact'='true' AND fact->>'error_owner' IN('provider','platform') AND COALESCE(fact->>'exclusion_reason','')='' AND fact->>'qualification_reason'='qualified_final_failure' LEFT JOIN usage_logs charged ON charged.user_id=u.user_id AND charged.actual_cost>0 AND charged.request_id=fact->>'request_id' WHERE u.draft_id=$1 AND u.eligible=TRUE AND u.included=TRUE AND u.evidence_complete=TRUE AND (u.balance_benefit_cny_fen+u.builder_pass_benefit_cny_fen)>0 GROUP BY u.id,u.user_id,u.balance_benefit_cny_fen,u.builder_pass_benefit_cny_fen,i.customer_impact_started_at,i.customer_impact_ended_at ORDER BY u.user_id`, draftID)
	if err != nil {
		return nil, err
	}
	var result []compensationNoticeSnapshot
	for rows.Next() {
		var draftUserID, userID, balanceFen, builderFen int64
		var impactStart, impactEnd time.Time
		var products pq.StringArray
		var charged bool
		if err := rows.Scan(&draftUserID, &userID, &balanceFen, &builderFen, &impactStart, &impactEnd, &products, &charged); err != nil {
			_ = rows.Close()
			return nil, err
		}
		services := []string(products)
		result = append(result, compensationNoticeSnapshot{DraftUserID: draftUserID, Preview: CompensationNoticePreview{UserID: userID, Services: services, BalanceCNYFen: balanceFen, BuilderPassCNYFen: builderFen, FailedRequestsCharged: charged, Body: renderCompensationNoticeBody(services, impactStart, impactEnd, balanceFen, builderFen, charged)}})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func compensationNoticePreviews(snapshots []compensationNoticeSnapshot) []CompensationNoticePreview {
	result := make([]CompensationNoticePreview, 0, len(snapshots))
	for _, snapshot := range snapshots {
		result = append(result, snapshot.Preview)
	}
	return result
}

func compensationApprovalPreviewHash(draftID int64, evidenceHash string, notices []CompensationNoticePreview) (string, error) {
	payload, err := canonicalCompensationJSON(map[string]any{"draft_id": draftID, "draft_evidence_hash": evidenceHash, "notices": notices})
	if err != nil {
		return "", err
	}
	return sha256Hex(payload), nil
}

func freezeCompensationApprovalNoticesTx(ctx context.Context, tx *sql.Tx, approvalID int64, snapshots []compensationNoticeSnapshot) error {
	approvalIDs := make([]int64, 0, len(snapshots))
	draftUserIDs := make([]int64, 0, len(snapshots))
	userIDs := make([]int64, 0, len(snapshots))
	balances := make([]int64, 0, len(snapshots))
	builders := make([]int64, 0, len(snapshots))
	bodies := make([]string, 0, len(snapshots))
	payloads := make([]string, 0, len(snapshots))
	hashes := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		payload, err := canonicalCompensationJSON(snapshot.Preview)
		if err != nil {
			return err
		}
		approvalIDs = append(approvalIDs, approvalID)
		draftUserIDs = append(draftUserIDs, snapshot.DraftUserID)
		userIDs = append(userIDs, snapshot.Preview.UserID)
		balances = append(balances, snapshot.Preview.BalanceCNYFen)
		builders = append(builders, snapshot.Preview.BuilderPassCNYFen)
		bodies = append(bodies, snapshot.Preview.Body)
		payloads = append(payloads, string(payload))
		hashes = append(hashes, sha256Hex(payload))
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO compensation_approval_notices(approval_id,draft_user_id,user_id,balance_cny_fen,builder_pass_cny_fen,body,payload,payload_hash) SELECT approval_id,draft_user_id,user_id,balance_cny_fen,builder_pass_cny_fen,body,payload::jsonb,payload_hash FROM unnest($1::bigint[],$2::bigint[],$3::bigint[],$4::bigint[],$5::bigint[],$6::text[],$7::text[],$8::text[]) AS frozen(approval_id,draft_user_id,user_id,balance_cny_fen,builder_pass_cny_fen,body,payload,payload_hash)`, pq.Array(approvalIDs), pq.Array(draftUserIDs), pq.Array(userIDs), pq.Array(balances), pq.Array(builders), pq.Array(bodies), pq.Array(payloads), pq.Array(hashes)); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM compensation_approval_notices WHERE approval_id=$1`, approvalID).Scan(&count); err != nil {
		return err
	}
	if count != len(snapshots) {
		return fmt.Errorf("approval notice snapshot failed exact readback")
	}
	return nil
}

func loadFrozenCompensationNotices(ctx context.Context, queryer compensationNoticeQueryer, approvalID int64) ([]CompensationNoticePreview, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT user_id,balance_cny_fen,builder_pass_cny_fen,body,payload,payload_hash FROM compensation_approval_notices WHERE approval_id=$1 ORDER BY user_id`, approvalID)
	if err != nil {
		return nil, err
	}
	var result []CompensationNoticePreview
	for rows.Next() {
		var userID, balanceFen, builderFen int64
		var body string
		var raw []byte
		var storedHash string
		if err := rows.Scan(&userID, &balanceFen, &builderFen, &body, &raw, &storedHash); err != nil {
			_ = rows.Close()
			return nil, err
		}
		readHash, hashErr := canonicalCompensationJSONHash(raw)
		if hashErr != nil || readHash != storedHash {
			_ = rows.Close()
			return nil, fmt.Errorf("approval notice snapshot hash mismatch")
		}
		var item CompensationNoticePreview
		if err := json.Unmarshal(raw, &item); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if item.UserID != userID || item.BalanceCNYFen != balanceFen || item.BuilderPassCNYFen != builderFen || item.Body != body {
			_ = rows.Close()
			return nil, fmt.Errorf("approval notice snapshot failed exact readback")
		}
		result = append(result, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func renderCompensationNoticeBody(products []string, impactStart, impactEnd time.Time, balanceFen, builderFen int64, failedRequestsCharged bool) string {
	channels := []string{}
	if balanceFen > 0 {
		channels = append(channels, fmt.Sprintf("余额 ¥%.2f", float64(balanceFen)/100))
	}
	if builderFen > 0 {
		channels = append(channels, fmt.Sprintf("Builder Pass ¥%.2f", float64(builderFen)/100))
	}
	chargeStatement := "未发现最终失败请求扣费"
	if failedRequestsCharged {
		chargeStatement = "审批时发现最终失败请求存在扣费，请使用独立精确退款处理"
	}
	beijing := time.FixedZone("Asia/Shanghai", 8*60*60)
	return fmt.Sprintf("受影响服务：%s。影响时段：%s 至 %s。已发放：%s。%s。", strings.Join(products, "、"), impactStart.In(beijing).Format("2006-01-02 15:04 北京时间"), impactEnd.In(beijing).Format("2006-01-02 15:04 北京时间"), strings.Join(channels, "、"), chargeStatement)
}

func (s *CompensationExecutionService) GetExecution(ctx context.Context, id int64) (*CompensationExecution, error) {
	item := &CompensationExecution{}
	var applied, verified sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id,execution_key,approval_id,draft_user_id,incident_id,user_id,benefit_channel,amount_cny_fen,state,attempt_count,last_error,applied_at,verified_at FROM compensation_executions WHERE id=$1`, id).Scan(&item.ID, &item.ExecutionKey, &item.ApprovalID, &item.DraftUserID, &item.IncidentID, &item.UserID, &item.BenefitChannel, &item.AmountCNYFen, &item.State, &item.AttemptCount, &item.LastError, &applied, &verified)
	if err != nil {
		return nil, err
	}
	if applied.Valid {
		v := applied.Time.UTC()
		item.AppliedAt = &v
	}
	if verified.Valid {
		v := verified.Time.UTC()
		item.VerifiedAt = &v
	}
	var payload []byte
	var receipt CompensationExecutionReceipt
	err = s.db.QueryRowContext(ctx, `SELECT id,execution_id,payload,payload_hash,verified_at FROM compensation_execution_receipts WHERE execution_id=$1`, id).Scan(&receipt.ID, &receipt.ExecutionID, &payload, &receipt.PayloadHash, &receipt.VerifiedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		readHash, hashErr := canonicalCompensationJSONHash(payload)
		if hashErr != nil || readHash != receipt.PayloadHash {
			return nil, fmt.Errorf("compensation execution receipt failed hash readback")
		}
		if err := json.Unmarshal(payload, &receipt.Payload); err != nil {
			return nil, err
		}
		item.Receipt = &receipt
	}
	return item, nil
}

func fenToMicros(fen int64) (int64, error) {
	if fen <= 0 || fen > math.MaxInt64/10_000 {
		return 0, fmt.Errorf("benefit amount exceeds supported range")
	}
	return fen * 10_000, nil
}
func sha256Hex(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }

func canonicalCompensationJSONHash(raw []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	canonical, err := canonicalCompensationJSON(value)
	if err != nil {
		return "", err
	}
	return sha256Hex(canonical), nil
}
