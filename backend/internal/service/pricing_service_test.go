package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/claude"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type pricingRemoteClientSpy struct {
	pricingCalls int
	hashCalls    int
}

func (c *pricingRemoteClientSpy) FetchPricingJSON(_ context.Context, _ string) ([]byte, error) {
	c.pricingCalls++
	return []byte(`{}`), nil
}

func (c *pricingRemoteClientSpy) FetchHashText(_ context.Context, _ string) (string, error) {
	c.hashCalls++
	return "", nil
}

func bundledPricingFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json")
	require.FileExists(t, path)
	return path
}

func loadBundledPricingService(t *testing.T) *PricingService {
	t.Helper()

	svc := &PricingService{}
	require.NoError(t, svc.loadPricingData(bundledPricingFile(t)))
	return svc
}

func TestParsePricingData_ParsesPriorityAndServiceTierFields(t *testing.T) {
	svc := &PricingService{}
	body := []byte(`{
		"gpt-5.4": {
			"input_cost_per_token": 0.0000025,
			"input_cost_per_token_priority": 0.000005,
			"output_cost_per_token": 0.000015,
			"output_cost_per_token_priority": 0.00003,
			"cache_creation_input_token_cost": 0.0000025,
			"cache_read_input_token_cost": 0.00000025,
			"cache_read_input_token_cost_priority": 0.0000005,
			"supports_service_tier": true,
			"supports_prompt_caching": true,
			"litellm_provider": "openai",
			"mode": "chat"
		}
	}`)

	data, err := svc.parsePricingData(body)
	require.NoError(t, err)
	pricing := data["gpt-5.4"]
	require.NotNil(t, pricing)
	require.InDelta(t, 5e-6, pricing.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 3e-5, pricing.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 5e-7, pricing.CacheReadInputTokenCostPriority, 1e-12)
	require.True(t, pricing.SupportsServiceTier)
}

func TestGetModelPricing_Gpt53CodexSparkUsesGpt51CodexPricing(t *testing.T) {
	sparkPricing := &LiteLLMModelPricing{InputCostPerToken: 1}
	gpt53Pricing := &LiteLLMModelPricing{InputCostPerToken: 9}

	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": sparkPricing,
			"gpt-5.3":       gpt53Pricing,
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex-spark")
	require.Same(t, sparkPricing, got)
}

func TestGetModelPricing_Gpt53CodexFallbackStillUsesGpt52Codex(t *testing.T) {
	gpt52CodexPricing := &LiteLLMModelPricing{InputCostPerToken: 2}

	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.2-codex": gpt52CodexPricing,
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex")
	require.Same(t, gpt52CodexPricing, got)
}

func TestGetModelPricing_OpenAIFallbackMatchedLoggedAsInfo(t *testing.T) {
	logSink, restore := captureStructuredLog(t)
	defer restore()

	gpt52CodexPricing := &LiteLLMModelPricing{InputCostPerToken: 2}
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.2-codex": gpt52CodexPricing,
		},
	}

	got := svc.GetModelPricing("gpt-5.3-codex")
	require.Same(t, gpt52CodexPricing, got)

	require.True(t, logSink.ContainsMessageAtLevel("[Pricing] OpenAI fallback matched gpt-5.3-codex -> gpt-5.2-codex", "info"))
	require.False(t, logSink.ContainsMessageAtLevel("[Pricing] OpenAI fallback matched gpt-5.3-codex -> gpt-5.2-codex", "warn"))
}

func TestGetModelPricing_Gpt54UsesStaticFallbackWhenRemoteMissing(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": &LiteLLMModelPricing{InputCostPerToken: 1.25e-6},
			"gpt-5.5":       &LiteLLMModelPricing{InputCostPerToken: 9e-6},
		},
	}

	got := svc.GetModelPricing("gpt-5.4")
	require.NotNil(t, got)
	require.InDelta(t, 2.5e-6, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 1.5e-5, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 2.5e-7, got.CacheReadInputTokenCost, 1e-12)
	require.Equal(t, 272000, got.LongContextInputTokenThreshold)
	require.InDelta(t, 2.0, got.LongContextInputCostMultiplier, 1e-12)
	require.InDelta(t, 1.5, got.LongContextOutputCostMultiplier, 1e-12)
}

func TestGetModelPricing_Gpt55UsesOfficialStaticFallback(t *testing.T) {
	svc := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			"gpt-5.1-codex": &LiteLLMModelPricing{InputCostPerToken: 1.25e-6},
		},
	}

	got := svc.GetModelPricing("gpt-5.5-openai-compact")
	require.NotNil(t, got)
	require.InDelta(t, 5e-6, got.InputCostPerToken, 1e-12)
	require.InDelta(t, 12.5e-6, got.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 30e-6, got.OutputCostPerToken, 1e-12)
	require.InDelta(t, 75e-6, got.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 5e-6, got.CacheCreationInputTokenCost, 1e-12)
	require.InDelta(t, 0.5e-6, got.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 1.25e-6, got.CacheReadInputTokenCostPriority, 1e-12)
	require.Equal(t, 272000, got.LongContextInputTokenThreshold)
	require.InDelta(t, 2.0, got.LongContextInputCostMultiplier, 1e-12)
	require.InDelta(t, 1.5, got.LongContextOutputCostMultiplier, 1e-12)
}

func TestPricingService_RemoteSyncDisabledUsesBundledFallback(t *testing.T) {
	dataDir := t.TempDir()
	client := &pricingRemoteClientSpy{}
	svc := NewPricingService(&config.Config{
		Pricing: config.PricingConfig{
			DataDir:      dataDir,
			FallbackFile: bundledPricingFile(t),
		},
	}, client)

	require.NoError(t, svc.Initialize())
	defer svc.Stop()

	require.NotEmpty(t, svc.pricingData)
	require.FileExists(t, filepath.Join(dataDir, "model_pricing.json"))
	require.Zero(t, client.pricingCalls)
	require.Zero(t, client.hashCalls)
}

func TestPricingService_RemoteSyncDisabledDoesNotFetchOnLocalFile(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "model_pricing.json"), []byte(`{}`), 0644))

	client := &pricingRemoteClientSpy{}
	svc := NewPricingService(&config.Config{
		Pricing: config.PricingConfig{
			DataDir:      dataDir,
			FallbackFile: bundledPricingFile(t),
		},
	}, client)

	require.NoError(t, svc.checkAndUpdatePricing())
	require.NoError(t, svc.syncWithRemote())
	require.NotEmpty(t, svc.pricingData)
	require.Zero(t, client.pricingCalls)
	require.Zero(t, client.hashCalls)
}

func TestBundledPricingCoversDefaultGatewayModels(t *testing.T) {
	svc := loadBundledPricingService(t)

	for _, model := range openai.DefaultModelIDs() {
		t.Run("openai/"+model, func(t *testing.T) {
			requireUsableTokenPricing(t, svc.GetModelPricing(model))
		})
	}

	for _, model := range claude.DefaultModelIDs() {
		t.Run("claude/"+model, func(t *testing.T) {
			requireUsableTokenPricing(t, svc.GetModelPricing(model))
		})
	}
}

func TestBundledPricingMatchesCurrentOfficialGatewayModelPrices(t *testing.T) {
	svc := loadBundledPricingService(t)

	tests := []struct {
		model             string
		input             float64
		output            float64
		cacheRead         float64
		inputPriority     float64
		outputPriority    float64
		cacheReadPriority float64
		cacheWrite5m      float64
		cacheWrite1h      float64
	}{
		{
			model:             "gpt-5.5",
			input:             5e-6,
			output:            30e-6,
			cacheRead:         0.5e-6,
			inputPriority:     12.5e-6,
			outputPriority:    75e-6,
			cacheReadPriority: 1.25e-6,
		},
		{
			model:             "gpt-5.4",
			input:             2.5e-6,
			output:            15e-6,
			cacheRead:         0.25e-6,
			inputPriority:     5e-6,
			outputPriority:    30e-6,
			cacheReadPriority: 0.5e-6,
		},
		{
			model:             "gpt-5.3-codex",
			input:             1.75e-6,
			output:            14e-6,
			cacheRead:         0.175e-6,
			inputPriority:     3.5e-6,
			outputPriority:    28e-6,
			cacheReadPriority: 0.35e-6,
		},
		{
			model:        "claude-opus-4-7",
			input:        5e-6,
			output:       25e-6,
			cacheRead:    0.5e-6,
			cacheWrite5m: 6.25e-6,
			cacheWrite1h: 10e-6,
		},
		{
			model:        "claude-sonnet-4-6",
			input:        3e-6,
			output:       15e-6,
			cacheRead:    0.3e-6,
			cacheWrite5m: 3.75e-6,
		},
		{
			model:        "claude-haiku-4-5-20251001",
			input:        1e-6,
			output:       5e-6,
			cacheRead:    0.1e-6,
			cacheWrite5m: 1.25e-6,
			cacheWrite1h: 2e-6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := svc.GetModelPricing(tt.model)
			requireUsableTokenPricing(t, pricing)
			require.InDelta(t, tt.input, pricing.InputCostPerToken, 1e-12)
			require.InDelta(t, tt.output, pricing.OutputCostPerToken, 1e-12)
			require.InDelta(t, tt.cacheRead, pricing.CacheReadInputTokenCost, 1e-12)
			if tt.inputPriority > 0 {
				require.InDelta(t, tt.inputPriority, pricing.InputCostPerTokenPriority, 1e-12)
				require.InDelta(t, tt.outputPriority, pricing.OutputCostPerTokenPriority, 1e-12)
				require.InDelta(t, tt.cacheReadPriority, pricing.CacheReadInputTokenCostPriority, 1e-12)
			}
			if tt.cacheWrite5m > 0 {
				require.InDelta(t, tt.cacheWrite5m, pricing.CacheCreationInputTokenCost, 1e-12)
			}
			if tt.cacheWrite1h > 0 {
				require.InDelta(t, tt.cacheWrite1h, pricing.CacheCreationInputTokenCostAbove1hr, 1e-12)
			}
		})
	}
}

func requireUsableTokenPricing(t *testing.T, pricing *LiteLLMModelPricing) {
	t.Helper()

	require.NotNil(t, pricing)
	require.Greater(t, pricing.InputCostPerToken, 0.0)
	require.Greater(t, pricing.OutputCostPerToken, 0.0)
	require.GreaterOrEqual(t, pricing.CacheReadInputTokenCost, 0.0)
	require.NotEmpty(t, pricing.LiteLLMProvider)
}

func TestParsePricingData_PreservesPriorityAndServiceTierFields(t *testing.T) {
	raw := map[string]any{
		"gpt-5.4": map[string]any{
			"input_cost_per_token":                 2.5e-6,
			"input_cost_per_token_priority":        5e-6,
			"output_cost_per_token":                15e-6,
			"output_cost_per_token_priority":       30e-6,
			"cache_read_input_token_cost":          0.25e-6,
			"cache_read_input_token_cost_priority": 0.5e-6,
			"supports_service_tier":                true,
			"supports_prompt_caching":              true,
			"litellm_provider":                     "openai",
			"mode":                                 "chat",
		},
	}
	body, err := json.Marshal(raw)
	require.NoError(t, err)

	svc := &PricingService{}
	pricingMap, err := svc.parsePricingData(body)
	require.NoError(t, err)

	pricing := pricingMap["gpt-5.4"]
	require.NotNil(t, pricing)
	require.InDelta(t, 2.5e-6, pricing.InputCostPerToken, 1e-12)
	require.InDelta(t, 5e-6, pricing.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 15e-6, pricing.OutputCostPerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 0.25e-6, pricing.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 0.5e-6, pricing.CacheReadInputTokenCostPriority, 1e-12)
	require.True(t, pricing.SupportsServiceTier)
}

func TestParsePricingData_PreservesServiceTierPriorityFields(t *testing.T) {
	svc := &PricingService{}
	pricingData, err := svc.parsePricingData([]byte(`{
		"gpt-5.4": {
			"input_cost_per_token": 0.0000025,
			"input_cost_per_token_priority": 0.000005,
			"output_cost_per_token": 0.000015,
			"output_cost_per_token_priority": 0.00003,
			"cache_read_input_token_cost": 0.00000025,
			"cache_read_input_token_cost_priority": 0.0000005,
			"supports_service_tier": true,
			"litellm_provider": "openai",
			"mode": "chat"
		}
	}`))
	require.NoError(t, err)

	pricing := pricingData["gpt-5.4"]
	require.NotNil(t, pricing)
	require.InDelta(t, 0.0000025, pricing.InputCostPerToken, 1e-12)
	require.InDelta(t, 0.000005, pricing.InputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 0.000015, pricing.OutputCostPerToken, 1e-12)
	require.InDelta(t, 0.00003, pricing.OutputCostPerTokenPriority, 1e-12)
	require.InDelta(t, 0.00000025, pricing.CacheReadInputTokenCost, 1e-12)
	require.InDelta(t, 0.0000005, pricing.CacheReadInputTokenCostPriority, 1e-12)
	require.True(t, pricing.SupportsServiceTier)
}
