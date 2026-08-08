package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type OpenAIRouteHealthScope string

const (
	OpenAIRouteHealthScopeRoute    OpenAIRouteHealthScope = "route"
	OpenAIRouteHealthScopeProvider OpenAIRouteHealthScope = "provider"
)

type OpenAIRouteHealthStoreKey struct {
	Scope OpenAIRouteHealthScope

	GroupID       int64
	AccountID     int64
	FailureDomain string
	Model         string
	EndpointHash  string
	Transport     string
}

func OpenAIRouteHealthStoreKeyForRoute(key OpenAIRouteKey) OpenAIRouteHealthStoreKey {
	return OpenAIRouteHealthStoreKey{
		Scope:         OpenAIRouteHealthScopeRoute,
		GroupID:       key.GroupID,
		AccountID:     key.AccountID,
		FailureDomain: key.FailureDomain,
		Model:         key.Model,
		EndpointHash:  key.EndpointHash,
		Transport:     key.Transport,
	}
}

func OpenAIRouteHealthStoreKeyForProvider(key OpenAIRouteKey) OpenAIRouteHealthStoreKey {
	return OpenAIRouteHealthStoreKey{
		Scope:         OpenAIRouteHealthScopeProvider,
		GroupID:       key.GroupID,
		FailureDomain: key.FailureDomain,
		Model:         key.Model,
	}
}

func (k OpenAIRouteHealthStoreKey) Valid() bool {
	if k.GroupID <= 0 || strings.TrimSpace(k.Model) == "" {
		return false
	}
	switch k.Scope {
	case OpenAIRouteHealthScopeRoute:
		return k.AccountID > 0
	case OpenAIRouteHealthScopeProvider:
		return strings.TrimSpace(k.FailureDomain) != ""
	default:
		return false
	}
}

func (k OpenAIRouteHealthStoreKey) Fingerprint() string {
	canonical := fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s",
		k.Scope,
		k.GroupID,
		k.AccountID,
		strings.TrimSpace(k.FailureDomain),
		strings.TrimSpace(k.Model),
		strings.TrimSpace(k.EndpointHash),
		strings.TrimSpace(k.Transport),
	)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:16])
}

// OpenAIRouteHealthStore is the parallel-safe shared-state boundary. Redis
// implementations must make ApplyEvent and half-open permit acquisition atomic
// across application instances. ApplyEvent is for failures, probes, and
// recovery-state successes; ordinary healthy successes belong in the separate
// rolling metrics path and must not turn this CAS store into a per-token hot
// write.
type OpenAIRouteHealthStore interface {
	Get(ctx context.Context, key OpenAIRouteHealthStoreKey) (OpenAIRouteHealthState, error)
	ApplyEvent(ctx context.Context, key OpenAIRouteHealthStoreKey, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy) (OpenAIRouteHealthState, error)
	AcquireHalfOpenPermit(ctx context.Context, key OpenAIRouteHealthStoreKey, owner string) (bool, error)
	ReleaseHalfOpenPermit(ctx context.Context, key OpenAIRouteHealthStoreKey, owner string) error
}
