//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type teamSettlementUserRepoStub struct {
	UserRepository
	users        map[int64]*User
	deductUserID int64
	deductAmount float64
}

func (s *teamSettlementUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	user := s.users[id]
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *teamSettlementUserRepoStub) DeductBalance(_ context.Context, id int64, amount float64) error {
	s.deductUserID = id
	s.deductAmount = amount
	return nil
}

func TestGPTImageTaskSettlementUsesCapturedTeamOwnerAfterLifecycleChanges(t *testing.T) {
	const ownerID int64 = 1
	const memberID int64 = 2
	teamID := int64(11)
	groupID := int64(12)
	price := 0.25
	member := &User{ID: memberID, Status: StatusDisabled}
	owner := &User{ID: ownerID, Status: StatusActive}
	key := &APIKey{
		ID: 31, UserID: memberID, TeamID: &teamID, Status: StatusAPIKeyDisabled,
		User: member, ActorUser: member, GroupID: &groupID,
		Group: &Group{ID: groupID, Platform: PlatformGPTImage, RateMultiplier: 1, GPTImageCallPrice: &price},
	}
	apiKeyRepo := &authRepoStub{getByID: func(context.Context, int64) (*APIKey, error) { return key, nil }}
	userRepo := &teamSettlementUserRepoStub{users: map[int64]*User{ownerID: owner, memberID: member}}
	apiKeyService := NewAPIKeyService(apiKeyRepo, userRepo, nil, nil, nil, nil,
		&config.Config{Team: config.TeamConfig{Enabled: false}})
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	gateway := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	settlement := &GPTImageTaskSettlementService{apiKeyService: apiKeyService, gatewayService: gateway}

	err := settlement.settleClaimedUsage(context.Background(), "task-team-1", &GPTImagePendingTaskUsage{
		TaskID: "task-team-1", APIKeyID: key.ID, UserID: ownerID,
		Account: Account{ID: 41, Platform: PlatformGPTImage, Type: AccountTypeAPIKey},
		Model:   "gpt-image-2", UpstreamModel: "gpt-image-2", Resolution: "2K", ImageCount: 1,
	}, &OpenAIForwardResult{
		Model: "gpt-image-2", UpstreamModel: "gpt-image-2", ImageCount: 1,
		ImageSize: "2K", Duration: time.Second,
	}, 1)

	require.NoError(t, err)
	require.Equal(t, ownerID, userRepo.deductUserID)
	require.NotEqual(t, memberID, userRepo.deductUserID)
	require.Greater(t, userRepo.deductAmount, 0.0)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, ownerID, usageRepo.lastLog.UserID)
	require.NotNil(t, usageRepo.lastLog.AccountingCommand)
	require.Equal(t, ownerID, usageRepo.lastLog.AccountingCommand.UserID)
	require.Equal(t, memberID, key.ActorUser.ID)
}
