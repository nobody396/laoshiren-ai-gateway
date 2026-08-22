package service

import (
	"errors"
	"sort"
	"strings"
)

const (
	UniversalRouteMatchExact  = "exact"
	UniversalRouteMatchPrefix = "prefix"
	UniversalProtocolAny      = "any"
)

var ErrUniversalRouteNotFound = errors.New("universal route not found")

type UniversalRouteDecision struct {
	AccessGroupID  int64
	PublicModel    string
	Protocol       string
	TargetGroupID  int64
	Priority       int
	MatchedPattern string
}

func (g *Group) IsUniversal() bool {
	return g != nil && g.Platform == PlatformUniversal
}

func (g *Group) ResolveUniversalRoute(model, protocol string) (UniversalRouteDecision, error) {
	if !g.IsUniversal() {
		return UniversalRouteDecision{}, ErrUniversalRouteNotFound
	}
	model = strings.TrimSpace(model)
	protocol = NormalizeAPIProtocol(protocol)
	if model == "" || protocol == "" || protocol == APIProtocolAdaptive {
		return UniversalRouteDecision{}, ErrUniversalRouteNotFound
	}
	type candidate struct {
		route         UniversalRouteConfig
		matchStrength int
		protocolExact bool
	}
	var candidates []candidate
	for _, route := range g.UniversalRoutes {
		if !route.Enabled || route.TargetGroupID <= 0 {
			continue
		}
		routeProtocol := strings.ToLower(strings.TrimSpace(route.InboundProtocol))
		if routeProtocol == "" {
			routeProtocol = UniversalProtocolAny
		}
		if routeProtocol != UniversalProtocolAny && routeProtocol != protocol {
			continue
		}
		pattern := strings.TrimSpace(route.PublicModel)
		strength := 0
		switch strings.ToLower(strings.TrimSpace(route.MatchType)) {
		case UniversalRouteMatchPrefix:
			if strings.HasPrefix(strings.ToLower(model), strings.ToLower(pattern)) {
				strength = len(pattern)
			}
		default:
			if strings.EqualFold(model, pattern) {
				strength = 10000 + len(pattern)
			}
		}
		if strength > 0 {
			candidates = append(candidates, candidate{route: route, matchStrength: strength, protocolExact: routeProtocol == protocol})
		}
	}
	if len(candidates) == 0 {
		return UniversalRouteDecision{}, ErrUniversalRouteNotFound
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].matchStrength != candidates[j].matchStrength {
			return candidates[i].matchStrength > candidates[j].matchStrength
		}
		if candidates[i].protocolExact != candidates[j].protocolExact {
			return candidates[i].protocolExact
		}
		return candidates[i].route.Priority < candidates[j].route.Priority
	})
	selected := candidates[0].route
	return UniversalRouteDecision{
		AccessGroupID: g.ID, PublicModel: model, Protocol: protocol,
		TargetGroupID: selected.TargetGroupID, Priority: selected.Priority,
		MatchedPattern: selected.PublicModel,
	}, nil
}

func (g *Group) UniversalPublicModels() []string {
	seen := map[string]string{}
	for _, route := range g.UniversalRoutes {
		if !route.Enabled || route.TargetGroupID <= 0 || strings.EqualFold(strings.TrimSpace(route.MatchType), UniversalRouteMatchPrefix) {
			continue
		}
		model := strings.TrimSpace(route.PublicModel)
		if model != "" {
			seen[strings.ToLower(model)] = model
		}
	}
	models := make([]string, 0, len(seen))
	for _, model := range seen {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}
