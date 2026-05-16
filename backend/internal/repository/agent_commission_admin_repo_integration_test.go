//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAgentSettlementCreateIfAvailableD6ConcurrentOverpayment(t *testing.T) {
	ctx := context.Background()
	adminRepo := NewCommissionRepository(integrationEntClient, integrationDB).(service.AgentCommissionAdminRepository)

	agentID := createD6SettlementTestUser(t, service.RoleAgent)
	operatorID := createD6SettlementTestUser(t, service.RoleAdmin)
	userID := createD6SettlementTestUser(t, service.RoleUser)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM agent_settlements WHERE agent_id = $1`, agentID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM commission_records WHERE beneficiary_id = $1 OR user_id IN ($2, $3)`, agentID, userID, operatorID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3)`, agentID, operatorID, userID)
	})

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO commission_records (beneficiary_id, user_id, amount, source_amount, type, note)
		VALUES ($1, $2, 100, 100, $3, 'D6 concurrent settlement test')
	`, agentID, userID, service.CommissionTypeConsumption)
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs <- adminRepo.CreateAgentSettlementIfAvailable(ctx, &service.AgentSettlement{
				AgentID:    agentID,
				Amount:     75,
				OperatorID: operatorID,
				Note:       fmt.Sprintf("D6 concurrent settlement %d", i),
				Status:     service.AgentSettlementStatusCompleted,
			})
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	insufficient := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if infraerrors.Reason(err) == "SETTLEMENT_EXCEEDS_UNSETTLED" {
			insufficient++
			continue
		}
		require.NoError(t, err)
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, insufficient)

	var totalSettled float64
	var completedCount int
	err = integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0), COUNT(*)
		FROM agent_settlements
		WHERE agent_id = $1 AND status = 'completed'
	`, agentID).Scan(&totalSettled, &completedCount)
	require.NoError(t, err)
	require.LessOrEqual(t, totalSettled, 100.00000001)
	require.Equal(t, 1, completedCount)
}

func createD6SettlementTestUser(t *testing.T, role string) int64 {
	t.Helper()

	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO users (email, password_hash, role, status, balance, concurrency, created_at, updated_at)
		VALUES ($1, 'test-password-hash', $2, $3, 0, 5, NOW(), NOW())
		RETURNING id
	`, fmt.Sprintf("d6-settlement-%d@example.com", time.Now().UnixNano()), role, service.StatusActive).Scan(&id)
	require.NoError(t, err)
	return id
}
