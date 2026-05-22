package service

import (
	"context"
	"time"
)

const (
	CachePolicyModeSafe5m         = "safe_5m"
	CachePolicyModeShadowAdaptive = "shadow_adaptive"
	CachePolicyModeAdaptive       = "adaptive"
	CachePolicyModeForce5m        = "force_5m"
	CachePolicyModeForce1h        = "force_1h"
)

type CachePolicyDecision struct {
	RequestID              string
	ClientRequestID        string
	UserID                 *int64
	APIKeyID               *int64
	GroupID                *int64
	AccountID              *int64
	ClientType             string
	Model                  string
	PolicyMode             string
	PolicyVersion          string
	ActualTTL              string
	ShadowTTL              string
	DecisionReason         string
	CacheControlPathsCount int
	Normalized             bool
	Downgraded             bool
	Retried                bool
	DurationMs             *int
	FirstTokenMs           *int
	CacheCreation5mTokens  int
	CacheCreation1hTokens  int
	CacheReadTokens        int
	Cost                   float64
	CreatedAt              time.Time
}

type CachePolicyRepository interface {
	CreateDecision(ctx context.Context, decision *CachePolicyDecision) error
	UpsertDailyReport(ctx context.Context, report *CachePolicyDailyReport) error
}

type CachePolicyDailyReport struct {
	ReportDate           time.Time
	Phase                string
	PolicyVersion        string
	ClientProfilesJSON   string
	GroupPerformanceJSON string
	RecommendedJSON      string
	AutoAction           string
	AutoActionReason     string
	RiskMetricsJSON      string
	TTLOrder400Count     int
	AdaptiveEnabledRatio float64
	CostDeltaPct         float64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
