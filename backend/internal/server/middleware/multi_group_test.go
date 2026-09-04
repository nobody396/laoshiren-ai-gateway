package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type multiGroupsFixture struct {
	groups map[int64]*service.Group
	denied map[int64]bool
	checks []int64
}

func (f *multiGroupsFixture) GetByID(_ context.Context, id int64) (*service.Group, error) {
	return f.groups[id], nil
}
func (f *multiGroupsFixture) AuthorizeMultiGroupTarget(_ context.Context, k *service.APIKey, g *service.Group) (*service.User, error) {
	f.checks = append(f.checks, g.ID)
	if f.denied[g.ID] {
		return nil, errors.New("revoked")
	}
	return k.User, nil
}
func multiRouter(f *multiGroupsFixture, key *service.APIKey, catalog func(context.Context, *service.Group) ([]string, error)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key) })
	r.Use(MultiGroupRouting(f, f, nil, func(ctx context.Context, g *service.Group) (*service.GroupModelDeclaration, error) {
		names, err := catalog(ctx, g)
		return &service.GroupModelDeclaration{Models: names}, err
	}, &config.Config{RunMode: config.RunModeSimple}))
	r.Any("/*path", func(c *gin.Context) {
		k, _ := GetAPIKeyFromContext(c)
		if k.GroupID != nil {
			c.JSON(200, gin.H{"group": *k.GroupID, "fallback": k.Group.FallbackGroupID})
		} else {
			c.Status(204)
		}
	})
	return r
}
func TestMultiGroupRoutingPriorityAndNoAuthorizationFallback(t *testing.T) {
	first, second := int64(6), int64(59)
	f := &multiGroupsFixture{groups: map[int64]*service.Group{first: {ID: first, Platform: service.PlatformOpenAI, FallbackGroupID: &second}, second: {ID: second, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{}}
	key := &service.APIKey{GroupIDs: []int64{first, second}, User: &service.User{ID: 2}}
	catalog := func(context.Context, *service.Group) ([]string, error) { return []string{"gpt-5.4"}, nil }
	r := multiRouter(f, key, catalog)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`)))
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"group":6,"fallback":null}`, w.Body.String())
	require.Nil(t, key.GroupID)
	require.NotNil(t, f.groups[first].FallbackGroupID)
	f.denied[first] = true
	f.checks = nil
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`)))
	require.Equal(t, 403, w.Code)
	require.Equal(t, []int64{first}, f.checks)
}
func TestMultiGroupDiscoveryFiltersRevokedAndDeduplicates(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{6: {ID: 6, Platform: service.PlatformOpenAI}, 5: {ID: 5, Platform: service.PlatformAnthropic}}, denied: map[int64]bool{5: true}}
	key := &service.APIKey{GroupIDs: []int64{6, 5}, User: &service.User{ID: 2}}
	r := multiRouter(f, key, func(_ context.Context, g *service.Group) ([]string, error) {
		if g.ID == 5 {
			return []string{"claude-test"}, nil
		}
		return []string{"gpt-test", "gpt-test"}, nil
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "claude-test")
	require.Equal(t, 1, strings.Count(w.Body.String(), "gpt-test"))
}
func TestMultiGroupProtocolAndRequestBodyPreserved(t *testing.T) {
	for _, tt := range []struct{ path, body, protocol, model string }{{"/v1/messages", `{"model":"claude-test","messages":[]}`, "messages", "claude-test"}, {"/v1/chat/completions", `{"model":"gpt-test"}`, "chat_completions", "gpt-test"}, {"/v1/responses", `{"model":"gpt-test"}`, "responses", "gpt-test"}, {"/v1beta/models/gemini-test:streamGenerateContent", `{"contents":[]}`, "generate_content", "gemini-test"}} {
		req := httptest.NewRequest("POST", tt.path, strings.NewReader(tt.body))
		protocol, model, err := multiGroupRequestIdentity(req)
		require.NoError(t, err)
		require.Equal(t, tt.protocol, protocol)
		require.Equal(t, tt.model, model)
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, tt.body, string(body))
	}
}
func TestMultiGroupUnsupportedPathsFailClosedButLegacyUnchanged(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{}}
	catalog := func(context.Context, *service.Group) ([]string, error) {
		t.Fatal("must not guess group")
		return nil, nil
	}
	for _, path := range []string{"/v1/responses", "/v1/videos/job", "/gpt-image/v1/tasks/job"} {
		w := httptest.NewRecorder()
		multiRouter(f, &service.APIKey{GroupIDs: []int64{6}}, catalog).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 400, w.Code)
	}
	w := httptest.NewRecorder()
	multiRouter(f, &service.APIKey{}, catalog).ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`)))
	require.Equal(t, 204, w.Code)
}

func TestMultiGroupDuplicateModelAndTrailingJSONRejected(t *testing.T) {
	for _, body := range []string{`{"model":"allowed","model":"other"}`, `{"model":"allowed"} {}`, `[]`, `{"model":7}`} {
		_, err := multiGroupBodyModel([]byte(body))
		require.Error(t, err)
	}
	model, err := multiGroupBodyModel([]byte(`{"model":"allowed","extra":{"model":"nested"}}`))
	require.NoError(t, err)
	require.Equal(t, "allowed", model)
}

func TestMultiGroupSubscriptionFailureDoesNotChargeWalletGroup(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription}, 6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{}}
	key := &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2, Balance: 100}}
	subRepo := &multiGroupSubscriptionRepo{}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	subscriptions := service.NewSubscriptionService(nil, subRepo, nil, nil, cfg)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key) })
	r.Use(MultiGroupRouting(f, f, subscriptions, func(context.Context, *service.Group) (*service.GroupModelDeclaration, error) {
		return &service.GroupModelDeclaration{Models: []string{"gpt-test"}}, nil
	}, cfg))
	r.POST("/v1/responses", func(c *gin.Context) { t.Fatal("must not execute wallet route") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"gpt-test"}`)))
	require.Equal(t, 403, w.Code)
	require.Equal(t, []int64{5}, f.checks)
}

type multiGroupSubscriptionRepo struct {
	service.UserSubscriptionRepository
}

func (*multiGroupSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*service.UserSubscription, error) {
	return nil, errors.New("expired")
}

func TestMultiGroupDiscoveryDoesNotAdvertiseBlockedPriorityFallback(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformOpenAI}, 6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{5: true}}
	r := multiRouter(f, &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2}}, func(context.Context, *service.Group) ([]string, error) { return []string{"gpt-test"}, nil })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "gpt-test")
}

func TestMultiGroupModelConfigurationChangesAreDynamic(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{}}
	declared := []string{"model-old"}
	r := multiRouter(f, &service.APIKey{GroupIDs: []int64{6}, User: &service.User{ID: 2}}, func(context.Context, *service.Group) ([]string, error) {
		return append([]string(nil), declared...), nil
	})
	call := func(model string) int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"`+model+`"}`)))
		return w.Code
	}
	require.Equal(t, 200, call("model-old"))
	declared = []string{"model-new"}
	require.Equal(t, 404, call("model-old"))
	require.Equal(t, 200, call("model-new"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Contains(t, w.Body.String(), "model-new")
	require.NotContains(t, w.Body.String(), "model-old")
}

func TestMultiGroupCatalogFailureCannotFallbackToAnotherGroup(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformOpenAI}, 6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{}}
	r := multiRouter(f, &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2}}, func(_ context.Context, g *service.Group) ([]string, error) {
		if g.ID == 5 {
			return nil, errors.New("catalog disabled")
		}
		t.Fatal("must not choose cheaper/wallet group")
		return []string{"model-test"}, nil
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"model-test"}`)))
	require.Equal(t, 503, w.Code)
}

func TestMultiGroupWildcardDeclarationBlocksLowerPriorityDiscovery(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformOpenAI}, 6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{5: true}}
	r := multiRouter(f, &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2}}, func(_ context.Context, g *service.Group) ([]string, error) {
		if g.ID == 5 {
			return []string{"family-*"}, nil
		}
		return []string{"family-new"}, nil
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "family-new")
	require.NotContains(t, w.Body.String(), "family-*")
}

func TestMultiGroupAuthorizedWildcardCanDiscoverLaterConcreteNames(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformOpenAI}, 6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{}}
	r := multiRouter(f, &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2}}, func(_ context.Context, g *service.Group) ([]string, error) {
		if g.ID == 5 {
			return []string{"family-*"}, nil
		}
		return []string{"family-new"}, nil
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "family-new")
	require.NotContains(t, w.Body.String(), "family-*")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"family-new"}`)))
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"group":5,"fallback":null}`, w.Body.String())
}

func TestMultiGroupWalletRouteDoesNotExposeTypedNilSubscription(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{}}
	key := &service.APIKey{GroupIDs: []int64{6}, User: &service.User{ID: 2, Balance: 10}}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key) })
	r.Use(MultiGroupRouting(f, f, nil, func(context.Context, *service.Group) (*service.GroupModelDeclaration, error) {
		return &service.GroupModelDeclaration{Models: []string{"model-test"}}, nil
	}, &config.Config{RunMode: config.RunModeStandard}))
	r.POST("/v1/responses", func(c *gin.Context) {
		subscription, ok := GetSubscriptionFromContext(c)
		require.False(t, ok)
		require.Nil(t, subscription)
		c.Status(204)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"model-test"}`)))
	require.Equal(t, 204, w.Code)
}

type legacyCatalogChannels struct{ service.ChannelRepository }

func (*legacyCatalogChannels) ListAll(context.Context) ([]service.Channel, error) { return nil, nil }

type legacyCatalogInventory struct{}

func (*legacyCatalogInventory) ListGroupModelInventory(_ context.Context, id int64) ([]service.AccountModelInventory, error) {
	mapping := map[string]any{}
	if id == 6 {
		mapping["future-model"] = "future-model"
	}
	return []service.AccountModelInventory{{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, ModelMapping: mapping}}, nil
}
func TestMultiGroupRealCatalogEmptyMappingPreservesBillingPriority(t *testing.T) {
	f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformOpenAI}, 6: {ID: 6, Platform: service.PlatformOpenAI}}, denied: map[int64]bool{5: true}}
	key := &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2, Balance: 100}}
	catalog := service.NewGroupModelCatalog(service.NewChannelService(&legacyCatalogChannels{}, nil), &legacyCatalogInventory{})
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key) })
	r.Use(MultiGroupRouting(f, f, nil, catalog.Declaration, &config.Config{RunMode: config.RunModeSimple}))
	r.Any("/*path", func(c *gin.Context) { t.Fatal("must not run wallet route") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"model":"future-model"}`)))
	require.Equal(t, 403, w.Code)
	require.Equal(t, []int64{5}, f.checks)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/models", nil))
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "future-model")
}

type legacyAnthropicInventory struct{ kind string }

func (i *legacyAnthropicInventory) ListGroupModelInventory(_ context.Context, id int64) ([]service.AccountModelInventory, error) {
	kind := i.kind
	mapping := map[string]any{"claude-haiku-4-5-20251001": "claude-haiku-4-5-20251001"}
	if kind == service.AccountTypeBedrock {
		mapping = map[string]any{"custom": "claude-opus-4-6"}
	}
	if id == 6 {
		kind = service.AccountTypeAPIKey
		mapping = map[string]any{"claude-haiku-4-5": "claude-haiku-4-5"}
	}
	return []service.AccountModelInventory{{Platform: service.PlatformAnthropic, Type: kind, ModelMapping: mapping}}, nil
}
func TestMultiGroupLegacyAnthropicKindsCannotSkipPriorityPayer(t *testing.T) {
	for _, kind := range []string{service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeBedrock} {
		t.Run(kind, func(t *testing.T) {
			f := &multiGroupsFixture{groups: map[int64]*service.Group{5: {ID: 5, Platform: service.PlatformAnthropic}, 6: {ID: 6, Platform: service.PlatformAnthropic}}, denied: map[int64]bool{5: true}}
			key := &service.APIKey{GroupIDs: []int64{5, 6}, User: &service.User{ID: 2, Balance: 100}}
			catalog := service.NewGroupModelCatalog(service.NewChannelService(&legacyCatalogChannels{}, nil), &legacyAnthropicInventory{kind: kind})
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key) })
			r.Use(MultiGroupRouting(f, f, nil, catalog.Declaration, &config.Config{RunMode: config.RunModeSimple}))
			r.Any("/*path", func(c *gin.Context) { t.Fatal("must not fall through to wallet") })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-haiku-4-5"}`)))
			require.Equal(t, 403, w.Code)
			require.Equal(t, []int64{5}, f.checks)
		})
	}
}
