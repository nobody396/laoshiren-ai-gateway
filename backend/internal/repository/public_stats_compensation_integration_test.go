//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPublicStatsCompensationSumsLedgerDynamically(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	prefix := uuid.NewString()

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx,
		`INSERT INTO users(email,password_hash) VALUES($1,'hash') RETURNING id`,
		fmt.Sprintf("public-comp-%s@example.com", prefix),
	).Scan(&userID))

	groupIDs := make([]int64, 3)
	for i := range groupIDs {
		require.NoError(t, tx.QueryRowContext(ctx,
			`INSERT INTO groups(name) VALUES($1) RETURNING id`,
			fmt.Sprintf("public-comp-%s-%d", prefix, i),
		).Scan(&groupIDs[i]))
	}

	// Legacy direct compensation is recognizable from its audited note.
	_, err := tx.ExecContext(ctx, `
INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,notes)
VALUES($1,'balance','admin_adjustment',100,'admin_manual','体感赔付批次 integration-test')`, userID)
	require.NoError(t, err)

	// Structured compensation is the preferred path for new executions.
	_, err = tx.ExecContext(ctx, `
INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,notes)
VALUES($1,'builder_pass_credit','compensation',20,'compensation_execution','approved benefit')`, userID)
	require.NoError(t, err)

	// An ordinary admin adjustment must never inflate the public counter.
	_, err = tx.ExecContext(ctx, `
INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,notes)
VALUES($1,'balance','admin_adjustment',500,'admin_manual','ordinary balance correction')`, userID)
	require.NoError(t, err)

	// The long-lived owned/internal probe identity is explicitly outside public
	// customer compensation statistics.
	_, err = tx.ExecContext(ctx, `
INSERT INTO users(id,email,password_hash)
VALUES(2,$1,'hash')
ON CONFLICT(id) DO NOTHING`, fmt.Sprintf("public-comp-internal-%s@example.com", prefix))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,notes)
VALUES(2,'balance','compensation',999,'compensation_execution','internal probe compensation')`)
	require.NoError(t, err)

	// Builder Pass writes one raw-credit row per linked group. One run is worth
	// 2*10=20 customer-visible CNY; a later 0.5*10=5 reversal leaves 15.
	for _, groupID := range groupIDs {
		_, err = tx.ExecContext(ctx, `
INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,reference_no,group_id,notes)
VALUES($1,'subscription','admin_adjustment',2,'admin_manual',$2,$3,'monthly grant')`,
			userID, "monthly-comp-"+prefix, groupID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
INSERT INTO account_change_records(user_id,asset_type,reason,delta,source_type,reference_no,group_id,notes)
VALUES($1,'subscription','admin_adjustment',-0.5,'admin_manual',$2,$3,'monthly reversal')`,
			userID, "monthly-comp-"+prefix+"-reversal", groupID)
		require.NoError(t, err)
	}

	repo := newDashboardAggregationRepositoryWithSQL(tx)
	total, err := repo.SumPublicCompensationCNY(ctx)
	require.NoError(t, err)
	require.InDelta(t, 135.0, total, 1e-9)
}
