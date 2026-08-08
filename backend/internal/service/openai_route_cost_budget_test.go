package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testOpenAIRouteBudgetPolicy() OpenAIRoutePolicy {
	policy := DefaultOpenAIRoutePolicy()
	policy.TargetAverageMultiplier = 0.155
	policy.HardAverageMultiplier = 0.18
	policy.MaxCreditUSD = 100
	policy.EmergencyDebtLimitUSD = 0
	return policy
}

func settleOpenAIRouteCost(t *testing.T, ledger *OpenAIRouteBudgetLedger, rate, baseCost float64) {
	t.Helper()
	reservation, err := ledger.Reserve(rate, baseCost)
	require.NoError(t, err)
	require.NoError(t, ledger.Settle(reservation, baseCost, rate*baseCost))
}

func TestOpenAIRouteBudgetLedger_DynamicMultiplierMixMeetsTarget(t *testing.T) {
	tests := []struct {
		name  string
		rates []float64
	}{
		{
			name:  "ten percent at 0.20",
			rates: append(repeatOpenAIRouteRate(0.15, 90), repeatOpenAIRouteRate(0.20, 10)...),
		},
		{
			name:  "five percent at 0.25",
			rates: append(repeatOpenAIRouteRate(0.15, 95), repeatOpenAIRouteRate(0.25, 5)...),
		},
		{
			name: "mixed 0.20 and 0.25",
			rates: append(
				append(repeatOpenAIRouteRate(0.15, 92), repeatOpenAIRouteRate(0.20, 6)...),
				repeatOpenAIRouteRate(0.25, 2)...,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger, err := NewOpenAIRouteBudgetLedger(testOpenAIRouteBudgetPolicy(), 0, 0, 0)
			require.NoError(t, err)
			for _, rate := range tt.rates {
				settleOpenAIRouteCost(t, &ledger, rate, 1)
			}
			average, ok := ledger.AverageMultiplier()
			require.True(t, ok)
			require.InDelta(t, 0.155, average, 1e-12)
			require.InDelta(t, 0, ledger.CreditUSD, 1e-12)
		})
	}
}

func TestOpenAIRouteBudgetLedger_EmergencyDebtAndRecovery(t *testing.T) {
	policy := testOpenAIRouteBudgetPolicy()
	policy.EmergencyDebtLimitUSD = 0.01
	ledger, err := NewOpenAIRouteBudgetLedger(policy, 0, 0, 0)
	require.NoError(t, err)

	reservation, err := ledger.Reserve(0.20, 0.20)
	require.NoError(t, err)
	require.True(t, reservation.Emergency)
	require.InDelta(t, -0.009, ledger.CreditUSD, 1e-12)
	require.NoError(t, ledger.Settle(reservation, 0.20, 0.04))
	require.InDelta(t, -0.009, ledger.CreditUSD, 1e-12)

	settleOpenAIRouteCost(t, &ledger, 0.15, 1.8)
	require.InDelta(t, 0, ledger.CreditUSD, 1e-12)
}

func TestOpenAIRouteBudgetLedger_BlocksExpensiveRouteWhenBudgetOrHardLimitExceeded(t *testing.T) {
	policy := testOpenAIRouteBudgetPolicy()
	ledger, err := NewOpenAIRouteBudgetLedger(policy, 0, 0, 0)
	require.NoError(t, err)

	preview, err := ledger.Preview(0.20, 1)
	require.NoError(t, err)
	require.False(t, preview.Allowed)
	require.ErrorIs(t, func() error {
		_, reserveErr := ledger.Reserve(0.20, 1)
		return reserveErr
	}(), ErrOpenAIRouteBudgetExhausted)

	ledger, err = NewOpenAIRouteBudgetLedger(policy, 1, 10, 2)
	require.NoError(t, err)
	require.True(t, ledger.HardLimitExceeded())
	preview, err = ledger.Preview(0.20, 1)
	require.NoError(t, err)
	require.False(t, preview.Allowed)
	preview, err = ledger.Preview(0.15, 1)
	require.NoError(t, err)
	require.True(t, preview.Allowed)
}

func TestOpenAIRouteBudgetLedger_SettlementCorrectsEstimateAndCancelRefunds(t *testing.T) {
	policy := testOpenAIRouteBudgetPolicy()
	policy.HardAverageMultiplier = 0.30
	ledger, err := NewOpenAIRouteBudgetLedger(policy, 0.1, 0, 0)
	require.NoError(t, err)

	reservation, err := ledger.Reserve(0.20, 0.1)
	require.NoError(t, err)
	require.InDelta(t, 0.0955, ledger.CreditUSD, 1e-12)
	require.NoError(t, ledger.Settle(reservation, 0.2, 0.04))
	require.InDelta(t, 0.091, ledger.CreditUSD, 1e-12)
	require.Error(t, ledger.Settle(reservation, 0.2, 0.04))

	second, err := ledger.Reserve(0.25, 0.1)
	require.NoError(t, err)
	afterReserve := ledger.CreditUSD
	require.NoError(t, ledger.Cancel(second))
	require.Greater(t, ledger.CreditUSD, afterReserve)
	require.Error(t, ledger.Cancel(second))
}

func TestOpenAIRouteBudgetLedger_ZeroMultiplierEarnsBoundedCredit(t *testing.T) {
	policy := testOpenAIRouteBudgetPolicy()
	policy.MaxCreditUSD = 0.01
	ledger, err := NewOpenAIRouteBudgetLedger(policy, 0, 0, 0)
	require.NoError(t, err)

	settleOpenAIRouteCost(t, &ledger, 0, 100)
	require.InDelta(t, 0.01, ledger.CreditUSD, 1e-12)
}

func repeatOpenAIRouteRate(rate float64, count int) []float64 {
	values := make([]float64, count)
	for i := range values {
		values[i] = rate
	}
	return values
}
