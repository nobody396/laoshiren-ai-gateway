//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateUser_UpdatesRoleAndInvalidatesAuthCache(t *testing.T) {
	baseRepo := &userRepoStub{
		user: &User{
			ID:          7,
			Email:       "user@example.com",
			Role:        RoleUser,
			Status:      StatusActive,
			Concurrency: 5,
		},
	}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		authCacheInvalidator: invalidator,
	}

	user, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{
		Role: RoleAgent,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, RoleAgent, user.Role)
	require.Len(t, repo.updated, 1)
	require.Equal(t, RoleAgent, repo.updated[0].Role)
	require.Equal(t, []int64{7}, invalidator.userIDs)
}

func TestAdminService_UpdateUser_RejectsAdminRoleDowngrade(t *testing.T) {
	baseRepo := &userRepoStub{
		user: &User{
			ID:          1,
			Email:       "admin@example.com",
			Role:        RoleAdmin,
			Status:      StatusActive,
			Concurrency: 5,
		},
	}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	svc := &adminServiceImpl{userRepo: repo}

	user, err := svc.UpdateUser(context.Background(), 1, &UpdateUserInput{
		Role: RoleAgent,
	})
	require.ErrorContains(t, err, "cannot change admin user role")
	require.Nil(t, user)
	require.Empty(t, repo.updated)
}
