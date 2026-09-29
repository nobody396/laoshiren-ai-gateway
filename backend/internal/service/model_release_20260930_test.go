//go:build unit

package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSeptember30ModelTariffs(t *testing.T) {
	for _, tc := range []struct {
		model                               string
		input, output, read, write, write1h float64
		threshold                           int
	}{
		{"claude-sonnet-5-5", 2, 10, .2, 2.5, 4, 0},
	} {
		t.Run(tc.model, func(t *testing.T) {
			svc := newTestBillingService()
			p, err := svc.GetModelPricing(tc.model)
			require.NoError(t, err)
			require.InDelta(t, tc.input/1e6, p.InputPricePerToken, 1e-12)
			require.InDelta(t, tc.output/1e6, p.OutputPricePerToken, 1e-12)
			require.InDelta(t, tc.read/1e6, p.CacheReadPricePerToken, 1e-12)
			require.InDelta(t, tc.write/1e6, p.CacheCreationPricePerToken, 1e-12)
			require.InDelta(t, tc.write1h/1e6, p.CacheCreation1hPrice, 1e-12)
			require.Equal(t, tc.threshold, p.LongContextInputThreshold)
			tokens := UsageTokens{InputTokens: 300001, OutputTokens: 200, CacheReadTokens: 500}
			cost, err := svc.CalculateCost(tc.model, tokens, .4)
			require.NoError(t, err)
			want := (300001*tc.input + 200*tc.output + 500*tc.read) / 1e6
			require.InDelta(t, want, cost.TotalCost, 1e-10)
			require.InDelta(t, want*.4, cost.ActualCost, 1e-10)
		})
	}
}
