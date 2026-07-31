//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type balanceUserRepoStub struct {
	*userRepoStub
	updateErr          error
	adjustmentErr      error
	adjustmentCalls    int
	adjustmentEligible bool
	updated            []*User
}

func (s *balanceUserRepoStub) Update(ctx context.Context, user *User) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	if user == nil {
		return nil
	}
	clone := *user
	s.updated = append(s.updated, &clone)
	if s.userRepoStub != nil {
		s.userRepoStub.user = &clone
	}
	return nil
}

func (s *balanceUserRepoStub) ApplyAdminBalanceAdjustment(
	_ context.Context,
	_ int64,
	amount float64,
	operation string,
) (*AdminBalanceAdjustmentResult, error) {
	s.adjustmentCalls++
	if s.adjustmentErr != nil {
		return nil, s.adjustmentErr
	}
	oldBalance := s.userRepoStub.user.Balance
	newBalance := oldBalance
	switch operation {
	case "set":
		newBalance = amount
	case "add":
		newBalance += amount
	case "subtract":
		newBalance -= amount
	}
	if newBalance < oldBalance && s.adjustmentEligible {
		return nil, ErrAdminBalanceSourceReversalRequired
	}
	s.userRepoStub.user.Balance = newBalance
	return &AdminBalanceAdjustmentResult{
		OldBalance: oldBalance,
		NewBalance: newBalance,
	}, nil
}

type balanceRedeemRepoStub struct {
	*redeemRepoStub
	created []*RedeemCode
}

func (s *balanceRedeemRepoStub) Create(ctx context.Context, code *RedeemCode) error {
	if code == nil {
		return nil
	}
	clone := *code
	s.created = append(s.created, &clone)
	return nil
}

type authCacheInvalidatorStub struct {
	userIDs  []int64
	groupIDs []int64
	keys     []string
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	s.keys = append(s.keys, key)
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}

func (s *authCacheInvalidatorStub) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	s.groupIDs = append(s.groupIDs, groupID)
}

func TestAdminService_UpdateUserBalance_InvalidatesAuthCache(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 5, "add", "")
	require.NoError(t, err)
	require.Equal(t, []int64{7}, invalidator.userIDs)
	require.Empty(t, redeemRepo.created)
}

func TestAdminService_UpdateUserBalance_NoChangeNoInvalidate(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{userRepoStub: baseRepo}
	redeemRepo := &balanceRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       redeemRepo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 10, "set", "")
	require.NoError(t, err)
	require.Empty(t, invalidator.userIDs)
	require.Empty(t, redeemRepo.created)
	require.Equal(t, 1, repo.adjustmentCalls)
}

func TestAdminService_UpdateUserBalance_SetDecreaseBlockedByEligibleLot(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{
		userRepoStub:       baseRepo,
		adjustmentEligible: true,
	}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		authCacheInvalidator: invalidator,
	}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 5, "set", "")
	require.ErrorIs(t, err, ErrAdminBalanceSourceReversalRequired)
	require.Equal(t, 10.0, baseRepo.user.Balance)
	require.Empty(t, invalidator.userIDs)
}

func TestAdminService_UpdateUserBalance_SubtractBlockedByEligibleLot(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{
		userRepoStub:       baseRepo,
		adjustmentEligible: true,
	}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.UpdateUserBalance(context.Background(), 7, 1, "subtract", "")
	require.ErrorIs(t, err, ErrAdminBalanceSourceReversalRequired)
	require.Equal(t, 10.0, baseRepo.user.Balance)
}

func TestAdminService_UpdateUserBalance_AddAllowedWithEligibleLot(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{
		userRepoStub:       baseRepo,
		adjustmentEligible: true,
	}
	svc := &adminServiceImpl{userRepo: repo}

	user, err := svc.UpdateUserBalance(context.Background(), 7, 2, "add", "")
	require.NoError(t, err)
	require.Equal(t, 12.0, user.Balance)
	require.Equal(t, 12.0, baseRepo.user.Balance)
}

func TestAdminService_UpdateUserBalance_SetSameValueAllowedWithEligibleLot(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 7, Balance: 10}}
	repo := &balanceUserRepoStub{
		userRepoStub:       baseRepo,
		adjustmentEligible: true,
	}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		authCacheInvalidator: invalidator,
	}

	user, err := svc.UpdateUserBalance(context.Background(), 7, 10, "set", "")
	require.NoError(t, err)
	require.Equal(t, 10.0, user.Balance)
	require.Empty(t, invalidator.userIDs)
}
