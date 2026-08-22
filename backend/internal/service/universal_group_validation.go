package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

func (s *adminServiceImpl) validateUniversalRoutes(ctx context.Context, selfID int64, platform string, routes []UniversalRouteConfig) error {
	if platform != PlatformUniversal {
		if len(routes) > 0 {
			return infraerrors.BadRequest("UNIVERSAL_ROUTES_PLATFORM_MISMATCH", "universal_routes are only valid for universal groups")
		}
		return nil
	}
	seen := map[string]struct{}{}
	for i := range routes {
		route := &routes[i]
		route.PublicModel = strings.TrimSpace(route.PublicModel)
		route.MatchType = strings.ToLower(strings.TrimSpace(route.MatchType))
		if route.MatchType == "" {
			route.MatchType = UniversalRouteMatchExact
		}
		route.InboundProtocol = strings.ToLower(strings.TrimSpace(route.InboundProtocol))
		if route.PublicModel == "" || (route.MatchType != UniversalRouteMatchExact && route.MatchType != UniversalRouteMatchPrefix) {
			return infraerrors.Newf(http.StatusBadRequest, "INVALID_UNIVERSAL_ROUTE", "route #%d has invalid model or match_type", i+1)
		}
		if route.InboundProtocol == "" || NormalizeAPIProtocol(route.InboundProtocol) == "" || route.InboundProtocol == APIProtocolAdaptive {
			return infraerrors.Newf(http.StatusBadRequest, "INVALID_UNIVERSAL_ROUTE", "route #%d has invalid inbound_protocol", i+1)
		}
		if route.TargetGroupID <= 0 || route.TargetGroupID == selfID {
			return infraerrors.Newf(http.StatusBadRequest, "INVALID_UNIVERSAL_ROUTE", "route #%d has invalid target_group_id", i+1)
		}
		target, err := s.groupRepo.GetByIDLite(ctx, route.TargetGroupID)
		if err != nil {
			return infraerrors.Newf(http.StatusBadRequest, "UNIVERSAL_TARGET_NOT_FOUND", "route #%d target group %d not found", i+1, route.TargetGroupID)
		}
		if target.IsUniversal() {
			return infraerrors.Newf(http.StatusBadRequest, "UNIVERSAL_TARGET_NESTING_FORBIDDEN", "route #%d cannot target another universal group", i+1)
		}
		if !universalTargetSupportsProtocol(target.Platform, route.InboundProtocol) {
			return infraerrors.Newf(http.StatusBadRequest, "UNIVERSAL_TARGET_PROTOCOL_UNSUPPORTED", "route #%d target platform %s cannot serve %s", i+1, target.Platform, route.InboundProtocol)
		}
		key := fmt.Sprintf("%s|%s|%s|%d", strings.ToLower(route.PublicModel), route.MatchType, route.InboundProtocol, route.TargetGroupID)
		if _, exists := seen[key]; exists {
			return infraerrors.Newf(http.StatusBadRequest, "DUPLICATE_UNIVERSAL_ROUTE", "route #%d duplicates an existing route", i+1)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func universalTargetSupportsProtocol(platform, protocol string) bool {
	switch protocol {
	case APIProtocolAnthropic:
		return platform == PlatformAnthropic || platform == PlatformAntigravity || platform == PlatformOpenAI || platform == PlatformGrok
	case APIProtocolResponses:
		return platform == PlatformOpenAI || platform == PlatformGrok
	case APIProtocolChatCompletions:
		return platform == PlatformOpenAI || platform == PlatformGrok || platform == PlatformGemini
	default:
		return false
	}
}
