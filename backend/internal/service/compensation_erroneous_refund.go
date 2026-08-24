package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *CompensationExecutionService) RefundErroneousCharge(ctx context.Context, command ErroneousChargeRefundCommand) (*ErroneousChargeRefund, error) {
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.Reason = strings.TrimSpace(command.Reason)
	if s == nil || s.db == nil || !rbacActorIsSuperAdmin(ctx) || command.IdempotencyKey == "" || len(command.IdempotencyKey) > 180 || command.UsageLogID <= 0 || command.UserID <= 0 || command.ActorUserID <= 0 || command.Reason == "" || len(command.Reason) > 500 {
		return nil, fmt.Errorf("refund key, usage, user, actor, and reason are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('erroneous-charge-refund'),$1)`, command.UsageLogID); err != nil {
		return nil, err
	}
	existing := &ErroneousChargeRefund{}
	err = tx.QueryRowContext(ctx, `SELECT id,idempotency_key,usage_log_id,user_id,amount_micros,billing_type,asset_type,asset_id,reason,created_at FROM erroneous_charge_refunds WHERE idempotency_key=$1 OR usage_log_id=$2 ORDER BY (idempotency_key=$1) DESC LIMIT 1`, command.IdempotencyKey, command.UsageLogID).Scan(&existing.ID, &existing.IdempotencyKey, &existing.UsageLogID, &existing.UserID, &existing.AmountMicros, &existing.BillingType, &existing.AssetType, &existing.AssetID, &existing.Reason, &existing.CreatedAt)
	if err == nil {
		if existing.UsageLogID != command.UsageLogID || existing.UserID != command.UserID || existing.IdempotencyKey != command.IdempotencyKey || existing.Reason != command.Reason {
			return nil, fmt.Errorf("refund key already exists with different input")
		}
		if err = verifyErroneousChargeRefundTx(ctx, tx, existing); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	var amountMicros int64
	var billingType int16
	var subscriptionID sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT ROUND(u.actual_cost*1000000)::bigint,u.billing_type,u.subscription_id FROM usage_logs u JOIN billing_usage_entries b ON b.usage_log_id=u.id AND b.applied=TRUE WHERE u.id=$1 AND u.user_id=$2 FOR UPDATE OF u`, command.UsageLogID, command.UserID).Scan(&amountMicros, &billingType, &subscriptionID); err != nil {
		return nil, fmt.Errorf("applied usage charge is required: %w", err)
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("usage charge has no positive amount to refund")
	}
	item := &ErroneousChargeRefund{IdempotencyKey: command.IdempotencyKey, UsageLogID: command.UsageLogID, UserID: command.UserID, AmountMicros: amountMicros, BillingType: billingType, Reason: command.Reason}
	evidence := map[string]any{"usage_log_id": command.UsageLogID, "user_id": command.UserID, "amount_micros": amountMicros, "billing_type": billingType, "reason": command.Reason}
	var confirmedReversal, consumptionID int64
	switch billingType {
	case int16(BillingTypeBalance):
		var before, after string
		if err = tx.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1 FOR UPDATE`, command.UserID).Scan(&before); err != nil {
			return nil, err
		}
		if err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+($2::numeric/1000000),updated_at=NOW() WHERE id=$1 RETURNING balance::text`, command.UserID, amountMicros).Scan(&after); err != nil {
			return nil, err
		}
		var appliedMicros int64
		if err = tx.QueryRowContext(ctx, `SELECT ROUND(($1::numeric-$2::numeric)*1000000)::bigint`, after, before).Scan(&appliedMicros); err != nil {
			return nil, err
		}
		if appliedMicros != amountMicros {
			return nil, fmt.Errorf("balance refund failed exact readback")
		}
		sourceKey := "erroneous-charge-refund:" + command.IdempotencyKey
		if err = tx.QueryRowContext(ctx, `INSERT INTO balance_lots(user_id,source_type,source_id,source_key,original_amount_micros,remaining_amount_micros,affiliate_eligible,occurred_at) VALUES($1,'erroneous_charge_refund',$2,$3,$4,$4,FALSE,NOW()) RETURNING id`, command.UserID, command.UsageLogID, sourceKey, amountMicros).Scan(&item.AssetID); err != nil {
			return nil, err
		}
		item.AssetType = "balance_lot"
		evidence["balance_before"] = before
		evidence["balance_after"] = after
		evidence["balance_lot_id"] = item.AssetID
	case int16(BillingTypeSubscription):
		if !subscriptionID.Valid {
			return nil, fmt.Errorf("subscription usage charge is missing subscription attribution")
		}
		var usedMicros, confirmedMicros int64
		if err = tx.QueryRowContext(ctx, `SELECT c.id,c.used_credit_micros,c.confirmed_consumption_micros,e.id,e.confirmed_amount_micros FROM monthly_entitlement_consumptions e JOIN monthly_entitlement_cycles c ON c.id=e.cycle_id WHERE e.usage_log_id=$1 AND e.user_id=$2 FOR UPDATE OF c`, command.UsageLogID, command.UserID).Scan(&item.AssetID, &usedMicros, &confirmedMicros, &consumptionID, &confirmedReversal); err != nil {
			return nil, fmt.Errorf("exact monthly entitlement consumption is required: %w", err)
		}
		if usedMicros < amountMicros || confirmedMicros < confirmedReversal {
			return nil, fmt.Errorf("monthly entitlement totals are smaller than the attributed charge")
		}
		var usedAfter, confirmedAfter int64
		if err = tx.QueryRowContext(ctx, `UPDATE monthly_entitlement_cycles SET used_credit_micros=used_credit_micros-$2,confirmed_consumption_micros=confirmed_consumption_micros-$3,updated_at=NOW() WHERE id=$1 RETURNING used_credit_micros,confirmed_consumption_micros`, item.AssetID, amountMicros, confirmedReversal).Scan(&usedAfter, &confirmedAfter); err != nil {
			return nil, err
		}
		if usedAfter != usedMicros-amountMicros || confirmedAfter != confirmedMicros-confirmedReversal {
			return nil, fmt.Errorf("monthly entitlement refund failed exact readback")
		}
		item.AssetType = "monthly_entitlement_cycle"
		evidence["monthly_entitlement_cycle_id"] = item.AssetID
		evidence["consumption_id"] = consumptionID
		evidence["confirmed_reversal_micros"] = confirmedReversal
		evidence["used_credit_before_micros"] = usedMicros
		evidence["used_credit_after_micros"] = usedAfter
		evidence["confirmed_consumption_before_micros"] = confirmedMicros
		evidence["confirmed_consumption_after_micros"] = confirmedAfter
	default:
		return nil, fmt.Errorf("unsupported billing type")
	}
	raw, err := canonicalCompensationJSON(evidence)
	if err != nil {
		return nil, err
	}
	hash := sha256Hex(raw)
	err = tx.QueryRowContext(ctx, `INSERT INTO erroneous_charge_refunds(idempotency_key,usage_log_id,user_id,amount_micros,billing_type,asset_type,asset_id,reason,created_by_user_id,evidence,evidence_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id,created_at`, item.IdempotencyKey, item.UsageLogID, item.UserID, item.AmountMicros, item.BillingType, item.AssetType, item.AssetID, command.Reason, command.ActorUserID, raw, hash).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	if billingType == int16(BillingTypeSubscription) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO monthly_entitlement_consumption_reversals(refund_id,consumption_id,confirmed_amount_micros) VALUES($1,$2,$3)`, item.ID, consumptionID, confirmedReversal); err != nil {
			return nil, err
		}
	}
	dedupe := "erroneous-charge-refund:" + command.IdempotencyKey
	expectedAssetType := map[bool]string{true: "balance", false: "builder_pass_credit"}[billingType == int16(BillingTypeBalance)]
	var ledgerID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,source_id,reference_no,notes,operator_user_id,validity_days,dedupe_key) VALUES($1,$2,'erroneous_charge_refund',$3::numeric/1000000,'erroneous_charge_refund',$4,$5,$6,$7,0,$5) RETURNING id`, item.UserID, expectedAssetType, amountMicros, item.ID, dedupe, command.Reason, command.ActorUserID).Scan(&ledgerID); err != nil {
		return nil, err
	}
	var ledgerUserID, ledgerSourceID, ledgerOperatorID, ledgerMicros int64
	var ledgerAssetType, ledgerReason, ledgerSourceType, ledgerReference, ledgerNotes string
	if err = tx.QueryRowContext(ctx, `SELECT user_id,asset_type,reason,ROUND(delta*1000000)::bigint,source_type,source_id,reference_no,notes,operator_user_id FROM account_change_records WHERE id=$1`, ledgerID).Scan(&ledgerUserID, &ledgerAssetType, &ledgerReason, &ledgerMicros, &ledgerSourceType, &ledgerSourceID, &ledgerReference, &ledgerNotes, &ledgerOperatorID); err != nil {
		return nil, err
	}
	if ledgerUserID != item.UserID || ledgerAssetType != expectedAssetType || ledgerReason != "erroneous_charge_refund" || ledgerMicros != amountMicros || ledgerSourceType != "erroneous_charge_refund" || ledgerSourceID != item.ID || ledgerReference != dedupe || ledgerNotes != command.Reason || ledgerOperatorID != command.ActorUserID {
		return nil, fmt.Errorf("erroneous-charge refund ledger failed exact readback")
	}
	var readEvidence []byte
	var readHash string
	if err = tx.QueryRowContext(ctx, `SELECT evidence,evidence_hash FROM erroneous_charge_refunds WHERE id=$1`, item.ID).Scan(&readEvidence, &readHash); err != nil {
		return nil, err
	}
	canonicalReadHash, hashErr := canonicalCompensationJSONHash(readEvidence)
	if hashErr != nil || readHash != hash || canonicalReadHash != hash {
		return nil, fmt.Errorf("erroneous-charge refund evidence failed exact readback")
	}
	if billingType == int16(BillingTypeBalance) {
		var lotUserID, lotSourceID, lotOriginal, lotRemaining int64
		var lotSourceType, lotSourceKey string
		if err = tx.QueryRowContext(ctx, `SELECT user_id,source_type,source_id,source_key,original_amount_micros,remaining_amount_micros FROM balance_lots WHERE id=$1`, item.AssetID).Scan(&lotUserID, &lotSourceType, &lotSourceID, &lotSourceKey, &lotOriginal, &lotRemaining); err != nil {
			return nil, err
		}
		if lotUserID != item.UserID || lotSourceType != "erroneous_charge_refund" || lotSourceID != item.UsageLogID || lotSourceKey != dedupe || lotOriginal != amountMicros || lotRemaining != amountMicros {
			return nil, fmt.Errorf("erroneous-charge balance lot failed exact readback")
		}
	} else {
		var refundID, reversedConsumptionID, reversedMicros int64
		if err = tx.QueryRowContext(ctx, `SELECT refund_id,consumption_id,confirmed_amount_micros FROM monthly_entitlement_consumption_reversals WHERE refund_id=$1`, item.ID).Scan(&refundID, &reversedConsumptionID, &reversedMicros); err != nil {
			return nil, err
		}
		if refundID != item.ID || reversedConsumptionID != consumptionID || reversedMicros != confirmedReversal {
			return nil, fmt.Errorf("monthly entitlement reversal failed exact readback")
		}
	}
	if err = verifyErroneousChargeRefundTx(ctx, tx, item); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if s.billingCache != nil {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if billingType == int16(BillingTypeBalance) {
			_ = s.billingCache.InvalidateUserBalance(cacheCtx, item.UserID)
		} else if subscriptionID.Valid {
			var groupID int64
			if s.db.QueryRowContext(cacheCtx, `SELECT group_id FROM user_subscriptions WHERE id=$1`, subscriptionID.Int64).Scan(&groupID) == nil {
				_ = s.billingCache.InvalidateSubscription(cacheCtx, item.UserID, groupID)
			}
		}
	}
	return item, nil
}

func verifyErroneousChargeRefundTx(ctx context.Context, tx *sql.Tx, item *ErroneousChargeRefund) error {
	var evidence []byte
	var evidenceHash string
	var createdBy int64
	if err := tx.QueryRowContext(ctx, `SELECT evidence,evidence_hash,created_by_user_id FROM erroneous_charge_refunds WHERE id=$1`, item.ID).Scan(&evidence, &evidenceHash, &createdBy); err != nil {
		return err
	}
	readHash, err := canonicalCompensationJSONHash(evidence)
	if err != nil || readHash != evidenceHash {
		return fmt.Errorf("erroneous-charge refund evidence failed exact readback")
	}
	expectedAssetType := map[bool]string{true: "balance", false: "builder_pass_credit"}[item.BillingType == int16(BillingTypeBalance)]
	var ledgerUserID, ledgerSourceID, ledgerOperatorID, ledgerMicros int64
	var ledgerAssetType, ledgerReason, ledgerSourceType, ledgerReference, ledgerNotes string
	err = tx.QueryRowContext(ctx, `SELECT user_id,asset_type,reason,ROUND(delta*1000000)::bigint,source_type,source_id,reference_no,notes,operator_user_id FROM account_change_records WHERE source_type='erroneous_charge_refund' AND source_id=$1`, item.ID).Scan(&ledgerUserID, &ledgerAssetType, &ledgerReason, &ledgerMicros, &ledgerSourceType, &ledgerSourceID, &ledgerReference, &ledgerNotes, &ledgerOperatorID)
	if err != nil {
		return err
	}
	expectedReference := "erroneous-charge-refund:" + item.IdempotencyKey
	if ledgerUserID != item.UserID || ledgerAssetType != expectedAssetType || ledgerReason != "erroneous_charge_refund" || ledgerMicros != item.AmountMicros || ledgerSourceType != "erroneous_charge_refund" || ledgerSourceID != item.ID || ledgerReference != expectedReference || ledgerNotes != item.Reason || ledgerOperatorID != createdBy {
		return fmt.Errorf("erroneous-charge refund ledger failed exact readback")
	}
	if item.BillingType == int16(BillingTypeBalance) {
		var userID, sourceID, originalMicros int64
		var sourceType, sourceKey string
		if err = tx.QueryRowContext(ctx, `SELECT user_id,source_type,source_id,source_key,original_amount_micros FROM balance_lots WHERE id=$1`, item.AssetID).Scan(&userID, &sourceType, &sourceID, &sourceKey, &originalMicros); err != nil {
			return err
		}
		if userID != item.UserID || sourceType != "erroneous_charge_refund" || sourceID != item.UsageLogID || sourceKey != expectedReference || originalMicros != item.AmountMicros {
			return fmt.Errorf("erroneous-charge balance asset failed exact readback")
		}
		return nil
	}
	var refundID, cycleID int64
	if err = tx.QueryRowContext(ctx, `SELECT r.refund_id,e.cycle_id FROM monthly_entitlement_consumption_reversals r JOIN monthly_entitlement_consumptions e ON e.id=r.consumption_id WHERE r.refund_id=$1`, item.ID).Scan(&refundID, &cycleID); err != nil {
		return err
	}
	if refundID != item.ID || cycleID != item.AssetID {
		return fmt.Errorf("erroneous-charge Builder Pass reversal failed exact readback")
	}
	return nil
}
