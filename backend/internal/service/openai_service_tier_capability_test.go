package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/claude"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type serviceTierPricingStub struct {
	pricing *ChannelModelPricing
}

func TestAnthropicFastModeUsesTheSameGlobalAndChannelGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("anthropic-beta", claude.BetaFastMode)

	upstreamBody := "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5.4\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\ndata: [DONE]\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:              upstream,
		serviceTierCapabilityGate: true,
	}
	ctx := withOpenAIFastPolicyContext(context.Background(), DefaultOpenAIFastPolicySettings())

	result, err := svc.ForwardAsAnthropic(ctx, c, &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test", "chatgpt_account_id": "test"},
	}, body, "", "gpt-5.4")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.ServiceTier, "filtered Anthropic Fast must not be billed as priority")
	require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
}

func (s serviceTierPricingStub) GetChannelModelPricing(context.Context, int64, string) *ChannelModelPricing {
	return s.pricing
}

func TestOpenAIServiceTierCapabilityGateDefaultsClosed(t *testing.T) {
	group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	svc := &OpenAIGatewayService{serviceTierCapabilityGate: true, serviceTierPricing: serviceTierPricingStub{pricing: &ChannelModelPricing{}}}

	action, _ := svc.evaluateOpenAIFastPolicy(ctx, nil, "gpt-5.4", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionFilter, action)
}

func TestOpenAIServiceTierCapabilityGateAllowsVerifiedTierOnlyAfterGlobalPass(t *testing.T) {
	group := &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	verifiedAt := time.Now().UTC().Add(-time.Hour)
	multiplier := 2.0
	svc := &OpenAIGatewayService{serviceTierCapabilityGate: true, serviceTierPricing: serviceTierPricingStub{pricing: &ChannelModelPricing{
		FastSupported: true, FastVerifiedAt: &verifiedAt, FastMultiplier: &multiplier,
	}}}

	action, _ := svc.evaluateOpenAIFastPolicy(ctx, nil, "gpt-5.4", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionPass, action)

	ctx = withOpenAIFastPolicyContext(ctx, DefaultOpenAIFastPolicySettings())
	action, _ = svc.evaluateOpenAIFastPolicy(ctx, nil, "gpt-5.4", OpenAIFastTierPriority)
	require.Equal(t, BetaPolicyActionFilter, action, "the existing global policy remains authoritative")
}

func TestChannelPricingSupportsServiceTierRejectsFlagWithoutPriceOrTimestamp(t *testing.T) {
	now := time.Now().UTC()
	multiplier := 2.0
	require.False(t, channelPricingSupportsServiceTier(&ChannelModelPricing{FastSupported: true}, OpenAIFastTierPriority))
	require.False(t, channelPricingSupportsServiceTier(&ChannelModelPricing{FastSupported: true, FastVerifiedAt: &now}, OpenAIFastTierPriority))
	require.True(t, channelPricingSupportsServiceTier(&ChannelModelPricing{
		FastSupported: true, FastVerifiedAt: &now, FastMultiplier: &multiplier,
	}, OpenAIFastTierPriority))
}
