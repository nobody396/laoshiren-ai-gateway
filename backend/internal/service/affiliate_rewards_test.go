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

func TestAffiliateFirstPaidRewards_OrdinaryInviterAndInviteeReceiveFivePercentTZero(t *testing.T) {
	t.Parallel()
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:             true,
		Claimed:                 true,
		PurchaseID:              88,
		BindingKind:             AffiliateBindingOrdinary,
		InviterUserID:           7,
		OrdinaryReferralRateBPS: 500,
		OrdinaryInviteeRateBPS:  500,
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
	if result.OrdinaryReferralMicros != 2_500_000 ||
		result.OrdinaryInviteeMicros != 2_500_000 ||
		len(repo.posted) != 2 {
		t.Fatalf("ordinary referral result = %+v, posted=%d", result, len(repo.posted))
	}
	if len(repo.scheduled) != 0 {
		t.Fatalf("v3 must not schedule a fixed T+1 bonus")
	}
	if result.SourcePolicy != AffiliateSourcePolicyOrdinaryFirstPaid || result.DirectPartnerID != 7 {
		t.Fatalf("ordinary source policy = %+v", result)
	}
	for _, posted := range repo.posted {
		if !posted.AvailableAt.Equal(at) {
			t.Fatalf("reward is not T+0: %+v", posted)
		}
	}
}

func TestAffiliateFirstPaidRewards_AgentBindingDoesNotStackOrdinaryReward(t *testing.T) {
	t.Parallel()
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:             true,
		Claimed:                 true,
		PurchaseID:              90,
		BindingKind:             AffiliateBindingAgent,
		InviterUserID:           7,
		BindingAgentID:          7,
		BindingCustomerRateBPS:  300,
		BindingPartnerRateBPS:   700,
		OrdinaryReferralRateBPS: 500,
		OrdinaryInviteeRateBPS:  500,
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
	if len(repo.scheduled) != 0 ||
		result.SourcePolicy != AffiliateSourcePolicyPartnerUsage ||
		result.CustomerRebateRateBPS != 300 ||
		result.PartnerCommissionRateBPS != 700 {
		t.Fatalf("agent-bound source policy is incorrect: %+v", result)
	}
}

func TestAffiliateHistoricalDirectStartsPartnerUsageOnlyAfterActivation(t *testing.T) {
	t.Parallel()
	activatedAt := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:               true,
		BindingKind:               AffiliateBindingOrdinary,
		InviterUserID:             7,
		InviterPartnerStatus:      "active",
		InviterPartnerActivatedAt: &activatedAt,
	}}
	svc := NewAffiliateRewardService(repo)

	before, err := svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       11,
		PurchaseType: AffiliatePurchaseBalanceTopup,
		PurchaseKey:  "before",
		AmountMicros: 20_000_000,
		OccurredAt:   activatedAt.Add(-time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if before.SourcePolicy != AffiliateSourcePolicyOrdinaryFirstPaid {
		t.Fatalf("pre-activation policy = %s", before.SourcePolicy)
	}

	after, err := svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       11,
		PurchaseType: AffiliatePurchaseBalanceTopup,
		PurchaseKey:  "after",
		AmountMicros: 20_000_000,
		OccurredAt:   activatedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if after.SourcePolicy != AffiliateSourcePolicyPartnerUsage ||
		after.CustomerRebateRateBPS != 0 ||
		after.PartnerCommissionRateBPS != 1000 {
		t.Fatalf("post-activation policy = %+v", after)
	}
}

func TestAffiliateTerminatedHistoricalInviterCannotFallBackToOrdinaryReward(t *testing.T) {
	t.Parallel()
	activatedAt := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:               true,
		Claimed:                   true,
		PurchaseID:                120,
		BindingKind:               AffiliateBindingOrdinary,
		InviterUserID:             7,
		InviterPartnerStatus:      "terminated",
		InviterPartnerActivatedAt: &activatedAt,
		OrdinaryReferralRateBPS:   500,
		OrdinaryInviteeRateBPS:    500,
	}}
	svc := NewAffiliateRewardService(repo)

	result, err := svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       11,
		PurchaseType: AffiliatePurchaseBalanceTopup,
		PurchaseKey:  "after-termination",
		AmountMicros: 100_000_000,
		OccurredAt:   activatedAt.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SourcePolicy != AffiliateSourcePolicyNone ||
		result.DirectPartnerID != 0 ||
		result.OrdinaryReferralMicros != 0 ||
		result.OrdinaryInviteeMicros != 0 ||
		len(repo.posted) != 0 {
		t.Fatalf("terminated inviter received fallback reward: result=%+v posted=%d", result, len(repo.posted))
	}
}
