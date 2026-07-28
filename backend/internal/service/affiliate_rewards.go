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
	AffiliateRewardFirstPaidBonus   = "first_paid_bonus"
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
	ProgramLive                   bool
	Claimed                       bool
	PurchaseID                    int64
	BindingKind                   string
	InviterUserID                 int64
	OrdinaryReferralRateBPS       int32
	FirstPaidBonusThresholdMicros int64
	FirstPaidBonusMicros          int64
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
	ProgramLive             bool
	Claimed                 bool
	OrdinaryReferralMicros  int64
	FirstPaidBonusScheduled bool
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
		ProgramLive: firstPaid.ProgramLive,
		Claimed:     firstPaid.Claimed,
	}
	if !firstPaid.ProgramLive || !firstPaid.Claimed {
		return result, nil
	}

	if firstPaid.BindingKind == AffiliateBindingOrdinary && firstPaid.OrdinaryReferralRateBPS > 0 {
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

	// The fixed invitee bonus is available once the first real payment reaches
	// the configured threshold. For the default ¥50 balance card, exactly ¥50
	// should be eligible.
	if input.AmountMicros >= firstPaid.FirstPaidBonusThresholdMicros &&
		firstPaid.FirstPaidBonusMicros > 0 {
		if err := s.repo.SchedulePlatformReward(ctx, AffiliatePlatformRewardInput{
			BeneficiaryUserID:  input.UserID,
			ConsumerUserID:     input.UserID,
			RewardType:         AffiliateRewardFirstPaidBonus,
			AmountMicros:       firstPaid.FirstPaidBonusMicros,
			SourceAmountMicros: input.AmountMicros,
			AvailableAt:        affiliateNextBeijingDay(input.OccurredAt),
			SourceType:         input.PurchaseType,
			SourceID:           input.SourceID,
			IdempotencyKey:     fmt.Sprintf("first-paid:%d:invitee-bonus", firstPaid.PurchaseID),
		}); err != nil {
			return nil, err
		}
		result.FirstPaidBonusScheduled = true
	}
	return result, nil
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

func affiliateNextBeijingDay(at time.Time) time.Time {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	local := at.In(location)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location)
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
