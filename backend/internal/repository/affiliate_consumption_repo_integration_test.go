//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAffiliateConsumptionRepository_ReusesEntTransaction(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateConsumptionRepository(client)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-consumption-tx-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	sourceKey := "affiliate-consumption-tx:" + uuid.NewString()

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, tx)
	require.NoError(t, repo.RecordBalanceLot(txCtx, service.AffiliateBalanceLotInput{
		UserID:          user.ID,
		SourceType:      service.AffiliateSourcePaidRedeem,
		SourceID:        987,
		SourceKey:       sourceKey,
		AmountMicros:    20_000_000,
		AffiliatePolicy: service.AffiliateSourcePolicyNone,
		OccurredAt:      time.Now(),
	}))
	require.NoError(t, tx.Rollback())

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM balance_lots
		WHERE source_key = $1
	`, sourceKey).Scan(&count))
	require.Zero(t, count, "raw attribution write must roll back with the owning Ent transaction")
}
