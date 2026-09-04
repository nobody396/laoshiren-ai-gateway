package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// ListGroupModelInventory projects model_mapping only. Do not reuse the
// schedulable/active-account listing: temporary errors must not change the
// selected billing group or trigger a subscription-to-wallet fallback.
func (r *accountRepository) ListGroupModelInventory(ctx context.Context, groupID int64) ([]service.AccountModelInventory, error) {
	if r.sql == nil {
		return nil, errors.New("model inventory storage unavailable")
	}
	rows, err := r.sql.QueryContext(ctx, `
 SELECT a.platform, a.type, COALESCE(a.credentials->'model_mapping', '{}'::jsonb)
 FROM accounts a JOIN account_groups ag ON ag.account_id=a.id
 WHERE ag.group_id=$1 AND a.deleted_at IS NULL
 ORDER BY a.id`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	inventory := []service.AccountModelInventory{}
	for rows.Next() {
		var row service.AccountModelInventory
		var mapping []byte
		if err := rows.Scan(&row.Platform, &row.Type, &mapping); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(mapping, &row.ModelMapping); err != nil {
			return nil, errors.New("invalid model inventory declaration")
		}
		inventory = append(inventory, row)
	}
	return inventory, rows.Err()
}
