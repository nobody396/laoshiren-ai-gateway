package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	ContextKeyUniversalAccessGroup ContextKey = "universal_access_group"
	ContextKeyUniversalRoute       ContextKey = "universal_route"
)

type UniversalTargetGroupLoader interface {
	GetByID(ctx context.Context, id int64) (*service.Group, error)
}

func UniversalGroupRouting(groupService UniversalTargetGroupLoader, subscriptionService *service.SubscriptionService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsUniversal() {
			c.Next()
			return
		}
		if c.Request.Method == http.MethodGet {
			if strings.HasSuffix(c.Request.URL.Path, "/models") || strings.HasSuffix(c.Request.URL.Path, "/usage") {
				c.Next()
				return
			}
			writeUniversalRouteError(c, http.StatusBadRequest, "UNIVERSAL_PROTOCOL_UNSUPPORTED", "Universal WebSocket routing is not available yet")
			return
		}
		if groupService == nil {
			writeUniversalRouteError(c, http.StatusServiceUnavailable, "UNIVERSAL_ROUTER_UNAVAILABLE", "Universal routing is unavailable")
			return
		}

		protocol := universalInboundProtocol(c.Request.URL.Path)
		if protocol == "" {
			writeUniversalRouteError(c, http.StatusNotFound, "UNIVERSAL_PROTOCOL_UNSUPPORTED", "This endpoint is not available for universal keys")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			writeUniversalRouteError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
		decision, err := apiKey.Group.ResolveUniversalRoute(model, protocol)
		if err != nil {
			writeUniversalRouteError(c, http.StatusNotFound, "UNIVERSAL_ROUTE_NOT_FOUND", "The requested model is not available for this API protocol")
			return
		}
		target, err := groupService.GetByID(c.Request.Context(), decision.TargetGroupID)
		if err != nil || target == nil || !target.IsActive() || target.IsUniversal() {
			writeUniversalRouteError(c, http.StatusServiceUnavailable, "UNIVERSAL_TARGET_UNAVAILABLE", "The selected model route is unavailable")
			return
		}
		if apiKey.User == nil || !apiKey.User.CanBindGroup(target.ID, target.IsExclusive) {
			writeUniversalRouteError(c, http.StatusForbidden, "GROUP_ACCESS_DENIED", "You do not have access to the selected model group")
			return
		}

		var subscription *service.UserSubscription
		if cfg == nil || cfg.RunMode != config.RunModeSimple {
			if target.IsSubscriptionType() {
				if subscriptionService == nil {
					writeUniversalRouteError(c, http.StatusServiceUnavailable, "SUBSCRIPTION_SERVICE_UNAVAILABLE", "Subscription validation is unavailable")
					return
				}
				subscription, err = subscriptionService.GetActiveSubscription(c.Request.Context(), apiKey.User.ID, target.ID)
				if err != nil {
					writeUniversalRouteError(c, http.StatusForbidden, "SUBSCRIPTION_NOT_FOUND", "No active subscription found for the selected model group")
					return
				}
				needsMaintenance, validateErr := subscriptionService.ValidateAndCheckLimits(subscription, target)
				if validateErr != nil {
					status := infraerrors.Code(validateErr)
					code := infraerrors.Reason(validateErr)
					message := infraerrors.Message(validateErr)
					if status <= 0 {
						status = http.StatusForbidden
					}
					if code == "" {
						code = "SUBSCRIPTION_INVALID"
					}
					writeUniversalRouteError(c, status, code, message)
					return
				}
				if needsMaintenance {
					copy := *subscription
					subscriptionService.DoWindowMaintenance(&copy)
				}
			} else if apiKey.User.Balance <= 0 {
				writeUniversalRouteError(c, http.StatusForbidden, "INSUFFICIENT_BALANCE", "Insufficient account balance")
				return
			}
		}

		accessGroup := apiKey.Group
		cloned := *apiKey
		targetID := target.ID
		cloned.GroupID = &targetID
		cloned.Group = target
		c.Set(string(ContextKeyAPIKey), &cloned)
		if subscription != nil {
			c.Set(string(ContextKeySubscription), subscription)
		} else {
			c.Set(string(ContextKeySubscription), nil)
		}
		c.Set(string(ContextKeyUniversalAccessGroup), accessGroup)
		c.Set(string(ContextKeyUniversalRoute), decision)
		requestCtx := c.Request.Context()
		requestCtx = context.WithValue(requestCtx, ctxkey.UniversalAccessGroupID, accessGroup.ID)
		requestCtx = context.WithValue(requestCtx, ctxkey.UniversalInboundProtocol, protocol)
		requestCtx = context.WithValue(requestCtx, ctxkey.UniversalPublicModel, model)
		if tier := normalizeUniversalRequestedServiceTier(gjson.GetBytes(body, "service_tier").String()); tier != "" {
			requestCtx = context.WithValue(requestCtx, ctxkey.OpenAIRequestedServiceTier, tier)
		}
		c.Request = c.Request.WithContext(requestCtx)
		setGroupContext(c, target)
		c.Next()
	}
}

func normalizeUniversalRequestedServiceTier(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fast", service.OpenAIFastTierPriority:
		return service.OpenAIFastTierPriority
	case service.OpenAIFastTierFlex:
		return service.OpenAIFastTierFlex
	case "default", "auto", "scale":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func universalInboundProtocol(path string) string {
	path = strings.ToLower(path)
	switch {
	case strings.Contains(path, "/messages"):
		return service.APIProtocolAnthropic
	case strings.Contains(path, "/responses"):
		return service.APIProtocolResponses
	case strings.Contains(path, "/chat/completions"):
		return service.APIProtocolChatCompletions
	default:
		return ""
	}
}

func writeUniversalRouteError(c *gin.Context, status int, code, message string) {
	if c == nil {
		return
	}
	if universalInboundProtocol(c.Request.URL.Path) == service.APIProtocolAnthropic {
		c.AbortWithStatusJSON(status, gin.H{"type": "error", "error": gin.H{"type": code, "message": message}})
		return
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": code, "message": message}})
}
