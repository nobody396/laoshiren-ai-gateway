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
	RequestClass  OpenAIRouteRequestClass
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
		RequestClass:  key.RequestClass,
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
		RequestClass:  key.RequestClass,
	}
}

func (k OpenAIRouteHealthStoreKey) Valid() bool {
	if k.GroupID <= 0 || strings.TrimSpace(k.Model) == "" || !k.RequestClass.Valid() {
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
	canonical := fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s|%s",
		k.Scope,
		k.GroupID,
		k.AccountID,
		strings.TrimSpace(k.FailureDomain),
		strings.TrimSpace(k.Model),
		k.RequestClass,
		strings.TrimSpace(k.EndpointHash),
		strings.TrimSpace(k.Transport),
	)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:16])
}

// OpenAIRouteHealthStore is the parallel-safe shared-state boundary. Redis
// implementations must batch hot-path reads and make ApplyEvent and half-open
// permit acquisition atomic across application instances. ApplyEvent is for failures, probes, and
// recovery-state successes; ordinary healthy successes belong in the separate
// rolling metrics path and must not turn this CAS store into a per-token hot
// write.
type OpenAIRouteHealthStore interface {
	Get(ctx context.Context, key OpenAIRouteHealthStoreKey) (OpenAIRouteHealthState, error)
	GetBatch(ctx context.Context, keys []OpenAIRouteHealthStoreKey) (map[string]OpenAIRouteHealthState, error)
	ApplyEvent(ctx context.Context, key OpenAIRouteHealthStoreKey, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy) (OpenAIRouteHealthState, error)
	RecordProviderEvidence(ctx context.Context, routeKey OpenAIRouteKey, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy, minDistinctAccounts int) (OpenAIRouteProviderEvidenceResult, error)
	AcquireHalfOpenPermit(ctx context.Context, key OpenAIRouteHealthStoreKey, owner string) (bool, error)
	ReleaseHalfOpenPermit(ctx context.Context, key OpenAIRouteHealthStoreKey, owner string) error
}

type OpenAIRouteProviderEvidenceResult struct {
	DistinctFailingAccounts int
	ProviderEventApplied    bool
	State                   OpenAIRouteHealthState
}

func OpenAIRouteHasSharedFailureDomain(key OpenAIRouteKey) bool {
	domain := strings.TrimSpace(key.FailureDomain)
	return domain != "" && domain != fmt.Sprintf("account:%d", key.AccountID)
}
