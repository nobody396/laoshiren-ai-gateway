//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type inviteActivityUserRepoStub struct {
	mockUserRepo
	users             map[int64]*User
	markTopupAffected int
	balanceUpdates    []inviteActivityBalanceUpdate
}

type inviteActivityBalanceUpdate struct {
	userID int64
	amount float64
}

func (s *inviteActivityUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if u := s.users[id]; u != nil {
		copied := *u
		return &copied, nil
	}
	return nil, ErrUserNotFound
}

func (s *inviteActivityUserRepoStub) UpdateBalance(_ context.Context, id int64, amount float64) error {
	s.balanceUpdates = append(s.balanceUpdates, inviteActivityBalanceUpdate{userID: id, amount: amount})
	return nil
}

func (s *inviteActivityUserRepoStub) MarkFirstInvitedTopup(context.Context, int64, int64) (int, error) {
	return s.markTopupAffected, nil
}

type inviteActivityRepoStub struct {
	recordUsageCommissionRepoStub
	rates    *CommissionRates
	activity *InviteActivityConfig
}

func (s *inviteActivityRepoStub) GetCommissionRates(context.Context) (*CommissionRates, error) {
	if s.rates != nil {
		copied := *s.rates
		return &copied, nil
	}
	return defaultCommissionRates(), nil
}

func (s *inviteActivityRepoStub) UpdateCommissionRates(_ context.Context, rates *CommissionRates) error {
	copied := *rates
	s.rates = &copied
	return nil
}

func (s *inviteActivityRepoStub) GetAgentRateConfig(context.Context, int64) (*AgentRateConfig, error) {
	return nil, nil
}

func (s *inviteActivityRepoStub) UpsertAgentRateConfig(context.Context, *AgentRateConfig) error {
	return nil
}

func (s *inviteActivityRepoStub) ResolveAgentConsumptionRate(context.Context, int64) (float64, string, error) {
	return 0.06, "global", nil
}

func (s *inviteActivityRepoStub) GetInviteActivityConfig(context.Context) (*InviteActivityConfig, error) {
	if s.activity != nil {
		copied := *s.activity
		return &copied, nil
	}
	return defaultInviteActivityConfig(), nil
}

func (s *inviteActivityRepoStub) UpdateInviteActivityConfig(_ context.Context, activity *InviteActivityConfig) error {
	copied := *activity
	s.activity = &copied
	return nil
}

func TestCommissionServiceInviteActivityRegistrationBonusGrantsActiveInvitedUser(t *testing.T) {
	now := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	inviterID := int64(7)
	userRepo := &inviteActivityUserRepoStub{
		users: map[int64]*User{
			42: {ID: 42, Role: RoleUser, InviterID: &inviterID, CreatedAt: now},
		},
	}
	commissionRepo := &inviteActivityRepoStub{
		activity: &InviteActivityConfig{
			Enabled:                 true,
			Name:                    "公测活动",
			StartAt:                 &start,
			EndAt:                   &end,
			RegistrationBonusAmount: 5,
		},
	}
	svc := NewCommissionService(userRepo, commissionRepo)
	svc.nowFunc = func() time.Time { return now }

	err := svc.ProcessInviteActivityRegistrationBonus(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, []inviteActivityBalanceUpdate{{userID: 42, amount: 5}}, userRepo.balanceUpdates)
	require.Len(t, commissionRepo.records, 1)
	require.Equal(t, CommissionTypeInviteActivityRegistrationBonus, commissionRepo.records[0].Type)
	require.Equal(t, int64(42), commissionRepo.records[0].BeneficiaryID)
	require.Equal(t, 5.0, commissionRepo.records[0].Amount)
	require.Equal(t, "invite_activity", commissionRepo.records[0].RateSource)
	require.Contains(t, *commissionRepo.records[0].Note, "公测活动")
}

func TestCommissionServiceInviteActivityDoesNotOverrideFirstInvitedTopupBonus(t *testing.T) {
	now := time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC)
	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	agentID := int64(7)
	userRepo := &inviteActivityUserRepoStub{
		markTopupAffected: 1,
		users: map[int64]*User{
			42: {ID: 42, Role: RoleUser, AgentID: &agentID, CreatedAt: start.Add(2 * time.Hour)},
		},
	}
	commissionRepo := &inviteActivityRepoStub{
		rates: &CommissionRates{
			ConsumptionRate:           0.06,
			FirstRechargeInviteeRate:  0.01,
			FirstRechargeReferralRate: 0.05,
		},
		activity: &InviteActivityConfig{
			Enabled:                 true,
			Name:                    "公测活动",
			StartAt:                 &start,
			EndAt:                   &end,
			RegistrationBonusAmount: 5,
		},
	}
	svc := NewCommissionService(userRepo, commissionRepo)
	svc.nowFunc = func() time.Time { return now }

	err := svc.ProcessFirstInvitedTopupBonus(context.Background(), 42, 20, 1001)
	require.NoError(t, err)
	require.Equal(t, []inviteActivityBalanceUpdate{{userID: 42, amount: 0.2}}, userRepo.balanceUpdates)
	require.Len(t, commissionRepo.records, 1)
	require.Equal(t, CommissionTypeFirstRechargeInvitee, commissionRepo.records[0].Type)
	require.Equal(t, 0.01, commissionRepo.records[0].Rate)
	require.Equal(t, "global", commissionRepo.records[0].RateSource)
}

func TestCommissionServiceActiveInviteActivityEmailWhitelistUsesActivityList(t *testing.T) {
	now := time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC)
	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	commissionRepo := &inviteActivityRepoStub{
		activity: &InviteActivityConfig{
			Enabled:                 true,
			Name:                    "公测活动",
			StartAt:                 &start,
			EndAt:                   &end,
			RegistrationBonusAmount: 5,
			EmailRestrictionEnabled: true,
			EmailSuffixWhitelist:    []string{"qq.com", "@GMAIL.com"},
		},
	}
	svc := NewCommissionService(&inviteActivityUserRepoStub{}, commissionRepo)
	svc.nowFunc = func() time.Time { return now }

	whitelist, active := svc.ActiveInviteActivityEmailSuffixWhitelist(context.Background(), []string{"@fallback.com"})

	require.True(t, active)
	require.Equal(t, []string{"@qq.com", "@gmail.com"}, whitelist)
}

func TestCommissionServiceActiveInviteActivityEmailWhitelistFallsBackToSettingsList(t *testing.T) {
	now := time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC)
	start := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	commissionRepo := &inviteActivityRepoStub{
		activity: &InviteActivityConfig{
			Enabled:                 true,
			Name:                    "公测活动",
			StartAt:                 &start,
			EndAt:                   &end,
			RegistrationBonusAmount: 5,
			EmailRestrictionEnabled: true,
		},
	}
	svc := NewCommissionService(&inviteActivityUserRepoStub{}, commissionRepo)
	svc.nowFunc = func() time.Time { return now }

	whitelist, active := svc.ActiveInviteActivityEmailSuffixWhitelist(context.Background(), []string{"@QQ.com"})

	require.True(t, active)
	require.Equal(t, []string{"@qq.com"}, whitelist)
}
