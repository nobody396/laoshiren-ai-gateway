//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newTestBillingService() *BillingService {
	return NewBillingService(&config.Config{}, nil)
}

func TestCalculateCost_BasicComputation(t *testing.T) {
	svc := newTestBillingService()

	// 使用 claude-sonnet-4 的回退价格：Input $3/MTok, Output $15/MTok
	tokens := UsageTokens{
		InputTokens:  1000,
		OutputTokens: 500,
	}
	cost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	// 1000 * 3e-6 = 0.003, 500 * 15e-6 = 0.0075
	expectedInput := 1000 * 3e-6
	expectedOutput := 500 * 15e-6
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.ActualCost, 1e-10)
}

func TestCalculateCost_WithCacheTokens(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 2000,
		CacheReadTokens:     3000,
	}
	cost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	expectedCacheCreation := 2000 * 3.75e-6
	expectedCacheRead := 3000 * 0.3e-6
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10)
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10)

	expectedTotal := cost.InputCost + cost.OutputCost + expectedCacheCreation + expectedCacheRead
	require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
}

func TestCalculateCost_RateMultiplier(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}

	cost1x, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	cost2x, err := svc.CalculateCost("claude-sonnet-4", tokens, 2.0)
	require.NoError(t, err)

	// TotalCost 不受倍率影响，ActualCost 翻倍
	require.InDelta(t, cost1x.TotalCost, cost2x.TotalCost, 1e-10)
	require.InDelta(t, cost1x.ActualCost*2, cost2x.ActualCost, 1e-10)
}

func TestCalculateCost_ZeroMultiplierDefaultsToOne(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 1000}

	costZero, err := svc.CalculateCost("claude-sonnet-4", tokens, 0)
	require.NoError(t, err)

	costOne, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, costOne.ActualCost, costZero.ActualCost, 1e-10)
}

func TestCalculateCost_NegativeMultiplierDefaultsToOne(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 1000}

	costNeg, err := svc.CalculateCost("claude-sonnet-4", tokens, -1.0)
	require.NoError(t, err)

	costOne, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, costOne.ActualCost, costNeg.ActualCost, 1e-10)
}

func TestGetModelPricing_FallbackMatchesByFamily(t *testing.T) {
	svc := newTestBillingService()

	tests := []struct {
		model         string
		expectedInput float64
	}{
		{"claude-opus-4.8-20260528", 5e-6},
		{"claude-opus-4.5-20250101", 5e-6},
		{"claude-3-opus-20240229", 15e-6},
		{"claude-sonnet-4-20250514", 3e-6},
		{"claude-3-5-sonnet-20241022", 3e-6},
		{"claude-3-5-haiku-20241022", 1e-6},
		{"claude-3-haiku-20240307", 0.25e-6},
	}

	for _, tt := range tests {
		pricing, err := svc.GetModelPricing(tt.model)
		require.NoError(t, err, "模型 %s", tt.model)
		require.InDelta(t, tt.expectedInput, pricing.InputPricePerToken, 1e-12, "模型 %s 输入价格", tt.model)
	}
}

func TestGetModelPricing_CaseInsensitive(t *testing.T) {
	svc := newTestBillingService()

	p1, err := svc.GetModelPricing("Claude-Sonnet-4")
	require.NoError(t, err)

	p2, err := svc.GetModelPricing("claude-sonnet-4")
	require.NoError(t, err)

	require.Equal(t, p1.InputPricePerToken, p2.InputPricePerToken)
}

func TestGetModelPricing_GLM53UsesCatalogPricing(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("GLM-5.3")
	require.NoError(t, err)
	require.InDelta(t, 8e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 28e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 2e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.Zero(t, pricing.LongContextInputThreshold)
}

func TestGetModelPricing_GeneratedCatalogAndRuntimeStayInSync(t *testing.T) {
	svc := newTestBillingService()

	for model, catalogPricing := range generatedCatalogBillingPrices {
		model := model
		catalogPricing := catalogPricing
		t.Run(model, func(t *testing.T) {
			runtimePricing, err := svc.GetModelPricing(model)
			require.NoError(t, err)
			for label, actual := range map[string]*ModelPricing{
				"runtime":  runtimePricing,
				"fallback": svc.fallbackPrices[model],
			} {
				require.InDelta(t, catalogPricing.InputPricePerToken, actual.InputPricePerToken, 1e-12,
					"%s catalog-managed model %s input price drift", label, model)
				require.InDelta(t, catalogPricing.OutputPricePerToken, actual.OutputPricePerToken, 1e-12,
					"%s catalog-managed model %s output price drift", label, model)
				require.InDelta(t, catalogPricing.CacheReadPricePerToken, actual.CacheReadPricePerToken, 1e-12,
					"%s catalog-managed model %s cache-read price drift", label, model)
				require.Equal(t, catalogPricing.LongContextInputThreshold, actual.LongContextInputThreshold,
					"%s catalog-managed model %s long-context threshold drift", label, model)
				require.InDelta(t, catalogPricing.LongContextInputMultiplier, actual.LongContextInputMultiplier, 1e-12,
					"%s catalog-managed model %s long-context input multiplier drift", label, model)
				require.InDelta(t, catalogPricing.LongContextOutputMultiplier, actual.LongContextOutputMultiplier, 1e-12,
					"%s catalog-managed model %s long-context output multiplier drift", label, model)
			}
		})
	}
}

func TestGetModelPricing_GLM53FlashUsesSupplierRateCard(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("GLM-5.3-Flash")
	require.NoError(t, err)
	require.InDelta(t, 0.4e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 1.4e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 0.115e-6, pricing.CacheReadPricePerToken, 1e-12)
}

func TestCalculateCost_Grok46UsesPomoRateCardAndLongContextTier(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("GROK-4.6")
	require.NoError(t, err)
	require.InDelta(t, 2e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 0.5e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 6e-6, pricing.OutputPricePerToken, 1e-12)
	require.Equal(t, 200000, pricing.LongContextInputThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 2.0, pricing.LongContextOutputMultiplier, 1e-12)

	tokens := UsageTokens{InputTokens: 200001, CacheReadTokens: 10, OutputTokens: 100}
	cost, err := svc.CalculateCost("grok-4.6", tokens, 0.4)
	require.NoError(t, err)
	expectedInput := float64(tokens.InputTokens) * 2e-6 * 2
	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.5e-6 * 2
	expectedOutput := float64(tokens.OutputTokens) * 6e-6 * 2
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, (expectedInput+expectedCacheRead+expectedOutput)*0.4, cost.ActualCost, 1e-10)
}

func TestGetModelPricing_UnknownClaudeModelFallsBackToSonnet(t *testing.T) {
	svc := newTestBillingService()

	// 不包含 opus/sonnet/haiku 关键词的 Claude 模型会走默认 Sonnet 价格
	pricing, err := svc.GetModelPricing("claude-unknown-model")
	require.NoError(t, err)
	require.InDelta(t, 3e-6, pricing.InputPricePerToken, 1e-12)
}

func TestGetModelPricing_UnknownOpenAIModelReturnsError(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-unknown-model")
	require.Error(t, err)
	require.Nil(t, pricing)
	require.Contains(t, err.Error(), "pricing not found")
}

func TestGetModelPricing_OpenAIGPT51Fallback(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-5.1")
	require.NoError(t, err)
	require.NotNil(t, pricing)
	require.InDelta(t, 1.25e-6, pricing.InputPricePerToken, 1e-12)
}

func TestGetModelPricing_OpenAIGPT54Fallback(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, pricing)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 15e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 0.25e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.Equal(t, 272000, pricing.LongContextInputThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
}

func TestGetModelPricing_OpenAIGPT54MiniFallbackMatchesObservedBilling(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-5.4-mini")
	require.NoError(t, err)
	require.NotNil(t, pricing)
	require.InDelta(t, 0.75e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 1.6e-6, pricing.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 4.5e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 6.4e-6, pricing.OutputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 0.075e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 1.6e-7, pricing.CacheReadPricePerTokenPriority, 1e-12)

	cost := (21*pricing.InputPricePerToken + 5*pricing.OutputPricePerToken) * 0.12
	require.InDelta(t, 0.00000459, cost, 0.0000001)
}

func TestGetModelPricing_OpenAIGPT56OfficialPricing(t *testing.T) {
	svc := newTestBillingService()

	cases := map[string]struct {
		model       string
		input       float64
		output      float64
		cacheWrite  float64
		cacheRead   float64
		longContext bool
	}{
		"sol":   {model: "gpt-5.6-sol", input: 5e-6, output: 30e-6, cacheWrite: 6.25e-6, cacheRead: 0.5e-6, longContext: true},
		"terra": {model: "gpt-5.6-terra-high", input: 2e-6, output: 12e-6, cacheWrite: 2.5e-6, cacheRead: 0.2e-6, longContext: true},
		"luna":  {model: "gpt-5.6-luna", input: 0.2e-6, output: 1.2e-6, cacheWrite: 0.25e-6, cacheRead: 0.02e-6, longContext: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pricing, err := svc.GetModelPricing(tc.model)
			require.NoError(t, err)
			require.InDelta(t, tc.input, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, tc.output, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, tc.cacheWrite, pricing.CacheCreationPricePerToken, 1e-12)
			require.InDelta(t, tc.cacheRead, pricing.CacheReadPricePerToken, 1e-12)
			if tc.longContext {
				require.Equal(t, 272000, pricing.LongContextInputThreshold)
				require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
				require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
			}
		})
	}
}

func TestGetModelPricing_OpenAIGPT6AstraOfficialPricing(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := svc.GetModelPricing("gpt-6-astra")
	require.NoError(t, err)
	require.InDelta(t, 10e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 50e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 12.5e-6, pricing.CacheCreationPricePerToken, 1e-12)
	require.InDelta(t, 1e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.Equal(t, 272000, pricing.LongContextInputThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
}

func TestGetModelPricing_OpenAIGPT55FallbackMatchesOfficialPricing(t *testing.T) {
	svc := newTestBillingService()

	gpt54, err := svc.GetModelPricing("gpt-5.4")
	require.NoError(t, err)
	gpt55, err := svc.GetModelPricing("gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, gpt55)

	require.InDelta(t, gpt54.InputPricePerToken*2, gpt55.InputPricePerToken, 1e-12)
	require.InDelta(t, gpt54.OutputPricePerToken*2, gpt55.OutputPricePerToken, 1e-12)
	require.InDelta(t, gpt54.CacheCreationPricePerToken*2, gpt55.CacheCreationPricePerToken, 1e-12)
	require.InDelta(t, gpt54.CacheReadPricePerToken*2, gpt55.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 12.5e-6, gpt55.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 75e-6, gpt55.OutputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 1.25e-6, gpt55.CacheReadPricePerTokenPriority, 1e-12)
	require.Equal(t, gpt54.LongContextInputThreshold, gpt55.LongContextInputThreshold)
	require.InDelta(t, gpt54.LongContextInputMultiplier, gpt55.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, gpt54.LongContextOutputMultiplier, gpt55.LongContextOutputMultiplier, 1e-12)
}

func TestCalculateCost_OpenAIGPT54LongContextAppliesWholeSessionMultipliers(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:  300000,
		OutputTokens: 4000,
	}

	cost, err := svc.CalculateCost("gpt-5.4-2026-03-05", tokens, 1.0)
	require.NoError(t, err)

	expectedInput := float64(tokens.InputTokens) * 2.5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 15e-6 * 1.5
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.ActualCost, 1e-10)
}

func TestCalculateCost_OpenAIGPT56SolLongContextAppliesOfficialMultipliers(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:         280000,
		OutputTokens:        4000,
		CacheReadTokens:     20000,
		CacheCreationTokens: 10000,
	}

	cost, err := svc.CalculateCost("gpt-5.6-sol", tokens, 1.0)
	require.NoError(t, err)

	expectedInput := float64(tokens.InputTokens) * 5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 30e-6 * 1.5
	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.5e-6 * 2.0
	expectedCacheCreation := float64(tokens.CacheCreationTokens) * 6.25e-6 * 2.0
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10)
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10)
	expectedTotal := expectedInput + expectedOutput + expectedCacheRead + expectedCacheCreation
	require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedTotal, cost.ActualCost, 1e-10)
}

func TestCalculateCost_DaybreakLongContextAppliesAcrossGroupTypes(t *testing.T) {
	svc := newTestBillingService()
	resolver := &ModelPricingResolver{}
	basePricing := &ModelPricing{
		InputPricePerToken:         5e-6,
		OutputPricePerToken:        30e-6,
		CacheCreationPricePerToken: 6.25e-6,
		CacheReadPricePerToken:     0.25e-6,
	}
	tokens := UsageTokens{
		InputTokens:         280000,
		OutputTokens:        4000,
		CacheReadTokens:     20000,
		CacheCreationTokens: 10000,
	}

	expectedInput := float64(tokens.InputTokens) * 5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 30e-6 * 1.5
	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.25e-6 * 2.0
	expectedCacheCreation := float64(tokens.CacheCreationTokens) * 6.25e-6 * 2.0
	expectedTotal := expectedInput + expectedOutput + expectedCacheRead + expectedCacheCreation

	groupRates := map[string]float64{
		"monthly-card": 0.37,
		"public":       0.5,
		"cyber":        2.0,
	}
	for groupType, groupRate := range groupRates {
		t.Run(groupType, func(t *testing.T) {
			cost, err := svc.CalculateCostUnified(CostInput{
				Model:          "gpt-daybreak-blue-latest",
				Tokens:         tokens,
				RateMultiplier: groupRate,
				Resolver:       resolver,
				Resolved: &ResolvedPricing{
					Mode:        BillingModeToken,
					BasePricing: basePricing,
				},
			})
			require.NoError(t, err)
			require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
			require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
			require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10)
			require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10)
			require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
			require.InDelta(t, expectedTotal*groupRate, cost.ActualCost, 1e-10)
		})
	}
}

func TestOpenAILongContextTierEligibility(t *testing.T) {
	for _, model := range []string{
		"gpt-5.4",
		"gpt-5.4-openai-compact",
		"gpt-5.5",
		"codex-auto-review",
		"gpt-5.6-sol",
		"gpt-5.6-terra-xhigh",
		"gpt-5.6-luna-openai-compact",
		"gpt-daybreak-blue-latest",
	} {
		require.True(t, isOpenAILongContextTierModel(model), model)
	}
	for _, model := range []string{
		"gpt-5.4-mini",
		"gpt-5.4-nano",
		"gpt-5.3-codex",
		"claude-opus-5",
		"grok-4.6",
	} {
		require.False(t, isOpenAILongContextTierModel(model), model)
	}
}

func TestCalculateCost_OpenAIGPT55UsesDoubleGPT54AndLongContext(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:  300000,
		OutputTokens: 4000,
	}

	cost, err := svc.CalculateCost("gpt-5.5-high", tokens, 1.0)
	require.NoError(t, err)

	expectedInput := float64(tokens.InputTokens) * 5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 30e-6 * 1.5
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.ActualCost, 1e-10)
}

// 回归测试 #2293：长上下文计费触发时，cache_read_tokens 也应应用 LongContextInputMultiplier。
// 修复前：CacheReadCost = tokens * 0.25e-6 （漏乘倍率，少计费用）。
// 修复后：CacheReadCost = tokens * 0.25e-6 * LongContextInputMultiplier(=2.0)。
func TestCalculateCost_OpenAIGPT54LongContextAppliesMultiplierToCacheRead(t *testing.T) {
	svc := newTestBillingService()

	// InputTokens + CacheReadTokens = 1000 + 300000 = 301000 > 272000 阈值
	tokens := UsageTokens{
		InputTokens:     1000,
		CacheReadTokens: 300000,
		OutputTokens:    1000,
	}

	cost, err := svc.CalculateCost("gpt-5.4-2026-03-05", tokens, 1.0)
	require.NoError(t, err)

	expectedInput := float64(tokens.InputTokens) * 2.5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 15e-6 * 1.5
	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.25e-6 * 2.0

	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10,
		"cache_read_cost should be scaled by LongContextInputMultiplier when long-context pricing applies (issue #2293)")

	expectedTotal := expectedInput + expectedOutput + expectedCacheRead
	require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedTotal, cost.ActualCost, 1e-10)
}

// 阴性测试：未触发长上下文时，cache_read_price 不应被错误地乘以倍率。
func TestCalculateCost_OpenAIGPT54NoLongContextKeepsCacheReadAtBasePrice(t *testing.T) {
	svc := newTestBillingService()

	// InputTokens + CacheReadTokens = 1000 + 100000 = 101000 < 272000 阈值，不触发长上下文
	tokens := UsageTokens{
		InputTokens:     1000,
		CacheReadTokens: 100000,
		OutputTokens:    1000,
	}

	cost, err := svc.CalculateCost("gpt-5.4-2026-03-05", tokens, 1.0)
	require.NoError(t, err)

	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.25e-6
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10,
		"cache_read_cost should remain at base price when below long-context threshold")
}

// 回归测试 #2816 follow-up：长上下文计费触发时，cache_creation_tokens 也应应用
// LongContextInputMultiplier。computeCacheCreationCost 直接读取 pricing.* 价格，
// 不经过 computeTokenBreakdown 内的 inputPrice / cacheReadPrice 倍率修改，因此
// 修复前 cache_creation 部分会按基础价计算，少计费用约 50%（默认倍率 2.0）。
func TestCalculateCost_OpenAIGPT54LongContextAppliesMultiplierToCacheCreation(t *testing.T) {
	svc := newTestBillingService()

	// InputTokens + CacheReadTokens = 1000 + 300000 = 301000 > 272000 阈值
	tokens := UsageTokens{
		InputTokens:         1000,
		CacheReadTokens:     300000,
		CacheCreationTokens: 10000,
		OutputTokens:        1000,
	}

	cost, err := svc.CalculateCost("gpt-5.4-2026-03-05", tokens, 1.0)
	require.NoError(t, err)

	// gpt-5.4 fallback: CacheCreationPricePerToken = 2.5e-6, LongContextInputMultiplier = 2.0
	expectedCacheCreation := float64(tokens.CacheCreationTokens) * 2.5e-6 * 2.0
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10,
		"cache_creation_cost should be scaled by LongContextInputMultiplier when long-context pricing applies")
}

// 阴性测试：未触发长上下文时，cache_creation_price 不应被错误地乘以倍率。
func TestCalculateCost_OpenAIGPT54NoLongContextKeepsCacheCreationAtBasePrice(t *testing.T) {
	svc := newTestBillingService()

	// InputTokens + CacheReadTokens = 1000 + 100000 = 101000 < 272000 阈值，不触发长上下文
	tokens := UsageTokens{
		InputTokens:         1000,
		CacheReadTokens:     100000,
		CacheCreationTokens: 10000,
		OutputTokens:        1000,
	}

	cost, err := svc.CalculateCost("gpt-5.4-2026-03-05", tokens, 1.0)
	require.NoError(t, err)

	expectedCacheCreation := float64(tokens.CacheCreationTokens) * 2.5e-6
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10,
		"cache_creation_cost should remain at base price when below long-context threshold")
}

// 覆盖 5m / 1h ephemeral 分类计费路径：长上下文触发时两档价格都应被倍率缩放。
// 使用手工构造的 pricing（参考 TestCalculateCost_SupportsCacheBreakdown 的写法）
// 以便同时控制 SupportsCacheBreakdown + 长上下文阈值。
func TestCalculateCost_LongContextAppliesMultiplierToCacheCreation5mAnd1h(t *testing.T) {
	svc := &BillingService{
		cfg: &config.Config{},
		fallbackPrices: map[string]*ModelPricing{
			"claude-sonnet-4": {
				InputPricePerToken:          3e-6,
				OutputPricePerToken:         15e-6,
				CacheReadPricePerToken:      0.3e-6,
				SupportsCacheBreakdown:      true,
				CacheCreation5mPrice:        4e-6,
				CacheCreation1hPrice:        5e-6,
				LongContextInputThreshold:   272000,
				LongContextInputMultiplier:  2.0,
				LongContextOutputMultiplier: 1.5,
			},
		},
	}

	// InputTokens + CacheReadTokens = 1000 + 300000 = 301000 > 272000 阈值
	tokens := UsageTokens{
		InputTokens:           1000,
		CacheReadTokens:       300000,
		CacheCreation5mTokens: 8000,
		CacheCreation1hTokens: 4000,
		OutputTokens:          1000,
	}

	cost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	expected5m := float64(tokens.CacheCreation5mTokens) * 4e-6 * 2.0
	expected1h := float64(tokens.CacheCreation1hTokens) * 5e-6 * 2.0
	require.InDelta(t, expected5m+expected1h, cost.CacheCreationCost, 1e-10,
		"both 5m and 1h cache_creation prices should be scaled by LongContextInputMultiplier")
}

func TestGetFallbackPricing_FamilyMatching(t *testing.T) {
	svc := newTestBillingService()

	tests := []struct {
		name             string
		model            string
		expectedInput    float64
		expectNilPricing bool
	}{
		{name: "empty model", model: "   ", expectNilPricing: true},
		{name: "claude opus 4.8", model: "claude-opus-4.8-20260528", expectedInput: 5e-6},
		{name: "claude opus 4.6", model: "claude-opus-4.6-20260201", expectedInput: 5e-6},
		{name: "claude opus 4.5 alt separator", model: "claude-opus-4-5-20260101", expectedInput: 5e-6},
		{name: "claude generic model fallback sonnet", model: "claude-foo-bar", expectedInput: 3e-6},
		{name: "gemini explicit fallback", model: "gemini-3-1-pro", expectedInput: 2e-6},
		{name: "gemini unknown no fallback", model: "gemini-2.0-pro", expectNilPricing: true},
		{name: "openai gpt5.6 sol", model: "gpt-5.6-sol", expectedInput: 5e-6},
		{name: "openai gpt5.6 terra", model: "gpt-5.6-terra-high", expectedInput: 2e-6},
		{name: "openai gpt5.6 luna", model: "gpt-5.6-luna", expectedInput: 0.2e-6},
		{name: "openai gpt5.5", model: "gpt-5.5-openai-compact", expectedInput: 5e-6},
		{name: "openai gpt5.1", model: "gpt-5.1", expectedInput: 1.25e-6},
		{name: "openai gpt5.4", model: "gpt-5.4", expectedInput: 2.5e-6},
		{name: "openai gpt5.3 codex", model: "gpt-5.3-codex", expectedInput: 1.5e-6},
		{name: "openai gpt5.1 codex max alias", model: "gpt-5.1-codex-max", expectedInput: 1.5e-6},
		{name: "openai codex mini latest alias", model: "codex-mini-latest", expectedInput: 1.5e-6},
		{name: "openai unknown no fallback", model: "gpt-unknown-model", expectNilPricing: true},
		{name: "non supported family", model: "qwen-max", expectNilPricing: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := svc.getFallbackPricing(tt.model)
			if tt.expectNilPricing {
				require.Nil(t, pricing)
				return
			}
			require.NotNil(t, pricing)
			require.InDelta(t, tt.expectedInput, pricing.InputPricePerToken, 1e-12)
		})
	}
}
func TestCalculateCostWithLongContext_BelowThreshold(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:     50000,
		OutputTokens:    1000,
		CacheReadTokens: 100000,
	}
	// 总输入 150k < 200k 阈值，应走正常计费
	cost, err := svc.CalculateCostWithLongContext("claude-sonnet-4", tokens, 1.0, 200000, 2.0)
	require.NoError(t, err)

	normalCost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, normalCost.ActualCost, cost.ActualCost, 1e-10)
}

func TestCalculateCostWithLongContext_AboveThreshold_CacheExceedsThreshold(t *testing.T) {
	svc := newTestBillingService()

	// 缓存 210k + 输入 10k = 220k > 200k 阈值
	// 缓存已超阈值：范围内 200k 缓存，范围外 10k 缓存 + 10k 输入
	tokens := UsageTokens{
		InputTokens:     10000,
		OutputTokens:    1000,
		CacheReadTokens: 210000,
	}
	cost, err := svc.CalculateCostWithLongContext("claude-sonnet-4", tokens, 1.0, 200000, 2.0)
	require.NoError(t, err)

	// 范围内：200k cache + 0 input + 1k output
	inRange, _ := svc.CalculateCost("claude-sonnet-4", UsageTokens{
		InputTokens:     0,
		OutputTokens:    1000,
		CacheReadTokens: 200000,
	}, 1.0)

	// 范围外：10k cache + 10k input，倍率 2.0
	outRange, _ := svc.CalculateCost("claude-sonnet-4", UsageTokens{
		InputTokens:     10000,
		CacheReadTokens: 10000,
	}, 2.0)

	require.InDelta(t, inRange.ActualCost+outRange.ActualCost, cost.ActualCost, 1e-10)
}

func TestCalculateCostWithLongContext_AboveThreshold_CacheBelowThreshold(t *testing.T) {
	svc := newTestBillingService()

	// 缓存 100k + 输入 150k = 250k > 200k 阈值
	// 缓存未超阈值：范围内 100k 缓存 + 100k 输入，范围外 50k 输入
	tokens := UsageTokens{
		InputTokens:     150000,
		OutputTokens:    1000,
		CacheReadTokens: 100000,
	}
	cost, err := svc.CalculateCostWithLongContext("claude-sonnet-4", tokens, 1.0, 200000, 2.0)
	require.NoError(t, err)

	require.True(t, cost.ActualCost > 0, "费用应大于 0")

	// 正常费用不含长上下文
	normalCost, _ := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.True(t, cost.ActualCost > normalCost.ActualCost, "长上下文费用应高于正常费用")
}

func TestCalculateCostWithLongContext_DisabledThreshold(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 300000, CacheReadTokens: 0}

	// threshold <= 0 应禁用长上下文计费
	cost1, err := svc.CalculateCostWithLongContext("claude-sonnet-4", tokens, 1.0, 0, 2.0)
	require.NoError(t, err)

	cost2, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, cost2.ActualCost, cost1.ActualCost, 1e-10)
}

func TestCalculateCostWithLongContext_ExtraMultiplierLessEqualOne(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 300000}

	// extraMultiplier <= 1 应禁用长上下文计费
	cost, err := svc.CalculateCostWithLongContext("claude-sonnet-4", tokens, 1.0, 200000, 1.0)
	require.NoError(t, err)

	normalCost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	require.InDelta(t, normalCost.ActualCost, cost.ActualCost, 1e-10)
}

func TestCalculateImageCost(t *testing.T) {
	svc := newTestBillingService()

	price := 0.134
	cfg := &ImagePriceConfig{Price1K: &price}
	cost := svc.CalculateImageCost("gpt-image-1", "1K", 3, cfg, 1.0)

	require.InDelta(t, 0.134*3, cost.TotalCost, 1e-10)
	require.InDelta(t, 0.134*3, cost.ActualCost, 1e-10)
}

func TestCalculateVideoCostUsesSeparateConfig(t *testing.T) {
	svc := newTestBillingService()

	imagePrice := 0.4
	videoPrice := 0.08
	imageCost := svc.CalculateImageCost("grok-imagine-video", "2K", 1, &ImagePriceConfig{Price2K: &imagePrice}, 1.0)
	videoCost := svc.CalculateVideoCost("grok-imagine-video", "480p", 1, 10, &VideoPriceConfig{Price480P: &videoPrice}, 0.5)

	require.InDelta(t, 0.4, imageCost.TotalCost, 1e-10)
	require.InDelta(t, 0.8, videoCost.TotalCost, 1e-10)
	require.InDelta(t, 0.4, videoCost.ActualCost, 1e-10)
	require.Equal(t, string(BillingModeVideo), videoCost.BillingMode)
}

func TestCalculateVideoCostBillsPerSecond(t *testing.T) {
	svc := newTestBillingService()

	oneSecond := svc.CalculateVideoCost("grok-imagine-video", "720p", 1, 1, nil, 1.0)
	fifteenSeconds := svc.CalculateVideoCost("grok-imagine-video", "720p", 1, 15, nil, 1.0)
	defaultDuration := svc.CalculateVideoCost("grok-imagine-video", "720p", 1, 0, nil, 1.0)
	clampedDuration := svc.CalculateVideoCost("grok-imagine-video", "720p", 1, 999, nil, 1.0)

	require.InDelta(t, 0.07, oneSecond.TotalCost, 1e-10)
	require.InDelta(t, 0.07*15, fifteenSeconds.TotalCost, 1e-10)
	require.InDelta(t, 0.07*8, defaultDuration.TotalCost, 1e-10)
	require.InDelta(t, 0.07*15, clampedDuration.TotalCost, 1e-10)
}

func TestCalculateGrokImagineRateCard(t *testing.T) {
	svc := newTestBillingService()

	image1K := svc.CalculateImageCost("grok-imagine-image", "1K", 1, nil, 1.0)
	imageQuality2K := svc.CalculateImageCost("grok-imagine-image-quality", "2K", 1, nil, 1.0)
	video480P := svc.CalculateVideoCost("grok-imagine-video", "480p", 1, 1, nil, 1.0)
	video720P := svc.CalculateVideoCost("grok-imagine-video", "720p", 1, 1, nil, 1.0)
	video15_1080P := svc.CalculateVideoCost("grok-imagine-video-1.5", "1080p", 1, 1, nil, 1.0)

	require.InDelta(t, 0.02, image1K.TotalCost, 1e-10)
	require.InDelta(t, 0.07, imageQuality2K.TotalCost, 1e-10)
	require.InDelta(t, 0.05, video480P.TotalCost, 1e-10)
	require.InDelta(t, 0.07, video720P.TotalCost, 1e-10)
	require.InDelta(t, 0.25, video15_1080P.TotalCost, 1e-10)
}

func TestIsModelSupported(t *testing.T) {
	svc := newTestBillingService()

	require.True(t, svc.IsModelSupported("claude-sonnet-4"))
	require.True(t, svc.IsModelSupported("Claude-Opus-4.5"))
	require.True(t, svc.IsModelSupported("claude-3-haiku"))
	require.False(t, svc.IsModelSupported("gpt-4o"))
	require.False(t, svc.IsModelSupported("gemini-pro"))
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	svc := newTestBillingService()

	cost, err := svc.CalculateCost("claude-sonnet-4", UsageTokens{}, 1.0)
	require.NoError(t, err)
	require.Equal(t, 0.0, cost.TotalCost)
	require.Equal(t, 0.0, cost.ActualCost)
}

func TestCalculateCostWithConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.5
	svc := NewBillingService(cfg, nil)

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}
	cost, err := svc.CalculateCostWithConfig("claude-sonnet-4", tokens)
	require.NoError(t, err)

	expected, _ := svc.CalculateCost("claude-sonnet-4", tokens, 1.5)
	require.InDelta(t, expected.ActualCost, cost.ActualCost, 1e-10)
}

func TestCalculateCostWithConfig_ZeroMultiplier(t *testing.T) {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 0
	svc := NewBillingService(cfg, nil)

	tokens := UsageTokens{InputTokens: 1000}
	cost, err := svc.CalculateCostWithConfig("claude-sonnet-4", tokens)
	require.NoError(t, err)

	// 倍率 <=0 时默认 1.0
	expected, _ := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.InDelta(t, expected.ActualCost, cost.ActualCost, 1e-10)
}

func TestGetEstimatedCost(t *testing.T) {
	svc := newTestBillingService()

	est, err := svc.GetEstimatedCost("claude-sonnet-4", 1000, 500)
	require.NoError(t, err)
	require.True(t, est > 0)
}

func TestListSupportedModels(t *testing.T) {
	svc := newTestBillingService()

	models := svc.ListSupportedModels()
	require.NotEmpty(t, models)
	require.GreaterOrEqual(t, len(models), 6)
}

func TestGetPricingServiceStatus_NilService(t *testing.T) {
	svc := newTestBillingService()

	status := svc.GetPricingServiceStatus()
	require.NotNil(t, status)
	require.Equal(t, "using fallback", status["last_updated"])
}

func TestForceUpdatePricing_NilService(t *testing.T) {
	svc := newTestBillingService()

	err := svc.ForceUpdatePricing()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not initialized")
}

func TestCalculateCostWithLongContext_PropagatesError(t *testing.T) {
	// 使用空的 fallback prices 让 GetModelPricing 失败
	svc := &BillingService{
		cfg:            &config.Config{},
		fallbackPrices: make(map[string]*ModelPricing),
	}

	tokens := UsageTokens{InputTokens: 300000, CacheReadTokens: 0}
	_, err := svc.CalculateCostWithLongContext("unknown-model", tokens, 1.0, 200000, 2.0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pricing not found")
}

func TestCalculateCost_SupportsCacheBreakdown(t *testing.T) {
	svc := &BillingService{
		cfg: &config.Config{},
		fallbackPrices: map[string]*ModelPricing{
			"claude-sonnet-4": {
				InputPricePerToken:     3e-6,
				OutputPricePerToken:    15e-6,
				SupportsCacheBreakdown: true,
				CacheCreation5mPrice:   4e-6, // per token
				CacheCreation1hPrice:   5e-6, // per token
			},
		},
	}

	tokens := UsageTokens{
		InputTokens:           1000,
		OutputTokens:          500,
		CacheCreation5mTokens: 100000,
		CacheCreation1hTokens: 50000,
	}
	cost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	expected5m := float64(tokens.CacheCreation5mTokens) * 4e-6
	expected1h := float64(tokens.CacheCreation1hTokens) * 5e-6
	require.InDelta(t, expected5m+expected1h, cost.CacheCreationCost, 1e-10)
}

func TestComputeCacheCreationCostCapsContradictoryBreakdownAtAggregate(t *testing.T) {
	svc := &BillingService{}
	pricing := &ModelPricing{SupportsCacheBreakdown: true, CacheCreation5mPrice: 1, CacheCreation1hPrice: 1}
	tokens := UsageTokens{CacheCreationTokens: 463184, CacheCreation5mTokens: 463184, CacheCreation1hTokens: 463184}

	cost := svc.computeCacheCreationCost(pricing, tokens, 1)

	require.Equal(t, float64(tokens.CacheCreationTokens), cost)
}

func TestNormalizeCacheCreationBreakdownPreservesRatioAndBounds(t *testing.T) {
	fiveMinute, oneHour := normalizeCacheCreationBreakdown(UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: 90, CacheCreation1hTokens: 60})
	require.Equal(t, 60, fiveMinute)
	require.Equal(t, 40, oneHour)

	fiveMinute, oneHour = normalizeCacheCreationBreakdown(UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: -50, CacheCreation1hTokens: 150})
	require.Equal(t, 0, fiveMinute)
	require.Equal(t, 100, oneHour)
}

func TestCalculateCost_LargeTokenCount(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	}
	cost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	// Input: 1M * 3e-6 = $3, Output: 1M * 15e-6 = $15
	require.InDelta(t, 3.0, cost.InputCost, 1e-6)
	require.InDelta(t, 15.0, cost.OutputCost, 1e-6)
	require.False(t, math.IsNaN(cost.TotalCost))
	require.False(t, math.IsInf(cost.TotalCost, 0))
}

func TestServiceTierCostMultiplier(t *testing.T) {
	require.InDelta(t, 2.0, serviceTierCostMultiplier("priority"), 1e-12)
	require.InDelta(t, 2.0, serviceTierCostMultiplier(" Priority "), 1e-12)
	require.InDelta(t, 0.5, serviceTierCostMultiplier("flex"), 1e-12)
	require.InDelta(t, 1.0, serviceTierCostMultiplier(""), 1e-12)
	require.InDelta(t, 1.0, serviceTierCostMultiplier("default"), 1e-12)
}

func TestCalculateCostWithServiceTier_OpenAIPriorityUsesPriorityPricing(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 20}

	baseCost, err := svc.CalculateCost("gpt-5.1-codex", tokens, 1.0)
	require.NoError(t, err)

	priorityCost, err := svc.CalculateCostWithServiceTier("gpt-5.1-codex", tokens, 1.0, "priority")
	require.NoError(t, err)

	require.InDelta(t, baseCost.InputCost*2, priorityCost.InputCost, 1e-10)
	require.InDelta(t, baseCost.OutputCost*2, priorityCost.OutputCost, 1e-10)
	require.InDelta(t, baseCost.CacheReadCost*2, priorityCost.CacheReadCost, 1e-10)
	require.InDelta(t, baseCost.TotalCost*2, priorityCost.TotalCost, 1e-10)
}

func TestCalculateCostWithServiceTier_FlexAppliesHalfMultiplier(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 40, CacheReadTokens: 20}

	baseCost, err := svc.CalculateCost("gpt-5.4", tokens, 1.0)
	require.NoError(t, err)

	flexCost, err := svc.CalculateCostWithServiceTier("gpt-5.4", tokens, 1.0, "flex")
	require.NoError(t, err)

	require.InDelta(t, baseCost.InputCost*0.5, flexCost.InputCost, 1e-10)
	require.InDelta(t, baseCost.OutputCost*0.5, flexCost.OutputCost, 1e-10)
	require.InDelta(t, baseCost.CacheCreationCost*0.5, flexCost.CacheCreationCost, 1e-10)
	require.InDelta(t, baseCost.CacheReadCost*0.5, flexCost.CacheReadCost, 1e-10)
	require.InDelta(t, baseCost.TotalCost*0.5, flexCost.TotalCost, 1e-10)
}

func TestCalculateCostWithServiceTier_PriorityFallsBackToTierMultiplierWithoutExplicitPriorityPrice(t *testing.T) {
	svc := newTestBillingService()
	tokens := UsageTokens{InputTokens: 120, OutputTokens: 30, CacheCreationTokens: 12, CacheReadTokens: 8}

	baseCost, err := svc.CalculateCost("claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	priorityCost, err := svc.CalculateCostWithServiceTier("claude-sonnet-4", tokens, 1.0, "priority")
	require.NoError(t, err)

	require.InDelta(t, baseCost.InputCost*2, priorityCost.InputCost, 1e-10)
	require.InDelta(t, baseCost.OutputCost*2, priorityCost.OutputCost, 1e-10)
	require.InDelta(t, baseCost.CacheCreationCost*2, priorityCost.CacheCreationCost, 1e-10)
	require.InDelta(t, baseCost.CacheReadCost*2, priorityCost.CacheReadCost, 1e-10)
	require.InDelta(t, baseCost.TotalCost*2, priorityCost.TotalCost, 1e-10)
}

func TestBillingServiceGetModelPricing_UsesDynamicPriorityFields(t *testing.T) {
	pricingSvc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.4": {
				InputCostPerToken:               2.5e-6,
				InputCostPerTokenPriority:       5e-6,
				OutputCostPerToken:              15e-6,
				OutputCostPerTokenPriority:      30e-6,
				CacheCreationInputTokenCost:     2.5e-6,
				CacheReadInputTokenCost:         0.25e-6,
				CacheReadInputTokenCostPriority: 0.5e-6,
				LongContextInputTokenThreshold:  272000,
				LongContextInputCostMultiplier:  2.0,
				LongContextOutputCostMultiplier: 1.5,
			},
		},
	}
	svc := NewBillingService(&config.Config{}, pricingSvc)

	pricing, err := svc.GetModelPricing("gpt-5.4")
	require.NoError(t, err)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 5e-6, pricing.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 15e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.OutputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 0.25e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 0.5e-6, pricing.CacheReadPricePerTokenPriority, 1e-12)
	require.Equal(t, 272000, pricing.LongContextInputThreshold)
	require.InDelta(t, 2.0, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.5, pricing.LongContextOutputMultiplier, 1e-12)
}

func TestBillingServiceGetModelPricing_OpenAIFallbackGpt52Variants(t *testing.T) {
	svc := newTestBillingService()

	gpt52, err := svc.GetModelPricing("gpt-5.2")
	require.NoError(t, err)
	require.NotNil(t, gpt52)
	require.InDelta(t, 1.75e-6, gpt52.InputPricePerToken, 1e-12)
	require.InDelta(t, 3.5e-6, gpt52.InputPricePerTokenPriority, 1e-12)

	gpt52Codex, err := svc.GetModelPricing("gpt-5.2-codex")
	require.NoError(t, err)
	require.NotNil(t, gpt52Codex)
	require.InDelta(t, 1.75e-6, gpt52Codex.InputPricePerToken, 1e-12)
	require.InDelta(t, 3.5e-6, gpt52Codex.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 28e-6, gpt52Codex.OutputPricePerTokenPriority, 1e-12)
}

func TestCalculateCostWithServiceTier_PriorityFallsBackToTierMultiplierWhenExplicitPriceMissing(t *testing.T) {
	svc := NewBillingService(&config.Config{}, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"custom-no-priority": {
				InputCostPerToken:           1e-6,
				OutputCostPerToken:          2e-6,
				CacheCreationInputTokenCost: 0.5e-6,
				CacheReadInputTokenCost:     0.25e-6,
			},
		},
	})
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 40, CacheReadTokens: 20}

	baseCost, err := svc.CalculateCost("custom-no-priority", tokens, 1.0)
	require.NoError(t, err)

	priorityCost, err := svc.CalculateCostWithServiceTier("custom-no-priority", tokens, 1.0, "priority")
	require.NoError(t, err)

	require.InDelta(t, baseCost.InputCost*2, priorityCost.InputCost, 1e-10)
	require.InDelta(t, baseCost.OutputCost*2, priorityCost.OutputCost, 1e-10)
	require.InDelta(t, baseCost.CacheCreationCost*2, priorityCost.CacheCreationCost, 1e-10)
	require.InDelta(t, baseCost.CacheReadCost*2, priorityCost.CacheReadCost, 1e-10)
	require.InDelta(t, baseCost.TotalCost*2, priorityCost.TotalCost, 1e-10)
}

func TestGetModelPricing_OpenAIGpt52FallbacksExposePriorityPrices(t *testing.T) {
	svc := newTestBillingService()

	gpt52, err := svc.GetModelPricing("gpt-5.2")
	require.NoError(t, err)
	require.InDelta(t, 1.75e-6, gpt52.InputPricePerToken, 1e-12)
	require.InDelta(t, 3.5e-6, gpt52.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 14e-6, gpt52.OutputPricePerToken, 1e-12)
	require.InDelta(t, 28e-6, gpt52.OutputPricePerTokenPriority, 1e-12)

	gpt52Codex, err := svc.GetModelPricing("gpt-5.2-codex")
	require.NoError(t, err)
	require.InDelta(t, 1.75e-6, gpt52Codex.InputPricePerToken, 1e-12)
	require.InDelta(t, 3.5e-6, gpt52Codex.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 14e-6, gpt52Codex.OutputPricePerToken, 1e-12)
	require.InDelta(t, 28e-6, gpt52Codex.OutputPricePerTokenPriority, 1e-12)
}

func TestGetModelPricing_MapsDynamicPriorityFieldsIntoBillingPricing(t *testing.T) {
	svc := NewBillingService(&config.Config{}, &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"dynamic-tier-model": {
				InputCostPerToken:                   1e-6,
				InputCostPerTokenPriority:           2e-6,
				OutputCostPerToken:                  3e-6,
				OutputCostPerTokenPriority:          6e-6,
				CacheCreationInputTokenCost:         4e-6,
				CacheCreationInputTokenCostAbove1hr: 5e-6,
				CacheReadInputTokenCost:             7e-7,
				CacheReadInputTokenCostPriority:     8e-7,
				LongContextInputTokenThreshold:      999,
				LongContextInputCostMultiplier:      1.5,
				LongContextOutputCostMultiplier:     1.25,
			},
		},
	})

	pricing, err := svc.GetModelPricing("dynamic-tier-model")
	require.NoError(t, err)
	require.InDelta(t, 1e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 2e-6, pricing.InputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 3e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 6e-6, pricing.OutputPricePerTokenPriority, 1e-12)
	require.InDelta(t, 4e-6, pricing.CacheCreation5mPrice, 1e-12)
	require.InDelta(t, 5e-6, pricing.CacheCreation1hPrice, 1e-12)
	require.True(t, pricing.SupportsCacheBreakdown)
	require.InDelta(t, 7e-7, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 8e-7, pricing.CacheReadPricePerTokenPriority, 1e-12)
	require.Equal(t, 999, pricing.LongContextInputThreshold)
	require.InDelta(t, 1.5, pricing.LongContextInputMultiplier, 1e-12)
	require.InDelta(t, 1.25, pricing.LongContextOutputMultiplier, 1e-12)
}
