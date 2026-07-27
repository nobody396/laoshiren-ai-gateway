package service

import (
	"context"
	"math"
	"strings"
	"time"
)

const affiliateMicrosPerUnit int64 = 1_000_000

const (
	AffiliateSourcePaidRedeem         = "paid_redeem"
	AffiliateSourcePaidTopup          = "paid_topup"
	AffiliateSourceGift               = "gift"
	AffiliateSourceCompensation       = "compensation"
	AffiliateSourceInternalTest       = "internal_test"
	AffiliateSourceLegacyUnattributed = "legacy_unattributed"
	AffiliateSourceAdminAdjustment    = "admin_adjustment"
)

// AffiliateMicrosFromFloat floors positive values to micro-units. Rewards must
// never be calculated from more value than the billing ledger actually charged.
func AffiliateMicrosFromFloat(value float64) int64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	scaled := value * float64(affiliateMicrosPerUnit)
	if scaled >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(math.Floor(scaled + 1e-7))
}

type AffiliateBalanceLotInput struct {
	UserID            int64
	SourceType        string
	SourceID          int64
	SourceKey         string
	AmountMicros      int64
	AffiliateEligible bool
	OccurredAt        time.Time
}

type AffiliateMonthlySubscription struct {
	UserSubscriptionID int64
	GroupID            int64
}

type AffiliateMonthlyEntitlementInput struct {
	UserID            int64
	SourceType        string
	SourceID          int64
	SourceKey         string
	ProductCode       string
	SalePriceMicros   int64
	CreditLimitMicros int64
	AffiliateEligible bool
	StartsAt          time.Time
	EndsAt            time.Time
	Subscriptions     []AffiliateMonthlySubscription
}

type AffiliateConsumptionRepository interface {
	RecordBalanceLot(ctx context.Context, input AffiliateBalanceLotInput) error
	RecordMonthlyEntitlement(ctx context.Context, input AffiliateMonthlyEntitlementInput) error
}

func AffiliateSourceFromRedeem(purpose, salesStatus string) (sourceType string, eligible bool) {
	purpose = strings.TrimSpace(strings.ToLower(purpose))
	salesStatus = strings.TrimSpace(strings.ToLower(salesStatus))
	switch purpose {
	case RedeemCodePurposeSaleRecharge:
		if salesStatus == RedeemCodeSalesStatusSold {
			return AffiliateSourcePaidRedeem, true
		}
		return AffiliateSourceAdminAdjustment, false
	case RedeemCodePurposeGift:
		return AffiliateSourceGift, false
	case RedeemCodePurposeCompensation:
		return AffiliateSourceCompensation, false
	case RedeemCodePurposeInternalTest:
		return AffiliateSourceInternalTest, false
	case RedeemCodePurposeMigration:
		return AffiliateSourceLegacyUnattributed, false
	default:
		return AffiliateSourceAdminAdjustment, false
	}
}

// AffiliateMonthlyCreditLimitMicros returns one logical pool limit. Shared
// subscription groups mirror one quota, so the smallest positive limit wins.
func AffiliateMonthlyCreditLimitMicros(groups []*Group, validityDays int) int64 {
	var limit float64
	for _, group := range groups {
		if group == nil {
			continue
		}
		candidate := 0.0
		if group.MonthlyLimitUSD != nil && *group.MonthlyLimitUSD > 0 {
			candidate = *group.MonthlyLimitUSD
		} else if group.DailyLimitUSD != nil && *group.DailyLimitUSD > 0 && validityDays > 0 {
			candidate = *group.DailyLimitUSD * float64(validityDays)
		}
		if candidate > 0 && (limit == 0 || candidate < limit) {
			limit = candidate
		}
	}
	return AffiliateMicrosFromFloat(limit)
}
