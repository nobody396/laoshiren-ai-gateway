//go:build unit

package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSeptember23ModelTariffs(t *testing.T) {
	for _, tc := range []struct {
		model                      string
		input, output, read, write float64
		threshold                  int
	}{
		{"gpt-6-sol", 2, 10, .2, 2.5, 272000},
		{"gpt-6-luna", .1, .5, .01, .125, 272000},
		{"claude-opus-5-5", 4, 20, .2, 5, 0},
	} {
		t.Run(tc.model, func(t *testing.T) {
			svc := newTestBillingService()
			p, err := svc.GetModelPricing(tc.model)
			require.NoError(t, err)
			require.InDelta(t, tc.input/1e6, p.InputPricePerToken, 1e-12)
			require.InDelta(t, tc.output/1e6, p.OutputPricePerToken, 1e-12)
			require.InDelta(t, tc.read/1e6, p.CacheReadPricePerToken, 1e-12)
			require.InDelta(t, tc.write/1e6, p.CacheCreationPricePerToken, 1e-12)
			require.Equal(t, tc.threshold, p.LongContextInputThreshold)
			tokens := UsageTokens{InputTokens: 1000, OutputTokens: 200, CacheReadTokens: 500, CacheCreationTokens: 100}
			cost, err := svc.CalculateCost(tc.model, tokens, .5)
			require.NoError(t, err)
			want := (1000*tc.input + 200*tc.output + 500*tc.read + 100*tc.write) / 1e6
			require.InDelta(t, want, cost.TotalCost, 1e-10)
			require.InDelta(t, want*.5, cost.ActualCost, 1e-10)
			if tc.threshold > 0 {
				tokens.InputTokens = 300000
				cost, err = svc.CalculateCost(tc.model, tokens, .5)
				require.NoError(t, err)
				want = (300000*tc.input*2 + 200*tc.output*1.5 + 500*tc.read*2 + 100*tc.write*2) / 1e6
				require.InDelta(t, want, cost.TotalCost, 1e-10)
			}
		})
	}
}

func TestSeptember23CatalogCacheTariffsOverrideDynamicRates(t *testing.T) {
	for _, model := range []string{"gpt-6-sol", "gpt-6-luna", "claude-opus-5-5"} {
		t.Run(model, func(t *testing.T) {
			catalog := generatedCatalogBillingPrices[model]
			require.NotNil(t, catalog)
			auxiliary := &ModelPricing{CacheCreationPricePerToken: 99, CacheCreation5mPrice: 99, CacheCreation1hPrice: 99, InputPricePerTokenPriority: 7}
			merged := mergeGeneratedCatalogPricing(catalog, auxiliary)
			require.Equal(t, catalog.CacheCreationPricePerToken, merged.CacheCreationPricePerToken)
			require.Equal(t, catalog.CacheCreation5mPrice, merged.CacheCreation5mPrice)
			if catalog.CacheCreation1hPrice > 0 {
				require.Equal(t, catalog.CacheCreation1hPrice, merged.CacheCreation1hPrice)
			}
			require.Equal(t, catalog.SupportsCacheBreakdown, merged.SupportsCacheBreakdown)
			require.Equal(t, 7.0, merged.InputPricePerTokenPriority)
		})
	}
	// Unspecified cache rates must still be filled from the auxiliary source.
	merged := mergeGeneratedCatalogPricing(&ModelPricing{}, &ModelPricing{CacheCreationPricePerToken: 2, CacheCreation5mPrice: 3, CacheCreation1hPrice: 4, SupportsCacheBreakdown: true})
	require.Equal(t, 2.0, merged.CacheCreationPricePerToken)
	require.Equal(t, 3.0, merged.CacheCreation5mPrice)
	require.Equal(t, 4.0, merged.CacheCreation1hPrice)
	require.True(t, merged.SupportsCacheBreakdown)
}
