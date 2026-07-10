//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func accountingUsageFixture(t *testing.T, suffix string) (*usageLogRepository, *service.UsageLog) {
	t.Helper()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("accounting-%s-%d@example.com", suffix, time.Now().UnixNano()), Balance: 100})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-accounting-" + uuid.NewString(), Name: "accounting"})
	account := mustCreateAccount(t, client, &service.Account{Name: "accounting-" + suffix + "-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	requestID := "accounting-" + uuid.NewString()
	log := &service.UsageLog{
		UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
		RequestID: requestID, Model: "test-model", InputTokens: 1,
		TotalCost: 0.01, ActualCost: 0.01, CreatedAt: time.Now().UTC(),
		AccountingCommand: &service.UsageBillingCommand{
			RequestID: requestID, APIKeyID: apiKey.ID, UserID: user.ID,
			AccountID: account.ID, AccountType: service.AccountTypeAPIKey,
			BalanceCost: 0.01,
		},
	}
	return newUsageLogRepositoryWithSQL(client, integrationDB), log
}

func TestAccountingCommand_UsageLogAndCommandCommitTogether(t *testing.T) {
	ctx := context.Background()
	usageRepo, log := accountingUsageFixture(t, "commit")
	inserted, err := usageRepo.Create(ctx, log)
	require.NoError(t, err)
	require.True(t, inserted)
	require.NotZero(t, log.ID)

	command, err := NewAccountingCommandRepository(integrationDB).GetByUsageLogID(ctx, log.ID)
	require.NoError(t, err)
	require.Equal(t, service.AccountingCommandStatusPending, command.Status)
	require.Equal(t, log.RequestID, command.Payload.RequestID)
	require.Equal(t, log.ID, command.Payload.UsageLogID)

	var payloadText string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload::text FROM usage_accounting_commands WHERE id = $1`, command.ID).Scan(&payloadText))
	require.NotContains(t, payloadText, "prompt")
	require.NotContains(t, payloadText, "jwt")
	require.NotContains(t, payloadText, "api_key_value")
}

func TestAccountingCommand_EnqueueFailureRollsBackUsageLog(t *testing.T) {
	ctx := context.Background()
	usageRepo, log := accountingUsageFixture(t, "rollback")
	_, err := integrationDB.ExecContext(ctx, `ALTER TABLE usage_accounting_commands RENAME TO usage_accounting_commands_u10_unavailable`)
	require.NoError(t, err)
	restored := false
	t.Cleanup(func() {
		if !restored {
			_, _ = integrationDB.ExecContext(context.Background(), `ALTER TABLE usage_accounting_commands_u10_unavailable RENAME TO usage_accounting_commands`)
		}
	})

	_, err = usageRepo.Create(ctx, log)
	require.Error(t, err)
	_, restoreErr := integrationDB.ExecContext(ctx, `ALTER TABLE usage_accounting_commands_u10_unavailable RENAME TO usage_accounting_commands`)
	require.NoError(t, restoreErr)
	restored = true

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2`, log.RequestID, log.APIKeyID).Scan(&count))
	require.Zero(t, count)
}

func TestAccountingCommand_LeaseExpiryAndSkipLocked(t *testing.T) {
	ctx := context.Background()
	usageRepo, log := accountingUsageFixture(t, "lease")
	_, err := usageRepo.Create(ctx, log)
	require.NoError(t, err)
	repo := NewAccountingCommandRepository(integrationDB)

	first, err := repo.ClaimDue(ctx, "worker-a", time.Minute, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	second, err := repo.ClaimDue(ctx, "worker-b", time.Minute, 1)
	require.NoError(t, err)
	require.Empty(t, second, "active lease must not be processed concurrently")

	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_accounting_commands SET lease_expires_at = NOW() - INTERVAL '1 second' WHERE id = $1`, first[0].ID)
	require.NoError(t, err)
	replayed, err := repo.ClaimDue(ctx, "worker-b", time.Minute, 1)
	require.NoError(t, err)
	require.Len(t, replayed, 1)
	require.Equal(t, first[0].ID, replayed[0].ID)
	require.Equal(t, 2, replayed[0].Attempts)
	require.NoError(t, repo.Complete(ctx, replayed[0].ID, "worker-b"))
}

func TestAccountingCommand_MultipleWorkersClaimDistinctRows(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		usageRepo, log := accountingUsageFixture(t, fmt.Sprintf("multi-%d", i))
		_, err := usageRepo.Create(ctx, log)
		require.NoError(t, err)
	}
	repo := NewAccountingCommandRepository(integrationDB)
	var wg sync.WaitGroup
	claimed := make(chan int64, 16)
	for i := 0; i < 2; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			commands, err := repo.ClaimDue(ctx, workerID, time.Minute, 8)
			require.NoError(t, err)
			for _, command := range commands {
				claimed <- command.ID
			}
		}()
	}
	wg.Wait()
	close(claimed)
	seen := map[int64]struct{}{}
	for id := range claimed {
		_, duplicate := seen[id]
		require.False(t, duplicate)
		seen[id] = struct{}{}
	}
	require.GreaterOrEqual(t, len(seen), 8)
}
