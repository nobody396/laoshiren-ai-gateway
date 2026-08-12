package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const codexNativeImageBridgeTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

type codexNativeImageBridgeAccountRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r codexNativeImageBridgeAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform != platform {
			continue
		}
		for _, membership := range account.AccountGroups {
			if membership.GroupID == groupID {
				accounts = append(accounts, account)
				break
			}
		}
	}
	return accounts, nil
}

type codexNativeImageBridgeUpstream struct {
	service.HTTPUpstream
	lastRequest *http.Request
}

type codexNativeImageBridgeFailoverUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
}

func (u *codexNativeImageBridgeFailoverUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	body := `{
		"id":"resp_codex_native_image_bridge",
		"status":"completed",
		"model":"gpt-5.6-sol",
		"output":[{"type":"image_generation_call","status":"completed","result":"` + codexNativeImageBridgeTestPNG + `"}],
		"usage":{"input_tokens":12,"output_tokens":24}
	}`
	if accountID == 33 {
		body = `{
			"id":"resp_codex_native_image_no_tool",
			"status":"completed",
			"model":"gpt-5.6-sol",
			"output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":""}]}],
			"usage":{"input_tokens":467,"output_tokens":8}
		}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (u *codexNativeImageBridgeUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.lastRequest = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"x-request-id": []string{"req_codex_native_image_bridge"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"resp_codex_native_image_bridge",
			"object":"response",
			"status":"completed",
			"model":"gpt-5.6-sol",
			"output":[{"id":"ig_bridge","type":"image_generation_call","status":"completed","result":"` + codexNativeImageBridgeTestPNG + `"}],
			"usage":{"input_tokens":12,"output_tokens":24,"total_tokens":36}
		}`)),
	}, nil
}

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

func TestOpenAIImages_OfficialCodexGPTImage2BridgesForMonthlyAndPublicGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groups := []*service.Group{
		{ID: 7, Name: "GPT Lite monthly", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeCredit},
		{ID: 6, Name: "OpenAI public", Platform: service.PlatformOpenAI},
	}

	for _, group := range groups {
		t.Run(group.Name, func(t *testing.T) {
			account := service.Account{
				ID:          3300 + group.ID,
				Name:        "image-route-for-" + group.Name,
				Platform:    service.PlatformOpenAI,
				Type:        service.AccountTypeAPIKey,
				Status:      service.StatusActive,
				Schedulable: true,
				Concurrency: 4,
				Credentials: map[string]any{
					"api_key":  "test-only-key",
					"base_url": "https://upstream.example.test/v1",
					"model_mapping": map[string]any{
						"gpt-5.6-sol": "gpt-5.6-sol",
					},
				},
				Extra: map[string]any{
					service.OpenAIImageGenerationPriorityExtraKey: 1,
					service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
				},
				AccountGroups: []service.AccountGroup{{AccountID: 3300 + group.ID, GroupID: group.ID, Priority: 1}},
			}
			upstream := &codexNativeImageBridgeUpstream{}
			concurrencyCache := &concurrencyCacheMock{
				acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
				acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
			}
			gatewayCfg := &config.Config{RunMode: config.RunModeStandard}
			billingCfg := &config.Config{RunMode: config.RunModeSimple}
			billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, billingCfg)
			concurrencyService := service.NewConcurrencyService(concurrencyCache)
			gatewayService := service.NewOpenAIGatewayService(
				codexNativeImageBridgeAccountRepo{accounts: []service.Account{account}},
				nil, nil, nil, nil, nil, nil, gatewayCfg, nil, concurrencyService, nil, nil,
				billingCacheService, upstream,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)
			handler := NewOpenAIGatewayHandler(
				gatewayService,
				concurrencyService,
				billingCacheService,
				&service.APIKeyService{},
				nil, nil, gatewayCfg,
			)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(
				`{"model":"gpt-image-2","prompt":"帮我生成一张一个男人吃早饭的图片","n":1,"response_format":"b64_json"}`,
			))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("User-Agent", "Codex Desktop/0.147.0-alpha.1.2 (Mac OS 26.3.2; arm64) (Codex Desktop; 26.730.61639)")
			apiKey := &service.APIKey{ID: 44 + group.ID, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

			handler.Images(c)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.NotContains(t, recorder.Body.String(), "No available compatible accounts")
			require.Equal(t, codexNativeImageBridgeTestPNG, gjson.GetBytes(recorder.Body.Bytes(), "data.0.b64_json").String())
			require.NotNil(t, upstream.lastRequest)
			require.Equal(t, "/v1/responses", upstream.lastRequest.URL.Path)
			upstreamBody, err := io.ReadAll(upstream.lastRequest.Body)
			require.NoError(t, err)
			require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(upstreamBody, "model").String())
			require.Equal(t, "image_generation", gjson.GetBytes(upstreamBody, "tools.0.type").String())
			require.Equal(t, "image_generation", gjson.GetBytes(upstreamBody, "tool_choice.type").String())
			require.Equal(t, account.ID, c.GetInt64(opsAccountIDKey), "successful bridge must select a concrete account")
		})
	}
}

func TestOpenAIImages_EmptyImageCompletionFailsOverForMonthlyAndPublicGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groups := []*service.Group{
		{ID: 7, Name: "GPT Lite monthly", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeCredit},
		{ID: 6, Name: "OpenAI public", Platform: service.PlatformOpenAI},
	}

	for _, group := range groups {
		t.Run(group.Name, func(t *testing.T) {
			newAccount := func(id int64, name string, imagePriority int) service.Account {
				return service.Account{
					ID: id, Name: name, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Concurrency: 4,
					Credentials: map[string]any{
						"api_key": "test-only-key", "base_url": "https://upstream.example.test/v1",
						"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
					},
					Extra: map[string]any{
						service.OpenAIImageGenerationPriorityExtraKey: imagePriority,
						service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
					},
					AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: group.ID, Priority: imagePriority}},
				}
			}
			accounts := []service.Account{
				newAccount(33, "MoreCode primary image route", 1),
				newAccount(23, "PomoAI fallback image route", 2),
			}
			upstream := &codexNativeImageBridgeFailoverUpstream{}
			concurrencyCache := &concurrencyCacheMock{
				acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
				acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
			}
			gatewayCfg := &config.Config{RunMode: config.RunModeStandard}
			billingCfg := &config.Config{RunMode: config.RunModeSimple}
			billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, billingCfg)
			concurrencyService := service.NewConcurrencyService(concurrencyCache)
			gatewayService := service.NewOpenAIGatewayService(
				codexNativeImageBridgeAccountRepo{accounts: accounts},
				nil, nil, nil, nil, nil, nil, gatewayCfg, nil, concurrencyService, nil, nil,
				billingCacheService, upstream,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)
			handler := NewOpenAIGatewayHandler(
				gatewayService, concurrencyService, billingCacheService, &service.APIKeyService{},
				nil, nil, gatewayCfg,
			)

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(
				`{"model":"gpt-image-2","prompt":"生成一个女人做面膜的照片","n":1,"response_format":"b64_json"}`,
			))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("User-Agent", "Codex Desktop/0.147.0-alpha.1.2 (Mac OS 26.3.2; arm64) (Codex Desktop; 26.730.61639)")
			apiKey := &service.APIKey{ID: 44 + group.ID, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

			handler.Images(c)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, codexNativeImageBridgeTestPNG, gjson.GetBytes(recorder.Body.Bytes(), "data.0.b64_json").String())
			require.Equal(t, []int64{33, 23}, upstream.accountIDs, "MoreCode must remain first and PomoAI must be the bounded fallback")
			require.Equal(t, int64(23), c.GetInt64(opsAccountIDKey), "usage attribution must point at the account that returned the image")
		})
	}
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
