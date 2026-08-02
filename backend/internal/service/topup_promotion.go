package service

import (
	"fmt"
	"time"
)

const (
	TopupPromotion500PaidFen     = 50_000
	TopupPromotion500BonusFen    = 7_500
	TopupPromotion1000PaidFen    = 100_000
	TopupPromotion1000BonusFen   = 20_000
	topupCNYFenToAffiliateMicros = int64(10_000)
)

// TopupCreditQuote separates cash paid from promotional value credited. Only
// PaidAmountCNYFen is affiliate-eligible; BonusAmountCNYFen is always a gift.
type TopupCreditQuote struct {
	PaidAmountCNYFen     int
	BonusAmountCNYFen    int
	CreditedAmountCNYFen int
}

func QuoteTopupCredit(amountCNYFen int) TopupCreditQuote {
	bonus := 0
	switch amountCNYFen {
	case TopupPromotion500PaidFen:
		bonus = TopupPromotion500BonusFen
	case TopupPromotion1000PaidFen:
		bonus = TopupPromotion1000BonusFen
	}
	return StoredTopupCreditQuote(amountCNYFen, bonus)
}

// StoredTopupCreditQuote uses the bonus persisted on the order. It must be
// used for completion and historical display so new catalog rules never apply
// retroactively to old orders with the same paid amount.
func StoredTopupCreditQuote(amountCNYFen, bonusAmountCNYFen int) TopupCreditQuote {
	if bonusAmountCNYFen < 0 {
		bonusAmountCNYFen = 0
	}
	return TopupCreditQuote{
		PaidAmountCNYFen:     amountCNYFen,
		BonusAmountCNYFen:    bonusAmountCNYFen,
		CreditedAmountCNYFen: amountCNYFen + bonusAmountCNYFen,
	}
}

func (q TopupCreditQuote) HasPromotion() bool {
	return q.BonusAmountCNYFen > 0
}

func buildTopupBalanceLots(
	order *TopupOrder,
	quote TopupCreditQuote,
	policy string,
	partnerID int64,
	customerRate int32,
	partnerRate int32,
	occurredAt time.Time,
) []AffiliateBalanceLotInput {
	if order == nil || quote.PaidAmountCNYFen <= 0 {
		return nil
	}
	lots := []AffiliateBalanceLotInput{{
		UserID:                   order.UserID,
		SourceType:               AffiliateSourcePaidTopup,
		SourceID:                 order.ID,
		SourceKey:                fmt.Sprintf("topup:balance:%d", order.ID),
		AmountMicros:             int64(quote.PaidAmountCNYFen) * topupCNYFenToAffiliateMicros,
		AffiliatePolicy:          policy,
		DirectPartnerID:          partnerID,
		CustomerRebateRateBPS:    customerRate,
		PartnerCommissionRateBPS: partnerRate,
		OccurredAt:               occurredAt,
	}}
	if quote.BonusAmountCNYFen > 0 {
		lots = append(lots, AffiliateBalanceLotInput{
			UserID:          order.UserID,
			SourceType:      AffiliateSourceGift,
			SourceID:        order.ID,
			SourceKey:       fmt.Sprintf("topup:promotion:%d", order.ID),
			AmountMicros:    int64(quote.BonusAmountCNYFen) * topupCNYFenToAffiliateMicros,
			AffiliatePolicy: AffiliateSourcePolicyNone,
			OccurredAt:      occurredAt,
		})
	}
	return lots
}
