package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRedeemCodeSaleEvidence(t *testing.T) {
	t.Run("sold requires order evidence", func(t *testing.T) {
		err := validateRedeemCodeSaleEvidence(
			RedeemCodePurposeSaleRecharge,
			RedeemCodeSalesStatusSold,
			"",
			"",
		)
		require.Error(t, err)
	})

	t.Run("order number is accepted", func(t *testing.T) {
		require.NoError(t, validateRedeemCodeSaleEvidence(
			RedeemCodePurposeSaleRecharge,
			RedeemCodeSalesStatusSold,
			"LD-ORDER-1",
			"",
		))
	})

	t.Run("payment evidence url is accepted", func(t *testing.T) {
		require.NoError(t, validateRedeemCodeSaleEvidence(
			RedeemCodePurposeSaleRecharge,
			RedeemCodeSalesStatusSold,
			"",
			"https://payments.example.test/receipt/1",
		))
	})

	t.Run("tests can never be sold", func(t *testing.T) {
		err := validateRedeemCodeSaleEvidence(
			RedeemCodePurposeInternalTest,
			RedeemCodeSalesStatusSold,
			"FAKE-ORDER",
			"",
		)
		require.Error(t, err)
	})
}

func TestApplyRedeemCodeBillingUpdate_PreservesEntitlement(t *testing.T) {
	usedBy := int64(1)
	groupID := int64(7)
	code := &RedeemCode{
		ID:           4839,
		Code:         "redacted",
		Type:         RedeemTypeSubscription,
		Value:        319,
		Status:       StatusUsed,
		UsedBy:       &usedBy,
		GroupID:      &groupID,
		GroupIDs:     []int64{7, 11, 35},
		ValidityDays: 31,
		Purpose:      RedeemCodePurposeSaleRecharge,
		SalesStatus:  RedeemCodeSalesStatusInventory,
	}
	purpose := RedeemCodePurposeInternalTest
	salesStatus := RedeemCodeSalesStatusVoid
	internalNotes := "verified administrator redemption test"

	applyRedeemCodeBillingUpdate(code, RedeemCodeBillingUpdateFields{
		Purpose:       &purpose,
		SalesStatus:   &salesStatus,
		InternalNotes: &internalNotes,
	})

	require.Equal(t, RedeemCodePurposeInternalTest, code.Purpose)
	require.Equal(t, RedeemCodeSalesStatusVoid, code.SalesStatus)
	require.Equal(t, internalNotes, code.InternalNotes)
	require.Equal(t, RedeemTypeSubscription, code.Type)
	require.Equal(t, float64(319), code.Value)
	require.Equal(t, StatusUsed, code.Status)
	require.Equal(t, usedBy, *code.UsedBy)
	require.Equal(t, groupID, *code.GroupID)
	require.Equal(t, []int64{7, 11, 35}, code.GroupIDs)
	require.Equal(t, 31, code.ValidityDays)
	require.NoError(t, validateRedeemCodeBillingClassification(code))
}
