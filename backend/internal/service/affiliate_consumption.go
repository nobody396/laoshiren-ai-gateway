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
	UserID                   int64
	SourceType               string
	SourceID                 int64
	SourceKey                string
	AmountMicros             int64
	AffiliatePolicy          string
	DirectPartnerID          int64
	CustomerRebateRateBPS    int32
	PartnerCommissionRateBPS int32
	OccurredAt               time.Time
}

type AffiliateMonthlySubscription struct {
	UserSubscriptionID int64
	GroupID            int64
}

type AffiliateMonthlyEntitlementInput struct {
	UserID                   int64
	SourceType               string
	SourceID                 int64
	SourceKey                string
	ProductCode              string
	SalePriceMicros          int64
	CreditLimitMicros        int64
	AffiliatePolicy          string
	DirectPartnerID          int64
	CustomerRebateRateBPS    int32
	PartnerCommissionRateBPS int32
	PricingTableVersion      string
	StartsAt                 time.Time
	EndsAt                   time.Time
	Subscriptions            []AffiliateMonthlySubscription
}

type AffiliateConsumptionRepository interface {
	RecordBalanceLot(ctx context.Context, input AffiliateBalanceLotInput) error
	RecordMonthlyEntitlement(ctx context.Context, input AffiliateMonthlyEntitlementInput) error
}

// AffiliateProgramHandlesPurchase reports whether Affiliate V3 owns the
// purchase path. Shadow owns it for projection purposes and must suppress
// legacy monetary reward writers just like live mode does.
func AffiliateProgramHandlesPurchase(result *AffiliateFirstPaidPurchaseResult) bool {
	return result != nil &&
		(result.ProgramLive || result.ProgramMode == AffiliateProgramModeShadow)
}

func AffiliatePolicyFromPurchaseResult(
	paid bool,
	result *AffiliateFirstPaidPurchaseResult,
) (policy string, directPartnerID int64, customerRateBPS, partnerRateBPS int32) {
	if !paid {
		return AffiliateSourcePolicyNone, 0, 0, 0
	}
	if !AffiliateProgramHandlesPurchase(result) {
		return AffiliateSourcePolicyNone, 0, 0, 0
	}
	switch result.SourcePolicy {
	case AffiliateSourcePolicyOrdinaryFirstPaid:
		return result.SourcePolicy, result.DirectPartnerID, 0, 0
	case AffiliateSourcePolicyPartnerUsage:
		return result.SourcePolicy, result.DirectPartnerID, result.CustomerRebateRateBPS, result.PartnerCommissionRateBPS
	default:
		return AffiliateSourcePolicyNone, 0, 0, 0
	}
}

func AffiliatePolicyTracksConsumption(policy string) bool {
	return policy == AffiliateSourcePolicyOrdinaryFirstPaid ||
		policy == AffiliateSourcePolicyPartnerUsage
}

func AffiliateMonthlyCatalogIdentity(groups []*Group) (productCode, pricingTableVersion string) {
	if len(groups) == 2 {
		names := map[string]struct{}{}
		for _, group := range groups {
			if group == nil {
				return "", "legacy"
			}
			names[group.Name] = struct{}{}
		}
		for planID, expected := range costAccountingMonthlyCardGroupNames {
			if len(expected) != len(names) {
				continue
			}
			complete := true
			for _, name := range expected {
				if _, ok := names[name]; !ok {
					complete = false
					break
				}
			}
			if complete {
				return "monthly-" + planID + "-v3-20260728", AffiliateCommercialPricingTableVersionV3
			}
		}
	}
	return "", "legacy"
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
