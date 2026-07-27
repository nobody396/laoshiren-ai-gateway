package service

import (
	"math"
	"testing"
)

func TestAffiliateMicrosFromFloatNeverOverAttributes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   float64
		want int64
	}{
		{name: "zero", in: 0, want: 0},
		{name: "negative", in: -1, want: 0},
		{name: "one", in: 1, want: 1_000_000},
		{name: "sub micro floors", in: 0.0000019, want: 1},
		{name: "money", in: 50.12345678, want: 50_123_456},
		{name: "nan", in: math.NaN(), want: 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := AffiliateMicrosFromFloat(tt.in); got != tt.want {
				t.Fatalf("AffiliateMicrosFromFloat(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestAffiliateSourceFromRedeem(t *testing.T) {
	t.Parallel()
	source, eligible := AffiliateSourceFromRedeem(RedeemCodePurposeSaleRecharge, RedeemCodeSalesStatusSold)
	if source != AffiliateSourcePaidRedeem || !eligible {
		t.Fatalf("paid sold code = (%q, %v)", source, eligible)
	}
	source, eligible = AffiliateSourceFromRedeem(RedeemCodePurposeSaleRecharge, RedeemCodeSalesStatusInventory)
	if source != AffiliateSourceAdminAdjustment || eligible {
		t.Fatalf("unsold code = (%q, %v)", source, eligible)
	}
	source, eligible = AffiliateSourceFromRedeem(RedeemCodePurposeGift, RedeemCodeSalesStatusGifted)
	if source != AffiliateSourceGift || eligible {
		t.Fatalf("gift code = (%q, %v)", source, eligible)
	}
}

func TestAffiliateMonthlyCreditLimitUsesOneSharedPool(t *testing.T) {
	t.Parallel()
	a, b := 4_500.0, 4_000.0
	got := AffiliateMonthlyCreditLimitMicros([]*Group{
		{MonthlyLimitUSD: &a},
		{MonthlyLimitUSD: &b},
	}, 31)
	if got != 4_000_000_000 {
		t.Fatalf("limit = %d, want %d", got, int64(4_000_000_000))
	}
}
