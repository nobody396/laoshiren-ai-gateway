//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/apikey"
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func (s *APIKeyRepoSuite) TestCreationGuardCountsDeletedKeysInEveryWindow() {
	for _, tc := range []struct {
		name  string
		count int
		age   time.Duration
	}{
		{"hour", 20, 0}, {"day", 100, 2 * time.Hour}, {"lifetime", 1000, 48 * time.Hour},
	} {
		s.Run(tc.name, func() {
			owner := s.mustCreateUser("creation-" + tc.name + "@test.com")
			now := time.Now()
			builders := make([]*dbent.APIKeyCreate, 0, tc.count)
			for i := 0; i < tc.count; i++ {
				builders = append(builders, s.client.APIKey.Create().SetUserID(owner.ID).SetKey(fmt.Sprintf("guard-%s-%d", tc.name, i)).SetName("deleted fixture").SetCreatedAt(now.Add(-tc.age)).SetDeletedAt(now))
			}
			_, err := s.client.APIKey.CreateBulk(builders...).Save(s.ctx)
			s.Require().NoError(err)
			key := &service.APIKey{UserID: owner.ID, Key: "guard-new-" + tc.name, Name: "replacement", Status: service.StatusActive}
			err = s.repo.Create(s.ctx, key)
			s.Require().Equal(429, errors.Code(err))
			s.Require().Equal("API_KEY_CREATION_LIMIT", errors.Reason(err))
			s.Require().ErrorContains(err, map[string]string{"hour": "hourly", "day": "daily", "lifetime": "lifetime"}[tc.name])
			s.Require().Zero(key.ID)
		})
	}
}

func (s *APIKeyRepoSuite) TestCreationGuardDeleteAndRecreateDoesNotResetBudget() {
	owner := s.mustCreateUser("delete-recreate@test.com")
	for i := 0; i < 20; i++ {
		key := &service.APIKey{UserID: owner.ID, Key: fmt.Sprintf("rotation-%d", i), Name: "rotated", Status: service.StatusActive}
		s.Require().NoError(s.repo.Create(s.ctx, key))
		s.Require().NoError(s.repo.Delete(s.ctx, key.ID))
	}
	err := s.repo.Create(s.ctx, &service.APIKey{UserID: owner.ID, Key: "rotation-21", Name: "blocked", Status: service.StatusActive})
	s.Require().Equal(429, errors.Code(err))
	count, err := s.client.APIKey.Query().Where(apikey.UserIDEQ(owner.ID)).Count(mixins.SkipSoftDelete(s.ctx))
	s.Require().NoError(err)
	s.Require().Equal(20, count)
}

func (s *APIKeyRepoSuite) TestCreationGuardWindowExpiry() {
	owner := s.mustCreateUser("creation-expiry@test.com")
	// Twenty old keys do not consume the hourly allowance.
	for i := 0; i < 20; i++ {
		_, err := s.client.APIKey.Create().SetUserID(owner.ID).SetName("old").SetKey(fmt.Sprintf("expiry-%d", i)).SetCreatedAt(time.Now().Add(-2 * time.Hour)).Save(s.ctx)
		s.Require().NoError(err)
	}
	key := &service.APIKey{UserID: owner.ID, Key: "expiry-new", Name: "new", Status: service.StatusActive}
	s.Require().NoError(s.repo.Create(s.ctx, key))
	count, err := s.client.APIKey.Query().Where(apikey.UserIDEQ(owner.ID)).Count(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(21, count, "old keys must not consume the hourly window")
}

func TestAPIKeyCreationGuardSerializesConcurrentCreates(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	owner, err := client.User.Create().SetEmail(fmt.Sprintf("creation-race-%d@test.com", time.Now().UnixNano())).SetPasswordHash("fixture").Save(ctx)
	require.NoError(t, err)
	repo := NewAPIKeyRepository(client, integrationDB)
	var success atomic.Int32
	errorsCh := make(chan error, 30)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			err := repo.Create(ctx, &service.APIKey{UserID: owner.ID, Key: fmt.Sprintf("race-%d-%d", owner.ID, i), Name: "race", Status: service.StatusActive})
			if err == nil {
				success.Add(1)
			} else {
				errorsCh <- err
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		require.Equal(t, 429, errors.Code(err))
	}
	require.Equal(t, int32(20), success.Load())
	count, err := client.APIKey.Query().Where(apikey.UserIDEQ(owner.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 20, count)
}

func (s *UserRepoSuite) TestIncludeDeletedIsAnExplicitAuditFilterNotAuthenticationAccess() {
	user := s.mustCreateUser(&service.User{Email: "deleted-audit@test.com"})
	s.Require().NoError(s.repo.Delete(s.ctx, user.ID))
	params := pagination.PaginationParams{Page: 1, PageSize: 20}
	rows, page, err := s.repo.ListWithFilters(s.ctx, params, service.UserListFilters{Search: "deleted-audit@test.com"})
	s.Require().NoError(err)
	s.Require().Empty(rows)
	s.Require().Zero(page.Total)
	rows, page, err = s.repo.ListWithFilters(s.ctx, params, service.UserListFilters{Search: "deleted-audit@test.com", IncludeDeleted: true})
	s.Require().NoError(err)
	s.Require().Len(rows, 1)
	s.Require().Equal(int64(1), page.Total)
	s.Require().NotNil(rows[0].DeletedAt)
	_, err = s.repo.GetByID(s.ctx, user.ID)
	s.Require().ErrorIs(err, service.ErrUserNotFound)
}

func TestAPIKeyCreationGuardFailedInsertDoesNotSpendBudget(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	owner, err := client.User.Create().SetEmail(fmt.Sprintf("creation-duplicate-%d@test.com", time.Now().UnixNano())).SetPasswordHash("fixture").Save(ctx)
	require.NoError(t, err)
	repo := NewAPIKeyRepository(client, integrationDB)
	key := func() *service.APIKey {
		return &service.APIKey{UserID: owner.ID, Key: fmt.Sprintf("duplicate-%d", owner.ID), Name: "new", Status: service.StatusActive}
	}
	require.NoError(t, repo.Create(ctx, key()))
	require.ErrorIs(t, repo.Create(ctx, key()), service.ErrAPIKeyExists)
	count, err := client.APIKey.Query().Where(apikey.UserIDEQ(owner.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
