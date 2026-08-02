package service

import (
	"testing"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestSummarizeRedeemCodeBillingUsesActualPaidValue(t *testing.T) {
	codes := []*dbent.RedeemCode{
		{
			ID:          1,
			Value:       550,
			PaidValue:   500,
			Purpose:     RedeemCodePurposeSaleRecharge,
			SalesStatus: RedeemCodeSalesStatusSold,
			Status:      StatusUsed,
		},
		{
			ID:          2,
			Value:       100,
			Purpose:     RedeemCodePurposeSaleRecharge,
			SalesStatus: RedeemCodeSalesStatusSold,
			Status:      StatusUnused,
		},
	}

	summary := summarizeRedeemCodeBilling(codes, map[int64]*dbent.AccountChangeRecord{})
	require.Equal(t, 600.0, summary.SoldFaceValue)
	require.Equal(t, 500.0, summary.RedeemedSaleAmount)
	require.Equal(t, 100.0, summary.SoldUnredeemedFaceValue)
}
