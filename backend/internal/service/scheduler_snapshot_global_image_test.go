package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type globalImageSnapshotRepoStub struct {
	AccountRepository
	allOpenAI     []Account
	groupOpenAI   []Account
	platformCalls int
	groupCalls    int
}

func (r *globalImageSnapshotRepoStub) ListSchedulableByPlatform(_ context.Context, platform string) ([]Account, error) {
	r.platformCalls++
	if platform != PlatformOpenAI {
		return nil, nil
	}
	return append([]Account(nil), r.allOpenAI...), nil
}

func (r *globalImageSnapshotRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]Account, error) {
	r.groupCalls++
	if platform != PlatformOpenAI {
		return nil, nil
	}
	return append([]Account(nil), r.groupOpenAI...), nil
}

type globalImageSnapshotCacheStub struct {
	SchedulerCache
	snapshots map[SchedulerBucket][]Account
	setCalls  map[SchedulerBucket]int
}

func newGlobalImageSnapshotCacheStub() *globalImageSnapshotCacheStub {
	return &globalImageSnapshotCacheStub{
		snapshots: make(map[SchedulerBucket][]Account),
		setCalls:  make(map[SchedulerBucket]int),
	}
}

func (c *globalImageSnapshotCacheStub) GetSnapshot(_ context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	accounts, ok := c.snapshots[bucket]
	if !ok {
		return nil, false, nil
	}
	result := make([]*Account, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		result = append(result, &account)
	}
	return result, true, nil
}

func (c *globalImageSnapshotCacheStub) SetSnapshot(_ context.Context, bucket SchedulerBucket, accounts []Account) error {
	c.snapshots[bucket] = append([]Account(nil), accounts...)
	c.setCalls[bucket]++
	return nil
}

func (c *globalImageSnapshotCacheStub) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return true, nil
}

func (c *globalImageSnapshotCacheStub) UnlockBucket(context.Context, SchedulerBucket) error {
	return nil
}

func TestSchedulerSnapshotService_GlobalImagePoolIsIndependentFromGroupSnapshot(t *testing.T) {
	ctx := context.Background()
	textAccount := Account{ID: 23, Platform: PlatformOpenAI}
	imageAccount := Account{ID: 38, Platform: PlatformOpenAI}
	repo := &globalImageSnapshotRepoStub{
		allOpenAI:   []Account{textAccount, imageAccount},
		groupOpenAI: []Account{textAccount},
	}
	cache := newGlobalImageSnapshotCacheStub()
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)

	globalAccounts, err := svc.ListGlobalImageAccounts(ctx, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, []int64{23, 38}, []int64{globalAccounts[0].ID, globalAccounts[1].ID})
	require.Equal(t, 1, repo.platformCalls)

	groupID := int64(6)
	groupAccounts, _, err := svc.ListSchedulableAccounts(ctx, &groupID, PlatformOpenAI, false)
	require.NoError(t, err)
	require.Len(t, groupAccounts, 1)
	require.Equal(t, int64(23), groupAccounts[0].ID)
	require.Equal(t, 1, repo.groupCalls)

	_, err = svc.ListGlobalImageAccounts(ctx, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, 1, repo.platformCalls, "global pool must reuse its own cache bucket")
	require.Equal(t, 1, cache.setCalls[SchedulerBucket{GroupID: 0, Platform: PlatformOpenAI, Mode: SchedulerModeGlobalImage}])
}

func TestSchedulerSnapshotService_OpenAIAccountChangeRebuildsGlobalImagePool(t *testing.T) {
	ctx := context.Background()
	repo := &globalImageSnapshotRepoStub{allOpenAI: []Account{{ID: 38, Platform: PlatformOpenAI}}}
	cache := newGlobalImageSnapshotCacheStub()
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)
	bucket := SchedulerBucket{GroupID: 0, Platform: PlatformOpenAI, Mode: SchedulerModeGlobalImage}

	require.NoError(t, svc.rebuildGlobalImageBucket(ctx, "account_change", make(map[batchSeenKey]struct{})))
	require.Equal(t, []Account{{ID: 38, Platform: PlatformOpenAI}}, cache.snapshots[bucket])

	repo.allOpenAI = []Account{{ID: 40, Platform: PlatformOpenAI}}
	require.NoError(t, svc.rebuildByAccount(ctx, &Account{ID: 40, Platform: PlatformOpenAI}, []int64{6}, "account_change", make(map[batchSeenKey]struct{})))
	require.Equal(t, []Account{{ID: 40, Platform: PlatformOpenAI}}, cache.snapshots[bucket])
	require.Equal(t, 2, cache.setCalls[bucket])
}
