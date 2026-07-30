package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"
)

const (
	AffiliatePurchaseBalanceRedeem  = "balance_redeem"
	AffiliatePurchaseBalanceTopup   = "balance_topup"
	AffiliatePurchaseMonthlyRedeem  = "monthly_redeem"
	AffiliatePurchaseMonthlyPayment = "monthly_payment"

	AffiliateBindingOrdinary = "ordinary"
	AffiliateBindingAgent    = "agent"

	AffiliateRewardOrdinaryReferral = "ordinary_referral"
	AffiliateRewardOrdinaryInvitee  = "ordinary_invitee"

	AffiliateSourcePolicyNone              = "NONE"
	AffiliateSourcePolicyOrdinaryFirstPaid = "ORDINARY_FIRST_PAID"
	AffiliateSourcePolicyPartnerUsage      = "PARTNER_USAGE"
	AffiliateSourcePolicyPartnerSelfUsage  = "PARTNER_SELF_USAGE"
)

type AffiliateFirstPaidPurchaseInput struct {
	UserID       int64
	PurchaseType string
	SourceID     int64
	PurchaseKey  string
	AmountMicros int64
	OccurredAt   time.Time
}

type AffiliateFirstPaidContext struct {
	ProgramMode               string
	ProgramLive               bool
	ProgramStartedAt          *time.Time
	Claimed                   bool
	PurchaseID                int64
	BindingKind               string
	BindingBoundAt            *time.Time
	InviterUserID             int64
	OrdinaryReferralRateBPS   int32
	OrdinaryInviteeRateBPS    int32
	BindingAgentID            int64
	BindingCustomerRateBPS    int32
	BindingPartnerRateBPS     int32
	InviterPartnerStatus      string
	InviterPartnerActivatedAt *time.Time
	HasUpstreamRelationship   bool
	SelfPartnerStatus         string
	SelfPartnerRiskStatus     string
	SelfCommissionEnabled     bool
	SelfCommissionRateBPS     int32
	SelfCommissionEffectiveAt *time.Time
}

type AffiliatePlatformRewardInput struct {
	BeneficiaryUserID  int64
	ConsumerUserID     int64
	RewardType         string
	AmountMicros       int64
	SourceAmountMicros int64
	RateBPS            *int32
	AvailableAt        time.Time
	SourceType         string
	SourceID           int64
	IdempotencyKey     string
}

type AffiliateRewardRepository interface {
	ClaimFirstPaidPurchase(ctx context.Context, input AffiliateFirstPaidPurchaseInput) (*AffiliateFirstPaidContext, error)
	PostPlatformReward(ctx context.Context, input AffiliatePlatformRewardInput) error
	SchedulePlatformReward(ctx context.Context, input AffiliatePlatformRewardInput) error
	PostDuePlatformRewards(ctx context.Context, limit int) (int, error)
}

type AffiliateFirstPaidPurchaseResult struct {
	ProgramMode              string
	ProgramLive              bool
	Claimed                  bool
	OrdinaryReferralMicros   int64
	OrdinaryInviteeMicros    int64
	SourcePolicy             string
	DirectPartnerID          int64
	CustomerRebateRateBPS    int32
	PartnerCommissionRateBPS int32
}

type AffiliateRewardService struct {
	repo AffiliateRewardRepository

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func NewAffiliateRewardService(repo AffiliateRewardRepository) *AffiliateRewardService {
	return &AffiliateRewardService{
		repo:   repo,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (s *AffiliateRewardService) ProcessFirstPaidPurchase(
	ctx context.Context,
	input AffiliateFirstPaidPurchaseInput,
) (*AffiliateFirstPaidPurchaseResult, error) {
	if s == nil || s.repo == nil {
		return &AffiliateFirstPaidPurchaseResult{}, nil
	}
	if input.UserID <= 0 || input.AmountMicros <= 0 || input.PurchaseKey == "" {
		return nil, errors.New("invalid affiliate first-paid purchase input")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now()
	}
	firstPaid, err := s.repo.ClaimFirstPaidPurchase(ctx, input)
	if err != nil {
		return nil, err
	}
	result := &AffiliateFirstPaidPurchaseResult{
		ProgramMode:  firstPaid.ProgramMode,
		ProgramLive:  firstPaid.ProgramLive,
		Claimed:      firstPaid.Claimed,
		SourcePolicy: AffiliateSourcePolicyNone,
	}
	if firstPaid.ProgramMode != AffiliateProgramModeShadow && !firstPaid.ProgramLive {
		return result, nil
	}

	switch {
	case firstPaid.BindingKind == AffiliateBindingAgent && firstPaid.BindingAgentID > 0:
		result.SourcePolicy = AffiliateSourcePolicyPartnerUsage
		result.DirectPartnerID = firstPaid.BindingAgentID
		result.CustomerRebateRateBPS = firstPaid.BindingCustomerRateBPS
		result.PartnerCommissionRateBPS = firstPaid.BindingPartnerRateBPS
	case firstPaid.BindingKind == AffiliateBindingOrdinary &&
		firstPaid.InviterUserID > 0 &&
		affiliatePartnerStatusEarnsOrHolds(firstPaid.InviterPartnerStatus) &&
		firstPaid.InviterPartnerActivatedAt != nil &&
		!input.OccurredAt.Before(*firstPaid.InviterPartnerActivatedAt):
		result.SourcePolicy = AffiliateSourcePolicyPartnerUsage
		result.DirectPartnerID = firstPaid.InviterUserID
		result.CustomerRebateRateBPS = 0
		result.PartnerCommissionRateBPS = AffiliateAgentPoolRateBPS
	case firstPaid.BindingKind == AffiliateBindingOrdinary &&
		firstPaid.InviterUserID > 0 &&
		firstPaid.InviterPartnerStatus == "terminated":
		// A terminated former partner cannot fall back to the ordinary 5%
		// referral reward. The permanent binding remains for audit only.
		result.SourcePolicy = AffiliateSourcePolicyNone
		result.DirectPartnerID = 0
	case firstPaid.BindingKind == AffiliateBindingOrdinary &&
		firstPaid.InviterUserID > 0 &&
		(!firstPaid.ProgramLive ||
			(firstPaid.ProgramStartedAt != nil &&
				firstPaid.BindingBoundAt != nil &&
				!firstPaid.BindingBoundAt.Before(*firstPaid.ProgramStartedAt))):
		result.SourcePolicy = AffiliateSourcePolicyOrdinaryFirstPaid
		result.DirectPartnerID = firstPaid.InviterUserID
	case firstPaid.BindingKind == "" &&
		!firstPaid.HasUpstreamRelationship &&
		(!firstPaid.ProgramLive ||
			(firstPaid.ProgramStartedAt != nil &&
				!input.OccurredAt.Before(*firstPaid.ProgramStartedAt))) &&
		firstPaid.SelfPartnerStatus == "active" &&
		firstPaid.SelfPartnerRiskStatus == "clear" &&
		firstPaid.SelfCommissionEnabled &&
		firstPaid.SelfCommissionRateBPS == AffiliateAgentPoolRateBPS &&
		firstPaid.SelfCommissionEffectiveAt != nil &&
		!input.OccurredAt.Before(*firstPaid.SelfCommissionEffectiveAt):
		// Self-consumption commission is a separately administered policy. It
		// never creates a synthetic self-binding and it is considered only
		// after every real binding path above, so an upstream relationship
		// always wins and the fixed 10% pool can never stack.
		result.SourcePolicy = AffiliateSourcePolicyPartnerSelfUsage
		result.DirectPartnerID = input.UserID
		result.CustomerRebateRateBPS = 0
		result.PartnerCommissionRateBPS = AffiliateAgentPoolRateBPS
	}

	// Shadow mode must preserve the exact projected source policy used by the
	// real purchase-to-usage path, but it must never claim the first purchase or
	// post platform-credit rewards.
	if firstPaid.ProgramMode == AffiliateProgramModeShadow {
		return result, nil
	}

	if !firstPaid.Claimed || result.SourcePolicy != AffiliateSourcePolicyOrdinaryFirstPaid {
		return result, nil
	}

	if firstPaid.OrdinaryReferralRateBPS > 0 {
		rewardMicros := affiliateRateAmountMicros(input.AmountMicros, firstPaid.OrdinaryReferralRateBPS)
		if rewardMicros > 0 {
			rate := firstPaid.OrdinaryReferralRateBPS
			if err := s.repo.PostPlatformReward(ctx, AffiliatePlatformRewardInput{
				BeneficiaryUserID:  firstPaid.InviterUserID,
				ConsumerUserID:     input.UserID,
				RewardType:         AffiliateRewardOrdinaryReferral,
				AmountMicros:       rewardMicros,
				SourceAmountMicros: input.AmountMicros,
				RateBPS:            &rate,
				AvailableAt:        input.OccurredAt,
				SourceType:         input.PurchaseType,
				SourceID:           input.SourceID,
				IdempotencyKey:     fmt.Sprintf("first-paid:%d:ordinary-referral", firstPaid.PurchaseID),
			}); err != nil {
				return nil, err
			}
			result.OrdinaryReferralMicros = rewardMicros
		}
	}

	if firstPaid.OrdinaryInviteeRateBPS > 0 {
		rewardMicros := affiliateRateAmountMicros(input.AmountMicros, firstPaid.OrdinaryInviteeRateBPS)
		if rewardMicros <= 0 {
			return result, nil
		}
		rate := firstPaid.OrdinaryInviteeRateBPS
		if err := s.repo.PostPlatformReward(ctx, AffiliatePlatformRewardInput{
			BeneficiaryUserID:  input.UserID,
			ConsumerUserID:     input.UserID,
			RewardType:         AffiliateRewardOrdinaryInvitee,
			AmountMicros:       rewardMicros,
			SourceAmountMicros: input.AmountMicros,
			RateBPS:            &rate,
			AvailableAt:        input.OccurredAt,
			SourceType:         input.PurchaseType,
			SourceID:           input.SourceID,
			IdempotencyKey:     fmt.Sprintf("first-paid:%d:ordinary-invitee", firstPaid.PurchaseID),
		}); err != nil {
			return nil, err
		}
		result.OrdinaryInviteeMicros = rewardMicros
	}
	return result, nil
}

func affiliatePartnerStatusEarnsOrHolds(status string) bool {
	switch status {
	case "active", "suspended":
		return true
	default:
		return false
	}
}

func affiliateRateAmountMicros(sourceMicros int64, rateBPS int32) int64 {
	if sourceMicros <= 0 || rateBPS <= 0 {
		return 0
	}
	product := new(big.Int).Mul(big.NewInt(sourceMicros), big.NewInt(int64(rateBPS)))
	product.Quo(product, big.NewInt(10_000))
	if !product.IsInt64() {
		return 0
	}
	return product.Int64()
}

func (s *AffiliateRewardService) Start() {
	if s == nil || s.repo == nil {
		close(s.doneCh)
		return
	}
	go s.runMaturityWorker()
}

func (s *AffiliateRewardService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh) })
	<-s.doneCh
}

func (s *AffiliateRewardService) runMaturityWorker() {
	defer close(s.doneCh)
	s.postDueRewards()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.postDueRewards()
		case <-s.stopCh:
			return
		}
	}
}

func (s *AffiliateRewardService) postDueRewards() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		count, err := s.repo.PostDuePlatformRewards(ctx, 100)
		if err != nil {
			slog.Error("affiliate reward maturity failed", "error", err)
			return
		}
		if count < 100 {
			return
		}
	}
}
