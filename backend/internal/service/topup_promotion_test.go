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
		{name: "promotion 500", paidFen: 50_000, bonusFen: 7_500, creditFen: 57_500},
		{name: "non-card 600", paidFen: 60_000, bonusFen: 0, creditFen: 60_000},
		{name: "promotion 1000", paidFen: 100_000, bonusFen: 20_000, creditFen: 120_000},
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

func TestStoredTopupCreditQuoteDoesNotRetroactivelyPromoteHistoricalOrder(t *testing.T) {
	quote := StoredTopupCreditQuote(TopupPromotion500PaidFen, 0)
	require.Equal(t, TopupPromotion500PaidFen, quote.CreditedAmountCNYFen)
	require.Zero(t, quote.BonusAmountCNYFen)
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
	require.Equal(t, int64(200_000_000), bonus.AmountMicros)
	require.Equal(t, AffiliateSourcePolicyNone, bonus.AffiliatePolicy)
	require.Zero(t, bonus.DirectPartnerID)
	require.Zero(t, bonus.CustomerRebateRateBPS)
	require.Zero(t, bonus.PartnerCommissionRateBPS)
}
