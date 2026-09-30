//go:build unit

package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGPT61SolExactTariff(t *testing.T) {
	svc := newTestBillingService()
	p, err := svc.GetModelPricing("gpt-6.1-sol")
	require.NoError(t, err)
	require.InDelta(t, 2e-6, p.InputPricePerToken, 1e-12)
	require.InDelta(t, .1e-6, p.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 10e-6, p.OutputPricePerToken, 1e-12)
	require.InDelta(t, 2.5e-6, p.CacheCreationPricePerToken, 1e-12)
	require.Equal(t, 272000, p.LongContextInputThreshold)
	for _, input := range []int{1000, 271500, 271501} {
		tokens := UsageTokens{InputTokens: input, OutputTokens: 200, CacheReadTokens: 500, CacheCreationTokens: 100}
		cost, err := svc.CalculateCost("gpt-6.1-sol", tokens, .5)
		require.NoError(t, err)
		inMultiplier, outMultiplier := 1.0, 1.0
		if input+500 > 272000 {
			inMultiplier, outMultiplier = 2, 1.5
		}
		expected := (float64(input)*2*inMultiplier + 200*10*outMultiplier + 500*.1*inMultiplier + 100*2.5*inMultiplier) / 1e6
		require.InDelta(t, expected, cost.TotalCost, 1e-10)
		require.InDelta(t, expected*.5, cost.ActualCost, 1e-10)
	}
}
