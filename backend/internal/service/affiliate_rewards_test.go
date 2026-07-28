package service

import (
	"context"
	"testing"
	"time"
)

type affiliateRewardRepoStub struct {
	firstPaid AffiliateFirstPaidContext
	posted    []AffiliatePlatformRewardInput
	scheduled []AffiliatePlatformRewardInput
}

func (r *affiliateRewardRepoStub) ClaimFirstPaidPurchase(context.Context, AffiliateFirstPaidPurchaseInput) (*AffiliateFirstPaidContext, error) {
	out := r.firstPaid
	return &out, nil
}

func (r *affiliateRewardRepoStub) PostPlatformReward(_ context.Context, input AffiliatePlatformRewardInput) error {
	r.posted = append(r.posted, input)
	return nil
}

func (r *affiliateRewardRepoStub) SchedulePlatformReward(_ context.Context, input AffiliatePlatformRewardInput) error {
	r.scheduled = append(r.scheduled, input)
	return nil
}

func (r *affiliateRewardRepoStub) PostDuePlatformRewards(context.Context, int) (int, error) {
	return 0, nil
}

func TestAffiliateRateAmountMicrosFloors(t *testing.T) {
	t.Parallel()
	if got := affiliateRateAmountMicros(50_000_001, 500); got != 2_500_000 {
		t.Fatalf("reward = %d, want 2500000", got)
	}
}

func TestAffiliateNextBeijingDay(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 26, 15, 59, 59, 0, time.UTC) // 23:59:59 BJT
	got := affiliateNextBeijingDay(at)
	want := time.Date(2026, 7, 27, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if !got.Equal(want) {
		t.Fatalf("next Beijing day = %s, want %s", got, want)
	}
}

func TestAffiliateFirstPaidRewards_InclusiveThresholdAndOrdinaryFivePercent(t *testing.T) {
	t.Parallel()
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:                   true,
		Claimed:                       true,
		PurchaseID:                    88,
		BindingKind:                   AffiliateBindingOrdinary,
		InviterUserID:                 7,
		OrdinaryReferralRateBPS:       500,
		FirstPaidBonusThresholdMicros: 50_000_000,
		FirstPaidBonusMicros:          5_000_000,
	}}
	svc := NewAffiliateRewardService(repo)
	at := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

	result, err := svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       9,
		PurchaseType: AffiliatePurchaseBalanceTopup,
		SourceID:     100,
		PurchaseKey:  "topup:100",
		AmountMicros: 50_000_000,
		OccurredAt:   at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OrdinaryReferralMicros != 2_500_000 || len(repo.posted) != 1 {
		t.Fatalf("ordinary referral result = %+v, posted=%d", result, len(repo.posted))
	}
	if !result.FirstPaidBonusScheduled || len(repo.scheduled) != 1 {
		t.Fatalf("exactly ¥50 must schedule fixed bonus: result=%+v scheduled=%d", result, len(repo.scheduled))
	}

	repo.firstPaid.PurchaseID = 89
	result, err = svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       10,
		PurchaseType: AffiliatePurchaseBalanceTopup,
		SourceID:     101,
		PurchaseKey:  "topup:101",
		AmountMicros: 50_000_001,
		OccurredAt:   at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.FirstPaidBonusScheduled || len(repo.scheduled) != 2 {
		t.Fatalf("over ¥50 must schedule another fixed bonus: result=%+v scheduled=%d", result, len(repo.scheduled))
	}
	if repo.scheduled[1].AmountMicros != 5_000_000 {
		t.Fatalf("fixed bonus = %d", repo.scheduled[1].AmountMicros)
	}
}

func TestAffiliateFirstPaidRewards_AgentBindingDoesNotStackOrdinaryReward(t *testing.T) {
	t.Parallel()
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:                   true,
		Claimed:                       true,
		PurchaseID:                    90,
		BindingKind:                   AffiliateBindingAgent,
		InviterUserID:                 7,
		OrdinaryReferralRateBPS:       500,
		FirstPaidBonusThresholdMicros: 50_000_000,
		FirstPaidBonusMicros:          5_000_000,
	}}
	svc := NewAffiliateRewardService(repo)
	result, err := svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       11,
		PurchaseType: AffiliatePurchaseMonthlyPayment,
		SourceID:     102,
		PurchaseKey:  "payment:102",
		AmountMicros: 100_000_000,
		OccurredAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OrdinaryReferralMicros != 0 || len(repo.posted) != 0 {
		t.Fatalf("agent binding stacked ordinary reward: result=%+v posted=%d", result, len(repo.posted))
	}
	if len(repo.scheduled) != 1 {
		t.Fatalf("agent-bound invitee should still get fixed T+1 bonus")
	}
}
