package handler

import (
	"context"
	"encoding/json"
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

const (
	codexNativeImageBridgeTestPNG           = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	codexGeneratedImageAckHandlerTestSecret = "unit-test-only-codex-image-ack-handler-secret"
)

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

func (r codexNativeImageBridgeAccountRepo) ListSchedulableByPlatform(_ context.Context, platform string) ([]service.Account, error) {
	accounts := make([]service.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if account.Platform == platform {
			accounts = append(accounts, account)
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
	paths      []string
	models     []string
}

type codexFixedImageThreeLegUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	paths      []string
	models     []string
}

func (u *codexFixedImageThreeLegUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	u.paths = append(u.paths, req.URL.Path)
	requestBody, _ := io.ReadAll(req.Body)
	u.models = append(u.models, gjson.GetBytes(requestBody, "model").String())

	body := `{"created":1710000000,"data":[{"b64_json":"` + codexNativeImageBridgeTestPNG + `"}]}`
	switch accountID {
	case 33:
		body = `{"id":"resp_no_image_tool","status":"completed","model":"gpt-5.6-sol","output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":""}]}],"usage":{"input_tokens":20,"output_tokens":2}}`
	case 38:
		body = `{"created":1710000000,"data":[]}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

type codexTextIsolationUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	lastBody   []byte
}

func (u *codexTextIsolationUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	u.lastBody, _ = io.ReadAll(req.Body)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_text","object":"response","status":"completed","model":"gpt-5.6-sol","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
		)),
	}, nil
}

func (u *codexNativeImageBridgeFailoverUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	u.paths = append(u.paths, req.URL.Path)
	requestBody, _ := io.ReadAll(req.Body)
	u.models = append(u.models, gjson.GetBytes(requestBody, "model").String())
	body := `{
		"id":"resp_codex_native_image_bridge",
		"status":"completed",
		"model":"gpt-5.6-sol",
		"output":[{"type":"image_generation_call","status":"completed","result":"` + codexNativeImageBridgeTestPNG + `"}],
		"usage":{"input_tokens":12,"output_tokens":24}
	}`
	switch accountID {
	case 33:
		body = `{
			"id":"resp_codex_native_image_no_tool",
			"status":"completed",
			"model":"gpt-5.6-sol",
			"output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":""}]}],
			"usage":{"input_tokens":467,"output_tokens":8}
		}`
	case 34:
		body = `{
			"created":1710000000,
			"data":[{"b64_json":"` + codexNativeImageBridgeTestPNG + `"}],
			"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}
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
	requestBody, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(strings.NewReader(string(requestBody)))
	model := gjson.GetBytes(requestBody, "model").String()
	if strings.TrimSpace(model) == "" {
		model = "gpt-5.6-sol"
	}
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
				"model":"` + model + `",
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
		{ID: 7, Name: "GPT Lite monthly", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeCredit, AllowImageGeneration: true},
		{ID: 6, Name: "OpenAI public", Platform: service.PlatformOpenAI, AllowImageGeneration: true},
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
				// The image execution account is intentionally not bound to the
				// customer's billing group. Global image routing must still select it.
				AccountGroups: []service.AccountGroup{{AccountID: 3300 + group.ID, GroupID: 999, Priority: 1}},
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

func TestOpenAIImages_EmptyImageCompletionFallsBackToNativeImagesForMonthlyAndPublicGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groups := []*service.Group{
		{ID: 7, Name: "GPT Lite monthly", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeCredit, AllowImageGeneration: true},
		{ID: 6, Name: "OpenAI public", Platform: service.PlatformOpenAI, AllowImageGeneration: true},
	}

	for _, group := range groups {
		t.Run(group.Name, func(t *testing.T) {
			newResponsesAccount := func(id int64, name string, imagePriority int) service.Account {
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
					AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: 999, Priority: imagePriority}},
				}
			}
			nativeAccount := service.Account{
				ID: 34, Name: "PomoAI native Images fallback", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true, Concurrency: 4,
				Credentials: map[string]any{
					"api_key": "test-only-key", "base_url": "https://upstream.example.test/v1",
					"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
				},
				Extra: map[string]any{
					"supports_images": true,
					service.OpenAIImageGenerationPriorityExtraKey:  2,
					service.OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
					service.OpenAIImageGenerationTransportExtraKey: service.OpenAIImageGenerationTransportImages,
				},
				AccountGroups: []service.AccountGroup{{AccountID: 34, GroupID: 999, Priority: 90}},
			}
			accounts := []service.Account{
				newResponsesAccount(33, "MoreCode primary image route", 1),
				nativeAccount,
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
			require.Equal(t, []int64{33, 34}, upstream.accountIDs, "MoreCode must remain first and native PomoAI must be the bounded fallback")
			require.Equal(t, []string{"/v1/responses", "/v1/images/generations"}, upstream.paths)
			require.Equal(t, []string{"gpt-5.6-sol", "gpt-image-2-count"}, upstream.models)
			require.Equal(t, int64(34), c.GetInt64(opsAccountIDKey), "usage attribution must point at the account that returned the image")
			require.Equal(t, "gpt-image-2-count", c.GetString(opsUpstreamModelKey))
		})
	}
}

func TestOpenAIResponses_OfficialCodexImageRoutingRejectsRetiredModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, group := range []*service.Group{
		{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		{ID: 7, Name: "GPT monthly", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeCredit, AllowImageGeneration: true},
	} {
		for _, tc := range []struct {
			model   string
			retired bool
		}{
			{model: "gpt-5.6-luna", retired: true},
			{model: "gpt-5.6-luna-xhigh", retired: true},
			{model: "gpt-5.4-mini", retired: true},
			{model: "gpt-5.6-terra"},
			{model: "gpt-5.6-sol"},
			{model: "gpt-5.5"},
			{model: "gpt-5.4"},
		} {
			textModel := tc.model
			t.Run(group.Name+"/"+textModel, func(t *testing.T) {
				account := service.Account{
					ID:          3300 + group.ID,
					Name:        "morecode-fixed-image-primary",
					Platform:    service.PlatformOpenAI,
					Type:        service.AccountTypeAPIKey,
					Status:      service.StatusActive,
					Schedulable: true,
					Concurrency: 4,
					Credentials: map[string]any{
						"api_key": "test-only-key", "base_url": "https://upstream.example.test/v1",
						"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
					},
					Extra: map[string]any{
						service.OpenAIImageGenerationPriorityExtraKey: 1,
						service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
					},
					AccountGroups: []service.AccountGroup{{AccountID: 3300 + group.ID, GroupID: 999, Priority: 90}},
				}
				upstream := &codexNativeImageBridgeUpstream{}
				concurrencyCache := &concurrencyCacheMock{
					acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
					acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
				}
				gatewayCfg := newCodexImagePreviewTestConfig(t)
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
					gatewayService, concurrencyService, billingCacheService, &service.APIKeyService{},
					nil, nil, gatewayCfg,
				)

				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
					codexDesktopLiteImageBody(textModel, "帮我生成一张月球橘猫的图片", false),
				))
				setCodexDesktopLiteHeaders(c)
				apiKey := &service.APIKey{ID: 144 + group.ID, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
				c.Set(string(middleware.ContextKeyAPIKey), apiKey)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

				handler.Responses(c)

				if tc.retired {
					require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
					require.Equal(t, service.ClientCodeModelNotSupported, gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
					require.Nil(t, upstream.lastRequest, "retired model must be rejected before any image upstream call")
					return
				}
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, textModel, gjson.GetBytes(recorder.Body.Bytes(), "model").String())
				require.Equal(t, "custom_tool_call", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
				require.Equal(t, "exec", gjson.GetBytes(recorder.Body.Bytes(), "output.0.name").String())
				renderInput := gjson.GetBytes(recorder.Body.Bytes(), "output.0.input").String()
				require.Contains(t, renderInput, "generatedImage({image_url:")
				require.Contains(t, renderInput, codexNativeImageBridgeTestPNG)
				require.NotContains(t, renderInput, "月球橘猫")
				require.NotContains(t, renderInput, "http")
				require.NotNil(t, upstream.lastRequest)
				require.Equal(t, "/v1/responses", upstream.lastRequest.URL.Path)
				upstreamBody, err := io.ReadAll(upstream.lastRequest.Body)
				require.NoError(t, err)
				require.Equal(t, service.CodexNativeImageBridgeModel(), gjson.GetBytes(upstreamBody, "model").String(),
					"the fixed adapter must restore the pre-global-pool Codex rendering path")
				require.False(t, gjson.GetBytes(upstreamBody, "stream").Bool(),
					"the fixed adapter buffers and validates the complete image before rendering")
				require.Equal(t, "image_generation", gjson.GetBytes(upstreamBody, "tool_choice.type").String())
				require.Equal(t, account.ID, c.GetInt64(opsAccountIDKey))
			})
		}
	}
}

func TestOpenAIResponses_OfficialCodexStreamingImageUsesFixedAdapterCompletionLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI, AllowImageGeneration: true}
	account := service.Account{
		ID: 33, Name: "morecode-fixed-image-primary", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://upstream.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 33, GroupID: group.ID, Priority: 90}},
	}
	upstream := &codexNativeImageBridgeUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{account}, upstream)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
		codexDesktopLiteImageBody("gpt-5.6-sol", "给我生成一张雪山的风景图。", true),
	))
	setCodexDesktopLiteHeaders(c)
	apiKey := &service.APIKey{ID: 97, GroupID: &group.ID, Group: group, User: &service.User{ID: 1, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 4})

	handler.Responses(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, upstream.lastRequest)
	upstreamBody, err := io.ReadAll(upstream.lastRequest.Body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(upstreamBody, "stream").Bool())
	eventTypes := make([]string, 0, 8)
	var completedPayload []byte
	for _, line := range strings.Split(recorder.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := []byte(strings.TrimPrefix(line, "data: "))
		eventType := gjson.GetBytes(payload, "type").String()
		eventTypes = append(eventTypes, eventType)
		if eventType == "response.completed" {
			completedPayload = payload
		}
	}
	require.Contains(t, eventTypes, "response.output_item.done")
	require.NotContains(t, eventTypes, "response.output_text.done")
	require.Equal(t, "custom_tool_call", gjson.GetBytes(completedPayload, "response.output.0.type").String())
	require.Equal(t, "completed", gjson.GetBytes(completedPayload, "response.output.0.status").String())
	require.Equal(t, "exec", gjson.GetBytes(completedPayload, "response.output.0.name").String())
	renderInput := gjson.GetBytes(completedPayload, "response.output.0.input").String()
	require.Contains(t, renderInput, "generatedImage({image_url:")
	require.Contains(t, renderInput, codexNativeImageBridgeTestPNG)
	require.NotContains(t, string(completedPayload), "/v1/codex-image/preview")
}

func TestOpenAIResponses_FixedImagePoolFailsOverSequentiallyMoreCodeAdobePomo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI, AllowImageGeneration: true}
	newAccount := func(id int64, name string, imagePriority int, transport string) service.Account {
		modelMapping := map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"}
		models := []any{"gpt-5.6-sol"}
		extra := map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: imagePriority,
			service.OpenAIImageGenerationModelsExtraKey:   models,
		}
		if transport == service.OpenAIImageGenerationTransportImages {
			modelMapping = map[string]any{"gpt-image-2": "gpt-image-2-count"}
			models = []any{"gpt-image-2"}
			extra[service.OpenAIImageGenerationModelsExtraKey] = models
			extra["supports_images"] = true
			extra[service.OpenAIImageGenerationTransportExtraKey] = transport
		}
		return service.Account{
			ID: id, Name: name, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, Concurrency: 4,
			Credentials: map[string]any{
				"api_key": "test-only-key", "base_url": "https://upstream.example.test/v1",
				"model_mapping": modelMapping,
			},
			Extra:         extra,
			AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: 999, Priority: 90}},
		}
	}
	accounts := []service.Account{
		newAccount(33, "MoreCode primary", 1, service.OpenAIImageGenerationTransportResponses),
		newAccount(38, "Adobe fallback", 2, service.OpenAIImageGenerationTransportImages),
		newAccount(40, "Pomo fallback", 3, service.OpenAIImageGenerationTransportImages),
	}
	upstream := &codexFixedImageThreeLegUpstream{}
	handler := newCodexResponsesTestHandler(t, accounts, upstream)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
		codexDesktopLiteImageBody("gpt-5.6-terra", "帮我生成一张月球橘猫的图片", false),
	))
	setCodexDesktopLiteHeaders(c)
	apiKey := &service.APIKey{ID: 150, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

	handler.Responses(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "gpt-5.6-terra", gjson.GetBytes(recorder.Body.Bytes(), "model").String())
	require.Equal(t, "custom_tool_call", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
	require.Equal(t, "exec", gjson.GetBytes(recorder.Body.Bytes(), "output.0.name").String())
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "output.0.input").String(), codexNativeImageBridgeTestPNG)
	require.NotContains(t, recorder.Body.String(), "/v1/codex-image/preview")
	require.Equal(t, []int64{33, 38, 40}, upstream.accountIDs)
	require.Equal(t, []string{"/v1/responses", "/v1/images/generations", "/v1/images/generations"}, upstream.paths)
	require.Equal(t, []string{service.CodexNativeImageBridgeModel(), "gpt-image-2-count", "gpt-image-2-count"}, upstream.models)
	require.Equal(t, int64(40), c.GetInt64(opsAccountIDKey))
	require.Equal(t, "gpt-image-2-count", c.GetString(opsUpstreamModelKey))
}

func TestOpenAIResponses_CodexDesktopWithoutGeneratedImageCapabilityFallsBackToLocalAgentPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI, AllowImageGeneration: true}
	textAccount := service.Account{
		ID: 23, Name: "ordinary-text-primary", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://text.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 23, GroupID: group.ID, Priority: 1}},
	}
	imageAccount := service.Account{
		ID: 33, Name: "global-image-only", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://image.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 33, GroupID: 999, Priority: 1}},
	}
	upstream := &codexTextIsolationUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{textAccount, imageAccount}, upstream)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
		`{"model":"gpt-5.6-sol","input":"给我生成一张雪山风景图","stream":false}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Codex Desktop/0.148.0-alpha.9 (Mac OS 26.5.2; arm64) unknown (Codex Desktop; 26.810.50856)")
	c.Request.Header.Set(openAIInternalCodexResponsesLiteHeader, "true")
	apiKey := &service.APIKey{ID: 97, GroupID: &group.ID, Group: group, User: &service.User{ID: 1, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 4})

	handler.Responses(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{23}, upstream.accountIDs, "unsupported delivery must stay on the local-agent text route")
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
	require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
	require.NotContains(t, recorder.Body.String(), "image_generation_call")
	require.NotContains(t, recorder.Body.String(), "/v1/codex-image/preview")
}

func TestOpenAIResponses_GeneratedImageToolContinuationDoesNotGenerateAgain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI, AllowImageGeneration: true}
	textAccount := service.Account{
		ID: 23, Name: "ordinary-text-primary", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://text.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 23, GroupID: group.ID, Priority: 1}},
	}
	imageAccount := service.Account{
		ID: 33, Name: "global-image-only", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://image.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 33, GroupID: 999, Priority: 1}},
	}
	upstream := &codexTextIsolationUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{textAccount, imageAccount}, upstream)
	body := `{
		"model":"gpt-5.6-sol","stream":false,
		"input":[
			{"type":"additional_tools","tools":[{"type":"namespace","name":"functions"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"生成一张雪山风景图"}]},
			{"type":"custom_tool_call","id":"ctc_image","call_id":"call_image","name":"exec","status":"completed","input":"generatedImage({...});"},
			{"type":"custom_tool_call_output","call_id":"call_image","output":"image rendered"}
		]
	}`

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	setCodexDesktopLiteHeaders(c)
	apiKey := &service.APIKey{ID: 97, GroupID: &group.ID, Group: group, User: &service.User{ID: 1, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 4})

	handler.Responses(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{23}, upstream.accountIDs, "tool continuation must return to text instead of paying for a second image")
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
	require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
}

func TestOpenAIResponses_SignedGeneratedImageContinuationCompletesLocallyAndIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI, AllowImageGeneration: true}
	textAccount := service.Account{
		ID: 23, Name: "ordinary-text-primary", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://text.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 23, GroupID: group.ID, Priority: 1}},
	}
	imageAccount := service.Account{
		ID: 33, Name: "global-image-only", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://image.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 33, GroupID: 999, Priority: 1}},
	}
	upstream := &codexNativeImageBridgeUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{textAccount, imageAccount}, upstream)
	apiKey := &service.APIKey{ID: 97, GroupID: &group.ID, Group: group, User: &service.User{ID: 1, Status: service.StatusActive}}

	firstRecorder := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(firstRecorder)
	firstContext.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
		codexDesktopLiteImageBody("gpt-5.6-sol", "生成一张雪山风景图", false),
	))
	setCodexDesktopLiteHeaders(firstContext)
	firstContext.Set(string(middleware.ContextKeyAPIKey), apiKey)
	firstContext.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 4})
	handler.Responses(firstContext)

	require.Equal(t, http.StatusOK, firstRecorder.Code, firstRecorder.Body.String())
	call := gjson.GetBytes(firstRecorder.Body.Bytes(), "output.0")
	require.Equal(t, "custom_tool_call", call.Get("type").String())
	require.Equal(t, "exec", call.Get("name").String())
	require.Regexp(t, `^call_img_[0-9a-f]{16}_[0-9a-f]{32}$`, call.Get("call_id").String())
	firstUpstreamRequest := upstream.lastRequest
	require.NotNil(t, firstUpstreamRequest)

	dataURL := "data:image/png;base64," + codexNativeImageBridgeTestPNG
	continuation, err := json.Marshal(map[string]any{
		"model":  "gpt-5.6-sol",
		"stream": true,
		"input": []any{
			map[string]any{"type": "additional_tools", "tools": []any{map[string]any{"type": "namespace", "name": "functions"}}},
			map[string]any{"type": "message", "role": "user", "content": "生成一张雪山风景图"},
			map[string]any{"type": "custom_tool_call", "id": call.Get("id").String(), "call_id": call.Get("call_id").String(), "name": "exec", "status": "completed", "input": call.Get("input").String()},
			map[string]any{"type": "custom_tool_call_output", "call_id": call.Get("call_id").String(), "output": []any{
				map[string]any{"type": "input_text", "text": "Script completed"},
				map[string]any{"type": "input_image", "image_url": dataURL, "detail": "high"},
				map[string]any{"type": "input_text", "text": "已生成第 1 张图片。"},
			}},
		},
	})
	require.NoError(t, err)

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(continuation)))
		setCodexDesktopLiteHeaders(c)
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 4})

		handler.Responses(c)

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), `"type":"response.completed"`)
		require.Contains(t, recorder.Body.String(), "图片已生成并显示。")
		require.NotContains(t, recorder.Body.String(), "custom_tool_call")
		require.Same(t, firstUpstreamRequest, upstream.lastRequest, "local acknowledgement must not invoke any upstream")
	}
}

func TestOpenAIResponses_PassiveImageToolCatalogKeepsOrdinaryTextRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "CodeX Pro20X", Platform: service.PlatformOpenAI}
	textAccount := service.Account{
		ID: 23, Name: "ordinary-text-primary", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://text.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 23, GroupID: group.ID, Priority: 1}},
	}
	imageAccount := service.Account{
		ID: 33, Name: "MoreCode image route", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://image.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 33, GroupID: group.ID, Priority: 90}},
	}
	upstream := &codexTextIsolationUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{textAccount, imageAccount}, upstream)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
		`{"model":"gpt-5.6-sol","input":"explain this code","tools":[{"type":"image_generation"}],"tool_choice":"auto","stream":false}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Codex Desktop/0.147.0-alpha.1.2 (Mac OS 26.3.2; arm64) (Codex Desktop; 26.730.61639)")
	apiKey := &service.APIKey{ID: 151, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

	handler.Responses(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{23}, upstream.accountIDs, "passive image tool metadata must not move normal text to the image pool")
	require.Equal(t, "explain this code", gjson.GetBytes(upstream.lastBody, "input").String())
	require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
}

func TestOpenAIResponses_NonCodexClientKeepsOriginalResponsesCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 6, Name: "OpenAI public", Platform: service.PlatformOpenAI, AllowImageGeneration: true}
	textAccount := service.Account{
		ID: 23, Name: "ordinary-text-primary", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://text.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 23, GroupID: group.ID, Priority: 1}},
	}
	imageAccount := service.Account{
		ID: 33, Name: "global-image-only", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 4,
		Credentials: map[string]any{
			"api_key": "test-only-key", "base_url": "https://image.example.test/v1",
			"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"},
		},
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
		AccountGroups: []service.AccountGroup{{AccountID: 33, GroupID: 999, Priority: 1}},
	}
	upstream := &codexTextIsolationUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{textAccount, imageAccount}, upstream)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(
		`{"model":"gpt-5.6-sol","input":"generate an image of a mountain","stream":false}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "curl/8.0")
	apiKey := &service.APIKey{ID: 152, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

	handler.Responses(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{23}, upstream.accountIDs)
	require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
	require.NotContains(t, recorder.Body.String(), "/v1/codex-image/preview?token=")
}

func newCodexResponsesTestHandler(t *testing.T, accounts []service.Account, upstream service.HTTPUpstream) *OpenAIGatewayHandler {
	t.Helper()
	concurrencyCache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	gatewayCfg := newCodexImagePreviewTestConfig(t)
	billingCfg := &config.Config{RunMode: config.RunModeSimple}
	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, billingCfg)
	concurrencyService := service.NewConcurrencyService(concurrencyCache)
	gatewayService := service.NewOpenAIGatewayService(
		codexNativeImageBridgeAccountRepo{accounts: accounts},
		nil, nil, nil, nil, nil, nil, gatewayCfg, nil, concurrencyService, nil, nil,
		billingCacheService, upstream,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	return NewOpenAIGatewayHandler(
		gatewayService, concurrencyService, billingCacheService, &service.APIKeyService{},
		nil, nil, gatewayCfg,
	)
}

func codexDesktopLiteImageBody(model string, prompt string, stream bool) string {
	streamValue := "false"
	if stream {
		streamValue = "true"
	}
	return `{"model":"` + model + `","input":[` +
		`{"type":"additional_tools","tools":[{"type":"namespace","name":"functions"}]},` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"` + prompt + `"}]}` +
		`],"stream":` + streamValue + `}`
}

func setCodexDesktopLiteHeaders(c *gin.Context) {
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Codex Desktop/0.148.0-alpha.9 (Mac OS 26.5.2; arm64) unknown (Codex Desktop; 26.810.50856)")
	c.Request.Header.Set(openAIInternalCodexResponsesLiteHeader, "true")
}

func TestIsCodexDesktopGeneratedImageDeliveryRequest(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		lite string
		body string
		want bool
	}{
		{
			name: "desktop responses lite additional tools",
			ua:   "Codex Desktop/0.148.0-alpha.9 (Mac OS; arm64)",
			lite: "true",
			body: codexDesktopLiteImageBody("gpt-5.6-sol", "draw", false),
			want: true,
		},
		{
			name: "desktop explicit exec",
			ua:   "Codex Desktop/0.148.0-alpha.9 (Mac OS; arm64)",
			body: `{"tools":[{"type":"custom","name":"exec"}],"input":"draw"}`,
			want: true,
		},
		{
			name: "desktop lite without envelope",
			ua:   "Codex Desktop/0.148.0-alpha.9 (Mac OS; arm64)",
			lite: "true",
			body: `{"input":"draw"}`,
		},
		{
			name: "cli is not desktop",
			ua:   "codex_cli_rs/0.148.0",
			lite: "true",
			body: codexDesktopLiteImageBody("gpt-5.6-sol", "draw", false),
		},
		{
			name: "third party cannot opt in with headers",
			ua:   "curl/8.0",
			lite: "true",
			body: codexDesktopLiteImageBody("gpt-5.6-sol", "draw", false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tt.body))
			c.Request.Header.Set("User-Agent", tt.ua)
			if tt.lite != "" {
				c.Request.Header.Set(openAIInternalCodexResponsesLiteHeader, tt.lite)
			}
			require.Equal(t, tt.want, isCodexDesktopGeneratedImageDeliveryRequest(c, []byte(tt.body)))
		})
	}
}

func newCodexImagePreviewTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		RunMode: config.RunModeStandard,
		JWT:     config.JWTConfig{Secret: codexGeneratedImageAckHandlerTestSecret},
		Gateway: config.GatewayConfig{CodexImagePreview: config.CodexImagePreviewConfig{
			Enabled: true, DataDir: t.TempDir(), TTLSeconds: 3600, MaxImageBytes: 1024 * 1024,
		}},
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
