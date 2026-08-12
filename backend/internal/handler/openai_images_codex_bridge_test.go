package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldBridgeCodexNativeImageRequest_OpenAIMonthlyAndPublicGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name  string
		group *service.Group
	}{
		{
			name: "monthly subscription group",
			group: &service.Group{
				ID:               7,
				Name:             "GPT Lite monthly",
				Platform:         service.PlatformOpenAI,
				SubscriptionType: service.SubscriptionTypeCredit,
			},
		},
		{
			name: "standard public pay as you go group",
			group: &service.Group{
				ID:       6,
				Name:     "OpenAI public",
				Platform: service.PlatformOpenAI,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCodexNativeImageBridgeTestContext(t, true)
			apiKey := &service.APIKey{GroupID: &tt.group.ID, Group: tt.group}
			parsed := &service.OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "a man eating breakfast"}

			require.True(t, shouldBridgeCodexNativeImageRequest(c, apiKey, parsed),
				"the Codex bridge must be selected before native Images account selection for every OpenAI billing group")
		})
	}
}

func TestShouldBridgeCodexNativeImageRequest_DoesNotHijackNativeImageModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	openAIGroup := &service.Group{ID: 6, Platform: service.PlatformOpenAI}
	grokGroup := &service.Group{ID: 2, Platform: service.PlatformGrok}

	tests := []struct {
		name          string
		model         string
		officialCodex bool
		group         *service.Group
		want          bool
	}{
		{name: "official Codex exact bridge model", model: "gpt-image-2", officialCodex: true, group: openAIGroup, want: true},
		{name: "model comparison is case insensitive", model: "GPT-IMAGE-2", officialCodex: true, group: openAIGroup, want: true},
		{name: "ordinary client keeps gpt-image-2 native", model: "gpt-image-2", officialCodex: false, group: openAIGroup, want: false},
		{name: "existing gpt-image-1 remains native", model: "gpt-image-1", officialCodex: true, group: openAIGroup, want: false},
		{name: "future gpt-image model remains native", model: "gpt-image-3", officialCodex: true, group: openAIGroup, want: false},
		{name: "Grok group remains on Grok media handler", model: "gpt-image-2", officialCodex: true, group: grokGroup, want: false},
		{name: "missing group fails closed", model: "gpt-image-2", officialCodex: true, group: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCodexNativeImageBridgeTestContext(t, tt.officialCodex)
			apiKey := &service.APIKey{Group: tt.group}
			if tt.group != nil {
				apiKey.GroupID = &tt.group.ID
			}
			parsed := &service.OpenAIImagesRequest{Model: tt.model, Prompt: "draw"}

			require.Equal(t, tt.want, shouldBridgeCodexNativeImageRequest(c, apiKey, parsed))
		})
	}

	t.Run("nil inputs fail closed", func(t *testing.T) {
		c := newCodexNativeImageBridgeTestContext(t, true)
		require.False(t, shouldBridgeCodexNativeImageRequest(c, nil, &service.OpenAIImagesRequest{Model: "gpt-image-2"}))
		require.False(t, shouldBridgeCodexNativeImageRequest(c, &service.APIKey{Group: openAIGroup}, nil))
		require.False(t, shouldBridgeCodexNativeImageRequest(nil, &service.APIKey{Group: openAIGroup}, &service.OpenAIImagesRequest{Model: "gpt-image-2"}))
	})
}

func newCodexNativeImageBridgeTestContext(t *testing.T, officialCodex bool) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	if officialCodex {
		c.Request.Header.Set("User-Agent", "Codex Desktop/0.147.0-alpha.1.2 (Mac OS 26.3.2; arm64) (Codex Desktop; 26.730.61639)")
	} else {
		c.Request.Header.Set("User-Agent", "curl/8.0")
	}
	return c
}
