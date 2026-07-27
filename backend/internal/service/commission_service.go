package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	apptimezone "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/timezone"
)

const (
	// inviteCodeCharset 邀请码字符集（大写字母 + 数字，去除易混淆字符 O/0/I/1）
	inviteCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	inviteCodeLen     = 8

	// 分佣比例
	commissionRateConsumption           = 0.06 // 代理商消耗长效分润 6%
	commissionRateFirstRechargeInvitee  = 0.10 // 被邀请用户首充奖励 10%
	commissionRateFirstRechargeReferral = 0.05 // 普通用户邀请首充奖励 5%
)

// CommissionService 分佣业务服务
type CommissionService struct {
	userRepo       UserRepository
	commissionRepo CommissionRepository
	rateRepo       CommissionRateRepository
	activityRepo   InviteActivityRepository
	adminRepo      AgentCommissionAdminRepository
	levelRepo      AgentLevelRepository
	paymentRepo    AgentPaymentRepository
	paymentReview  AgentPaymentReviewRepository
	affiliateLinks AffiliateLinkRepository
	nowFunc        func() time.Time
}

// NewCommissionService 创建分佣服务实例
func NewCommissionService(userRepo UserRepository, commissionRepo CommissionRepository) *CommissionService {
	s := &CommissionService{
		userRepo:       userRepo,
		commissionRepo: commissionRepo,
		nowFunc:        apptimezone.Now,
	}
	if repo, ok := commissionRepo.(CommissionRateRepository); ok {
		s.rateRepo = repo
	}
	if repo, ok := commissionRepo.(InviteActivityRepository); ok {
		s.activityRepo = repo
	}
	if repo, ok := commissionRepo.(AgentCommissionAdminRepository); ok {
		s.adminRepo = repo
	}
	if repo, ok := commissionRepo.(AgentLevelRepository); ok {
		s.levelRepo = repo
	}
	if repo, ok := commissionRepo.(AgentPaymentRepository); ok {
		s.paymentRepo = repo
	}
	if repo, ok := commissionRepo.(AgentPaymentReviewRepository); ok {
		s.paymentReview = repo
	}
	return s
}

func (s *CommissionService) SetAffiliateLinkRepository(repo AffiliateLinkRepository) {
	if s != nil {
		s.affiliateLinks = repo
	}
}

// GetOrCreateInviteCode 获取或生成用户的邀请码
// 如果用户已有邀请码则直接返回，否则生成一个唯一码并持久化
func (s *CommissionService) GetOrCreateInviteCode(ctx context.Context, userID int64) (string, error) {
	existing, err := s.userRepo.GetInviteCodeByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get invite code: %w", err)
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	// 生成唯一邀请码（最多重试 10 次）
	for i := 0; i < 10; i++ {
		code, err := generateInviteCode()
		if err != nil {
			return "", fmt.Errorf("generate invite code: %w", err)
		}
		// 检查是否已被使用
		_, err = s.userRepo.GetByInviteCode(ctx, code)
		if err != nil {
			// ErrUserNotFound 表示码未被使用，可以用
			if isNotFound(err) {
				if setErr := s.userRepo.SetInviteCode(ctx, userID, code); setErr != nil {
					return "", fmt.Errorf("set invite code: %w", setErr)
				}
				return code, nil
			}
			return "", fmt.Errorf("check invite code: %w", err)
		}
		// 码已被使用，重试
	}
	return "", fmt.Errorf("failed to generate unique invite code after retries")
}

// ValidateAndGetInviter 验证邀请码并返回邀请人信息
func (s *CommissionService) ValidateAndGetInviter(ctx context.Context, inviteCode string) (*User, error) {
	if inviteCode == "" {
		return nil, nil
	}
	user, err := s.userRepo.GetByInviteCode(ctx, inviteCode)
	if err != nil {
		if isNotFound(err) {
			if s.affiliateLinks == nil {
				return nil, nil
			}
			referral, linkErr := s.affiliateLinks.ResolveActiveLink(ctx, inviteCode)
			if errors.Is(linkErr, ErrAffiliateLinkNotFound) {
				return nil, nil
			}
			if linkErr != nil {
				return nil, fmt.Errorf("resolve affiliate link: %w", linkErr)
			}
			inviter, inviterErr := s.userRepo.GetByID(ctx, referral.AgentID)
			if inviterErr != nil {
				return nil, fmt.Errorf("get affiliate agent: %w", inviterErr)
			}
			return inviter, nil
		}
		return nil, fmt.Errorf("get inviter by code: %w", err)
	}
	if user.Role == RoleAgent && s.affiliateLinks != nil {
		_, linkErr := s.affiliateLinks.ResolveDefaultAgentLink(ctx, user.ID)
		if linkErr == nil {
			return user, nil
		}
		if !errors.Is(linkErr, ErrAffiliateLinkNotFound) {
			return nil, fmt.Errorf("resolve default affiliate link: %w", linkErr)
		}
	}
	return user, nil
}

// BindReferralCode resolves one code into exactly one permanent direct edge.
// Ordinary user codes and agent campaign links intentionally do not stack.
func (s *CommissionService) BindReferralCode(ctx context.Context, userID int64, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	inviter, err := s.userRepo.GetByInviteCode(ctx, code)
	if err == nil {
		if inviter.Role == RoleAgent && s.affiliateLinks != nil {
			referral, linkErr := s.affiliateLinks.ResolveDefaultAgentLink(ctx, inviter.ID)
			if linkErr == nil {
				if err := s.affiliateLinks.BindAgentReferral(ctx, userID, *referral); err != nil {
					return fmt.Errorf("bind default agent referral link: %w", err)
				}
				return nil
			}
			if !errors.Is(linkErr, ErrAffiliateLinkNotFound) {
				return fmt.Errorf("resolve default affiliate link: %w", linkErr)
			}
		}
		var legacyAgentID *int64
		if inviter.Role == RoleAgent {
			legacyAgentID = &inviter.ID
		}
		return s.userRepo.SetInviterAndAgent(ctx, userID, inviter.ID, legacyAgentID)
	}
	if !isNotFound(err) {
		return fmt.Errorf("resolve ordinary referral code: %w", err)
	}
	if s.affiliateLinks == nil {
		return nil
	}
	referral, err := s.affiliateLinks.ResolveActiveLink(ctx, code)
	if errors.Is(err, ErrAffiliateLinkNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("resolve agent referral link: %w", err)
	}
	if err := s.affiliateLinks.BindAgentReferral(ctx, userID, *referral); err != nil {
		return fmt.Errorf("bind agent referral link: %w", err)
	}
	return nil
}

// ProcessConsumptionCommission 处理 API 消耗分佣记录。
// 在 gateway postUsageBilling 中扣费成功后异步调用，仅记录代理商业绩，不直接增加代理商余额。
func (s *CommissionService) ProcessConsumptionCommission(ctx context.Context, userID int64, actualCost float64, sourceID int64) error {
	if actualCost <= 0 {
		return nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user for commission: %w", err)
	}
	if user.AgentID == nil {
		return nil // 无归属代理商，无需分佣
	}

	agentID := *user.AgentID
	rate, rateSource := s.resolveAgentConsumptionRate(ctx, agentID)
	commission := actualCost * rate

	// 仅记录流水，代理商看板基于 commission_records 聚合展示。
	note := fmt.Sprintf("消耗分佣，用户 #%d 消费 %.8f", userID, actualCost)
	record := &CommissionRecord{
		BeneficiaryID: agentID,
		UserID:        userID,
		Amount:        commission,
		SourceAmount:  actualCost,
		Type:          CommissionTypeConsumption,
		Rate:          rate,
		RateSource:    rateSource,
		SourceID:      &sourceID,
		Note:          &note,
	}
	if err := s.commissionRepo.Create(ctx, record); err != nil {
		return fmt.Errorf("create consumption commission record: %w", err)
	}
	return nil
}

// ProcessFirstRechargeBonus 处理旧版“首次余额型入账”奖励逻辑。
// 该逻辑已不再用于新的邀请首充规则，仅保留兼容用途。
func (s *CommissionService) ProcessFirstRechargeBonus(ctx context.Context, userID int64, rechargeAmount float64, redeemCodeID int64) error {
	if rechargeAmount <= 0 {
		return nil
	}

	// 原子标记首充，返回 0 表示已处理过
	affected, err := s.userRepo.SetFirstRecharged(ctx, userID)
	if err != nil {
		return fmt.Errorf("set first recharged: %w", err)
	}
	if affected == 0 {
		return nil // 已经处理过首充奖励，幂等退出
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user for first recharge bonus: %w", err)
	}

	sourceID := redeemCodeID
	rates := s.getCommissionRates(ctx)

	if user.AgentID != nil {
		// 分支A：代理商邀请 — 被邀请用户得 10%
		inviteeBonus := rechargeAmount * rates.FirstRechargeInviteeRate
		if err := s.userRepo.UpdateBalance(ctx, userID, inviteeBonus); err != nil {
			slog.Error("apply first recharge invitee bonus failed", "user_id", userID, "error", err)
		} else {
			note := fmt.Sprintf("首充奖励（代理商邀请），充值 %.8f", rechargeAmount)
			inviteeRecord := &CommissionRecord{
				BeneficiaryID: userID,
				UserID:        userID,
				Amount:        inviteeBonus,
				SourceAmount:  rechargeAmount,
				Type:          CommissionTypeFirstRechargeInvitee,
				Rate:          rates.FirstRechargeInviteeRate,
				RateSource:    "global",
				SourceID:      &sourceID,
				Note:          &note,
			}
			if err := s.commissionRepo.Create(ctx, inviteeRecord); err != nil {
				slog.Error("create first recharge invitee record failed", "error", err)
			}
		}
	} else if user.InviterID != nil {
		// 分支B：普通用户邀请 — 邀请人得 5%
		inviterID := *user.InviterID
		s.applyFirstRechargeReferralBonus(ctx, userID, inviterID, rechargeAmount, sourceID, rates.FirstRechargeReferralRate)
	}
	return nil
}

// ProcessFirstInvitedTopupBonus 处理“被邀请用户首次虎皮椒充值”奖励（幂等）。
// 仅当用户已绑定邀请关系且首次虎皮椒订单完成时触发。
func (s *CommissionService) ProcessFirstInvitedTopupBonus(ctx context.Context, userID int64, rechargeAmount float64, topupOrderID int64) error {
	if rechargeAmount <= 0 {
		return nil
	}

	affected, err := s.userRepo.MarkFirstInvitedTopup(ctx, userID, topupOrderID)
	if err != nil {
		return fmt.Errorf("mark first invited topup: %w", err)
	}
	if affected == 0 {
		return nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user for first invited topup bonus: %w", err)
	}

	sourceID := topupOrderID
	rates := s.getCommissionRates(ctx)

	if user.AgentID != nil {
		inviteeBonus := rechargeAmount * rates.FirstRechargeInviteeRate
		if err := s.userRepo.UpdateBalance(ctx, userID, inviteeBonus); err != nil {
			slog.Error("apply first invited topup agent invitee bonus failed", "user_id", userID, "error", err)
		} else {
			note := fmt.Sprintf("首充奖励（代理商邀请，虎皮椒充值），充值 %.8f", rechargeAmount)
			inviteeRecord := &CommissionRecord{
				BeneficiaryID: userID,
				UserID:        userID,
				Amount:        inviteeBonus,
				SourceAmount:  rechargeAmount,
				Type:          CommissionTypeFirstRechargeInvitee,
				Rate:          rates.FirstRechargeInviteeRate,
				RateSource:    "global",
				SourceID:      &sourceID,
				Note:          &note,
			}
			if err := s.commissionRepo.Create(ctx, inviteeRecord); err != nil {
				slog.Error("create first invited topup agent invitee record failed", "error", err)
			}
		}
	} else if user.InviterID != nil {
		inviterID := *user.InviterID
		s.applyFirstRechargeReferralBonus(ctx, userID, inviterID, rechargeAmount, sourceID, rates.FirstRechargeReferralRate)
	}

	return nil
}

// applyFirstRechargeReferralBonus 普通用户邀请首充奖励：邀请人得配置比例，被邀请用户得同等比例。
func (s *CommissionService) applyFirstRechargeReferralBonus(ctx context.Context, userID, inviterID int64, rechargeAmount float64, sourceID int64, rate float64) {
	bonus := rechargeAmount * rate

	// 邀请人入账
	if err := s.userRepo.UpdateBalance(ctx, inviterID, bonus); err != nil {
		slog.Error("apply first recharge referral bonus failed", "inviter_id", inviterID, "error", err)
	} else {
		note := fmt.Sprintf("邀请奖励，用户 #%d 首充 %.8f", userID, rechargeAmount)
		record := &CommissionRecord{
			BeneficiaryID: inviterID,
			UserID:        userID,
			Amount:        bonus,
			SourceAmount:  rechargeAmount,
			Type:          CommissionTypeFirstRechargeReferral,
			Rate:          rate,
			RateSource:    "global",
			SourceID:      &sourceID,
			Note:          &note,
		}
		if err := s.commissionRepo.Create(ctx, record); err != nil {
			slog.Error("create first recharge referral record failed", "error", err)
		}
	}

	// 被邀请用户入账
	if err := s.userRepo.UpdateBalance(ctx, userID, bonus); err != nil {
		slog.Error("apply first recharge invitee referral bonus failed", "user_id", userID, "error", err)
	} else {
		note := fmt.Sprintf("首充奖励（好友邀请），充值 %.8f", rechargeAmount)
		record := &CommissionRecord{
			BeneficiaryID: userID,
			UserID:        userID,
			Amount:        bonus,
			SourceAmount:  rechargeAmount,
			Type:          CommissionTypeFirstRechargeFriendInvitee,
			Rate:          rate,
			RateSource:    "global",
			SourceID:      &sourceID,
			Note:          &note,
		}
		if err := s.commissionRepo.Create(ctx, record); err != nil {
			slog.Error("create first recharge invitee referral record failed", "error", err)
		}
	}
}

// GetAgentDashboard 获取代理商总览统计
func (s *CommissionService) GetAgentDashboard(ctx context.Context, agentID int64, start, end *time.Time) (*AgentDashboard, error) {
	// 累计分佣总额
	totalCommission, err := s.commissionRepo.SumByBeneficiaryAndPeriod(ctx, agentID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("sum total commission: %w", err)
	}

	// 指定周期分佣
	periodCommission, err := s.commissionRepo.SumByBeneficiaryAndPeriod(ctx, agentID, start, end)
	if err != nil {
		return nil, fmt.Errorf("sum period commission: %w", err)
	}

	// 本月分佣
	now := apptimezone.Now()
	monthStart, nextAssessmentAt := agentLevelCurrentMonthWindow(now)
	assessmentPeriodEnd := nextAssessmentAt.Add(-time.Second)
	monthEnd := assessmentPeriodEnd
	thisMonthCommission, err := s.commissionRepo.SumByBeneficiaryAndPeriod(ctx, agentID, &monthStart, &monthEnd)
	if err != nil {
		return nil, fmt.Errorf("sum this month commission: %w", err)
	}
	settledCommission := 0.0
	if s.adminRepo != nil {
		settledCommission, err = s.adminRepo.SumAgentSettlements(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("sum settled commission: %w", err)
		}
	}
	unsettledCommission := totalCommission - settledCommission
	if unsettledCommission < 0 {
		unsettledCommission = 0
	}
	consumptionRate, rateSource := s.resolveAgentConsumptionRate(ctx, agentID)
	rates := s.getCommissionRates(ctx)
	var levelState *AgentLevelState
	rules := defaultAgentLevelRules()
	thisMonthConsumption := 0.0
	totalConsumption := 0.0
	if s.levelRepo != nil {
		if loadedRules, ruleErr := s.GetAgentLevelRules(ctx); ruleErr == nil {
			rules = loadedRules
		}
		levelState, _ = s.levelRepo.GetAgentLevelState(ctx, agentID)
		if stats, statsErr := s.levelRepo.GetAgentLevelUsageStats(ctx, agentID, monthStart, nextAssessmentAt); statsErr == nil && stats != nil {
			thisMonthConsumption = stats.LastMonthConsumption
			totalConsumption = stats.TotalConsumption
		}
	}

	// 邀请用户总数
	_, paginationResult, err := s.commissionRepo.ListInvitedUsersWithStats(
		ctx, agentID, pagination.PaginationParams{Page: 1, PageSize: 1}, nil, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("count invited users: %w", err)
	}

	var invitedCount int64
	if paginationResult != nil {
		invitedCount = paginationResult.Total
	}

	dashboard := &AgentDashboard{
		InvitedUserCount:         invitedCount,
		TotalCommission:          totalCommission,
		SettledCommission:        settledCommission,
		UnsettledCommission:      unsettledCommission,
		PeriodCommission:         periodCommission,
		ThisMonthCommission:      thisMonthCommission,
		ConsumptionRate:          consumptionRate,
		FirstRechargeInviteeRate: rates.FirstRechargeInviteeRate,
		RateSource:               rateSource,
		ThisMonthConsumption:     thisMonthConsumption,
		TotalConsumption:         totalConsumption,
		AssessmentPeriodStart:    &monthStart,
		AssessmentPeriodEnd:      &assessmentPeriodEnd,
		NextAssessmentAt:         &nextAssessmentAt,
	}
	settlementSettings := s.getAgentSettlementSettings(ctx)
	settlementReached := unsettledCommission+agentSettlementAmountEpsilon >= settlementSettings.MinimumAmount
	dashboard.SettlementMinimumAmount = settlementSettings.MinimumAmount
	dashboard.SettlementGap = settlementGap(unsettledCommission, settlementSettings.MinimumAmount)
	dashboard.SettlementEligible = settlementReached
	if s.paymentRepo != nil {
		if profile, profileErr := s.paymentRepo.GetAgentPaymentProfile(ctx, agentID); profileErr == nil && profile != nil {
			normalizeAgentPaymentProfile(profile)
			dashboard.PaymentProfileComplete = profile.Complete
		}
	}
	dashboard.SettlementEligible = settlementReached && dashboard.PaymentProfileComplete
	normalizedRules := enabledAgentLevelRules(normalizeAgentLevelRules(rules))
	if levelState != nil {
		dashboard.CurrentLevel = levelState.CurrentLevelKey
		dashboard.CurrentLevelName = agentLevelName(normalizedRules, levelState.CurrentLevelKey)
		dashboard.PermanentLevel = levelState.PermanentLevelKey
		dashboard.PermanentLevelName = agentLevelName(normalizedRules, levelState.PermanentLevelKey)
		dashboard.TemporaryLevel = levelState.TemporaryLevelKey
		if levelState.TemporaryLevelKey != nil {
			name := agentLevelName(normalizedRules, *levelState.TemporaryLevelKey)
			dashboard.TemporaryLevelName = &name
		}
		dashboard.LastMonthConsumption = levelState.LastMonthConsumption
		dashboard.NextLevelGap = levelState.NextLevelGap
		dashboard.LastEvaluatedPeriod = levelState.LastEvaluatedPeriod
		dashboard.EvaluatedAt = levelState.EvaluatedAt
		if dashboard.TotalConsumption == 0 && levelState.TotalConsumption > 0 {
			dashboard.TotalConsumption = levelState.TotalConsumption
		}
	} else if len(normalizedRules) > 0 {
		currentRule := levelForRate(normalizedRules, consumptionRate)
		dashboard.CurrentLevel = currentRule.LevelKey
		dashboard.CurrentLevelName = currentRule.LevelName
		dashboard.PermanentLevel = firstAgentLevelRule(normalizedRules).LevelKey
		dashboard.PermanentLevelName = firstAgentLevelRule(normalizedRules).LevelName
	}
	monthlyMinRate := consumptionRate
	permanentMinRate := 0.0
	if dashboard.PermanentLevel != "" {
		if permanentRule, ok := findAgentLevelRule(normalizedRules, dashboard.PermanentLevel); ok {
			permanentMinRate = permanentRule.Rate
		}
	}
	dashboard.NextMonthlyProgress = nextAgentLevelProgress(normalizedRules, dashboard.ThisMonthConsumption, monthlyMinRate, true)
	dashboard.NextCumulativeProgress = nextAgentLevelProgress(normalizedRules, dashboard.TotalConsumption, permanentMinRate, false)
	return dashboard, nil
}

// GetUserReferralDashboard 获取普通用户的邀请看板统计（邀请人数 + 获得的 referral 佣金）
func (s *CommissionService) GetUserReferralDashboard(ctx context.Context, userID int64) (*UserReferralDashboard, error) {
	rates := s.getCommissionRates(ctx)

	// 邀请用户总数（按 inviter_id 查）
	invitedCount, err := s.userRepo.CountInvitedByInviterID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count invited users: %w", err)
	}

	// 累计佣金（仅 first_recharge_referral 类型）
	totalCommission, err := s.commissionRepo.SumByBeneficiaryTypeAndPeriod(
		ctx, userID, CommissionTypeFirstRechargeReferral, nil, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("sum total referral commission: %w", err)
	}

	// 本月佣金
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)
	thisMonthCommission, err := s.commissionRepo.SumByBeneficiaryTypeAndPeriod(
		ctx, userID, CommissionTypeFirstRechargeReferral, &monthStart, &monthEnd,
	)
	if err != nil {
		return nil, fmt.Errorf("sum this month referral commission: %w", err)
	}

	return &UserReferralDashboard{
		InvitedUserCount:          invitedCount,
		TotalCommission:           totalCommission,
		ThisMonthCommission:       thisMonthCommission,
		FirstRechargeInviteeRate:  rates.FirstRechargeInviteeRate,
		FirstRechargeReferralRate: rates.FirstRechargeReferralRate,
	}, nil
}

// GetAgentInvitedUsers 获取代理商邀请的用户列表及消费统计
func (s *CommissionService) GetAgentInvitedUsers(
	ctx context.Context,
	agentID int64,
	params pagination.PaginationParams,
	start, end *time.Time,
) ([]InvitedUserStat, *pagination.PaginationResult, error) {
	return s.commissionRepo.ListInvitedUsersWithStats(ctx, agentID, params, start, end)
}

// GetAgentCommissions 获取代理商的分佣记录明细
func (s *CommissionService) GetAgentCommissions(
	ctx context.Context,
	agentID int64,
	params pagination.PaginationParams,
	typeFilter string,
	start, end *time.Time,
) ([]CommissionRecord, *pagination.PaginationResult, error) {
	return s.commissionRepo.ListByBeneficiary(ctx, agentID, params, typeFilter, start, end)
}

// generateInviteCode 生成随机 8 位邀请码
func generateInviteCode() (string, error) {
	result := make([]byte, inviteCodeLen)
	charsetLen := big.NewInt(int64(len(inviteCodeCharset)))
	for i := range result {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = inviteCodeCharset[n.Int64()]
	}
	return string(result), nil
}

func defaultCommissionRates() *CommissionRates {
	return &CommissionRates{
		ConsumptionRate:           commissionRateConsumption,
		FirstRechargeInviteeRate:  commissionRateFirstRechargeInvitee,
		FirstRechargeReferralRate: commissionRateFirstRechargeReferral,
	}
}

func (s *CommissionService) getCommissionRates(ctx context.Context) *CommissionRates {
	if s.rateRepo == nil {
		return defaultCommissionRates()
	}
	rates, err := s.rateRepo.GetCommissionRates(ctx)
	if err != nil || rates == nil {
		if err != nil {
			slog.Warn("load commission rates failed, using defaults", "error", err)
		}
		return defaultCommissionRates()
	}
	return rates
}

func (s *CommissionService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return apptimezone.Now()
}

func (s *CommissionService) resolveAgentConsumptionRate(ctx context.Context, agentID int64) (float64, string) {
	if s.rateRepo == nil {
		return commissionRateConsumption, "global"
	}
	rate, source, err := s.rateRepo.ResolveAgentConsumptionRate(ctx, agentID)
	if err != nil {
		slog.Warn("resolve agent consumption rate failed, using default", "agent_id", agentID, "error", err)
		return commissionRateConsumption, "global"
	}
	if rate < 0 {
		return commissionRateConsumption, "global"
	}
	if source == "" {
		source = "global"
	}
	return rate, source
}

// isNotFound 判断错误是否为"未找到"类型
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrUserNotFound)
}
