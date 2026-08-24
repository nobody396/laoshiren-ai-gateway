package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuoteTopupCredit(t *testing.T) {
	tests := []struct {
		name      string
		paidFen   int
		bonusFen  int
		creditFen int
	}{
		{name: "ordinary 100", paidFen: 10_000, bonusFen: 0, creditFen: 10_000},
		{name: "ordinary 499", paidFen: 49_900, bonusFen: 0, creditFen: 49_900},
		{name: "promotion 500", paidFen: 50_000, bonusFen: 5_000, creditFen: 55_000},
		{name: "non-card 600", paidFen: 60_000, bonusFen: 0, creditFen: 60_000},
		{name: "promotion 1000", paidFen: 100_000, bonusFen: 10_000, creditFen: 110_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote := QuoteTopupCredit(tt.paidFen)
			require.Equal(t, tt.paidFen, quote.PaidAmountCNYFen)
			require.Equal(t, tt.bonusFen, quote.BonusAmountCNYFen)
			require.Equal(t, tt.creditFen, quote.CreditedAmountCNYFen)
			require.Equal(t, tt.bonusFen > 0, quote.HasPromotion())
		})
	}
}

func TestQuoteTopupProductCreditMultipliesPerCardPromotion(t *testing.T) {
	tests := []struct {
		name      string
		unitFen   int
		quantity  int
		paidFen   int
		bonusFen  int
		creditFen int
	}{
		{name: "three ordinary 20 cards", unitFen: TopupProduct20PaidFen, quantity: 3, paidFen: 6_000, creditFen: 6_000},
		{name: "three promotional 500 cards", unitFen: TopupPromotion500PaidFen, quantity: 3, paidFen: 150_000, bonusFen: 15_000, creditFen: 165_000},
		{name: "two promotional 1000 cards", unitFen: TopupPromotion1000PaidFen, quantity: 2, paidFen: 200_000, bonusFen: 20_000, creditFen: 220_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, err := QuoteTopupProductCredit(tt.unitFen, tt.quantity)
			require.NoError(t, err)
			require.Equal(t, tt.paidFen, quote.PaidAmountCNYFen)
			require.Equal(t, tt.bonusFen, quote.BonusAmountCNYFen)
			require.Equal(t, tt.creditFen, quote.CreditedAmountCNYFen)
		})
	}
}

func TestQuoteTopupProductCreditRejectsUnsupportedOrOversizedProducts(t *testing.T) {
	_, err := QuoteTopupProductCredit(6_000, 1)
	require.ErrorIs(t, err, ErrTopupInvalidProduct)

	_, err = QuoteTopupProductCredit(TopupProduct20PaidFen, 0)
	require.ErrorIs(t, err, ErrTopupInvalidQuantity)

	_, err = QuoteTopupProductCredit(TopupPromotion1000PaidFen, 4)
	require.ErrorIs(t, err, ErrTopupMaxAmount)
}

func TestStoredTopupCreditQuoteDoesNotRetroactivelyPromoteHistoricalOrder(t *testing.T) {
	quote := StoredTopupCreditQuote(TopupPromotion500PaidFen, 0)
	require.Equal(t, TopupPromotion500PaidFen, quote.CreditedAmountCNYFen)
	require.Zero(t, quote.BonusAmountCNYFen)
}

func TestTopupPromotionCatalogKeepsThirtyPercentShopStressMargin(t *testing.T) {
	for _, paidFen := range []int{TopupPromotion500PaidFen, TopupPromotion1000PaidFen} {
		quote := QuoteTopupCredit(paidFen)
		paid := float64(quote.PaidAmountCNYFen) / 100
		credited := float64(quote.CreditedAmountCNYFen) / 100
		margin := affiliateContributionMargin(
			paid,
			credited*affiliatePayAsYouGoStressCostPerCredit,
			AffiliateCommercialShopFeeBPS,
			AffiliateCommercialMaxRewardPoolBPS+AffiliateCommercialOperationalReserveBPS,
		)
		require.InDelta(t, 30.0, margin, 0.000001, "paid=%v credited=%v", paid, credited)
	}
}

func TestBuildTopupBalanceLotsExcludesPromotionFromAffiliate(t *testing.T) {
	now := time.Now()
	order := &TopupOrder{ID: 91, UserID: 42, AmountCNYFen: TopupPromotion1000PaidFen}
	lots := buildTopupBalanceLots(
		order,
		QuoteTopupCredit(order.AmountCNYFen),
		AffiliateSourcePolicyPartnerUsage,
		7,
		300,
		700,
		now,
	)
	require.Len(t, lots, 2)

	paid := lots[0]
	require.Equal(t, AffiliateSourcePaidTopup, paid.SourceType)
	require.Equal(t, int64(1_000_000_000), paid.AmountMicros)
	require.Equal(t, AffiliateSourcePolicyPartnerUsage, paid.AffiliatePolicy)
	require.Equal(t, int64(7), paid.DirectPartnerID)
	require.Equal(t, int32(300), paid.CustomerRebateRateBPS)
	require.Equal(t, int32(700), paid.PartnerCommissionRateBPS)

	bonus := lots[1]
	require.Equal(t, AffiliateSourceGift, bonus.SourceType)
	require.Equal(t, int64(100_000_000), bonus.AmountMicros)
	require.Equal(t, AffiliateSourcePolicyNone, bonus.AffiliatePolicy)
	require.Zero(t, bonus.DirectPartnerID)
	require.Zero(t, bonus.CustomerRebateRateBPS)
	require.Zero(t, bonus.PartnerCommissionRateBPS)
}
