//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type universalGroupLoaderStub struct{ group *service.Group }

func (s universalGroupLoaderStub) GetByID(_ context.Context, id int64) (*service.Group, error) {
	if s.group != nil && s.group.ID == id {
		clone := *s.group
		return &clone, nil
	}
	return nil, service.ErrGroupNotFound
}

func TestUniversalGroupRoutingRebindsRequestToConcreteBillingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := &service.Group{ID: 6, Name: "GPT", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, SubscriptionType: service.SubscriptionTypeStandard}
	access := &service.Group{ID: 90, Name: "Universal", Platform: service.PlatformUniversal, Status: service.StatusActive, Hydrated: true, UniversalRoutes: []service.UniversalRouteConfig{{
		PublicModel: "gpt-5.6-sol", MatchType: service.UniversalRouteMatchExact, InboundProtocol: service.APIProtocolResponses, TargetGroupID: target.ID, Enabled: true,
	}}}
	user := &service.User{ID: 7, Status: service.StatusActive, Balance: 10}
	key := &service.APIKey{ID: 8, User: user, Group: access, Status: service.StatusActive}
	key.GroupID = &access.ID

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	router.Use(UniversalGroupRouting(universalGroupLoaderStub{group: target}, nil, &config.Config{RunMode: config.RunModeStandard}))
	router.POST("/v1/responses", func(c *gin.Context) {
		routed, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Equal(t, target.ID, routed.Group.ID)
		accessCtx, exists := c.Get(string(ContextKeyUniversalAccessGroup))
		require.True(t, exists)
		require.Equal(t, access.ID, accessCtx.(*service.Group).ID)
		require.Equal(t, access.ID, c.Request.Context().Value(ctxkey.UniversalAccessGroupID))
		require.Equal(t, service.APIProtocolResponses, c.Request.Context().Value(ctxkey.UniversalInboundProtocol))
		require.Equal(t, "gpt-5.6-sol", c.Request.Context().Value(ctxkey.UniversalPublicModel))
		require.Equal(t, service.OpenAIFastTierPriority, c.Request.Context().Value(ctxkey.OpenAIRequestedServiceTier))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.6-sol","service_tier":"fast"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestUniversalGroupRoutingRejectsProtocolWithoutConfiguredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	access := &service.Group{ID: 90, Platform: service.PlatformUniversal, Status: service.StatusActive, Hydrated: true}
	user := &service.User{ID: 7, Status: service.StatusActive, Balance: 10}
	key := &service.APIKey{ID: 8, User: user, Group: access, Status: service.StatusActive}
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	router.Use(UniversalGroupRouting(universalGroupLoaderStub{}, nil, &config.Config{RunMode: config.RunModeStandard}))
	router.POST("/v1/messages", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-5"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "UNIVERSAL_ROUTE_NOT_FOUND")
}

func TestUniversalGroupRoutingUsesTargetSubscriptionWithoutWalletBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	target := &service.Group{ID: 7, Name: "GPT monthly", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, SubscriptionType: service.SubscriptionTypeSubscription}
	access := &service.Group{ID: 90, Platform: service.PlatformUniversal, Status: service.StatusActive, Hydrated: true, UniversalRoutes: []service.UniversalRouteConfig{{
		PublicModel: "gpt-5.6-sol", MatchType: service.UniversalRouteMatchExact, InboundProtocol: service.APIProtocolResponses, TargetGroupID: target.ID, Enabled: true,
	}}}
	user := &service.User{ID: 7, Status: service.StatusActive, Balance: 0}
	key := &service.APIKey{ID: 8, User: user, Group: access, Status: service.StatusActive}
	sub := &service.UserSubscription{ID: 11, UserID: user.ID, GroupID: target.ID, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}
	subRepo := &stubUserSubscriptionRepo{getActive: func(_ context.Context, _, _ int64) (*service.UserSubscription, error) {
		clone := *sub
		return &clone, nil
	}}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	subscriptions := service.NewSubscriptionService(nil, subRepo, nil, nil, cfg)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(ContextKeyAPIKey), key); c.Next() })
	router.Use(UniversalGroupRouting(universalGroupLoaderStub{group: target}, subscriptions, cfg))
	router.POST("/v1/responses", func(c *gin.Context) {
		routedSub, ok := GetSubscriptionFromContext(c)
		require.True(t, ok)
		require.Equal(t, sub.ID, routedSub.ID)
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.6-sol"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}
