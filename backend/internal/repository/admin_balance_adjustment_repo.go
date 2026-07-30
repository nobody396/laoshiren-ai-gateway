package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func (r *userRepository) ApplyAdminBalanceAdjustment(
	ctx context.Context,
	userID int64,
	amount float64,
	operation string,
) (_ *service.AdminBalanceAdjustmentResult, err error) {
	if r == nil || r.client == nil {
		return nil, errors.New("user repository client is nil")
	}

	var result *service.AdminBalanceAdjustmentResult
	err = withinEntTransaction(ctx, r.client, func(client *dbent.Client) error {
		driver := client.Driver()

		var oldBalance float64
		userRows := &entsql.Rows{}
		// Paid balance grants and usage billing both mutate users.balance before
		// inserting/consuming their FIFO lot. Locking the user row first keeps
		// this guard ordered with both paths and closes the check/update race.
		if err := driver.Query(ctx, `
			SELECT balance
			FROM users
			WHERE id = $1
				AND deleted_at IS NULL
			FOR UPDATE
		`, []any{userID}, userRows); err != nil {
			return err
		}
		if !userRows.Next() {
			rowErr := userRows.Err()
			_ = userRows.Close()
			if rowErr != nil {
				return rowErr
			}
			return service.ErrUserNotFound
		}
		if err := userRows.Scan(&oldBalance); err != nil {
			_ = userRows.Close()
			return err
		}
		if err := userRows.Close(); err != nil {
			return err
		}

		newBalance := oldBalance
		switch operation {
		case "set":
			newBalance = amount
		case "add":
			newBalance += amount
		case "subtract":
			newBalance -= amount
		default:
			return fmt.Errorf("unsupported admin balance operation %q", operation)
		}
		if newBalance < 0 {
			return fmt.Errorf(
				"balance cannot be negative, current balance: %.2f, requested operation would result in: %.2f",
				oldBalance,
				newBalance,
			)
		}

		result = &service.AdminBalanceAdjustmentResult{
			OldBalance: oldBalance,
			NewBalance: newBalance,
		}
		if newBalance == oldBalance {
			return nil
		}

		if newBalance < oldBalance {
			lotRows := &entsql.Rows{}
			if err := driver.Query(ctx, `
				SELECT id
				FROM balance_lots
				WHERE user_id = $1
					AND remaining_amount_micros > 0
					AND affiliate_eligible = TRUE
				ORDER BY id
				LIMIT 1
				FOR UPDATE
			`, []any{userID}, lotRows); err != nil {
				return err
			}
			hasEligibleLot := lotRows.Next()
			if !hasEligibleLot {
				rowErr := lotRows.Err()
				_ = lotRows.Close()
				if rowErr != nil {
					return rowErr
				}
			} else if err := lotRows.Close(); err != nil {
				return err
			}
			if hasEligibleLot {
				return service.ErrAdminBalanceSourceReversalRequired
			}
		}

		var updateResult sql.Result
		if err := driver.Exec(ctx, `
			UPDATE users
			SET balance = $2,
				updated_at = NOW()
			WHERE id = $1
				AND deleted_at IS NULL
		`, []any{userID, newBalance}, &updateResult); err != nil {
			return err
		}
		affected, err := updateResult.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return service.ErrUserNotFound
		}
		if addedMicros := service.AffiliateMicrosFromFloat(newBalance - oldBalance); addedMicros > 0 {
			// Manual credits are a distinct FIFO source. They must never borrow
			// the affiliate policy of a later paid lot.
			var lotResult sql.Result
			if err := driver.Exec(ctx, `
				WITH next_lot AS (
					SELECT nextval(pg_get_serial_sequence('balance_lots', 'id')) AS id
				)
				INSERT INTO balance_lots (
					id, user_id, source_type, source_key,
					original_amount_micros, remaining_amount_micros,
					affiliate_eligible, affiliate_policy,
					direct_partner_id,
					customer_rebate_rate_bps, partner_commission_rate_bps,
					occurred_at
				)
				SELECT
					id, $1, 'admin_adjustment', 'admin_adjustment:' || id::text,
					$2, $2,
					FALSE, 'NONE',
					NULL,
					0, 0,
					NOW()
				FROM next_lot
			`, []any{userID, addedMicros}, &lotResult); err != nil {
				return err
			}
			affected, err := lotResult.RowsAffected()
			if err != nil {
				return err
			}
			if affected != 1 {
				return errors.New("failed to record non-affiliate admin balance lot")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
