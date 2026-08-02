package service

import (
	"fmt"
	"time"
)

// RedeemPaidValue returns the cash portion of a sellable balance code. Older
// codes have PaidValue=0, so they retain the historical Value-as-cash behavior.
func RedeemPaidValue(code *RedeemCode) float64 {
	if code == nil || code.Purpose != RedeemCodePurposeSaleRecharge {
		return 0
	}
	if code.PaidValue > 0 && code.PaidValue <= code.Value {
		return code.PaidValue
	}
	return code.Value
}

func RedeemBonusValue(code *RedeemCode) float64 {
	if code == nil {
		return 0
	}
	bonus := code.Value - RedeemPaidValue(code)
	if bonus < 0 {
		return 0
	}
	return bonus
}

func buildRedeemBalanceLots(
	code *RedeemCode,
	userID int64,
	sourceType string,
	paid bool,
	policy string,
	partnerID int64,
	customerRate int32,
	partnerRate int32,
	occurredAt time.Time,
) []AffiliateBalanceLotInput {
	if code == nil || code.Value <= 0 {
		return nil
	}
	if !paid {
		return []AffiliateBalanceLotInput{{
			UserID:          userID,
			SourceType:      sourceType,
			SourceID:        code.ID,
			SourceKey:       fmt.Sprintf("redeem:balance:%d", code.ID),
			AmountMicros:    AffiliateMicrosFromFloat(code.Value),
			AffiliatePolicy: AffiliateSourcePolicyNone,
			OccurredAt:      occurredAt,
		}}
	}

	paidValue := RedeemPaidValue(code)
	lots := []AffiliateBalanceLotInput{{
		UserID:                   userID,
		SourceType:               sourceType,
		SourceID:                 code.ID,
		SourceKey:                fmt.Sprintf("redeem:balance:%d", code.ID),
		AmountMicros:             AffiliateMicrosFromFloat(paidValue),
		AffiliatePolicy:          policy,
		DirectPartnerID:          partnerID,
		CustomerRebateRateBPS:    customerRate,
		PartnerCommissionRateBPS: partnerRate,
		OccurredAt:               occurredAt,
	}}
	if bonus := RedeemBonusValue(code); bonus > 0 {
		lots = append(lots, AffiliateBalanceLotInput{
			UserID:          userID,
			SourceType:      AffiliateSourceGift,
			SourceID:        code.ID,
			SourceKey:       fmt.Sprintf("redeem:promotion:%d", code.ID),
			AmountMicros:    AffiliateMicrosFromFloat(bonus),
			AffiliatePolicy: AffiliateSourcePolicyNone,
			OccurredAt:      occurredAt,
		})
	}
	return lots
}
