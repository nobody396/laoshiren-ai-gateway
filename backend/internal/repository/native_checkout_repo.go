package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type nativeCheckoutRepository struct {
	db *sql.DB
}

func NewNativeCheckoutRepository(db *sql.DB) *nativeCheckoutRepository {
	return &nativeCheckoutRepository{db: db}
}

func (r *nativeCheckoutRepository) IsNativeCheckoutRestricted(ctx context.Context, redeemCodeID int64) (bool, error) {
	var restricted bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM native_checkout_redeem_inventory WHERE redeem_code_id = $1
	)`, redeemCodeID).Scan(&restricted)
	return restricted, err
}

const nativeCheckoutOfferColumns = `
code, provider, provider_goods_key, name, description, product_kind,
pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type,
redeem_value::double precision, redeem_paid_value::double precision,
redeem_purpose, redeem_sales_status, redeem_group_ids, redeem_validity_days,
once_per_user, enabled, sort_order`

func (r *nativeCheckoutRepository) ListVisibleOffers(ctx context.Context, userID int64) ([]service.NativeCheckoutOffer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+nativeCheckoutOfferColumns+`
		FROM native_checkout_offers offer
		WHERE offer.enabled = TRUE
		   OR EXISTS (
		       SELECT 1 FROM native_checkout_offer_testers tester
		       WHERE tester.offer_code = offer.code AND tester.user_id = $1
		   )
		ORDER BY sort_order, code`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	offers := make([]service.NativeCheckoutOffer, 0)
	for rows.Next() {
		offer, err := scanNativeCheckoutOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, *offer)
	}
	return offers, rows.Err()
}

func (r *nativeCheckoutRepository) GetVisibleOffer(ctx context.Context, userID int64, code string) (*service.NativeCheckoutOffer, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+nativeCheckoutOfferColumns+`
		FROM native_checkout_offers offer
		WHERE offer.code = $1
		  AND (
		      offer.enabled = TRUE
		      OR EXISTS (
		          SELECT 1 FROM native_checkout_offer_testers tester
		          WHERE tester.offer_code = offer.code AND tester.user_id = $2
		      )
	  )`, code, userID)
	offer, err := scanNativeCheckoutOffer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrNativeCheckoutOfferNotFound
	}
	return offer, err
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func scanNativeCheckoutOffer(row sqlScanner) (*service.NativeCheckoutOffer, error) {
	var offer service.NativeCheckoutOffer
	var groupIDs []byte
	if err := row.Scan(
		&offer.Code, &offer.Provider, &offer.ProviderGoodsKey, &offer.Name, &offer.Description, &offer.ProductKind,
		&offer.PayAmountCNYFen, &offer.BenefitAmountCNYFen, &offer.RedeemType,
		&offer.RedeemValue, &offer.RedeemPaidValue, &offer.RedeemPurpose, &offer.RedeemSalesStatus,
		&groupIDs, &offer.RedeemValidityDays, &offer.OncePerUser, &offer.Enabled, &offer.SortOrder,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(groupIDs, &offer.RedeemGroupIDs); err != nil {
		return nil, fmt.Errorf("decode native checkout offer groups: %w", err)
	}
	return &offer, nil
}

func marshalNativeCheckoutGroupIDs(groupIDs []int64) ([]byte, error) {
	if groupIDs == nil {
		groupIDs = []int64{}
	}
	return json.Marshal(groupIDs)
}

const nativeCheckoutOrderColumns = `
id, order_no, user_id, offer_code, provider, provider_goods_key,
COALESCE(provider_trade_no, ''), COALESCE(payment_url, ''), COALESCE(payment_method, ''), contact_hash,
product_kind, pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type,
redeem_value::double precision, redeem_paid_value::double precision,
redeem_purpose, redeem_sales_status, redeem_group_ids, redeem_validity_days,
enforce_once, status, redeem_code_id, failure_code, check_count, next_check_at,
fulfillment_started_at, completed_at, created_at, updated_at`

func (r *nativeCheckoutRepository) GetLatestOrderForOffer(ctx context.Context, userID int64, offerCode string) (*service.NativeCheckoutOrder, error) {
	return r.scanOrderRow(r.db.QueryRowContext(ctx, `SELECT `+nativeCheckoutOrderColumns+`
		FROM native_checkout_orders
		WHERE user_id = $1 AND offer_code = $2
		ORDER BY created_at DESC, id DESC LIMIT 1`, userID, offerCode))
}

func (r *nativeCheckoutRepository) HasRedeemedOffer(ctx context.Context, userID int64, offerCode string) (bool, error) {
	var claimed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1
		FROM native_checkout_redeem_inventory inventory
		JOIN redeem_codes code ON code.id = inventory.redeem_code_id
		WHERE inventory.offer_code = $2
		  AND code.status = 'used'
		  AND code.used_by = $1
	)`, userID, offerCode).Scan(&claimed)
	return claimed, err
}

func (r *nativeCheckoutRepository) ReserveOrder(ctx context.Context, order *service.NativeCheckoutOrder) (*service.NativeCheckoutOrder, bool, error) {
	groupIDs, err := marshalNativeCheckoutGroupIDs(order.RedeemGroupIDs)
	if err != nil {
		return nil, false, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO native_checkout_orders (
			order_no, user_id, offer_code, provider, provider_goods_key, contact_hash,
			product_kind, pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type,
			redeem_value, redeem_paid_value, redeem_purpose, redeem_sales_status,
			redeem_group_ids, redeem_validity_days, enforce_once, status, next_check_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
		)
		ON CONFLICT DO NOTHING
		RETURNING `+nativeCheckoutOrderColumns,
		order.OrderNo, order.UserID, order.OfferCode, order.Provider, order.ProviderGoodsKey, order.ContactHash,
		order.ProductKind, order.PayAmountCNYFen, order.BenefitAmountCNYFen, order.RedeemType,
		order.RedeemValue, order.RedeemPaidValue, order.RedeemPurpose, order.RedeemSalesStatus,
		groupIDs, order.RedeemValidityDays, order.EnforceOnce, order.Status, order.NextCheckAt,
	)
	inserted, err := r.scanOrderRow(row)
	if err == nil {
		return inserted, true, nil
	}
	if !errors.Is(err, service.ErrNativeCheckoutOrderNotFound) {
		return nil, false, err
	}
	existing, err := r.GetLatestOrderForOffer(ctx, order.UserID, order.OfferCode)
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

func (r *nativeCheckoutRepository) ResetFailedOrder(ctx context.Context, id int64, contactHash string) (*service.NativeCheckoutOrder, bool, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE native_checkout_orders SET
		status = 'creating', provider_trade_no = NULL, payment_url = NULL, payment_method = NULL,
		contact_hash = $2, redeem_code_id = NULL, failure_code = '', check_count = 0,
		next_check_at = NOW(), fulfillment_started_at = NULL, completed_at = NULL,
		updated_at = NOW()
		WHERE id = $1 AND status = 'failed'
		RETURNING `+nativeCheckoutOrderColumns, id, contactHash)
	order, err := r.scanOrderRow(row)
	if err == nil {
		return order, true, nil
	}
	if !errors.Is(err, service.ErrNativeCheckoutOrderNotFound) {
		return nil, false, err
	}
	current, err := r.getOrderByID(ctx, id)
	return current, false, err
}

func (r *nativeCheckoutRepository) SetProviderOrder(ctx context.Context, id int64, providerTradeNo, paymentURL, paymentMethod string) (*service.NativeCheckoutOrder, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE native_checkout_orders SET
		provider_trade_no = $2, payment_url = $3, payment_method = $4, status = 'pending', failure_code = '',
		next_check_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'creating' AND provider_trade_no IS NULL
		RETURNING `+nativeCheckoutOrderColumns, id, providerTradeNo, paymentURL, paymentMethod)
	return r.scanOrderRow(row)
}

func (r *nativeCheckoutRepository) SetOrderState(ctx context.Context, id int64, status, failureCode string, nextCheckAt time.Time) (*service.NativeCheckoutOrder, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE native_checkout_orders SET
		status = $2, failure_code = $3, next_check_at = $4, updated_at = NOW()
		WHERE id = $1 AND status <> 'completed'
		RETURNING `+nativeCheckoutOrderColumns, id, status, failureCode, nextCheckAt)
	order, err := r.scanOrderRow(row)
	if err == nil {
		return order, nil
	}
	if !errors.Is(err, service.ErrNativeCheckoutOrderNotFound) {
		return nil, err
	}
	return r.getOrderByID(ctx, id)
}

func (r *nativeCheckoutRepository) GetOrderForUser(ctx context.Context, orderNo string, userID int64) (*service.NativeCheckoutOrder, error) {
	return r.scanOrderRow(r.db.QueryRowContext(ctx, `SELECT `+nativeCheckoutOrderColumns+`
		FROM native_checkout_orders WHERE order_no = $1 AND user_id = $2`, orderNo, userID))
}

func (r *nativeCheckoutRepository) GetOrder(ctx context.Context, orderNo string) (*service.NativeCheckoutOrder, error) {
	return r.scanOrderRow(r.db.QueryRowContext(ctx, `SELECT `+nativeCheckoutOrderColumns+`
		FROM native_checkout_orders WHERE order_no = $1`, orderNo))
}

func (r *nativeCheckoutRepository) getOrderByID(ctx context.Context, id int64) (*service.NativeCheckoutOrder, error) {
	return r.scanOrderRow(r.db.QueryRowContext(ctx, `SELECT `+nativeCheckoutOrderColumns+`
		FROM native_checkout_orders WHERE id = $1`, id))
}

func (r *nativeCheckoutRepository) ClaimReconcileOrders(ctx context.Context, limit int, fulfillingStaleBefore, leaseUntil time.Time) ([]service.NativeCheckoutOrder, error) {
	rows, err := r.db.QueryContext(ctx, `WITH due AS (
		SELECT id
		FROM native_checkout_orders
		WHERE next_check_at <= NOW()
		  AND (
			(status = 'creating' AND updated_at < $1)
			OR status IN ('pending', 'checking')
			OR (status = 'fulfilling' AND updated_at < $1)
		  )
		ORDER BY next_check_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT $2
	)
	UPDATE native_checkout_orders
	SET next_check_at = $3
	WHERE id IN (SELECT id FROM due)
	RETURNING `+nativeCheckoutOrderColumns, fulfillingStaleBefore, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	orders := make([]service.NativeCheckoutOrder, 0)
	for rows.Next() {
		order, err := r.scanOrderRow(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}
	return orders, rows.Err()
}

func (r *nativeCheckoutRepository) RecordPendingCheck(ctx context.Context, id int64, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE native_checkout_orders SET
		check_count = check_count + 1, next_check_at = $2, updated_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'checking')`, id, nextCheckAt)
	return err
}

func (r *nativeCheckoutRepository) ClaimFulfillment(ctx context.Context, id, redeemCodeID int64, staleBefore time.Time) (*service.NativeCheckoutOrder, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `UPDATE native_checkout_orders SET
		status = 'fulfilling', redeem_code_id = COALESCE(redeem_code_id, $2),
		fulfillment_started_at = COALESCE(fulfillment_started_at, NOW()),
		failure_code = '', updated_at = NOW()
		WHERE id = $1
		  AND (redeem_code_id IS NULL OR redeem_code_id = $2)
		  AND (status IN ('pending', 'checking') OR (status = 'fulfilling' AND updated_at < $3))
		RETURNING `+nativeCheckoutOrderColumns, id, redeemCodeID, staleBefore)
	order, err := r.scanOrderRow(row)
	if err == nil {
		result, assignErr := tx.ExecContext(ctx, `UPDATE native_checkout_redeem_inventory SET
			assigned_order_id = COALESCE(assigned_order_id, $2),
			assigned_at = COALESCE(assigned_at, NOW())
			WHERE redeem_code_id = $1 AND offer_code = $3
			  AND (assigned_order_id IS NULL OR assigned_order_id = $2)`,
			redeemCodeID, order.ID, order.OfferCode)
		if assignErr != nil {
			return nil, false, assignErr
		}
		affected, assignErr := result.RowsAffected()
		if assignErr != nil {
			return nil, false, assignErr
		}
		if affected != 1 {
			return nil, false, errors.New("redeem code is not available for this native checkout offer")
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return order, true, nil
	}
	if !errors.Is(err, service.ErrNativeCheckoutOrderNotFound) {
		return nil, false, err
	}
	// Release the transaction before querying through the shared DB handle. On
	// a single-connection pool, leaving it open here would deadlock until the
	// caller's context expires.
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return nil, false, err
	}
	current, err := r.getOrderByID(ctx, id)
	return current, false, err
}

func (r *nativeCheckoutRepository) LinkRedeemCode(ctx context.Context, redeemCodeID int64, providerTradeNo string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE redeem_codes SET
		external_order_no = $2, updated_at = NOW()
		WHERE id = $1
		  AND (external_order_no IS NULL OR BTRIM(external_order_no) = '' OR external_order_no = $2)`,
		redeemCodeID, providerTradeNo)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("redeem code is linked to another order")
	}
	return nil
}

func (r *nativeCheckoutRepository) CompleteOrder(ctx context.Context, id int64) (*service.NativeCheckoutOrder, error) {
	row := r.db.QueryRowContext(ctx, `UPDATE native_checkout_orders SET
		status = 'completed', failure_code = '', completed_at = COALESCE(completed_at, NOW()),
		updated_at = NOW()
		WHERE id = $1 AND status = 'fulfilling'
		RETURNING `+nativeCheckoutOrderColumns, id)
	order, err := r.scanOrderRow(row)
	if err == nil {
		return order, nil
	}
	if !errors.Is(err, service.ErrNativeCheckoutOrderNotFound) {
		return nil, err
	}
	return r.getOrderByID(ctx, id)
}

func (r *nativeCheckoutRepository) scanOrderRow(row sqlScanner) (*service.NativeCheckoutOrder, error) {
	var order service.NativeCheckoutOrder
	var groupIDs []byte
	var redeemCodeID sql.NullInt64
	var fulfillmentStartedAt, completedAt sql.NullTime
	if err := row.Scan(
		&order.ID, &order.OrderNo, &order.UserID, &order.OfferCode, &order.Provider, &order.ProviderGoodsKey,
		&order.ProviderTradeNo, &order.PaymentURL, &order.PaymentMethod, &order.ContactHash, &order.ProductKind,
		&order.PayAmountCNYFen, &order.BenefitAmountCNYFen, &order.RedeemType,
		&order.RedeemValue, &order.RedeemPaidValue, &order.RedeemPurpose, &order.RedeemSalesStatus,
		&groupIDs, &order.RedeemValidityDays, &order.EnforceOnce, &order.Status, &redeemCodeID,
		&order.FailureCode, &order.CheckCount, &order.NextCheckAt, &fulfillmentStartedAt, &completedAt,
		&order.CreatedAt, &order.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrNativeCheckoutOrderNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(groupIDs, &order.RedeemGroupIDs); err != nil {
		return nil, fmt.Errorf("decode native checkout order groups: %w", err)
	}
	if redeemCodeID.Valid {
		order.RedeemCodeID = &redeemCodeID.Int64
	}
	if fulfillmentStartedAt.Valid {
		order.FulfillmentStartedAt = &fulfillmentStartedAt.Time
	}
	if completedAt.Valid {
		order.CompletedAt = &completedAt.Time
	}
	return &order, nil
}
