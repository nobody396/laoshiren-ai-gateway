package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	AgentSettlementStatusCompleted = "completed"
)

func (s *CommissionService) GetCommissionRates(ctx context.Context) (*CommissionRates, error) {
	if s.rateRepo == nil {
		return defaultCommissionRates(), nil
	}
	rates, err := s.rateRepo.GetCommissionRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("get commission rates: %w", err)
	}
	if rates == nil {
		return defaultCommissionRates(), nil
	}
	return rates, nil
}

func (s *CommissionService) UpdateCommissionRates(ctx context.Context, rates *CommissionRates) (*CommissionRates, error) {
	if s.rateRepo == nil {
		return nil, fmt.Errorf("commission rate repository is not configured")
	}
	if rates == nil {
		return nil, infraerrors.BadRequest("INVALID_COMMISSION_RATES", "commission rates are required")
	}
	if err := validateCommissionRate("consumption_rate", rates.ConsumptionRate); err != nil {
		return nil, err
	}
	if err := validateCommissionRate("first_recharge_invitee_rate", rates.FirstRechargeInviteeRate); err != nil {
		return nil, err
	}
	if err := validateCommissionRate("first_recharge_referral_rate", rates.FirstRechargeReferralRate); err != nil {
		return nil, err
	}
	if err := s.rateRepo.UpdateCommissionRates(ctx, rates); err != nil {
		return nil, fmt.Errorf("update commission rates: %w", err)
	}
	return s.GetCommissionRates(ctx)
}

func (s *CommissionService) GetAgentRateConfig(ctx context.Context, agentID int64) (*AgentRateConfig, error) {
	if s.rateRepo == nil {
		return nil, fmt.Errorf("commission rate repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	config, err := s.rateRepo.GetAgentRateConfig(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent rate config: %w", err)
	}
	if config == nil {
		rate, source := s.resolveAgentConsumptionRate(ctx, agentID)
		return &AgentRateConfig{
			AgentID:                  agentID,
			ConsumptionRate:          rate,
			Enabled:                  false,
			EffectiveConsumptionRate: rate,
			RateSource:               source,
		}, nil
	}
	return config, nil
}

func (s *CommissionService) UpdateAgentRateConfig(ctx context.Context, config *AgentRateConfig) (*AgentRateConfig, error) {
	if s.rateRepo == nil {
		return nil, fmt.Errorf("commission rate repository is not configured")
	}
	if config == nil || config.AgentID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_AGENT_RATE_CONFIG", "agent rate config is required")
	}
	if err := s.ensureAgent(ctx, config.AgentID); err != nil {
		return nil, err
	}
	if err := validateCommissionRate("consumption_rate", config.ConsumptionRate); err != nil {
		return nil, err
	}
	if err := s.rateRepo.UpsertAgentRateConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("update agent rate config: %w", err)
	}
	return s.GetAgentRateConfig(ctx, config.AgentID)
}

func (s *CommissionService) ListAdminAgents(ctx context.Context, params pagination.PaginationParams, filters AdminAgentListFilters) ([]AdminAgentSummary, *pagination.PaginationResult, error) {
	if s.adminRepo == nil {
		return nil, nil, fmt.Errorf("agent admin repository is not configured")
	}
	items, result, err := s.adminRepo.ListAdminAgents(ctx, params, filters)
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		enrichAgentSettlementStatus(&items[i])
	}
	return items, result, nil
}

func (s *CommissionService) GetAdminAgent(ctx context.Context, agentID int64, start, end *time.Time) (*AdminAgentSummary, error) {
	if s.adminRepo == nil {
		return nil, fmt.Errorf("agent admin repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	item, err := s.adminRepo.GetAdminAgent(ctx, agentID, start, end)
	if err != nil {
		return nil, err
	}
	enrichAgentSettlementStatus(item)
	return item, nil
}

func (s *CommissionService) ListAdminAgentUsers(ctx context.Context, agentID int64, params pagination.PaginationParams, start, end *time.Time) ([]AdminAgentUserStat, *pagination.PaginationResult, error) {
	if s.adminRepo == nil {
		return nil, nil, fmt.Errorf("agent admin repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, nil, err
	}
	return s.adminRepo.ListAdminAgentUsers(ctx, agentID, params, start, end)
}

func (s *CommissionService) BindUserToAgent(ctx context.Context, agentID int64, input BindAgentUserInput) (*BindAgentUserResult, error) {
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(input.Email)
	if email == "" {
		return nil, infraerrors.BadRequest("AGENT_BIND_USER_EMAIL_REQUIRED", "user email is required")
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user.ID == agentID {
		return nil, infraerrors.BadRequest("AGENT_BIND_SELF", "agent cannot be bound to itself")
	}
	if user.Role != RoleUser {
		return nil, infraerrors.BadRequest("AGENT_BIND_USER_ROLE_INVALID", "only regular users can be bound to an agent")
	}

	result := &BindAgentUserResult{
		UserID:          user.ID,
		Email:           user.Email,
		Username:        user.Username,
		AgentID:         agentID,
		PreviousAgentID: user.AgentID,
		InviterID:       user.InviterID,
	}

	if user.AgentID != nil {
		if *user.AgentID == agentID {
			result.AlreadyBound = true
		} else if !input.OverwriteAgent {
			return nil, infraerrors.Conflict("AGENT_BIND_USER_ALREADY_BOUND", "user is already bound to another agent")
		} else {
			result.Overwritten = true
		}
	}

	shouldSetInviter := user.InviterID == nil
	needsUpdate := user.AgentID == nil || *user.AgentID != agentID || shouldSetInviter
	if needsUpdate {
		if err := s.userRepo.AdminBindUserToAgent(ctx, user.ID, agentID, shouldSetInviter); err != nil {
			return nil, fmt.Errorf("bind user to agent: %w", err)
		}
		if shouldSetInviter {
			result.InviterID = &agentID
			result.InviterIDChanged = true
		}
	}

	return result, nil
}

func (s *CommissionService) ListAdminAgentCommissions(ctx context.Context, agentID int64, params pagination.PaginationParams, typeFilter string, start, end *time.Time) ([]AdminAgentCommissionRecord, *pagination.PaginationResult, error) {
	if s.adminRepo == nil {
		return nil, nil, fmt.Errorf("agent admin repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, nil, err
	}
	return s.adminRepo.ListAdminAgentCommissions(ctx, agentID, params, typeFilter, start, end)
}

func (s *CommissionService) ListAgentSettlements(ctx context.Context, agentID int64, params pagination.PaginationParams) ([]AgentSettlement, *pagination.PaginationResult, error) {
	if s.adminRepo == nil {
		return nil, nil, fmt.Errorf("agent admin repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, nil, err
	}
	return s.adminRepo.ListAgentSettlements(ctx, agentID, params)
}

func (s *CommissionService) CreateAgentSettlement(ctx context.Context, agentID, operatorID int64, amount float64, note, paymentReference string) (*AgentSettlement, error) {
	if s.adminRepo == nil {
		return nil, fmt.Errorf("agent admin repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, infraerrors.BadRequest("INVALID_SETTLEMENT_AMOUNT", "settlement amount must be greater than 0")
	}
	settings := s.getAgentSettlementSettings(ctx)
	if amount+agentSettlementAmountEpsilon < settings.MinimumAmount {
		return nil, infraerrors.BadRequest("SETTLEMENT_BELOW_MINIMUM", fmt.Sprintf("settlement amount must be at least %.2f", settings.MinimumAmount))
	}
	totalCommission, err := s.commissionRepo.SumByBeneficiaryAndPeriod(ctx, agentID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("sum agent commission: %w", err)
	}
	settledCommission, err := s.adminRepo.SumAgentSettlements(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("sum agent settlements: %w", err)
	}
	available := totalCommission - settledCommission
	if available < 0 {
		available = 0
	}
	if available+agentSettlementAmountEpsilon < settings.MinimumAmount {
		return nil, infraerrors.BadRequest("SETTLEMENT_BELOW_MINIMUM", fmt.Sprintf("unsettled commission must reach %.2f before settlement", settings.MinimumAmount))
	}
	if amount-available > agentSettlementAmountEpsilon {
		return nil, infraerrors.BadRequest("SETTLEMENT_EXCEEDS_UNSETTLED", "settlement amount exceeds unsettled commission")
	}

	if s.paymentRepo == nil {
		return nil, fmt.Errorf("agent payment repository is not configured")
	}
	profile, err := s.paymentRepo.GetAgentPaymentProfile(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent payment profile: %w", err)
	}
	if profile == nil {
		profile = emptyAgentPaymentProfile(agentID)
	}
	normalizeAgentPaymentProfile(profile)
	if !profile.Complete {
		return nil, infraerrors.BadRequest("AGENT_PAYMENT_PROFILE_INCOMPLETE", "agent payment profile is incomplete")
	}

	settlement := &AgentSettlement{
		AgentID:                agentID,
		Amount:                 amount,
		OperatorID:             operatorID,
		Note:                   strings.TrimSpace(note),
		Status:                 AgentSettlementStatusCompleted,
		PaymentAlipayRealName:  profile.AlipayRealName,
		PaymentAlipayAccount:   profile.AlipayAccount,
		PaymentContactPhone:    profile.ContactPhone,
		PaymentNote:            profile.PaymentNote,
		PaymentQRCodeObjectKey: profile.AlipayQRCodeObjectKey,
		PaymentReference:       strings.TrimSpace(paymentReference),
	}
	if err := s.adminRepo.CreateAgentSettlementIfAvailable(ctx, settlement); err != nil {
		return nil, fmt.Errorf("create agent settlement: %w", err)
	}
	return settlement, nil
}

func enrichAgentSettlementStatus(item *AdminAgentSummary) {
	if item == nil {
		return
	}
	if item.SettlementMinimumAmount <= 0 {
		item.SettlementMinimumAmount = defaultAgentSettlementMinimumAmount
	}
	item.SettlementGap = settlementGap(item.UnsettledCommission, item.SettlementMinimumAmount)
	item.SettlementEligible = item.PaymentProfileComplete && item.UnsettledCommission+agentSettlementAmountEpsilon >= item.SettlementMinimumAmount
}

func (s *CommissionService) ensureAgent(ctx context.Context, agentID int64) error {
	if agentID <= 0 {
		return infraerrors.BadRequest("INVALID_AGENT_ID", "invalid agent ID")
	}
	user, err := s.userRepo.GetByID(ctx, agentID)
	if err != nil {
		return err
	}
	if user.Role != RoleAgent {
		return infraerrors.BadRequest("USER_IS_NOT_AGENT", "user is not an agent")
	}
	return nil
}

func validateCommissionRate(field string, rate float64) error {
	if rate < 0 || rate > 1 {
		return infraerrors.BadRequest("INVALID_COMMISSION_RATE", fmt.Sprintf("%s must be between 0 and 1", field))
	}
	return nil
}
