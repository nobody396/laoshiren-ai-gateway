//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type outboxReplayRepoStub struct {
	events []SchedulerOutboxEvent
}

func (s *outboxReplayRepoStub) ListAfter(context.Context, int64, int) ([]SchedulerOutboxEvent, error) {
	return s.events, nil
}
func (s *outboxReplayRepoStub) MaxID(context.Context) (int64, error) {
	if len(s.events) == 0 {
		return 0, nil
	}
	return s.events[len(s.events)-1].ID, nil
}

type outboxReplayCacheStub struct {
	watermark          int64
	watermarkWrites    []int64
	updateLastUsedErr  error
	updateLastUsedRuns int
}

func (s *outboxReplayCacheStub) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return nil, false, nil
}
func (s *outboxReplayCacheStub) SetSnapshot(context.Context, SchedulerBucket, []Account) error {
	return nil
}
func (s *outboxReplayCacheStub) GetAccount(context.Context, int64) (*Account, error) { return nil, nil }
func (s *outboxReplayCacheStub) SetAccount(context.Context, *Account) error          { return nil }
func (s *outboxReplayCacheStub) DeleteAccount(context.Context, int64) error          { return nil }
func (s *outboxReplayCacheStub) UpdateLastUsed(context.Context, map[int64]time.Time) error {
	s.updateLastUsedRuns++
	return s.updateLastUsedErr
}
func (s *outboxReplayCacheStub) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return true, nil
}
func (s *outboxReplayCacheStub) UnlockBucket(context.Context, SchedulerBucket) error { return nil }
func (s *outboxReplayCacheStub) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	return []SchedulerBucket{{Platform: PlatformOpenAI, Mode: SchedulerModeSingle}}, nil
}
func (s *outboxReplayCacheStub) GetOutboxWatermark(context.Context) (int64, error) {
	return s.watermark, nil
}
func (s *outboxReplayCacheStub) SetOutboxWatermark(_ context.Context, id int64) error {
	s.watermark = id
	s.watermarkWrites = append(s.watermarkWrites, id)
	return nil
}

func TestSchedulerSnapshotService_FailedEventDoesNotAdvanceWatermark(t *testing.T) {
	cache := &outboxReplayCacheStub{updateLastUsedErr: errors.New("redis unavailable")}
	repo := &outboxReplayRepoStub{events: []SchedulerOutboxEvent{{
		ID:        17,
		EventType: SchedulerOutboxEventAccountLastUsed,
		Payload: map[string]any{
			"last_used": map[string]any{"42": float64(time.Now().Unix())},
		},
	}}}
	svc := NewSchedulerSnapshotService(cache, repo, nil, nil, nil)

	svc.pollOutbox()
	require.Zero(t, cache.watermark)
	require.Empty(t, cache.watermarkWrites)
	require.Equal(t, 1, cache.updateLastUsedRuns)

	cache.updateLastUsedErr = nil
	svc.pollOutbox()
	require.Equal(t, int64(17), cache.watermark)
	require.Equal(t, []int64{17}, cache.watermarkWrites)
	require.Equal(t, 2, cache.updateLastUsedRuns, "failed delivery must replay idempotently")
}
