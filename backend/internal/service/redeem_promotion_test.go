package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedeemPromotionSplitsPaidAndGiftAffiliateLots(t *testing.T) {
	code := &RedeemCode{
		ID:          88,
		Type:        RedeemTypeBalance,
		Value:       550,
		PaidValue:   500,
		Purpose:     RedeemCodePurposeSaleRecharge,
		SalesStatus: RedeemCodeSalesStatusSold,
	}
	require.Equal(t, 500.0, RedeemPaidValue(code))
	require.Equal(t, 50.0, RedeemBonusValue(code))

	lots := buildRedeemBalanceLots(code, 42, AffiliateSourcePaidRedeem, true, AffiliateSourcePolicyPartnerUsage, 7, 300, 700, time.Now())
	require.Len(t, lots, 2)
	require.Equal(t, int64(500_000_000), lots[0].AmountMicros)
	require.Equal(t, AffiliateSourcePolicyPartnerUsage, lots[0].AffiliatePolicy)
	require.Equal(t, int64(50_000_000), lots[1].AmountMicros)
	require.Equal(t, AffiliateSourceGift, lots[1].SourceType)
	require.Equal(t, AffiliateSourcePolicyNone, lots[1].AffiliatePolicy)
	require.Zero(t, lots[1].DirectPartnerID)
}

func TestRedeemPromotionKeepsLegacyPaidValueFallback(t *testing.T) {
	code := &RedeemCode{Value: 100, Purpose: RedeemCodePurposeSaleRecharge}
	require.Equal(t, 100.0, RedeemPaidValue(code))
	require.Zero(t, RedeemBonusValue(code))
}
