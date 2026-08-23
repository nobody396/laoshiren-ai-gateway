package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func buildFinalReliabilityObservation(c *gin.Context, startedAt time.Time, entry *service.OpsInsertErrorLogInput) *service.ReliabilityFinalOutcome {
	if c == nil || c.Request == nil {
		return nil
	}
	requestID, clientRequestID, correlationID := reliabilityCorrelation(c, entry)
	if correlationID == "" {
		return nil
	}
	status := c.Writer.Status()
	statusCode := status
	model := reliabilityContextString(c, opsModelKey)
	platform := guessPlatformFromPath(c.Request.URL.Path)
	var userID, groupID, accountID *int64
	if apiKey, ok := middleware2.GetAPIKeyFromContext(c); ok && apiKey != nil {
		if apiKey.User != nil && apiKey.User.ID > 0 {
			value := apiKey.User.ID
			userID = &value
		} else if apiKey.UserID > 0 {
			value := apiKey.UserID
			userID = &value
		}
		if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
			value := *apiKey.GroupID
			groupID = &value
		}
		if apiKey.Group != nil && strings.TrimSpace(apiKey.Group.Platform) != "" {
			platform = apiKey.Group.Platform
		}
	}
	if value, ok := c.Get(opsAccountIDKey); ok {
		if parsed, ok := value.(int64); ok && parsed > 0 {
			accountID = reliabilityInt64Pointer(parsed)
		}
	}

	outcome := service.ReliabilityOutcomeSuccess
	errorOwner := ""
	exclusionReason := ""
	if entry == nil && status < 400 {
		if value, exists := c.Get(service.OpsUpstreamErrorsKey); exists {
			if events, ok := value.([]*service.OpsUpstreamErrorEvent); ok && len(events) > 0 {
				outcome = service.ReliabilityOutcomeRecovered
			}
		}
	}
	if isCountTokensRequest(c) {
		outcome = service.ReliabilityOutcomeExcluded
		exclusionReason = "count_tokens"
	} else if entry != nil {
		if entry.UserID != nil {
			userID = reliabilityInt64Pointer(*entry.UserID)
		}
		if entry.GroupID != nil {
			groupID = reliabilityInt64Pointer(*entry.GroupID)
		}
		if entry.AccountID != nil {
			accountID = reliabilityInt64Pointer(*entry.AccountID)
		}
		if entry.Platform != "" {
			platform = entry.Platform
		}
		if entry.Model != "" {
			model = entry.Model
		}
		statusCode = entry.StatusCode
		errorOwner = strings.ToLower(strings.TrimSpace(entry.ErrorOwner))
		switch {
		case entry.IsCountTokens:
			outcome = service.ReliabilityOutcomeExcluded
			exclusionReason = "count_tokens"
		case entry.IsBusinessLimited:
			outcome = service.ReliabilityOutcomeExcluded
			exclusionReason = "business_limited"
		case errors.Is(c.Request.Context().Err(), context.Canceled) || strings.Contains(strings.ToLower(entry.ErrorMessage), opsErrContextCanceled):
			outcome = service.ReliabilityOutcomeExcluded
			exclusionReason = "client_cancelled"
		case errorOwner == "provider" || errorOwner == "platform":
			outcome = service.ReliabilityOutcomeFailure
		default:
			outcome = service.ReliabilityOutcomeExcluded
			exclusionReason = "client_or_unowned"
		}
	} else if status >= 400 {
		return nil
	}

	return &service.ReliabilityFinalOutcome{
		RequestIdentity: correlationID,
		RequestID:       requestID,
		ClientRequestID: clientRequestID,
		UserID:          userID,
		GroupID:         groupID,
		AccountID:       accountID,
		Platform:        platform,
		Model:           model,
		RequestClass:    reliabilityRequestClass(c.Request.URL.Path),
		Protocol:        reliabilityProtocol(c),
		Outcome:         outcome,
		StatusCode:      &statusCode,
		ErrorOwner:      errorOwner,
		ExclusionReason: exclusionReason,
		LatencyMs:       max(time.Since(startedAt).Milliseconds(), 0),
		ObservedAt:      time.Now(),
	}
}

func buildAttemptReliabilityObservations(entry *service.OpsInsertErrorLogInput, protocol, requestClass string) []*service.ReliabilityAttemptOutcome {
	if entry == nil || len(entry.UpstreamErrors) == 0 {
		return nil
	}
	correlationID := strings.TrimSpace(entry.ClientRequestID)
	if correlationID == "" {
		correlationID = strings.TrimSpace(entry.RequestID)
	}
	if correlationID == "" {
		return nil
	}
	observations := make([]*service.ReliabilityAttemptOutcome, 0, len(entry.UpstreamErrors))
	for index, event := range entry.UpstreamErrors {
		if event == nil {
			continue
		}
		statusCode := event.UpstreamStatusCode
		observedAt := entry.CreatedAt
		if event.AtUnixMs > 0 {
			observedAt = time.UnixMilli(event.AtUnixMs)
		}
		if observedAt.IsZero() {
			observedAt = time.Now()
		}
		platform := strings.TrimSpace(event.Platform)
		if platform == "" {
			platform = entry.Platform
		}
		accountID := event.AccountID
		observations = append(observations, &service.ReliabilityAttemptOutcome{
			RequestIdentity:    correlationID,
			AttemptIdentity:    strconv.Itoa(index + 1),
			RequestID:          entry.RequestID,
			ClientRequestID:    entry.ClientRequestID,
			UserID:             reliabilityCopyInt64(entry.UserID),
			GroupID:            reliabilityCopyInt64(entry.GroupID),
			AccessGroupID:      event.AccessGroupID,
			AccountID:          reliabilityOptionalPositiveInt64(accountID),
			Platform:           platform,
			Model:              entry.Model,
			RequestClass:       requestClass,
			Protocol:           protocol,
			Transport:          event.UpstreamTransport,
			EndpointHash:       event.EndpointHash,
			RoutingFingerprint: event.RoutingFingerprint,
			Outcome:            service.ReliabilityOutcomeFailure,
			StatusCode:         reliabilityOptionalStatus(statusCode),
			ErrorOwner:         "provider",
			ObservedAt:         observedAt,
		})
	}
	return observations
}

func buildSuccessfulAttemptReliabilityObservation(c *gin.Context) *service.ReliabilityAttemptOutcome {
	if c == nil || c.Request == nil {
		return nil
	}
	accountValue, _ := c.Get(opsAccountIDKey)
	accountID, ok := accountValue.(int64)
	if !ok || accountID <= 0 {
		return nil
	}
	requestID, clientRequestID, correlationID := reliabilityCorrelation(c, nil)
	if correlationID == "" {
		return nil
	}
	var userID, groupID *int64
	platform := guessPlatformFromPath(c.Request.URL.Path)
	if apiKey, exists := middleware2.GetAPIKeyFromContext(c); exists && apiKey != nil {
		if apiKey.User != nil && apiKey.User.ID > 0 {
			userID = reliabilityInt64Pointer(apiKey.User.ID)
		} else if apiKey.UserID > 0 {
			userID = reliabilityInt64Pointer(apiKey.UserID)
		}
		groupID = reliabilityCopyInt64(apiKey.GroupID)
		if apiKey.Group != nil && strings.TrimSpace(apiKey.Group.Platform) != "" {
			platform = apiKey.Group.Platform
		}
	}
	model := reliabilityContextString(c, opsModelKey)
	protocol := reliabilityProtocol(c)
	latencyMs := int64(0)
	if value, exists := c.Get(service.OpsUpstreamLatencyMsKey); exists {
		switch parsed := value.(type) {
		case int64:
			latencyMs = max(parsed, 0)
		case int:
			latencyMs = int64(max(parsed, 0))
		}
	}
	statusCode := c.Writer.Status()
	routeIdentity, _ := service.GetOpsReliabilityRouteIdentity(c)
	return &service.ReliabilityAttemptOutcome{
		RequestIdentity:    correlationID,
		AttemptIdentity:    "success",
		RequestID:          requestID,
		ClientRequestID:    clientRequestID,
		UserID:             userID,
		GroupID:            groupID,
		AccessGroupID:      routeIdentity.AccessGroupID,
		AccountID:          reliabilityInt64Pointer(accountID),
		Platform:           platform,
		Model:              model,
		RequestClass:       reliabilityRequestClass(c.Request.URL.Path),
		Protocol:           protocol,
		Transport:          routeIdentity.UpstreamTransport,
		EndpointHash:       routeIdentity.EndpointHash,
		RoutingFingerprint: routeIdentity.RoutingFingerprint,
		Outcome:            service.ReliabilityOutcomeSuccess,
		StatusCode:         &statusCode,
		LatencyMs:          latencyMs,
		ObservedAt:         time.Now(),
	}
}

func reliabilityCorrelation(c *gin.Context, entry *service.OpsInsertErrorLogInput) (string, string, string) {
	requestID := ""
	clientRequestID := ""
	if entry != nil {
		requestID = strings.TrimSpace(entry.RequestID)
		clientRequestID = strings.TrimSpace(entry.ClientRequestID)
	}
	if requestID == "" && c != nil {
		requestID = strings.TrimSpace(c.Writer.Header().Get("X-Request-Id"))
		if requestID == "" {
			requestID = strings.TrimSpace(c.Writer.Header().Get("x-request-id"))
		}
	}
	requestID = reliabilityBoundedExternalID(requestID, 128)
	if clientRequestID == "" && c != nil && c.Request != nil {
		clientRequestID, _ = c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		clientRequestID = strings.TrimSpace(clientRequestID)
	}
	clientRequestID = reliabilityBoundedExternalID(clientRequestID, 128)
	correlationID := clientRequestID
	if correlationID == "" {
		correlationID = requestID
	}
	return requestID, clientRequestID, correlationID
}

func reliabilityBoundedExternalID(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxBytes {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:16])
}

func reliabilityProtocol(c *gin.Context) string {
	endpoint := strings.ToLower(strings.TrimSpace(GetInboundEndpoint(c)))
	transportPrefix := ""
	if service.GetOpenAIClientTransport(c) == service.OpenAIClientTransportWS {
		transportPrefix = "websocket_"
	}
	switch {
	case strings.Contains(endpoint, "/responses"):
		return transportPrefix + "responses"
	case strings.Contains(endpoint, "/messages"):
		return transportPrefix + "messages"
	case strings.Contains(endpoint, "/chat/completions"):
		return transportPrefix + "chat_completions"
	case strings.Contains(endpoint, "/images"):
		return "images"
	case strings.Contains(endpoint, "/videos"):
		return "videos"
	default:
		if transportPrefix != "" {
			return "websocket"
		}
		return "http"
	}
}

func reliabilityRequestClass(path string) string {
	path = strings.ToLower(path)
	switch {
	case strings.Contains(path, "/images"):
		return service.ReliabilityRequestClassImage
	case strings.Contains(path, "/videos"):
		return service.ReliabilityRequestClassVideo
	default:
		return service.ReliabilityRequestClassText
	}
}

func reliabilityContextString(c *gin.Context, key string) string {
	if c == nil {
		return ""
	}
	value, _ := c.Get(key)
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func reliabilityOptionalPositiveInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return reliabilityInt64Pointer(value)
}

func reliabilityOptionalStatus(value int) *int {
	if value < 100 || value > 599 {
		return nil
	}
	out := value
	return &out
}

func reliabilityCopyInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	return reliabilityInt64Pointer(*value)
}

func reliabilityInt64Pointer(value int64) *int64 { return &value }
