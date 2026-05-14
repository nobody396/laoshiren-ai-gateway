package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type balanceAlertCacheStub struct {
	notified           bool
	dispatchLocked     bool
	acquireAllowed     bool
	cachedConfig       *BalanceAlertConfig
	globalEnabled      *bool
	globalThreshold    *float64
	clearNotifiedCount int
	clearDispatchCount int
}

func (s *balanceAlertCacheStub) IsNotified(context.Context, int64) (bool, error) {
	return s.notified, nil
}

func (s *balanceAlertCacheStub) SetNotified(context.Context, int64) error {
	s.notified = true
	return nil
}

func (s *balanceAlertCacheStub) ClearNotified(context.Context, int64) error {
	s.notified = false
	s.clearNotifiedCount++
	return nil
}

func (s *balanceAlertCacheStub) AcquireDispatchLock(context.Context, int64) (bool, error) {
	if s.dispatchLocked || !s.acquireAllowed {
		return false, nil
	}
	s.dispatchLocked = true
	return true, nil
}

func (s *balanceAlertCacheStub) HasDispatchLock(context.Context, int64) (bool, error) {
	return s.dispatchLocked, nil
}

func (s *balanceAlertCacheStub) ClearDispatchLock(context.Context, int64) error {
	s.dispatchLocked = false
	s.clearDispatchCount++
	return nil
}

func (s *balanceAlertCacheStub) GetCachedConfig(context.Context, int64) (*BalanceAlertConfig, error) {
	return s.cachedConfig, nil
}

func (s *balanceAlertCacheStub) SetCachedConfig(_ context.Context, _ int64, cfg *BalanceAlertConfig) error {
	if cfg == nil {
		s.cachedConfig = nil
		return nil
	}
	copyCfg := *cfg
	s.cachedConfig = &copyCfg
	return nil
}

func (s *balanceAlertCacheStub) ClearCachedConfig(context.Context, int64) error {
	s.cachedConfig = nil
	return nil
}

func (s *balanceAlertCacheStub) GetGlobalEnabled(context.Context) (*bool, error) {
	return s.globalEnabled, nil
}

func (s *balanceAlertCacheStub) SetGlobalEnabled(_ context.Context, enabled bool) error {
	s.globalEnabled = &enabled
	return nil
}

func (s *balanceAlertCacheStub) GetGlobalThreshold(context.Context) (*float64, error) {
	return s.globalThreshold, nil
}

func (s *balanceAlertCacheStub) SetGlobalThreshold(_ context.Context, threshold float64) error {
	s.globalThreshold = &threshold
	return nil
}

func (s *balanceAlertCacheStub) ClearGlobalCache(context.Context) error {
	s.globalEnabled = nil
	s.globalThreshold = nil
	return nil
}

type balanceAlertDefRepoStub struct {
	defs []UserAttributeDefinition
}

func (s *balanceAlertDefRepoStub) Create(context.Context, *UserAttributeDefinition) error {
	panic("unexpected call")
}

func (s *balanceAlertDefRepoStub) GetByID(context.Context, int64) (*UserAttributeDefinition, error) {
	panic("unexpected call")
}

func (s *balanceAlertDefRepoStub) GetByKey(context.Context, string) (*UserAttributeDefinition, error) {
	panic("unexpected call")
}

func (s *balanceAlertDefRepoStub) Update(context.Context, *UserAttributeDefinition) error {
	panic("unexpected call")
}

func (s *balanceAlertDefRepoStub) Delete(context.Context, int64) error {
	panic("unexpected call")
}

func (s *balanceAlertDefRepoStub) List(context.Context, bool) ([]UserAttributeDefinition, error) {
	return s.defs, nil
}

func (s *balanceAlertDefRepoStub) UpdateDisplayOrders(context.Context, map[int64]int) error {
	panic("unexpected call")
}

func (s *balanceAlertDefRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	panic("unexpected call")
}

type balanceAlertValRepoStub struct {
	values []UserAttributeValue
}

func (s *balanceAlertValRepoStub) GetByUserID(context.Context, int64) ([]UserAttributeValue, error) {
	return s.values, nil
}

func (s *balanceAlertValRepoStub) GetByUserIDs(context.Context, []int64) ([]UserAttributeValue, error) {
	panic("unexpected call")
}

func (s *balanceAlertValRepoStub) UpsertBatch(context.Context, int64, []UpdateUserAttributeInput) error {
	panic("unexpected call")
}

func (s *balanceAlertValRepoStub) DeleteByAttributeID(context.Context, int64) error {
	panic("unexpected call")
}

func (s *balanceAlertValRepoStub) DeleteByUserID(context.Context, int64) error {
	panic("unexpected call")
}

type balanceAlertSettingRepoStub struct {
	values map[string]string
	errs   map[string]error
}

func (s *balanceAlertSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected call")
}

func (s *balanceAlertSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if err := s.errs[key]; err != nil {
		return "", err
	}
	if val, ok := s.values[key]; ok {
		return val, nil
	}
	return "", ErrSettingNotFound
}

func (s *balanceAlertSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected call")
}

func (s *balanceAlertSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected call")
}

func (s *balanceAlertSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected call")
}

func (s *balanceAlertSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected call")
}

func (s *balanceAlertSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected call")
}

type balanceAlertUserReaderStub struct {
	user *User
}

func (s *balanceAlertUserReaderStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

func TestBalanceAlertServiceCheckAndAlertQueuesTask(t *testing.T) {
	cache := &balanceAlertCacheStub{acquireAllowed: true}
	queue := &EmailQueueService{taskChan: make(chan EmailTask, 1)}
	svc := NewBalanceAlertService(
		cache,
		&balanceAlertDefRepoStub{defs: []UserAttributeDefinition{
			{ID: 1, Key: attrKeyBalanceAlertEnabled},
			{ID: 2, Key: attrKeyBalanceAlertThreshold},
			{ID: 3, Key: attrKeyBalanceAlertEmail},
		}},
		&balanceAlertValRepoStub{},
		&balanceAlertSettingRepoStub{values: map[string]string{
			SettingKeyBalanceAlertEnabled:          "true",
			SettingKeyBalanceAlertDefaultThreshold: "5.00",
			SettingKeySiteName:                     "DragonCode",
			SettingKeyFrontendURL:                  "https://example.com",
		}},
		queue,
		&balanceAlertUserReaderStub{user: &User{Email: "user@example.com", Username: "test-user"}},
	)

	err := svc.checkAndAlertInternal(context.Background(), 42, 4.25)
	require.NoError(t, err)
	require.True(t, cache.dispatchLocked)

	select {
	case task := <-queue.taskChan:
		require.Equal(t, int64(42), task.AlertUserID)
		require.Equal(t, "user@example.com", task.Email)
		require.Equal(t, "DragonCode", task.SiteName)
		require.Equal(t, "test-user", task.Username)
		require.Equal(t, "4.25", task.Balance)
		require.Equal(t, "5.00", task.AlertThreshold)
		require.Equal(t, "https://example.com/redeem", task.TopUpURL)
	default:
		t.Fatal("expected balance alert task to be queued")
	}
}

func TestBalanceAlertServiceCheckAndAlertSkipsWhenDispatchLocked(t *testing.T) {
	cache := &balanceAlertCacheStub{dispatchLocked: true, acquireAllowed: true}
	queue := &EmailQueueService{taskChan: make(chan EmailTask, 1)}
	svc := NewBalanceAlertService(
		cache,
		&balanceAlertDefRepoStub{defs: []UserAttributeDefinition{
			{ID: 1, Key: attrKeyBalanceAlertEnabled},
		}},
		&balanceAlertValRepoStub{},
		&balanceAlertSettingRepoStub{values: map[string]string{
			SettingKeyBalanceAlertEnabled:          "true",
			SettingKeyBalanceAlertDefaultThreshold: "5.00",
		}},
		queue,
		&balanceAlertUserReaderStub{user: &User{Email: "user@example.com"}},
	)

	err := svc.checkAndAlertInternal(context.Background(), 42, 4.00)
	require.NoError(t, err)

	select {
	case <-queue.taskChan:
		t.Fatal("did not expect task to be queued while dispatch lock is held")
	default:
	}
}

func TestBalanceAlertServiceResetNotifiedFlagClearsDispatchState(t *testing.T) {
	cache := &balanceAlertCacheStub{notified: true, dispatchLocked: true}
	svc := NewBalanceAlertService(
		cache,
		&balanceAlertDefRepoStub{defs: []UserAttributeDefinition{
			{ID: 1, Key: attrKeyBalanceAlertEnabled},
			{ID: 2, Key: attrKeyBalanceAlertThreshold},
		}},
		&balanceAlertValRepoStub{},
		&balanceAlertSettingRepoStub{values: map[string]string{
			SettingKeyBalanceAlertEnabled:          "true",
			SettingKeyBalanceAlertDefaultThreshold: "5.00",
		}},
		nil,
		&balanceAlertUserReaderStub{user: &User{Email: "user@example.com"}},
	)

	svc.ResetNotifiedFlag(context.Background(), 42, 8.00)

	require.False(t, cache.notified)
	require.False(t, cache.dispatchLocked)
	require.Equal(t, 1, cache.clearNotifiedCount)
	require.Equal(t, 1, cache.clearDispatchCount)
}

func TestBalanceAlertServiceGetGlobalEnabledDefaultsTrueWhenSettingMissing(t *testing.T) {
	cache := &balanceAlertCacheStub{}
	svc := NewBalanceAlertService(
		cache,
		&balanceAlertDefRepoStub{},
		&balanceAlertValRepoStub{},
		&balanceAlertSettingRepoStub{},
		nil,
		&balanceAlertUserReaderStub{user: &User{Email: "user@example.com"}},
	)

	enabled, err := svc.getGlobalEnabled(context.Background())
	require.NoError(t, err)
	require.True(t, enabled)
	require.NotNil(t, cache.globalEnabled)
	require.True(t, *cache.globalEnabled)
}
