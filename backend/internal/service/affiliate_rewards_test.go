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
	at := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	startedAt := at.Add(-2 * time.Hour)
	boundAt := at.Add(-time.Hour)
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:             true,
		ProgramStartedAt:        &startedAt,
		Claimed:                 true,
		PurchaseID:              88,
		BindingKind:             AffiliateBindingOrdinary,
		BindingBoundAt:          &boundAt,
		InviterUserID:           7,
		OrdinaryReferralRateBPS: 500,
		OrdinaryInviteeRateBPS:  500,
	}}
	svc := NewAffiliateRewardService(repo)

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

func TestAffiliateFirstPaidRewards_ShadowProjectsPartnerPolicyWithoutMoney(t *testing.T) {
	t.Parallel()
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramMode:               AffiliateProgramModeShadow,
		ProgramLive:               false,
		Claimed:                   false,
		BindingKind:               AffiliateBindingAgent,
		InviterUserID:             7,
		BindingAgentID:            7,
		BindingCustomerRateBPS:    300,
		BindingPartnerRateBPS:     700,
		OrdinaryReferralRateBPS:   500,
		OrdinaryInviteeRateBPS:    500,
		InviterPartnerStatus:      "active",
		InviterPartnerActivatedAt: nil,
	}}
	svc := NewAffiliateRewardService(repo)
	result, err := svc.ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       11,
		PurchaseType: AffiliatePurchaseBalanceRedeem,
		SourceID:     103,
		PurchaseKey:  "shadow-redeem:103",
		AmountMicros: 100_000_000,
		OccurredAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ProgramMode != AffiliateProgramModeShadow ||
		result.ProgramLive ||
		result.Claimed ||
		result.SourcePolicy != AffiliateSourcePolicyPartnerUsage ||
		result.DirectPartnerID != 7 ||
		result.CustomerRebateRateBPS != 300 ||
		result.PartnerCommissionRateBPS != 700 {
		t.Fatalf("shadow projection = %+v", result)
	}
	if len(repo.posted) != 0 || len(repo.scheduled) != 0 {
		t.Fatalf("shadow wrote rewards: posted=%d scheduled=%d", len(repo.posted), len(repo.scheduled))
	}
	policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(true, result)
	if policy != AffiliateSourcePolicyPartnerUsage ||
		partnerID != 7 ||
		customerRate != 300 ||
		partnerRate != 700 {
		t.Fatalf("shadow consumption policy = (%s,%d,%d,%d)", policy, partnerID, customerRate, partnerRate)
	}
}

func TestAffiliatePolicyFromPurchaseResult_OffDoesNotTrackConsumption(t *testing.T) {
	t.Parallel()
	policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(
		true,
		&AffiliateFirstPaidPurchaseResult{
			ProgramMode:              AffiliateProgramModeOff,
			SourcePolicy:             AffiliateSourcePolicyPartnerUsage,
			DirectPartnerID:          7,
			CustomerRebateRateBPS:    300,
			PartnerCommissionRateBPS: 700,
		},
	)
	if policy != AffiliateSourcePolicyNone ||
		partnerID != 0 ||
		customerRate != 0 ||
		partnerRate != 0 {
		t.Fatalf("off policy = (%s,%d,%d,%d)", policy, partnerID, customerRate, partnerRate)
	}
}

func TestAffiliateProgramHandlesPurchase_ShadowSuppressesLegacyRewards(t *testing.T) {
	t.Parallel()
	if AffiliateProgramHandlesPurchase(nil) {
		t.Fatal("nil result must not suppress the legacy writer")
	}
	if AffiliateProgramHandlesPurchase(&AffiliateFirstPaidPurchaseResult{ProgramMode: AffiliateProgramModeOff}) {
		t.Fatal("off mode must not suppress the legacy writer")
	}
	if !AffiliateProgramHandlesPurchase(&AffiliateFirstPaidPurchaseResult{ProgramMode: AffiliateProgramModeShadow}) {
		t.Fatal("shadow mode must suppress the legacy monetary writer")
	}
	if !AffiliateProgramHandlesPurchase(&AffiliateFirstPaidPurchaseResult{ProgramLive: true}) {
		t.Fatal("live mode must suppress the legacy monetary writer")
	}
}

func TestAffiliateHistoricalDirectStartsPartnerUsageOnlyAfterActivation(t *testing.T) {
	t.Parallel()
	activatedAt := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
	startedAt := activatedAt.Add(-48 * time.Hour)
	boundAt := activatedAt.Add(-24 * time.Hour)
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:               true,
		ProgramStartedAt:          &startedAt,
		BindingKind:               AffiliateBindingOrdinary,
		BindingBoundAt:            &boundAt,
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

func TestAffiliatePreLiveOrdinaryBindingCannotClaimLaunchReward(t *testing.T) {
	t.Parallel()
	startedAt := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	boundAt := startedAt.Add(-time.Hour)
	repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
		ProgramLive:             true,
		ProgramStartedAt:        &startedAt,
		Claimed:                 true,
		PurchaseID:              121,
		BindingKind:             AffiliateBindingOrdinary,
		BindingBoundAt:          &boundAt,
		InviterUserID:           7,
		OrdinaryReferralRateBPS: 500,
		OrdinaryInviteeRateBPS:  500,
	}}

	result, err := NewAffiliateRewardService(repo).ProcessFirstPaidPurchase(
		context.Background(),
		AffiliateFirstPaidPurchaseInput{
			UserID:       11,
			PurchaseType: AffiliatePurchaseBalanceTopup,
			PurchaseKey:  "pre-live-binding",
			AmountMicros: 100_000_000,
			OccurredAt:   startedAt.Add(time.Hour),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SourcePolicy != AffiliateSourcePolicyNone ||
		result.DirectPartnerID != 0 ||
		len(repo.posted) != 0 {
		t.Fatalf("pre-live binding received launch reward: result=%+v posted=%d", result, len(repo.posted))
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

func TestAffiliateSelfCommissionSnapshotsEveryFuturePaidPurchase(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	startedAt := at.Add(-time.Hour)
	effectiveAt := at.Add(-time.Minute)
	for _, purchaseType := range []string{
		AffiliatePurchaseBalanceRedeem,
		AffiliatePurchaseBalanceTopup,
		AffiliatePurchaseMonthlyRedeem,
		AffiliatePurchaseMonthlyPayment,
	} {
		purchaseType := purchaseType
		t.Run(purchaseType, func(t *testing.T) {
			t.Parallel()
			repo := &affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
				ProgramMode:               AffiliateProgramModeLive,
				ProgramLive:               true,
				ProgramStartedAt:          &startedAt,
				SelfPartnerStatus:         "active",
				SelfPartnerRiskStatus:     "clear",
				SelfCommissionEnabled:     true,
				SelfCommissionRateBPS:     AffiliateAgentPoolRateBPS,
				SelfCommissionEffectiveAt: &effectiveAt,
			}}
			result, err := NewAffiliateRewardService(repo).ProcessFirstPaidPurchase(
				context.Background(),
				AffiliateFirstPaidPurchaseInput{
					UserID:       41,
					PurchaseType: purchaseType,
					PurchaseKey:  purchaseType + ":41",
					AmountMicros: 20_000_000,
					OccurredAt:   at,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.SourcePolicy != AffiliateSourcePolicyPartnerSelfUsage ||
				result.DirectPartnerID != 41 ||
				result.CustomerRebateRateBPS != 0 ||
				result.PartnerCommissionRateBPS != AffiliateAgentPoolRateBPS {
				t.Fatalf("self purchase policy = %+v", result)
			}
			if result.Claimed || len(repo.posted) != 0 || len(repo.scheduled) != 0 {
				t.Fatalf("self purchase must not claim first-paid rewards: %+v", result)
			}
		})
	}
}

func TestAffiliateSelfCommissionRequiresClearActiveUnboundEffectivePolicy(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	programStarted := at.Add(-time.Hour)
	futureProgramStart := at.Add(time.Hour)
	past := at.Add(-time.Minute)
	future := at.Add(time.Minute)
	tests := []struct {
		name   string
		mutate func(*AffiliateFirstPaidContext)
	}{
		{
			name: "program not started",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.ProgramStartedAt = &futureProgramStart
			},
		},
		{
			name: "legacy upstream",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.HasUpstreamRelationship = true
			},
		},
		{
			name: "suspended",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.SelfPartnerStatus = "suspended"
			},
		},
		{
			name: "risk hold",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.SelfPartnerRiskStatus = "review"
			},
		},
		{
			name: "disabled",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.SelfCommissionEnabled = false
			},
		},
		{
			name: "invalid rate",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.SelfCommissionRateBPS = 900
			},
		},
		{
			name: "not effective",
			mutate: func(value *AffiliateFirstPaidContext) {
				value.SelfCommissionEffectiveAt = &future
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			firstPaid := AffiliateFirstPaidContext{
				ProgramMode:               AffiliateProgramModeLive,
				ProgramLive:               true,
				ProgramStartedAt:          &programStarted,
				SelfPartnerStatus:         "active",
				SelfPartnerRiskStatus:     "clear",
				SelfCommissionEnabled:     true,
				SelfCommissionRateBPS:     AffiliateAgentPoolRateBPS,
				SelfCommissionEffectiveAt: &past,
			}
			tt.mutate(&firstPaid)
			result, err := NewAffiliateRewardService(
				&affiliateRewardRepoStub{firstPaid: firstPaid},
			).ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
				UserID:       42,
				PurchaseType: AffiliatePurchaseBalanceTopup,
				PurchaseKey:  "gated:" + tt.name,
				AmountMicros: 20_000_000,
				OccurredAt:   at,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.SourcePolicy != AffiliateSourcePolicyNone ||
				result.DirectPartnerID != 0 {
				t.Fatalf("gated self policy = %+v", result)
			}
		})
	}
}

func TestAffiliateBindingAlwaysPrecedesSelfCommission(t *testing.T) {
	t.Parallel()
	effectiveAt := time.Now().Add(-time.Hour)
	result, err := NewAffiliateRewardService(
		&affiliateRewardRepoStub{firstPaid: AffiliateFirstPaidContext{
			ProgramMode:               AffiliateProgramModeLive,
			ProgramLive:               true,
			BindingKind:               AffiliateBindingAgent,
			InviterUserID:             7,
			BindingAgentID:            7,
			BindingCustomerRateBPS:    300,
			BindingPartnerRateBPS:     700,
			SelfPartnerStatus:         "active",
			SelfPartnerRiskStatus:     "clear",
			SelfCommissionEnabled:     true,
			SelfCommissionRateBPS:     AffiliateAgentPoolRateBPS,
			SelfCommissionEffectiveAt: &effectiveAt,
		}},
	).ProcessFirstPaidPurchase(context.Background(), AffiliateFirstPaidPurchaseInput{
		UserID:       43,
		PurchaseType: AffiliatePurchaseBalanceTopup,
		PurchaseKey:  "binding-precedence",
		AmountMicros: 20_000_000,
		OccurredAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SourcePolicy != AffiliateSourcePolicyPartnerUsage ||
		result.DirectPartnerID != 7 ||
		result.CustomerRebateRateBPS != 300 ||
		result.PartnerCommissionRateBPS != 700 {
		t.Fatalf("binding did not precede self policy: %+v", result)
	}
}

func TestAffiliatePolicyFromPurchaseResultPreservesSelfPolicy(t *testing.T) {
	t.Parallel()
	policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(
		true,
		&AffiliateFirstPaidPurchaseResult{
			ProgramMode:              AffiliateProgramModeLive,
			ProgramLive:              true,
			SourcePolicy:             AffiliateSourcePolicyPartnerSelfUsage,
			DirectPartnerID:          44,
			CustomerRebateRateBPS:    0,
			PartnerCommissionRateBPS: AffiliateAgentPoolRateBPS,
		},
	)
	if policy != AffiliateSourcePolicyPartnerSelfUsage ||
		partnerID != 44 ||
		customerRate != 0 ||
		partnerRate != AffiliateAgentPoolRateBPS {
		t.Fatalf("self consumption policy = (%s,%d,%d,%d)", policy, partnerID, customerRate, partnerRate)
	}
}
