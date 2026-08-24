package service

import (
	"fmt"
	"time"
)

const (
	TopupProduct20PaidFen        = 2_000
	TopupProduct50PaidFen        = 5_000
	TopupProduct100PaidFen       = 10_000
	TopupProduct300PaidFen       = 30_000
	TopupPromotion500PaidFen     = 50_000
	TopupPromotion500BonusFen    = 5_000
	TopupPromotion1000PaidFen    = 100_000
	TopupPromotion1000BonusFen   = 10_000
	topupCNYFenToAffiliateMicros = int64(10_000)
)

var supportedTopupProductAmounts = map[int]struct{}{
	TopupProduct20PaidFen:     {},
	TopupProduct50PaidFen:     {},
	TopupProduct100PaidFen:    {},
	TopupProduct300PaidFen:    {},
	TopupPromotion500PaidFen:  {},
	TopupPromotion1000PaidFen: {},
}

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

// QuoteTopupProductCredit prices a quantity of one catalog SKU. Promotions are
// calculated per card, so buying three ¥500 cards credits 3 × ¥550 rather than
// losing the bonus because the aggregate payment is ¥1500.
func QuoteTopupProductCredit(productAmountCNYFen, quantity int) (TopupCreditQuote, error) {
	if quantity < 1 {
		return TopupCreditQuote{}, ErrTopupInvalidQuantity
	}
	if _, ok := supportedTopupProductAmounts[productAmountCNYFen]; !ok {
		return TopupCreditQuote{}, ErrTopupInvalidProduct
	}
	if productAmountCNYFen > TopupMaxAmountFen/quantity {
		return TopupCreditQuote{}, ErrTopupMaxAmount
	}

	unitQuote := QuoteTopupCredit(productAmountCNYFen)
	paid := productAmountCNYFen * quantity
	if paid < TopupMinAmountFen {
		return TopupCreditQuote{}, ErrTopupMinAmount
	}
	if paid > TopupMaxAmountFen {
		return TopupCreditQuote{}, ErrTopupMaxAmount
	}
	return StoredTopupCreditQuote(paid, unitQuote.BonusAmountCNYFen*quantity), nil
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
