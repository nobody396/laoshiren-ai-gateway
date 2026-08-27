//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type teamContextErrorRepository struct {
	TeamRepository
	err error
}

type teamContextRepositoryStub struct {
	TeamRepository
	teamContext *TeamContext
}

func (r *teamContextRepositoryStub) GetContextByUserID(context.Context, int64) (*TeamContext, error) {
	return r.teamContext, nil
}

type teamBillingUserRepositoryStub struct {
	UserRepository
	users map[int64]*User
}

func (r *teamBillingUserRepositoryStub) GetByID(_ context.Context, id int64) (*User, error) {
	return r.users[id], nil
}

func (r *teamContextErrorRepository) GetContextByUserID(context.Context, int64) (*TeamContext, error) {
	return nil, r.err
}

func validTeamAPIKeyForLifecycleTest() *APIKey {
	teamID := int64(11)
	createdAt := time.Now()
	return &APIKey{
		ID:        31,
		UserID:    2,
		TeamID:    &teamID,
		Status:    StatusAPIKeyActive,
		CreatedAt: createdAt,
		Team:      &Team{ID: teamID, Status: TeamStatusActive},
		TeamMembership: &TeamMembership{
			TeamID:   teamID,
			UserID:   2,
			Role:     TeamRoleMember,
			JoinedAt: createdAt.Add(-time.Minute),
		},
		ActorUser: &User{ID: 2, Status: StatusActive},
		User:      &User{ID: 1, Status: StatusActive},
	}
}

func TestAPIKeyOwnerLockAlwaysDisablesKey(t *testing.T) {
	key := validTeamAPIKeyForLifecycleTest()
	require.True(t, key.IsActive())

	key.TeamOwnerDisabled = true
	require.False(t, key.IsActive())
}

func TestValidateTeamKeyLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*APIKey)
		cfg    *config.Config
		want   error
	}{
		{name: "valid", cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}},
		{name: "feature_disabled", cfg: &config.Config{}, want: ErrTeamFeatureDisabled},
		{name: "membership_missing", cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}, mutate: func(key *APIKey) { key.TeamMembership = nil }, want: ErrTeamMembershipRequired},
		{name: "membership_rejoined_after_key", cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}, mutate: func(key *APIKey) { key.TeamMembership.JoinedAt = key.CreatedAt.Add(time.Second) }, want: ErrTeamMembershipRequired},
		{name: "team_suspended", cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}, mutate: func(key *APIKey) { key.Team.Status = TeamStatusSuspended }, want: ErrTeamSuspended},
		{name: "actor_inactive", cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}, mutate: func(key *APIKey) { key.ActorUser.Status = StatusDisabled }, want: ErrTeamActorInactive},
		{name: "owner_inactive", cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}, mutate: func(key *APIKey) { key.User.Status = StatusDisabled }, want: ErrTeamBillingOwnerInactive},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := validTeamAPIKeyForLifecycleTest()
			if test.mutate != nil {
				test.mutate(key)
			}
			err := (&APIKeyService{cfg: test.cfg}).ValidateTeamKeyLifecycle(key)
			if test.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestCheckTeamMemberLimitsStopsNextRequestAtSettledThreshold(t *testing.T) {
	svc := &APIKeyService{cfg: &config.Config{Team: config.TeamConfig{Enabled: true}}}
	for _, test := range []struct {
		name  string
		apply func(*TeamMembership)
		want  error
	}{
		{name: "below", apply: func(m *TeamMembership) { m.DailyLimitUSD, m.DailyUsageUSD = 1, 0.99 }},
		{name: "daily", apply: func(m *TeamMembership) { m.DailyLimitUSD, m.DailyUsageUSD = 1, 1 }, want: ErrTeamMemberDailyExceeded},
		{name: "weekly", apply: func(m *TeamMembership) { m.WeeklyLimitUSD, m.WeeklyUsageUSD = 5, 5 }, want: ErrTeamMemberWeeklyExceeded},
		{name: "monthly", apply: func(m *TeamMembership) { m.MonthlyLimitUSD, m.MonthlyUsageUSD = 20, 20 }, want: ErrTeamMemberMonthlyExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			key := validTeamAPIKeyForLifecycleTest()
			test.apply(key.TeamMembership)
			err := svc.CheckTeamMemberLimits(key)
			if test.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.want)
			}
		})
	}
}

func TestHydrateTeamAPIKeyOnlyMapsMissingContextToMembershipError(t *testing.T) {
	repositoryFailure := errors.New("team repository unavailable")
	tests := []struct {
		name    string
		repoErr error
		want    error
	}{
		{name: "team_missing", repoErr: ErrTeamNotFound, want: ErrTeamMembershipRequired},
		{name: "repository_failure", repoErr: repositoryFailure, want: repositoryFailure},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := validTeamAPIKeyForLifecycleTest()
			key.Team = nil
			key.TeamMembership = nil
			key.ActorUser = nil
			key.User = nil
			service := &APIKeyService{
				teamRepo: &teamContextErrorRepository{err: test.repoErr},
				cfg:      &config.Config{Team: config.TeamConfig{Enabled: true}},
			}

			_, err := service.hydrateTeamAPIKey(context.Background(), key, nil)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestTeamAPIKeyAuthSnapshotRoundTripPreservesLifecycleContext(t *testing.T) {
	svc := &APIKeyService{}
	key := validTeamAPIKeyForLifecycleTest()
	key.Key = "sk-team-roundtrip"
	key.User = &User{ID: 1, Status: StatusActive, Role: RoleUser, Balance: 10, Concurrency: 5}
	key.ActorUser = &User{ID: 2, Status: StatusActive, Role: RoleUser, Concurrency: 2}

	snapshot := svc.snapshotFromAPIKey(key)
	restored := svc.snapshotToAPIKey(key.Key, snapshot)

	require.NotNil(t, restored.TeamID)
	require.Equal(t, *key.TeamID, *restored.TeamID)
	require.Equal(t, key.CreatedAt, restored.CreatedAt)
	require.Equal(t, key.ActorUser.ID, restored.ActorUser.ID)
	require.Equal(t, key.User.ID, restored.User.ID)
	require.Equal(t, key.Team.ID, restored.Team.ID)
	require.Equal(t, key.TeamMembership.UserID, restored.TeamMembership.UserID)
	require.NoError(t, svc.ValidateTeamKeyLifecycle(restored))
}

func TestAPIKeyServiceGetByIDHydratesCurrentTeamOwnerForAsyncBilling(t *testing.T) {
	key := validTeamAPIKeyForLifecycleTest()
	member := &User{ID: 2, Status: StatusActive}
	owner := &User{ID: 1, Status: StatusActive}
	key.User = member
	key.ActorUser = member
	repo := &authRepoStub{getByID: func(context.Context, int64) (*APIKey, error) { return key, nil }}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &teamBillingUserRepositoryStub{users: map[int64]*User{member.ID: member, owner.ID: owner}},
		teamRepo: &teamContextRepositoryStub{teamContext: &TeamContext{
			Team: key.Team, Membership: key.TeamMembership,
			Owner: &TeamMembership{TeamID: *key.TeamID, UserID: owner.ID, Role: TeamRoleOwner},
		}},
		cfg: &config.Config{Team: config.TeamConfig{Enabled: true}},
	}

	got, err := svc.GetByID(context.Background(), key.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, got.User.ID)
	require.Equal(t, member.ID, got.ActorUser.ID)
	require.Equal(t, owner.ID, apiKeyBillingUserID(got))
}

func TestHistoricalBillingUsesCapturedPayerWithoutCurrentTeamLifecycle(t *testing.T) {
	key := validTeamAPIKeyForLifecycleTest()
	key.Status = StatusAPIKeyDisabled
	member := &User{ID: 2, Status: StatusDisabled}
	owner := &User{ID: 1, Status: StatusActive}
	key.User = member
	repo := &authRepoStub{getByID: func(context.Context, int64) (*APIKey, error) { return key, nil }}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &teamBillingUserRepositoryStub{users: map[int64]*User{member.ID: member, owner.ID: owner}},
		cfg:        &config.Config{Team: config.TeamConfig{Enabled: false}},
	}

	got, err := svc.GetByIDForHistoricalBilling(context.Background(), key.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, got.User.ID)
	require.Equal(t, member.ID, got.ActorUser.ID)
	require.Equal(t, owner.ID, apiKeyBillingUserID(got))
}
