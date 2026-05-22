package service

import (
	"bytes"
	"context"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAnthropicCachePolicy_DowngradesIllegal5mThen1hOrder(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"tools":[{"name":"t","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
	}`)

	out, decision := (&GatewayService{}).applyAnthropicCachePolicy(context.Background(), nil, anthropicCachePolicyTestAccount(nil), "claude-sonnet-4-6", body)
	require.NotNil(t, decision)
	require.True(t, decision.Downgraded)
	require.True(t, decision.Normalized)
	require.Equal(t, cacheTTLTarget5m, decision.ActualTTL)
	require.Equal(t, 2, decision.CacheControlPathsCount)
	require.Equal(t, cacheTTLTarget5m, gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
	require.Equal(t, cacheTTLTarget5m, gjson.GetBytes(out, "system.0.cache_control.ttl").String())
}

func TestAnthropicCachePolicy_Force1hNormalizesAllBlocks(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"tools":[{"name":"t","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hello","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]
	}`)
	account := anthropicCachePolicyTestAccount(map[string]any{"anthropic_cache_policy_mode": CachePolicyModeForce1h})

	out, decision := (&GatewayService{}).applyAnthropicCachePolicy(context.Background(), nil, account, "claude-sonnet-4-6", body)
	require.NotNil(t, decision)
	require.False(t, decision.Downgraded)
	require.Equal(t, cacheTTLTarget1h, decision.ActualTTL)
	require.Equal(t, 3, decision.CacheControlPathsCount)
	require.Equal(t, cacheTTLTarget1h, gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
	require.Equal(t, cacheTTLTarget1h, gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.Equal(t, cacheTTLTarget1h, gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
}

func TestAnthropicCachePolicy_NoCacheControlLeavesBodyUnchanged(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}`)

	out, decision := (&GatewayService{}).applyAnthropicCachePolicy(context.Background(), nil, anthropicCachePolicyTestAccount(nil), "claude-sonnet-4-6", body)
	require.NotNil(t, decision)
	require.Equal(t, 0, decision.CacheControlPathsCount)
	require.False(t, decision.Downgraded)
	require.True(t, bytes.Equal(body, out))
}

func TestAnthropicCachePolicy_ShadowAdaptiveKeepsActual5m(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"tools":[{"name":"t","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"` + longCachePolicyText(180000) + `","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]
	}`)
	account := anthropicCachePolicyTestAccount(map[string]any{"anthropic_cache_policy_mode": CachePolicyModeShadowAdaptive})

	out, decision := (&GatewayService{}).applyAnthropicCachePolicy(context.WithValue(context.Background(), ctxkey.ClientRequestID, "claude-desktop-test"), nil, account, "claude-sonnet-4-6", body)
	require.NotNil(t, decision)
	require.Equal(t, cacheTTLTarget5m, decision.ActualTTL)
	require.Equal(t, cacheTTLTarget1h, decision.ShadowTTL)
	require.Equal(t, cacheTTLTarget5m, gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
	require.Equal(t, cacheTTLTarget5m, gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
}

func TestAnthropicCachePolicy_TTLOrder400Predicate(t *testing.T) {
	body := []byte(`{"type":"error","error":{"message":"messages.8.content.0.cache_control.ttl: a ttl='1h' cache_control block must not come after a ttl='5m' cache_control block. Note that blocks are processed in the following order: tools, system, messages."}}`)
	require.True(t, isAnthropicCacheTTLOrder400(body))
	require.False(t, isAnthropicCacheTTLOrder400([]byte(`{"error":{"message":"other 400"}}`)))
}

func anthropicCachePolicyTestAccount(extra map[string]any) *Account {
	if extra == nil {
		extra = map[string]any{}
	}
	return &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: extra}
}

func longCachePolicyText(size int) string {
	out := make([]byte, size)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}
