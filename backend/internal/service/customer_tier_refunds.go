package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RecordPaidValueRefund creates an immutable, idempotent reversal against the
// canonical included paid source. It never infers refunds from free-text notes.
func (s *CustomerTierService) RecordPaidValueRefund(ctx context.Context, command CustomerPaidValueRefundCommand) (*CustomerPaidValueRefund, error) {
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	command.Reason = strings.TrimSpace(command.Reason)
	if s == nil || s.db == nil || command.IdempotencyKey == "" || len(command.IdempotencyKey) > 180 || command.UserID <= 0 || command.SourceID <= 0 || command.AmountCNYFen <= 0 || command.Reason == "" || len(command.Reason) > 500 {
		return nil, fmt.Errorf("refund idempotency key, customer, paid source, amount, and reason are required")
	}
	if command.RefundedAt.IsZero() {
		command.RefundedAt = time.Now().UTC()
	}
	command.RefundedAt = command.RefundedAt.UTC().Truncate(time.Microsecond)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, command.UserID); err != nil {
		return nil, err
	}
	gross, err := canonicalizeCustomerPaidSourceTx(ctx, tx, &command)
	if err != nil {
		return nil, err
	}
	var existing CustomerPaidValueRefund
	var actor sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT id,idempotency_key,user_id,source_type,source_id,amount_cny_fen,reason,created_by_user_id,refunded_at,created_at FROM customer_paid_value_refunds WHERE idempotency_key=$1`, command.IdempotencyKey).Scan(&existing.ID, &existing.IdempotencyKey, &existing.UserID, &existing.SourceType, &existing.SourceID, &existing.AmountCNYFen, &existing.Reason, &actor, &existing.RefundedAt, &existing.CreatedAt)
	if err == nil {
		if existing.UserID != command.UserID || existing.SourceType != command.SourceType || existing.SourceID != command.SourceID || existing.AmountCNYFen != command.AmountCNYFen || existing.Reason != command.Reason || !existing.RefundedAt.Equal(command.RefundedAt) {
			return nil, fmt.Errorf("refund idempotency key was already used with different input")
		}
		if actor.Valid {
			v := actor.Int64
			existing.CreatedByUserID = &v
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	var refunded int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cny_fen),0)::bigint FROM customer_paid_value_refunds WHERE user_id=$1 AND source_type=$2 AND source_id=$3`, command.UserID, command.SourceType, command.SourceID).Scan(&refunded); err != nil {
		return nil, err
	}
	if refunded > gross-command.AmountCNYFen {
		return nil, fmt.Errorf("refund exceeds paid source amount")
	}
	item := &CustomerPaidValueRefund{IdempotencyKey: command.IdempotencyKey, UserID: command.UserID, SourceType: command.SourceType, SourceID: command.SourceID, AmountCNYFen: command.AmountCNYFen, Reason: command.Reason, RefundedAt: command.RefundedAt}
	var actorValue any
	if command.ActorUserID > 0 {
		v := command.ActorUserID
		item.CreatedByUserID = &v
		actorValue = v
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO customer_paid_value_refunds(idempotency_key,user_id,source_type,source_id,amount_cny_fen,reason,created_by_user_id,refunded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at`, item.IdempotencyKey, item.UserID, item.SourceType, item.SourceID, item.AmountCNYFen, item.Reason, actorValue, item.RefundedAt).Scan(&item.ID, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// The refund is authoritative even when projection refresh is temporarily
	// unavailable; the resumable evaluator will repair the projection.
	_, _ = s.EvaluateUserAt(ctx, command.UserID, time.Now().UTC())
	return item, nil
}

func canonicalizeCustomerPaidSourceTx(ctx context.Context, tx *sql.Tx, command *CustomerPaidValueRefundCommand) (int64, error) {
	var gross int64
	var status string
	var completed sql.NullTime
	switch command.SourceType {
	case "topup_order":
		err := tx.QueryRowContext(ctx, `SELECT amount_cny_fen::bigint,status,completed_at FROM topup_orders WHERE user_id=$1 AND id=$2`, command.UserID, command.SourceID).Scan(&gross, &status, &completed)
		if err != nil {
			return 0, customerPaidSourceLookupError(err)
		}
		if status != "completed" || !completed.Valid {
			return 0, fmt.Errorf("paid source is not canonical included value")
		}
	case "payment_order":
		err := tx.QueryRowContext(ctx, `SELECT amount_cents::bigint,status,completed_at FROM payment_orders WHERE user_id=$1 AND id=$2`, command.UserID, command.SourceID).Scan(&gross, &status, &completed)
		if err != nil {
			return 0, customerPaidSourceLookupError(err)
		}
		if status != "completed" || !completed.Valid {
			return 0, fmt.Errorf("paid source is not canonical included value")
		}
	case "native_checkout_order":
		var purpose, sales string
		err := tx.QueryRowContext(ctx, `SELECT pay_amount_cny_fen::bigint,status,completed_at,redeem_purpose,redeem_sales_status FROM native_checkout_orders WHERE user_id=$1 AND id=$2`, command.UserID, command.SourceID).Scan(&gross, &status, &completed, &purpose, &sales)
		if err != nil {
			return 0, customerPaidSourceLookupError(err)
		}
		if status != "completed" || !completed.Valid || purpose != "sale_recharge" || sales != "sold" {
			return 0, fmt.Errorf("paid source is not canonical included value")
		}
	case "redeem_code":
		var purpose, sales string
		var nativeID, nativeGross sql.NullInt64
		var nativePurpose, nativeSales sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT ROUND(r.paid_value*100)::bigint,r.status,r.used_at,r.purpose,r.sales_status,n.id,n.pay_amount_cny_fen,n.redeem_purpose,n.redeem_sales_status FROM redeem_codes r LEFT JOIN LATERAL (SELECT id,pay_amount_cny_fen,redeem_purpose,redeem_sales_status FROM native_checkout_orders WHERE redeem_code_id=r.id AND user_id=$1 AND status='completed' ORDER BY id LIMIT 1) n ON TRUE WHERE r.used_by=$1 AND r.id=$2`, command.UserID, command.SourceID).Scan(&gross, &status, &completed, &purpose, &sales, &nativeID, &nativeGross, &nativePurpose, &nativeSales)
		if err != nil {
			return 0, customerPaidSourceLookupError(err)
		}
		if nativeID.Valid {
			if !nativePurpose.Valid || !nativeSales.Valid || nativePurpose.String != "sale_recharge" || nativeSales.String != "sold" {
				return 0, fmt.Errorf("paid source is not canonical included value")
			}
			command.SourceType, command.SourceID, gross = "native_checkout_order", nativeID.Int64, nativeGross.Int64
		} else if status != "used" || purpose != "sale_recharge" || sales != "sold" || gross <= 0 {
			return 0, fmt.Errorf("paid source is not canonical included value")
		}
	default:
		return 0, fmt.Errorf("paid source type is unsupported")
	}
	if gross <= 0 {
		return 0, fmt.Errorf("paid source is not canonical included value")
	}
	return gross, nil
}

func customerPaidSourceLookupError(err error) error {
	if err == sql.ErrNoRows {
		return fmt.Errorf("paid source does not belong to customer")
	}
	return err
}
