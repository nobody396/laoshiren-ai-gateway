//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func setupSchedulerOutboxBindingFixture(t *testing.T) (*accountRepository, int64, []int64) {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	groups := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		group, err := integrationEntClient.Group.Create().
			SetName(fmt.Sprintf("u9-group-%d-%d", suffix, i)).
			SetPlatform(service.PlatformOpenAI).
			SetStatus(service.StatusActive).
			Save(ctx)
		require.NoError(t, err)
		groups = append(groups, group.ID)
	}
	account, err := integrationEntClient.Account.Create().
		SetName(fmt.Sprintf("u9-account-%d", suffix)).
		SetPlatform(service.PlatformOpenAI).
		SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).
		SetSchedulable(true).
		SetCredentials(map[string]any{}).
		SetExtra(map[string]any{}).
		Save(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE account_id = $1 OR group_id = ANY($2)`, account.ID, pq.Array(groups))
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id = $1`, account.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id = $1`, account.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id = ANY($1)`, pq.Array(groups))
	})
	return newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil), account.ID, groups
}

func outboxEventsForAccount(t *testing.T, accountID int64) []service.SchedulerOutboxEvent {
	t.Helper()
	events, err := NewSchedulerOutboxRepository(integrationDB).ListAfter(context.Background(), 0, 1000)
	require.NoError(t, err)
	filtered := make([]service.SchedulerOutboxEvent, 0)
	for _, event := range events {
		if event.AccountID != nil && *event.AccountID == accountID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func TestSchedulerOutbox_BindGroupsCommitsNonEmptyAndEmptyEvents(t *testing.T) {
	repo, accountID, groups := setupSchedulerOutboxBindingFixture(t)
	ctx := context.Background()

	require.NoError(t, repo.BindGroups(ctx, accountID, []int64{groups[0], groups[1]}))
	require.NoError(t, repo.BindGroups(ctx, accountID, nil))
	events := outboxEventsForAccount(t, accountID)
	require.Len(t, events, 2, "two different state payloads must both remain durable")
	require.Equal(t, service.SchedulerOutboxEventAccountGroupsChanged, events[0].EventType)
	require.ElementsMatch(t, []int64{groups[0], groups[1]}, parseSchedulerGroupIDs(events[0].Payload))
	require.ElementsMatch(t, []int64{groups[0], groups[1]}, parseSchedulerGroupIDs(events[1].Payload), "empty binding must invalidate old groups")

	bound, err := repo.GetGroups(ctx, accountID)
	require.NoError(t, err)
	require.Empty(t, bound)
}

func TestSchedulerOutbox_FailedInsertRollsBackBinding(t *testing.T) {
	repo, accountID, groups := setupSchedulerOutboxBindingFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.BindGroups(ctx, accountID, []int64{groups[0]}))

	_, err := integrationDB.ExecContext(ctx, `ALTER TABLE scheduler_outbox RENAME TO scheduler_outbox_u9_unavailable`)
	require.NoError(t, err)
	restored := false
	t.Cleanup(func() {
		if !restored {
			_, _ = integrationDB.ExecContext(context.Background(), `ALTER TABLE scheduler_outbox_u9_unavailable RENAME TO scheduler_outbox`)
		}
	})

	err = repo.BindGroups(ctx, accountID, []int64{groups[1]})
	require.Error(t, err)
	_, restoreErr := integrationDB.ExecContext(ctx, `ALTER TABLE scheduler_outbox_u9_unavailable RENAME TO scheduler_outbox`)
	require.NoError(t, restoreErr)
	restored = true

	bound, err := repo.GetGroups(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, bound, 1)
	require.Equal(t, groups[0], bound[0].ID, "business relation must roll back with failed outbox insert")
}

func TestSchedulerOutbox_ConcurrentBindingsSerialize(t *testing.T) {
	repo, accountID, groups := setupSchedulerOutboxBindingFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.BindGroups(ctx, accountID, []int64{groups[0]}))

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, groupID := range groups[1:] {
		groupID := groupID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- repo.BindGroups(ctx, accountID, []int64{groupID})
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	bound, err := repo.GetGroups(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, bound, 1)
	require.Contains(t, []int64{groups[1], groups[2]}, bound[0].ID)
	events := outboxEventsForAccount(t, accountID)
	require.Len(t, events, 3)
}

func parseSchedulerGroupIDs(payload map[string]any) []int64 {
	raw, _ := payload["group_ids"].([]any)
	result := make([]int64, 0, len(raw))
	for _, value := range raw {
		if number, ok := value.(float64); ok {
			result = append(result, int64(number))
		}
	}
	return result
}
