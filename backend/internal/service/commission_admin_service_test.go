//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type bindUserRepoStub struct {
	mockUserRepo
	byID      map[int64]*User
	byEmail   map[string]*User
	bindCalls []bindUserRepoCall
	bindErr   error
}

type bindUserRepoCall struct {
	userID            int64
	agentID           int64
	setInviterIfEmpty bool
}

func (s *bindUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if u := s.byID[id]; u != nil {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (s *bindUserRepoStub) GetByEmail(_ context.Context, email string) (*User, error) {
	if u := s.byEmail[email]; u != nil {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (s *bindUserRepoStub) AdminBindUserToAgent(_ context.Context, userID, agentID int64, setInviterIfEmpty bool) error {
	s.bindCalls = append(s.bindCalls, bindUserRepoCall{
		userID:            userID,
		agentID:           agentID,
		setInviterIfEmpty: setInviterIfEmpty,
	})
	if s.bindErr != nil {
		return s.bindErr
	}
	for _, u := range s.byEmail {
		if u.ID == userID {
			u.AgentID = &agentID
			if setInviterIfEmpty && u.InviterID == nil {
				u.InviterID = &agentID
			}
		}
	}
	return nil
}

func TestCommissionServiceBindUserToAgentBindsUnassignedUser(t *testing.T) {
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Username: "user", Role: RoleUser},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: " user@example.com "})
	require.NoError(t, err)
	require.Equal(t, int64(42), result.UserID)
	require.Equal(t, int64(7), result.AgentID)
	require.Nil(t, result.PreviousAgentID)
	require.NotNil(t, result.InviterID)
	require.Equal(t, int64(7), *result.InviterID)
	require.True(t, result.InviterIDChanged)
	require.False(t, result.AlreadyBound)
	require.False(t, result.Overwritten)
	require.Equal(t, []bindUserRepoCall{{userID: 42, agentID: 7, setInviterIfEmpty: true}}, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentRejectsExistingOtherAgentWithoutOverwrite(t *testing.T) {
	otherAgentID := int64(8)
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Role: RoleUser, AgentID: &otherAgentID},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: "user@example.com"})
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Empty(t, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentOverwritePreservesExistingInviter(t *testing.T) {
	otherAgentID := int64(8)
	inviterID := int64(99)
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Role: RoleUser, AgentID: &otherAgentID, InviterID: &inviterID},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: "user@example.com", OverwriteAgent: true})
	require.NoError(t, err)
	require.NotNil(t, result.PreviousAgentID)
	require.Equal(t, otherAgentID, *result.PreviousAgentID)
	require.NotNil(t, result.InviterID)
	require.Equal(t, inviterID, *result.InviterID)
	require.False(t, result.InviterIDChanged)
	require.True(t, result.Overwritten)
	require.Equal(t, []bindUserRepoCall{{userID: 42, agentID: 7, setInviterIfEmpty: false}}, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentSameAgentIsIdempotent(t *testing.T) {
	agentID := int64(7)
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			agentID: {ID: agentID, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Role: RoleUser, AgentID: &agentID, InviterID: &agentID},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), agentID, BindAgentUserInput{Email: "user@example.com"})
	require.NoError(t, err)
	require.True(t, result.AlreadyBound)
	require.Empty(t, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentRejectsNonUserTarget(t *testing.T) {
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"admin@example.com": {ID: 42, Email: "admin@example.com", Role: RoleAdmin},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: "admin@example.com"})
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Empty(t, repo.bindCalls)
}

type agentSettlementAdminRepoStub struct {
	recordUsageCommissionRepoStub

	getAdminAgentCalls             int
	createAgentSettlementCalls     int
	createIfAvailableCalls         int
	createIfAvailableErr           error
	createIfAvailableSettlement    *AgentSettlement
	createIfAvailableSettlementID  int64
	createIfAvailableSettlementNow time.Time
	totalCommission                float64
	settledCommission              float64
	paymentProfile                 *AgentPaymentProfile
	settlementSettings             *AgentSettlementSettings
}

func (s *agentSettlementAdminRepoStub) ListAdminAgents(context.Context, pagination.PaginationParams, AdminAgentListFilters) ([]AdminAgentSummary, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *agentSettlementAdminRepoStub) GetAdminAgent(context.Context, int64, *time.Time, *time.Time) (*AdminAgentSummary, error) {
	s.getAdminAgentCalls++
	return &AdminAgentSummary{UnsettledCommission: 100}, nil
}

func (s *agentSettlementAdminRepoStub) ListAdminAgentUsers(context.Context, int64, pagination.PaginationParams, *time.Time, *time.Time) ([]AdminAgentUserStat, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *agentSettlementAdminRepoStub) ListAdminAgentCommissions(context.Context, int64, pagination.PaginationParams, string, *time.Time, *time.Time) ([]AdminAgentCommissionRecord, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *agentSettlementAdminRepoStub) ListAgentSettlements(context.Context, int64, pagination.PaginationParams) ([]AgentSettlement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *agentSettlementAdminRepoStub) SumAgentSettlements(context.Context, int64) (float64, error) {
	return s.settledCommission, nil
}

func (s *agentSettlementAdminRepoStub) CreateAgentSettlement(context.Context, *AgentSettlement) error {
	s.createAgentSettlementCalls++
	return nil
}

func (s *agentSettlementAdminRepoStub) CreateAgentSettlementIfAvailable(_ context.Context, settlement *AgentSettlement) error {
	s.createIfAvailableCalls++
	copied := *settlement
	s.createIfAvailableSettlement = &copied
	if s.createIfAvailableErr != nil {
		return s.createIfAvailableErr
	}
	if s.createIfAvailableSettlementID != 0 {
		settlement.ID = s.createIfAvailableSettlementID
	}
	if !s.createIfAvailableSettlementNow.IsZero() {
		settlement.CreatedAt = s.createIfAvailableSettlementNow
	}
	return nil
}

func (s *agentSettlementAdminRepoStub) SumByBeneficiaryAndPeriod(context.Context, int64, *time.Time, *time.Time) (float64, error) {
	return s.totalCommission, nil
}

func (s *agentSettlementAdminRepoStub) GetAgentSettlementSettings(context.Context) (*AgentSettlementSettings, error) {
	if s.settlementSettings != nil {
		return s.settlementSettings, nil
	}
	return &AgentSettlementSettings{MinimumAmount: 50}, nil
}

func (s *agentSettlementAdminRepoStub) UpdateAgentSettlementSettings(_ context.Context, settings *AgentSettlementSettings) error {
	copied := *settings
	copied.UpdatedAt = time.Now()
	s.settlementSettings = &copied
	settings.UpdatedAt = copied.UpdatedAt
	return nil
}

func (s *agentSettlementAdminRepoStub) GetAgentPaymentProfile(context.Context, int64) (*AgentPaymentProfile, error) {
	if s.paymentProfile == nil {
		return nil, nil
	}
	copied := *s.paymentProfile
	return &copied, nil
}

func (s *agentSettlementAdminRepoStub) UpsertAgentPaymentProfile(context.Context, *AgentPaymentProfile) error {
	return nil
}

func (s *agentSettlementAdminRepoStub) UpdateAgentPaymentQRCode(context.Context, int64, string, string, string, int64) (*AgentPaymentProfile, error) {
	return s.paymentProfile, nil
}

func TestCommissionServiceCreateAgentSettlementUsesAtomicRepositoryPath(t *testing.T) {
	userRepo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
	}
	now := time.Date(2026, 5, 16, 9, 30, 0, 0, time.UTC)
	adminRepo := &agentSettlementAdminRepoStub{
		createIfAvailableSettlementID:  123,
		createIfAvailableSettlementNow: now,
		totalCommission:                100,
		paymentProfile: &AgentPaymentProfile{
			AlipayRealName:        "张三",
			AlipayAccount:         "agent@example.com",
			AlipayQRCodeObjectKey: "agent-payment-qrcodes/7/test.png",
		},
	}
	svc := NewCommissionService(userRepo, adminRepo)

	settlement, err := svc.CreateAgentSettlement(context.Background(), 7, 99, 75, "manual payout", "alipay-20260517")
	require.NoError(t, err)
	require.Equal(t, int64(123), settlement.ID)
	require.Equal(t, now, settlement.CreatedAt)
	require.Equal(t, int64(7), settlement.AgentID)
	require.Equal(t, int64(99), settlement.OperatorID)
	require.Equal(t, 75.0, settlement.Amount)
	require.Equal(t, AgentSettlementStatusCompleted, settlement.Status)
	require.Equal(t, "manual payout", settlement.Note)
	require.Equal(t, "张三", settlement.PaymentAlipayRealName)
	require.Equal(t, "agent@example.com", settlement.PaymentAlipayAccount)
	require.Equal(t, "agent-payment-qrcodes/7/test.png", settlement.PaymentQRCodeObjectKey)
	require.Equal(t, "alipay-20260517", settlement.PaymentReference)
	require.Equal(t, 0, adminRepo.getAdminAgentCalls)
	require.Equal(t, 0, adminRepo.createAgentSettlementCalls)
	require.Equal(t, 1, adminRepo.createIfAvailableCalls)
	require.NotNil(t, adminRepo.createIfAvailableSettlement)
	require.Equal(t, 75.0, adminRepo.createIfAvailableSettlement.Amount)
}

func TestCommissionServiceCreateAgentSettlementRejectsNonPositiveAmount(t *testing.T) {
	userRepo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
	}
	adminRepo := &agentSettlementAdminRepoStub{}
	svc := NewCommissionService(userRepo, adminRepo)

	settlement, err := svc.CreateAgentSettlement(context.Background(), 7, 99, 0, "manual payout", "")
	require.Nil(t, settlement)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_SETTLEMENT_AMOUNT", infraerrors.Reason(err))
	require.Equal(t, 0, adminRepo.createIfAvailableCalls)
}

func TestCommissionServiceCreateAgentSettlementReturnsInsufficientUnsettledError(t *testing.T) {
	userRepo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
	}
	adminRepo := &agentSettlementAdminRepoStub{
		createIfAvailableErr: infraerrors.BadRequest("SETTLEMENT_EXCEEDS_UNSETTLED", "settlement amount exceeds unsettled commission"),
		totalCommission:      100,
		paymentProfile: &AgentPaymentProfile{
			AlipayRealName:        "张三",
			AlipayAccount:         "agent@example.com",
			AlipayQRCodeObjectKey: "agent-payment-qrcodes/7/test.png",
		},
	}
	svc := NewCommissionService(userRepo, adminRepo)

	settlement, err := svc.CreateAgentSettlement(context.Background(), 7, 99, 75, "manual payout", "")
	require.Nil(t, settlement)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "SETTLEMENT_EXCEEDS_UNSETTLED", infraerrors.Reason(err))
	require.Equal(t, 1, adminRepo.createIfAvailableCalls)
}

func TestCommissionServiceCreateAgentSettlementRejectsBelowMinimumUnsettled(t *testing.T) {
	userRepo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
	}
	adminRepo := &agentSettlementAdminRepoStub{
		totalCommission: 49.99,
		paymentProfile: &AgentPaymentProfile{
			AlipayRealName:        "张三",
			AlipayAccount:         "agent@example.com",
			AlipayQRCodeObjectKey: "agent-payment-qrcodes/7/test.png",
		},
	}
	svc := NewCommissionService(userRepo, adminRepo)

	settlement, err := svc.CreateAgentSettlement(context.Background(), 7, 99, 49.99, "manual payout", "")
	require.Nil(t, settlement)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "SETTLEMENT_BELOW_MINIMUM", infraerrors.Reason(err))
	require.Equal(t, 0, adminRepo.createIfAvailableCalls)
}

func TestCommissionServiceCreateAgentSettlementRejectsIncompletePaymentProfile(t *testing.T) {
	userRepo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
	}
	adminRepo := &agentSettlementAdminRepoStub{
		totalCommission: 100,
		paymentProfile: &AgentPaymentProfile{
			AlipayRealName: "张三",
		},
	}
	svc := NewCommissionService(userRepo, adminRepo)

	settlement, err := svc.CreateAgentSettlement(context.Background(), 7, 99, 75, "manual payout", "")
	require.Nil(t, settlement)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "AGENT_PAYMENT_PROFILE_INCOMPLETE", infraerrors.Reason(err))
	require.Equal(t, 0, adminRepo.createIfAvailableCalls)
}
