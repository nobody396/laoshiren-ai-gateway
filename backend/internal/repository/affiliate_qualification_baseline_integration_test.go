//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAffiliateQualificationBaselineUsesActualInternalUnitsWithoutFX(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	cutoff := time.Now().UTC().Add(time.Minute)

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-baseline-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-affiliate-baseline-" + uuid.NewString(),
		Name:   "affiliate-baseline",
	})
	account := mustCreateAccount(t, client, &service.Account{
		Name: "affiliate-baseline-" + uuid.NewString(),
		Type: service.AccountTypeAPIKey,
	})

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO topup_orders (
			order_no, user_id, amount_cny_fen, pay_type,
			status, completed_at
		)
		VALUES ($1, $2, 10000, 'alipay', 'completed', $3)
	`, "BL"+uuid.NewString()[:20], user.ID, cutoff.Add(-2*time.Minute))
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetUserID(user.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID(uuid.NewString()).
		SetModel("baseline-test").
		SetTotalCost(700).
		SetActualCost(40).
		SetCreatedAt(cutoff.Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	var affected int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT refresh_affiliate_qualification_legacy_baseline($1)
	`, cutoff).Scan(&affected))
	require.GreaterOrEqual(t, affected, int64(1))

	var amountMicros int64
	var unit string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			confirmed_consumption_micros,
			metadata ->> 'unit'
		FROM affiliate_qualification_baseline_entries
		WHERE source_key = $1
	`, fmt.Sprintf("legacy-auto-v1:user:%d", user.ID)).Scan(
		&amountMicros,
		&unit,
	))
	require.Equal(t, int64(40_000_000), amountMicros)
	require.Equal(t, "1_internal_unit_equals_1_cny", unit)
}
