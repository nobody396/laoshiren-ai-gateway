package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	openAIRouteAuditWriteTimeout   = 200 * time.Millisecond
	defaultOpenAIRouteAuditWorkers = 8
	defaultOpenAIRouteAuditQueue   = 4096
)

var ErrOpenAIRouteAuditUnavailable = errors.New("OpenAI route decision audit is unavailable")

var errOpenAIRouteAuditQueueFull = errors.New("OpenAI route decision audit queue full")

type OpenAIRouteShadowAuditRouteVariant struct {
	AccountID    int64  `json:"account_id"`
	EndpointHash string `json:"endpoint_hash"`
}

// OpenAIRouteShadowAuditPolicy is the exact normalized policy used by one
// evaluation. Durations are stored as seconds so the JSON remains readable.
type OpenAIRouteShadowAuditPolicy struct {
	TargetAverageMultiplier float64                              `json:"target_average_multiplier"`
	HardAverageMultiplier   float64                              `json:"hard_average_multiplier"`
	EmergencyDebtLimitUSD   float64                              `json:"emergency_debt_limit_usd"`
	MaxCreditUSD            float64                              `json:"max_credit_usd"`
	PriceExponent           float64                              `json:"price_exponent"`
	LatencyBeta             float64                              `json:"latency_beta"`
	PriorityPenalty         float64                              `json:"priority_penalty"`
	MinHealthFactor         float64                              `json:"min_health_factor"`
	MaxAccountShare         float64                              `json:"max_account_share"`
	MaxProviderShare        float64                              `json:"max_provider_share"`
	NewAccountShare         float64                              `json:"new_account_share"`
	DegradedShare           float64                              `json:"degraded_share"`
	RecoveryShares          []float64                            `json:"recovery_shares"`
	GenericFailThreshold    int                                  `json:"generic_fail_threshold"`
	FailureWindowSeconds    int64                                `json:"failure_window_seconds"`
	ProbeBackoffSeconds     []int64                              `json:"probe_backoff_seconds"`
	HardShareCaps           bool                                 `json:"hard_share_caps"`
	BenchmarkPriorEnabled   bool                                 `json:"benchmark_prior_enabled,omitempty"`
	RouteVariants           []OpenAIRouteShadowAuditRouteVariant `json:"route_variants,omitempty"`
}

type OpenAIRouteShadowAuditBudgetWindow struct {
	Window          string                  `json:"window"`
	Epoch           string                  `json:"epoch"`
	TTLSeconds      int64                   `json:"ttl_seconds"`
	Before          OpenAIRouteBudgetLedger `json:"before"`
	ProjectedAfter  OpenAIRouteBudgetLedger `json:"projected_after"`
	ProjectionValid bool                    `json:"projection_valid"`
}

// OpenAIRouteShadowAuditCandidate deliberately contains only routing inputs
// and derived factors. EndpointHash is one-way; FailureDomain is the explicit
// internal routing identifier. Raw URLs, account names and credentials never
// enter the audit table.
type OpenAIRouteShadowAuditCandidate struct {
	AccountID                        int64                        `json:"account_id"`
	RouteFingerprint                 string                       `json:"route_fingerprint"`
	EndpointHash                     string                       `json:"endpoint_hash"`
	RouteVariant                     bool                         `json:"route_variant,omitempty"`
	FailureDomain                    string                       `json:"failure_domain"`
	Transport                        string                       `json:"transport"`
	RateMultiplier                   float64                      `json:"rate_multiplier"`
	Priority                         int                          `json:"priority"`
	CircuitState                     OpenAIRouteCircuitState      `json:"circuit_state"`
	ProviderCircuitState             OpenAIRouteCircuitState      `json:"provider_circuit_state"`
	RecoveryStep                     int                          `json:"recovery_step"`
	HasReliabilitySample             bool                         `json:"has_reliability_sample"`
	SuccessLowerBound                float64                      `json:"success_lower_bound"`
	TTFTMilliseconds                 float64                      `json:"ttft_ms"`
	P95CompletionLatencyMilliseconds float64                      `json:"p95_completion_latency_ms"`
	PartialStreamRate                float64                      `json:"partial_stream_rate"`
	ObservationSource                string                       `json:"observation_source"`
	ObservationSamples               uint64                       `json:"observation_samples"`
	RecentSamples                    uint64                       `json:"recent_samples"`
	HourOfWeekSamples                uint64                       `json:"hour_of_week_samples"`
	BenchmarkSamples                 uint64                       `json:"benchmark_samples,omitempty"`
	BenchmarkEffectiveSamples        uint64                       `json:"benchmark_effective_samples,omitempty"`
	BenchmarkRecentSamples           uint64                       `json:"benchmark_recent_samples,omitempty"`
	BenchmarkHourOfWeekSamples       uint64                       `json:"benchmark_hour_of_week_samples,omitempty"`
	BenchmarkConfidence              float64                      `json:"benchmark_confidence,omitempty"`
	BenchmarkRecencyWeight           float64                      `json:"benchmark_recency_weight,omitempty"`
	BenchmarkLastObservedAt          *time.Time                   `json:"benchmark_last_observed_at,omitempty"`
	LoadRatio                        float64                      `json:"load_ratio"`
	WaitingCount                     int                          `json:"waiting_count"`
	CurrentAccountShare              float64                      `json:"current_account_share"`
	CurrentProviderShare             float64                      `json:"current_provider_share"`
	ExplorationBoost                 float64                      `json:"exploration_boost"`
	EstimatedBaseCostUSD             float64                      `json:"estimated_base_cost_usd"`
	EstimatedAccountCostUSD          float64                      `json:"estimated_account_cost_usd"`
	ObservedMeanCostUSD              float64                      `json:"observed_mean_base_cost_usd,omitempty"`
	CostObservationSamples           uint64                       `json:"cost_observation_samples"`
	CostEstimateSource               string                       `json:"cost_estimate_source"`
	Rank                             int                          `json:"rank,omitempty"`
	Selected                         bool                         `json:"selected"`
	Weight                           float64                      `json:"weight,omitempty"`
	HealthFactor                     float64                      `json:"health_factor,omitempty"`
	LatencyFactor                    float64                      `json:"latency_factor,omitempty"`
	TailLatencyFactor                float64                      `json:"tail_latency_factor,omitempty"`
	StreamIntegrityFactor            float64                      `json:"stream_integrity_factor,omitempty"`
	HeadroomFactor                   float64                      `json:"headroom_factor,omitempty"`
	PriceFactor                      float64                      `json:"price_factor,omitempty"`
	PriorityFactor                   float64                      `json:"priority_factor,omitempty"`
	PredictedExtraCostUSD            float64                      `json:"predicted_extra_cost_usd,omitempty"`
	EmergencyBudgetUsed              bool                         `json:"emergency_budget_used"`
	ExclusionReasons                 []OpenAIRouteExclusionReason `json:"exclusion_reasons,omitempty"`
	LegacyScore                      float64                      `json:"legacy_score"`
	LegacyRank                       int                          `json:"legacy_rank,omitempty"`
	LegacyCompactTier                int                          `json:"legacy_compact_tier"`
}

type OpenAIRouteShadowAuditSnapshot struct {
	ActivationID                     string                               `json:"activation_id"`
	ShadowStartedAt                  time.Time                            `json:"shadow_started_at"`
	RequestClass                     OpenAIRouteRequestClass              `json:"request_class"`
	Policy                           OpenAIRouteShadowAuditPolicy         `json:"policy"`
	AdaptiveSeedHex                  string                               `json:"adaptive_seed_hex"`
	RequiredTransport                string                               `json:"required_transport"`
	RequireCompact                   bool                                 `json:"require_compact"`
	ExcludedAccountIDs               []int64                              `json:"excluded_account_ids"`
	EstimatedBaseCostUSD             float64                              `json:"estimated_base_cost_usd"`
	MinHealthyMultiplier             float64                              `json:"min_healthy_multiplier"`
	LegacySelectedEndpointHash       string                               `json:"legacy_selected_endpoint_hash,omitempty"`
	AdaptiveSelectedEndpointHash     string                               `json:"adaptive_selected_endpoint_hash,omitempty"`
	AdaptiveSelectedRouteFingerprint string                               `json:"adaptive_selected_route_fingerprint,omitempty"`
	BudgetWindows                    []OpenAIRouteShadowAuditBudgetWindow `json:"budget_windows"`
	Candidates                       []OpenAIRouteShadowAuditCandidate    `json:"candidates"`
	Exclusions                       []OpenAIRouteExclusion               `json:"exclusions"`
	LegacyTopK                       int                                  `json:"legacy_top_k"`
	LegacyLoadSkew                   float64                              `json:"legacy_load_skew"`
	LegacySelectionOrder             []int64                              `json:"legacy_selection_order"`
}

type OpenAIRouteShadowDecisionRecord struct {
	ID                               int64                           `json:"id"`
	DecisionID                       string                          `json:"decision_id"`
	RequestID                        string                          `json:"request_id"`
	ClientRequestID                  string                          `json:"client_request_id"`
	Attempt                          int                             `json:"attempt"`
	GroupID                          int64                           `json:"group_id"`
	Model                            string                          `json:"model"`
	RequestClass                     OpenAIRouteRequestClass         `json:"request_class"`
	PolicyMode                       OpenAIRoutePolicyMode           `json:"policy_mode"`
	PolicyVersion                    int                             `json:"policy_version"`
	ActivationID                     string                          `json:"activation_id"`
	ShadowStartedAt                  time.Time                       `json:"shadow_started_at,omitempty"`
	Reason                           string                          `json:"reason"`
	Evaluated                        bool                            `json:"evaluated"`
	EvaluationDurationMicros         int64                           `json:"evaluation_duration_us"`
	LegacySelectedAccountID          int64                           `json:"legacy_selected_account_id,omitempty"`
	AdaptiveSelectedAccountID        int64                           `json:"adaptive_selected_account_id,omitempty"`
	AdaptiveSelectedEndpointHash     string                          `json:"adaptive_selected_endpoint_hash,omitempty"`
	AdaptiveSelectedRouteFingerprint string                          `json:"adaptive_selected_route_fingerprint,omitempty"`
	AdaptiveSelectedRate             float64                         `json:"adaptive_selected_rate,omitempty"`
	CandidateCount                   int                             `json:"candidate_count"`
	ExcludedCount                    int                             `json:"excluded_count"`
	Diverged                         bool                            `json:"diverged"`
	Emergency                        bool                            `json:"emergency"`
	Snapshot                         *OpenAIRouteShadowAuditSnapshot `json:"snapshot"`
	CreatedAt                        time.Time                       `json:"created_at"`
}

type OpenAIRouteShadowDecisionFilter struct {
	StartTime       *time.Time
	EndTime         *time.Time
	GroupID         *int64
	Model           string
	RequestClass    OpenAIRouteRequestClass
	PolicyMode      OpenAIRoutePolicyMode
	PolicyVersion   *int
	ActivationID    string
	Reason          string
	RequestID       string
	ClientRequestID string
	Evaluated       *bool
	Diverged        *bool
	Emergency       *bool
	Page            int
	PageSize        int
}

type OpenAIRouteShadowDecisionList struct {
	Decisions []*OpenAIRouteShadowDecisionRecord `json:"decisions"`
	Total     int                                `json:"total"`
	Page      int                                `json:"page"`
	PageSize  int                                `json:"page_size"`
}

type OpenAIRouteShadowSelectedAccountStats struct {
	AccountID       int64   `json:"account_id"`
	RateMultiplier  float64 `json:"rate_multiplier"`
	SelectedCount   int64   `json:"selected_count"`
	SelectedPercent float64 `json:"selected_percent"`
}

type OpenAIRouteShadowSelectedProviderStats struct {
	ProviderKey     string  `json:"provider_key"`
	SelectedCount   int64   `json:"selected_count"`
	SelectedPercent float64 `json:"selected_percent"`
}

type OpenAIRouteShadowSelectedRouteStats struct {
	AccountID       int64   `json:"account_id"`
	EndpointHash    string  `json:"endpoint_hash"`
	FailureDomain   string  `json:"failure_domain"`
	RouteVariant    bool    `json:"route_variant"`
	SelectedCount   int64   `json:"selected_count"`
	SelectedPercent float64 `json:"selected_percent"`
}

type OpenAIRouteShadowDecisionStats struct {
	Total                          int64                                    `json:"total"`
	Evaluated                      int64                                    `json:"evaluated"`
	NotEvaluated                   int64                                    `json:"not_evaluated"`
	Diverged                       int64                                    `json:"diverged"`
	Emergency                      int64                                    `json:"emergency"`
	LinkedSuccessfulUsage          int64                                    `json:"linked_successful_usage"`
	LinkedLegacyFailure            int64                                    `json:"linked_legacy_failure"`
	AmbiguousOutcome               int64                                    `json:"ambiguous_outcome"`
	UnlinkedOutcome                int64                                    `json:"unlinked_outcome"`
	EvaluatedLinkedSuccessfulUsage int64                                    `json:"evaluated_linked_successful_usage"`
	EvaluatedLinkedLegacyFailure   int64                                    `json:"evaluated_linked_legacy_failure"`
	EvaluatedAmbiguousOutcome      int64                                    `json:"evaluated_ambiguous_outcome"`
	EvaluatedUnlinkedOutcome       int64                                    `json:"evaluated_unlinked_outcome"`
	PolicySnapshotVariants         int64                                    `json:"policy_snapshot_variants"`
	ActivationIDVariants           int64                                    `json:"activation_id_variants"`
	ShadowStartedAtVariants        int64                                    `json:"shadow_started_at_variants"`
	ShadowStartedAt                time.Time                                `json:"shadow_started_at,omitempty"`
	PolicyMaxAccountShare          float64                                  `json:"policy_max_account_share"`
	PolicyMaxProviderShare         float64                                  `json:"policy_max_provider_share"`
	CoveredHourBuckets             int64                                    `json:"covered_hour_buckets"`
	FirstDecisionAt                time.Time                                `json:"first_decision_at,omitempty"`
	LastDecisionAt                 time.Time                                `json:"last_decision_at,omitempty"`
	EvaluationDurationP50US        float64                                  `json:"evaluation_duration_p50_us"`
	EvaluationDurationP95US        float64                                  `json:"evaluation_duration_p95_us"`
	LegacyTTFTP50Ms                float64                                  `json:"legacy_ttft_p50_ms"`
	LegacyTTFTP95Ms                float64                                  `json:"legacy_ttft_p95_ms"`
	SelectedAccounts               []OpenAIRouteShadowSelectedAccountStats  `json:"selected_accounts"`
	SelectedProviders              []OpenAIRouteShadowSelectedProviderStats `json:"selected_providers"`
	SelectedRoutes                 []OpenAIRouteShadowSelectedRouteStats    `json:"selected_routes"`
}

type OpenAIRouteAuditHealth struct {
	Ready                           bool                                     `json:"ready"`
	StorageReady                    bool                                     `json:"storage_ready"`
	AuditCounterStartedAt           time.Time                                `json:"audit_counter_started_at,omitempty"`
	Attempted                       uint64                                   `json:"attempted"`
	Written                         uint64                                   `json:"written"`
	Failed                          uint64                                   `json:"failed"`
	Dropped                         uint64                                   `json:"dropped"`
	InFlight                        uint64                                   `json:"in_flight"`
	Waiting                         uint64                                   `json:"waiting"`
	Running                         int64                                    `json:"running"`
	Completeness                    float64                                  `json:"completeness"`
	StorageChecks                   uint64                                   `json:"storage_checks"`
	StorageCheckFailed              uint64                                   `json:"storage_check_failed"`
	LastSuccessAt                   time.Time                                `json:"last_success_at,omitempty"`
	LastFailureAt                   time.Time                                `json:"last_failure_at,omitempty"`
	LastError                       string                                   `json:"last_error"`
	ObservationCollectorAvailable   bool                                     `json:"observation_collector_available"`
	ObservationReady                bool                                     `json:"observation_ready"`
	ObservationCounterStartedAt     time.Time                                `json:"observation_counter_started_at,omitempty"`
	ObservationSubmitted            uint64                                   `json:"observation_submitted"`
	ObservationWritten              uint64                                   `json:"observation_written"`
	ObservationFailed               uint64                                   `json:"observation_failed"`
	ObservationDropped              uint64                                   `json:"observation_dropped"`
	ObservationRejected             uint64                                   `json:"observation_rejected"`
	ObservationInFlight             uint64                                   `json:"observation_in_flight"`
	ObservationWaiting              uint64                                   `json:"observation_waiting"`
	ObservationRunning              int64                                    `json:"observation_running"`
	ObservationCompleteness         float64                                  `json:"observation_completeness"`
	ObservationStorageChecks        uint64                                   `json:"observation_storage_checks"`
	ObservationStorageFailed        uint64                                   `json:"observation_storage_failed"`
	ObservationLastSuccessAt        time.Time                                `json:"observation_last_success_at,omitempty"`
	ObservationLastFailureAt        time.Time                                `json:"observation_last_failure_at,omitempty"`
	ObservationLastError            string                                   `json:"observation_last_error"`
	ObservationOutcomeApplied       uint64                                   `json:"observation_outcome_applied"`
	ObservationOutcomeFailed        uint64                                   `json:"observation_outcome_failed"`
	ObservationOutcomeInFlight      uint64                                   `json:"observation_outcome_in_flight"`
	ObservationOutcomeCompleteness  float64                                  `json:"observation_outcome_completeness"`
	ObservationOutcomeLastSuccessAt time.Time                                `json:"observation_outcome_last_success_at,omitempty"`
	ObservationOutcomeLastFailureAt time.Time                                `json:"observation_outcome_last_failure_at,omitempty"`
	ObservationOutcomeLastError     string                                   `json:"observation_outcome_last_error"`
	ObservationProfileCache         *OpenAIRouteObservationProfileCacheStats `json:"observation_profile_cache,omitempty"`
}

type OpenAIRouteDecisionRepository interface {
	CheckOpenAIRouteShadowDecisionStorage(ctx context.Context) error
	CreateOpenAIRouteShadowDecision(ctx context.Context, record *OpenAIRouteShadowDecisionRecord) error
	ListOpenAIRouteShadowDecisions(ctx context.Context, filter *OpenAIRouteShadowDecisionFilter) (*OpenAIRouteShadowDecisionList, error)
	GetOpenAIRouteShadowDecisionStats(ctx context.Context, filter *OpenAIRouteShadowDecisionFilter) (*OpenAIRouteShadowDecisionStats, error)
}

// OpenAIRouteAuditService makes audit durability an explicit shadow-routing
// dependency. A failed write never changes the user-visible legacy selection,
// but it invalidates that shadow sample and is exposed by Health.
type OpenAIRouteAuditService struct {
	repo OpenAIRouteDecisionRepository
	pool pond.Pool
	// counterStartedAt is part of the promotion evidence boundary. Counters are
	// intentionally process-local today, so a service restart must invalidate a
	// Shadow slice that began before this instance existed rather than silently
	// presenting a fresh 100% completeness ratio.
	counterStartedAt time.Time

	attempted     atomic.Uint64
	written       atomic.Uint64
	failed        atomic.Uint64
	dropped       atomic.Uint64
	lastSuccessNS atomic.Int64
	lastFailureNS atomic.Int64
	lastError     atomic.Value
	storageChecks atomic.Uint64
	storageFailed atomic.Uint64
	stopOnce      sync.Once
}

func NewOpenAIRouteAuditService(repo OpenAIRouteDecisionRepository) *OpenAIRouteAuditService {
	return NewOpenAIRouteAuditServiceWithOptions(repo, defaultOpenAIRouteAuditWorkers, defaultOpenAIRouteAuditQueue)
}

func NewOpenAIRouteAuditServiceWithOptions(repo OpenAIRouteDecisionRepository, workers, queue int) *OpenAIRouteAuditService {
	if workers <= 0 {
		workers = defaultOpenAIRouteAuditWorkers
	}
	if queue <= 0 {
		queue = defaultOpenAIRouteAuditQueue
	}
	s := &OpenAIRouteAuditService{
		repo:             repo,
		pool:             pond.NewPool(workers, pond.WithQueueSize(queue)),
		counterStartedAt: time.Now().UTC(),
	}
	s.lastError.Store("")
	return s
}

// Start verifies durable storage before any request is eligible for Shadow.
// The queue itself is ready immediately after construction.
func (s *OpenAIRouteAuditService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	_ = s.VerifyStorage(context.Background())
}

// Stop drains accepted decision writes. It never deletes persisted evidence.
func (s *OpenAIRouteAuditService) Stop() {
	if s == nil || s.pool == nil {
		return
	}
	s.stopOnce.Do(func() { s.pool.StopAndWait() })
}

func (s *OpenAIRouteAuditService) VerifyStorage(ctx context.Context) OpenAIRouteAuditHealth {
	if s == nil || s.repo == nil {
		return OpenAIRouteAuditHealth{}
	}
	baseCtx := ctx
	if baseCtx == nil || baseCtx.Err() != nil {
		baseCtx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(baseCtx, openAIRouteAuditWriteTimeout)
	defer cancel()
	s.storageChecks.Add(1)
	if err := s.repo.CheckOpenAIRouteShadowDecisionStorage(probeCtx); err != nil {
		s.storageFailed.Add(1)
		s.recordFailure(err)
		return s.Health()
	}
	s.recordSuccess()
	return s.Health()
}

func (s *OpenAIRouteAuditService) Record(ctx context.Context, record *OpenAIRouteShadowDecisionRecord) error {
	if s == nil || s.repo == nil {
		return ErrOpenAIRouteAuditUnavailable
	}
	s.attempted.Add(1)
	if err := prepareOpenAIRouteAuditRecord(record); err != nil {
		s.failed.Add(1)
		s.recordFailure(err)
		return err
	}
	return s.writePreparedRecord(ctx, record)
}

// TryRecord transfers an immutable decision record to a bounded background
// queue. Shadow audit persistence must not sit on the account-selection hot
// path; queue overflow is surfaced as explicit evidence loss and callers keep
// Legacy routing authoritative.
func (s *OpenAIRouteAuditService) TryRecord(record *OpenAIRouteShadowDecisionRecord) bool {
	if s == nil || s.repo == nil || s.pool == nil || s.pool.Stopped() {
		return false
	}
	s.attempted.Add(1)
	if err := prepareOpenAIRouteAuditRecord(record); err != nil {
		s.failed.Add(1)
		s.recordFailure(err)
		return false
	}
	_, ok := s.pool.TrySubmit(func() {
		_ = s.writePreparedRecord(context.Background(), record)
	})
	if !ok {
		s.failed.Add(1)
		s.dropped.Add(1)
		s.recordFailure(errOpenAIRouteAuditQueueFull)
	}
	return ok
}

func prepareOpenAIRouteAuditRecord(record *OpenAIRouteShadowDecisionRecord) error {
	if record == nil || strings.TrimSpace(record.DecisionID) == "" || record.GroupID <= 0 || strings.TrimSpace(record.Model) == "" || !record.RequestClass.Valid() || record.Snapshot == nil {
		return fmt.Errorf("%w: incomplete decision record", ErrOpenAIRouteAuditUnavailable)
	}
	if record.Attempt <= 0 {
		record.Attempt = 1
	}
	record.DecisionID = truncateOpenAIRouteAuditValue(strings.TrimSpace(record.DecisionID), 64)
	record.RequestID = truncateOpenAIRouteAuditValue(strings.TrimSpace(record.RequestID), 128)
	record.ClientRequestID = truncateOpenAIRouteAuditValue(strings.TrimSpace(record.ClientRequestID), 128)
	record.Model = truncateOpenAIRouteAuditValue(strings.TrimSpace(record.Model), 128)
	record.ActivationID = truncateOpenAIRouteAuditValue(strings.TrimSpace(record.ActivationID), 128)
	if !record.ShadowStartedAt.IsZero() {
		record.ShadowStartedAt = record.ShadowStartedAt.UTC()
	}
	record.Reason = truncateOpenAIRouteAuditValue(strings.TrimSpace(record.Reason), 64)
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	return nil
}

func (s *OpenAIRouteAuditService) writePreparedRecord(ctx context.Context, record *OpenAIRouteShadowDecisionRecord) error {
	baseCtx := ctx
	if baseCtx == nil || baseCtx.Err() != nil {
		baseCtx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(baseCtx, openAIRouteAuditWriteTimeout)
	defer cancel()
	if err := s.repo.CreateOpenAIRouteShadowDecision(writeCtx, record); err != nil {
		s.failed.Add(1)
		s.recordFailure(err)
		logger.FromContext(baseCtx).Warn("openai.route_shadow_audit_write_failed",
			zap.String("component", "routing.audit"),
			zap.String("decision_id", record.DecisionID),
			zap.String("request_id", record.RequestID),
			zap.Int64("group_id", record.GroupID),
			zap.String("model", record.Model),
			zap.Error(err),
		)
		return err
	}

	s.written.Add(1)
	s.recordSuccess()
	return nil
}

func (s *OpenAIRouteAuditService) recordFailure(err error) {
	if s == nil {
		return
	}
	s.lastFailureNS.Store(time.Now().UTC().UnixNano())
	if err != nil {
		s.lastError.Store(err.Error())
	}
}

func (s *OpenAIRouteAuditService) recordSuccess() {
	if s == nil {
		return
	}
	s.lastSuccessNS.Store(time.Now().UTC().UnixNano())
	s.lastError.Store("")
}

func truncateOpenAIRouteAuditValue(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func (s *OpenAIRouteAuditService) List(ctx context.Context, filter *OpenAIRouteShadowDecisionFilter) (*OpenAIRouteShadowDecisionList, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOpenAIRouteAuditUnavailable
	}
	return s.repo.ListOpenAIRouteShadowDecisions(ctx, filter)
}

func (s *OpenAIRouteAuditService) Stats(ctx context.Context, filter *OpenAIRouteShadowDecisionFilter) (*OpenAIRouteShadowDecisionStats, error) {
	if s == nil || s.repo == nil {
		return nil, ErrOpenAIRouteAuditUnavailable
	}
	return s.repo.GetOpenAIRouteShadowDecisionStats(ctx, filter)
}

func (s *OpenAIRouteAuditService) Health() OpenAIRouteAuditHealth {
	if s == nil {
		return OpenAIRouteAuditHealth{}
	}
	health := OpenAIRouteAuditHealth{
		Written:               s.written.Load(),
		Failed:                s.failed.Load(),
		Dropped:               s.dropped.Load(),
		StorageChecks:         s.storageChecks.Load(),
		StorageCheckFailed:    s.storageFailed.Load(),
		AuditCounterStartedAt: s.counterStartedAt,
	}
	// Record increments attempted before it starts validation or I/O and only
	// then increments one terminal counter. Read attempted last so concurrent
	// callers can be represented as in-flight instead of looking like silent
	// loss in the admin health response.
	health.Attempted = s.attempted.Load()
	completed := health.Written + health.Failed
	if health.Attempted > completed {
		health.InFlight = health.Attempted - completed
	}
	health.Completeness = 1
	if health.Attempted > 0 {
		health.Completeness = float64(health.Written) / float64(health.Attempted)
	}
	if s.pool != nil {
		health.Waiting = s.pool.WaitingTasks()
		health.Running = s.pool.RunningWorkers()
	}
	if value, ok := s.lastError.Load().(string); ok {
		health.LastError = strings.TrimSpace(value)
	}
	if ns := s.lastSuccessNS.Load(); ns > 0 {
		health.LastSuccessAt = time.Unix(0, ns).UTC()
	}
	if ns := s.lastFailureNS.Load(); ns > 0 {
		health.LastFailureAt = time.Unix(0, ns).UTC()
	}
	health.StorageReady = s.repo != nil && !health.LastSuccessAt.IsZero() && (health.LastFailureAt.IsZero() || health.LastSuccessAt.After(health.LastFailureAt))
	health.Ready = health.StorageReady && health.Completeness >= 0.99
	return health
}
