package service

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ReliabilityFactType string

const (
	ReliabilityFactCustomerRequest ReliabilityFactType = "customer_request"
	ReliabilityFactUpstreamAttempt ReliabilityFactType = "upstream_attempt"
	ReliabilityFactActiveProbe     ReliabilityFactType = "active_probe"
)

type ReliabilityOutcome string

const (
	ReliabilityOutcomeSuccess   ReliabilityOutcome = "success"
	ReliabilityOutcomeFailure   ReliabilityOutcome = "failure"
	ReliabilityOutcomeRecovered ReliabilityOutcome = "recovered"
	ReliabilityOutcomeExcluded  ReliabilityOutcome = "excluded"
)

const (
	ReliabilityRequestClassText  = "text"
	ReliabilityRequestClassImage = "image"
	ReliabilityRequestClassVideo = "video"
	ReliabilityRequestClassOther = "other"
)

var reliabilityRouteFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var reliabilityEndpointHashPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

func ReliabilityRouteFingerprint(platform string, accountID int64, endpointHash, model, protocol string) string {
	identity := fmt.Sprintf("%s|%d|%s|%s|%s", strings.ToLower(strings.TrimSpace(platform)), accountID, strings.ToLower(strings.TrimSpace(endpointHash)), strings.TrimSpace(model), strings.ToLower(strings.TrimSpace(protocol)))
	sum := md5.Sum([]byte(identity))
	return hex.EncodeToString(sum[:])
}

// ReliabilityObservation is a normalized, non-sensitive fact. It deliberately
// has no prompt, response body, credential, account name, or raw URL field.
type ReliabilityObservation struct {
	ID             int64
	IdempotencyKey string
	FactType       ReliabilityFactType
	Source         string
	SourceID       string

	RequestID       string
	ClientRequestID string
	UserID          *int64
	GroupID         *int64
	AccessGroupID   int64
	AccountID       *int64

	Platform           string
	Model              string
	RequestClass       string
	Protocol           string
	Transport          string
	EndpointHash       string
	RouteFingerprint   string
	RoutingFingerprint string

	Outcome         ReliabilityOutcome
	StatusCode      *int
	ErrorOwner      string
	ExclusionReason string
	CustomerImpact  bool
	LatencyMs       int64
	ObservedAt      time.Time
}

type ReliabilityFinalOutcome struct {
	RequestIdentity string
	RequestID       string
	ClientRequestID string
	UserID          *int64
	GroupID         *int64
	AccountID       *int64
	Platform        string
	Model           string
	RequestClass    string
	Protocol        string
	Outcome         ReliabilityOutcome
	StatusCode      *int
	ErrorOwner      string
	ExclusionReason string
	LatencyMs       int64
	ObservedAt      time.Time
}

type ReliabilityAttemptOutcome struct {
	RequestIdentity    string
	AttemptIdentity    string
	RequestID          string
	ClientRequestID    string
	UserID             *int64
	GroupID            *int64
	AccessGroupID      int64
	AccountID          *int64
	Platform           string
	Model              string
	RequestClass       string
	Protocol           string
	Transport          string
	EndpointHash       string
	RoutingFingerprint string
	Outcome            ReliabilityOutcome
	StatusCode         *int
	ErrorOwner         string
	LatencyMs          int64
	ObservedAt         time.Time
}

type ReliabilityProbeOutcome struct {
	ProbeIdentity   string
	AccountID       *int64
	Platform        string
	Model           string
	RequestClass    string
	Protocol        string
	Transport       string
	EndpointHash    string
	Outcome         ReliabilityOutcome
	StatusCode      *int
	ErrorOwner      string
	ExclusionReason string
	LatencyMs       int64
	ObservedAt      time.Time
}

type ReliabilityProbeClaim struct {
	ClaimIdentity    string
	RouteFingerprint string
	IntervalStart    time.Time
	ExpiresAt        time.Time
}

// ReliabilityEvidenceAttemptScope narrows a snapshot to exact adaptive-route
// attempts. Routing fingerprints already include group, account, model,
// request class, endpoint, transport, and failure domain; access group and
// inbound protocol remain independent request dimensions.
type ReliabilityEvidenceAttemptScope struct {
	GroupID             int64
	AccessGroupID       int64
	Protocol            string
	Transports          []string
	RoutingFingerprints []string
}

type ReliabilityEvidenceProbeRoute struct {
	AccountID    int64
	EndpointHash string
	Transport    string
}

type ReliabilityEvidenceProbeScope struct {
	Protocol string
	Routes   []ReliabilityEvidenceProbeRoute
}

type ReliabilityEvidenceQuery struct {
	Start                time.Time
	End                  time.Time
	FactTypes            []ReliabilityFactType
	GroupID              *int64
	RouteFingerprint     string
	CustomerImpact       *bool
	Limit                int
	AnyGroupIDs          []int64
	AnyAccountIDs        []int64
	AnyPlatforms         []string
	AnyRouteFingerprints []string
	AnyModelPatterns     []string
	AttemptScope         *ReliabilityEvidenceAttemptScope
	ProbeScope           *ReliabilityEvidenceProbeScope
	Outcomes             []ReliabilityOutcome
	BeforeObservedAt     *time.Time
	BeforeID             int64
}

type ReliabilityEvidenceSnapshot struct {
	GeneratedAt  time.Time                       `json:"generated_at"`
	Completeness ReliabilityEvidenceCompleteness `json:"completeness"`
	Observations []*ReliabilityObservation       `json:"observations"`
}

type ReliabilityEvidenceCompleteness struct {
	Enabled         bool       `json:"enabled"`
	Running         bool       `json:"running"`
	Enqueued        int64      `json:"enqueued"`
	Processed       int64      `json:"processed"`
	Written         int64      `json:"written"`
	Dropped         int64      `json:"dropped"`
	Failed          int64      `json:"failed"`
	QueueDepth      int64      `json:"queue_depth"`
	InFlight        int64      `json:"in_flight"`
	Ready           bool       `json:"ready"`
	OldestPendingAt *time.Time `json:"oldest_pending_at,omitempty"`
	PendingCutoffID uint64     `json:"pending_cutoff_id,omitempty"`
}

type ReliabilityEvidenceService struct {
	db                 *sql.DB
	enabled            bool
	batchInsert        func(context.Context, []*ReliabilityObservation) (int64, error)
	listEvidence       func(context.Context, *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error)
	claimProbe         func(context.Context, *ReliabilityProbeClaim) (bool, error)
	enqueued           atomic.Int64
	processed          atomic.Int64
	written            atomic.Int64
	dropped            atomic.Int64
	failed             atomic.Int64
	queueDepth         atomic.Int64
	inFlight           atomic.Int64
	running            atomic.Bool
	lastDropLogAt      atomic.Int64
	nextPendingID      atomic.Uint64
	publishedPendingID atomic.Uint64
	queueMu            sync.Mutex
	pendingMu          sync.Mutex
	pendingObserved    map[uint64]time.Time
	queueWG            sync.WaitGroup
	queue              chan reliabilityEvidenceJob
	queueStarted       bool
	queueStopping      bool
	queueSize          int
	workerCount        int
	dependentMu        sync.Mutex
	dependentStops     []func()
	dependentStopOnce  sync.Once
}

func (s *ReliabilityEvidenceService) registerDependentStop(stop func()) {
	if s == nil || stop == nil {
		return
	}
	s.dependentMu.Lock()
	s.dependentStops = append(s.dependentStops, stop)
	s.dependentMu.Unlock()
}

func (s *ReliabilityEvidenceService) stopDependents() {
	if s == nil {
		return
	}
	s.dependentStopOnce.Do(func() {
		s.dependentMu.Lock()
		stops := append([]func(){}, s.dependentStops...)
		s.dependentStops = nil
		s.dependentMu.Unlock()
		for _, stop := range stops {
			stop()
		}
	})
}

func NewReliabilityEvidenceService(db *sql.DB, settingRepo SettingRepository) *ReliabilityEvidenceService {
	service := &ReliabilityEvidenceService{db: db, queueSize: 4096, workerCount: 2}
	service.batchInsert = service.batchInsertPostgres
	service.listEvidence = service.listPostgres
	service.claimProbe = service.tryClaimProbePostgres
	if settingRepo == nil {
		return service
	}
	value, err := settingRepo.GetValue(context.Background(), SettingKeyReliabilityObservationEnabled)
	if err != nil {
		return service
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "enabled":
		service.enabled = true
	}
	return service
}

func (s *ReliabilityEvidenceService) Enabled() bool { return s != nil && s.enabled }

func (s *ReliabilityEvidenceService) Completeness() ReliabilityEvidenceCompleteness {
	if s == nil {
		return ReliabilityEvidenceCompleteness{}
	}
	return s.completenessThrough(s.publishedPendingID.Load())
}

func (s *ReliabilityEvidenceService) completenessThrough(cutoff uint64) ReliabilityEvidenceCompleteness {
	if s == nil {
		return ReliabilityEvidenceCompleteness{}
	}
	result := ReliabilityEvidenceCompleteness{
		Enabled: s.enabled, Running: s.running.Load(), Enqueued: s.enqueued.Load(), Processed: s.processed.Load(), Written: s.written.Load(),
		Dropped: s.dropped.Load(), Failed: s.failed.Load(), QueueDepth: s.queueDepth.Load(), InFlight: s.inFlight.Load(),
		PendingCutoffID: cutoff,
	}
	result.Ready = result.Enabled && result.Running && result.Dropped == 0 && result.Failed == 0 && result.QueueDepth == 0 && result.InFlight == 0 && result.Processed == result.Enqueued
	s.pendingMu.Lock()
	for pendingID, observedAt := range s.pendingObserved {
		if pendingID > cutoff {
			continue
		}
		if result.OldestPendingAt == nil || observedAt.Before(*result.OldestPendingAt) {
			value := observedAt
			result.OldestPendingAt = &value
		}
	}
	s.pendingMu.Unlock()
	return result
}

func (s *ReliabilityEvidenceService) recordFinalOutcomes(ctx context.Context, inputs []*ReliabilityFinalOutcome) (int64, error) {
	observations := make([]*ReliabilityObservation, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}
		identity := strings.TrimSpace(input.RequestIdentity)
		if identity == "" || len(identity) > 160 {
			return 0, fmt.Errorf("reliability final outcome identity is invalid")
		}
		errorOwner := strings.ToLower(strings.TrimSpace(input.ErrorOwner))
		outcome := input.Outcome
		exclusionReason := strings.ToLower(strings.TrimSpace(input.ExclusionReason))
		if outcome == ReliabilityOutcomeFailure && (input.UserID == nil || *input.UserID <= 0) {
			// A request that never resolved to a real customer cannot enter
			// Customer Availability, an Incident, or compensation. Preserve the
			// diagnostic owner/status while failing closed on attribution.
			outcome = ReliabilityOutcomeExcluded
			if exclusionReason == "" {
				exclusionReason = "client_or_unowned"
			}
		}
		customerImpact := outcome == ReliabilityOutcomeFailure && (errorOwner == "provider" || errorOwner == "platform")
		observations = append(observations, &ReliabilityObservation{
			IdempotencyKey: "customer:" + identity, FactType: ReliabilityFactCustomerRequest,
			Source: "gateway_final", SourceID: identity, RequestID: input.RequestID, ClientRequestID: input.ClientRequestID,
			UserID: input.UserID, GroupID: input.GroupID, AccountID: input.AccountID,
			Platform: input.Platform, Model: input.Model, RequestClass: input.RequestClass, Protocol: input.Protocol,
			Outcome: outcome, StatusCode: input.StatusCode, ErrorOwner: errorOwner,
			ExclusionReason: exclusionReason, CustomerImpact: customerImpact, LatencyMs: input.LatencyMs, ObservedAt: input.ObservedAt,
		})
	}
	return s.recordObservations(ctx, observations)
}

func (s *ReliabilityEvidenceService) recordAttemptOutcomes(ctx context.Context, inputs []*ReliabilityAttemptOutcome) (int64, error) {
	observations := make([]*ReliabilityObservation, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}
		requestIdentity := strings.TrimSpace(input.RequestIdentity)
		attemptIdentity := strings.TrimSpace(input.AttemptIdentity)
		if requestIdentity == "" || attemptIdentity == "" || len(requestIdentity)+len(attemptIdentity) > 160 {
			return 0, fmt.Errorf("reliability attempt outcome identity is invalid")
		}
		sourceID := requestIdentity + ":" + attemptIdentity
		accountID := int64(0)
		if input.AccountID != nil {
			accountID = *input.AccountID
		}
		observations = append(observations, &ReliabilityObservation{
			IdempotencyKey: "attempt:" + sourceID, FactType: ReliabilityFactUpstreamAttempt,
			Source: "gateway_attempt", SourceID: sourceID, RequestID: input.RequestID, ClientRequestID: input.ClientRequestID,
			UserID: input.UserID, GroupID: input.GroupID, AccessGroupID: input.AccessGroupID, AccountID: input.AccountID,
			Platform: input.Platform, Model: input.Model, RequestClass: input.RequestClass, Protocol: input.Protocol,
			Transport: input.Transport, EndpointHash: input.EndpointHash,
			RouteFingerprint:   ReliabilityRouteFingerprint(input.Platform, accountID, input.EndpointHash, input.Model, input.Protocol),
			RoutingFingerprint: input.RoutingFingerprint,
			Outcome:            input.Outcome, StatusCode: input.StatusCode, ErrorOwner: input.ErrorOwner,
			CustomerImpact: false, LatencyMs: input.LatencyMs, ObservedAt: input.ObservedAt,
		})
	}
	return s.recordObservations(ctx, observations)
}

func (s *ReliabilityEvidenceService) recordProbeOutcomes(ctx context.Context, inputs []*ReliabilityProbeOutcome) (int64, error) {
	observations := make([]*ReliabilityObservation, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}
		identity := strings.TrimSpace(input.ProbeIdentity)
		if identity == "" || len(identity) > 170 {
			return 0, fmt.Errorf("reliability probe outcome identity is invalid")
		}
		accountID := int64(0)
		if input.AccountID != nil {
			accountID = *input.AccountID
		}
		observations = append(observations, &ReliabilityObservation{
			IdempotencyKey: "probe:" + identity, FactType: ReliabilityFactActiveProbe,
			Source: "monthly_probe", SourceID: identity, AccountID: input.AccountID,
			Platform: input.Platform, Model: input.Model, RequestClass: input.RequestClass, Protocol: input.Protocol, Transport: input.Transport,
			EndpointHash:     input.EndpointHash,
			RouteFingerprint: ReliabilityRouteFingerprint(input.Platform, accountID, input.EndpointHash, input.Model, input.Protocol),
			Outcome:          input.Outcome, StatusCode: input.StatusCode, ErrorOwner: input.ErrorOwner,
			ExclusionReason: input.ExclusionReason, CustomerImpact: false, LatencyMs: input.LatencyMs, ObservedAt: input.ObservedAt,
		})
	}
	return s.recordObservations(ctx, observations)
}

func (s *ReliabilityEvidenceService) ClaimProbe(ctx context.Context, claim *ReliabilityProbeClaim) (bool, error) {
	if !s.Enabled() {
		return true, nil
	}
	if s.claimProbe == nil || claim == nil {
		return false, fmt.Errorf("reliability probe claim repository is not available")
	}
	copy := *claim
	copy.ClaimIdentity = strings.TrimSpace(copy.ClaimIdentity)
	copy.RouteFingerprint = strings.ToLower(strings.TrimSpace(copy.RouteFingerprint))
	if copy.ClaimIdentity == "" || len(copy.ClaimIdentity) > 180 || !reliabilityRouteFingerprintPattern.MatchString(copy.RouteFingerprint) {
		return false, fmt.Errorf("reliability probe claim identity is invalid")
	}
	if copy.IntervalStart.IsZero() || !copy.ExpiresAt.After(copy.IntervalStart) {
		return false, fmt.Errorf("reliability probe claim window is invalid")
	}
	return s.claimProbe(ctx, &copy)
}

func (s *ReliabilityEvidenceService) recordObservations(ctx context.Context, inputs []*ReliabilityObservation) (int64, error) {
	if len(inputs) == 0 || !s.Enabled() {
		return 0, nil
	}
	if s.batchInsert == nil {
		return 0, fmt.Errorf("reliability observation repository is not available")
	}
	prepared := make([]*ReliabilityObservation, 0, len(inputs))
	for _, input := range inputs {
		item, err := normalizeReliabilityObservation(input)
		if err != nil {
			return 0, err
		}
		prepared = append(prepared, item)
	}
	return s.batchInsert(ctx, prepared)
}

func (s *ReliabilityEvidenceService) Snapshot(ctx context.Context, query *ReliabilityEvidenceQuery) (*ReliabilityEvidenceSnapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("reliability observation repository is not available")
	}
	return s.snapshotAtPendingCutoff(ctx, query, s.publishedPendingID.Load())
}

func (s *ReliabilityEvidenceService) snapshotAtPendingCutoff(ctx context.Context, query *ReliabilityEvidenceQuery, cutoff uint64) (*ReliabilityEvidenceSnapshot, error) {
	if s == nil || s.listEvidence == nil {
		return nil, fmt.Errorf("reliability observation repository is not available")
	}
	if query == nil || query.Start.IsZero() || query.End.IsZero() || !query.Start.Before(query.End) {
		return nil, fmt.Errorf("reliability evidence window is invalid")
	}
	normalized := *query
	if normalized.End.Sub(normalized.Start) > 7*24*time.Hour {
		return nil, fmt.Errorf("reliability evidence window exceeds seven days")
	}
	if normalized.Limit <= 0 {
		normalized.Limit = 1000
	}
	if normalized.Limit > 5001 {
		normalized.Limit = 5001
	}
	seenFactTypes := make(map[ReliabilityFactType]struct{}, len(normalized.FactTypes))
	factTypes := make([]ReliabilityFactType, 0, len(normalized.FactTypes))
	for _, factType := range normalized.FactTypes {
		switch factType {
		case ReliabilityFactCustomerRequest, ReliabilityFactUpstreamAttempt, ReliabilityFactActiveProbe:
		default:
			return nil, fmt.Errorf("reliability evidence fact type is invalid")
		}
		if _, exists := seenFactTypes[factType]; exists {
			continue
		}
		seenFactTypes[factType] = struct{}{}
		factTypes = append(factTypes, factType)
	}
	normalized.FactTypes = factTypes
	normalized.RouteFingerprint = strings.ToLower(strings.TrimSpace(normalized.RouteFingerprint))
	if normalized.RouteFingerprint != "" && !reliabilityRouteFingerprintPattern.MatchString(normalized.RouteFingerprint) {
		return nil, fmt.Errorf("reliability evidence route fingerprint is invalid")
	}
	normalized.AnyGroupIDs = normalizeReliabilityInt64Selectors(normalized.AnyGroupIDs)
	normalized.AnyAccountIDs = normalizeReliabilityInt64Selectors(normalized.AnyAccountIDs)
	normalized.AnyPlatforms = normalizeReliabilityStringSelectors(normalized.AnyPlatforms)
	normalized.AnyRouteFingerprints = normalizeReliabilityStringSelectors(normalized.AnyRouteFingerprints)
	normalized.AnyModelPatterns = normalizeReliabilityStringSelectors(normalized.AnyModelPatterns)
	seenOutcomes := map[ReliabilityOutcome]struct{}{}
	outcomes := make([]ReliabilityOutcome, 0, len(normalized.Outcomes))
	for _, outcome := range normalized.Outcomes {
		switch outcome {
		case ReliabilityOutcomeSuccess, ReliabilityOutcomeFailure, ReliabilityOutcomeRecovered, ReliabilityOutcomeExcluded:
		default:
			return nil, fmt.Errorf("reliability evidence outcome selector is invalid")
		}
		if _, exists := seenOutcomes[outcome]; exists {
			continue
		}
		seenOutcomes[outcome] = struct{}{}
		outcomes = append(outcomes, outcome)
	}
	normalized.Outcomes = outcomes
	if normalized.BeforeObservedAt != nil {
		value := normalized.BeforeObservedAt.UTC()
		if value.IsZero() || normalized.BeforeID <= 0 {
			return nil, fmt.Errorf("reliability evidence cursor is invalid")
		}
		normalized.BeforeObservedAt = &value
	} else if normalized.BeforeID != 0 {
		return nil, fmt.Errorf("reliability evidence cursor is invalid")
	}
	if normalized.AttemptScope != nil {
		scope := *normalized.AttemptScope
		scope.Protocol = strings.ToLower(strings.TrimSpace(scope.Protocol))
		scope.Transports = normalizeReliabilityStringSelectors(scope.Transports)
		scope.RoutingFingerprints = normalizeReliabilityStringSelectors(scope.RoutingFingerprints)
		if scope.GroupID <= 0 || scope.AccessGroupID < 0 || scope.Protocol == "" || len(scope.Transports) == 0 || len(scope.RoutingFingerprints) == 0 {
			return nil, fmt.Errorf("reliability evidence attempt scope is invalid")
		}
		for _, fingerprint := range scope.RoutingFingerprints {
			if !reliabilityRouteFingerprintPattern.MatchString(fingerprint) {
				return nil, fmt.Errorf("reliability evidence attempt routing fingerprint is invalid")
			}
		}
		normalized.AttemptScope = &scope
	}
	if normalized.ProbeScope != nil {
		scope := *normalized.ProbeScope
		scope.Protocol = strings.ToLower(strings.TrimSpace(scope.Protocol))
		routes := make([]ReliabilityEvidenceProbeRoute, 0, len(scope.Routes))
		seen := make(map[string]struct{}, len(scope.Routes))
		for _, route := range scope.Routes {
			route.EndpointHash = strings.ToLower(strings.TrimSpace(route.EndpointHash))
			route.Transport = strings.ToLower(strings.TrimSpace(route.Transport))
			if route.AccountID <= 0 || !reliabilityEndpointHashPattern.MatchString(route.EndpointHash) || route.Transport == "" {
				return nil, fmt.Errorf("reliability evidence probe route is invalid")
			}
			key := fmt.Sprintf("%d|%s|%s", route.AccountID, route.EndpointHash, route.Transport)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			routes = append(routes, route)
		}
		if scope.Protocol == "" || len(routes) == 0 {
			return nil, fmt.Errorf("reliability evidence probe scope is invalid")
		}
		scope.Routes = routes
		normalized.ProbeScope = &scope
	}
	for _, fingerprint := range normalized.AnyRouteFingerprints {
		if !reliabilityRouteFingerprintPattern.MatchString(fingerprint) {
			return nil, fmt.Errorf("reliability evidence route fingerprint selector is invalid")
		}
	}
	before := s.completenessThrough(cutoff)
	observations, err := s.listEvidence(ctx, &normalized)
	if err != nil {
		return nil, err
	}
	after := s.completenessThrough(cutoff)
	return &ReliabilityEvidenceSnapshot{GeneratedAt: time.Now(), Completeness: mergeReliabilityCompletenessConservative(before, after), Observations: observations}, nil
}

func mergeReliabilityCompletenessConservative(left, right ReliabilityEvidenceCompleteness) ReliabilityEvidenceCompleteness {
	result := right
	result.Enabled = left.Enabled && right.Enabled
	result.Running = left.Running && right.Running
	result.Ready = left.Ready && right.Ready
	result.Enqueued = maxReliabilityInt64(left.Enqueued, right.Enqueued)
	result.Processed = maxReliabilityInt64(left.Processed, right.Processed)
	result.Written = maxReliabilityInt64(left.Written, right.Written)
	result.Dropped = maxReliabilityInt64(left.Dropped, right.Dropped)
	result.Failed = maxReliabilityInt64(left.Failed, right.Failed)
	result.QueueDepth = maxReliabilityInt64(left.QueueDepth, right.QueueDepth)
	result.InFlight = maxReliabilityInt64(left.InFlight, right.InFlight)
	result.PendingCutoffID = maxReliabilityUint64(left.PendingCutoffID, right.PendingCutoffID)
	if left.OldestPendingAt != nil && (result.OldestPendingAt == nil || left.OldestPendingAt.Before(*result.OldestPendingAt)) {
		value := *left.OldestPendingAt
		result.OldestPendingAt = &value
	}
	return result
}

func maxReliabilityInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func maxReliabilityUint64(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}

func normalizeReliabilityInt64Selectors(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeReliabilityStringSelectors(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *OpsService) SetReliabilityEvidenceService(evidence *ReliabilityEvidenceService) {
	if s != nil {
		s.reliabilityEvidence = evidence
	}
}

func normalizeReliabilityObservation(input *ReliabilityObservation) (*ReliabilityObservation, error) {
	if input == nil {
		return nil, fmt.Errorf("reliability observation is required")
	}
	item := *input
	item.IdempotencyKey = strings.TrimSpace(item.IdempotencyKey)
	item.Source = strings.TrimSpace(item.Source)
	item.SourceID = strings.TrimSpace(item.SourceID)
	item.RequestID = strings.TrimSpace(item.RequestID)
	item.ClientRequestID = strings.TrimSpace(item.ClientRequestID)
	item.Platform = strings.ToLower(strings.TrimSpace(item.Platform))
	item.Model = strings.TrimSpace(item.Model)
	item.RequestClass = strings.ToLower(strings.TrimSpace(item.RequestClass))
	item.Protocol = strings.ToLower(strings.TrimSpace(item.Protocol))
	item.Transport = strings.ToLower(strings.TrimSpace(item.Transport))
	item.EndpointHash = strings.ToLower(strings.TrimSpace(item.EndpointHash))
	item.RouteFingerprint = strings.ToLower(strings.TrimSpace(item.RouteFingerprint))
	item.RoutingFingerprint = strings.ToLower(strings.TrimSpace(item.RoutingFingerprint))
	item.ErrorOwner = strings.ToLower(strings.TrimSpace(item.ErrorOwner))
	item.ExclusionReason = strings.ToLower(strings.TrimSpace(item.ExclusionReason))
	item.RequestID = truncateString(item.RequestID, 128)
	item.ClientRequestID = truncateString(item.ClientRequestID, 128)
	item.Platform = truncateString(item.Platform, 32)
	item.Model = truncateString(item.Model, 128)
	item.Protocol = truncateString(item.Protocol, 32)
	item.Transport = truncateString(item.Transport, 32)
	item.ErrorOwner = truncateString(item.ErrorOwner, 32)
	item.ExclusionReason = truncateString(item.ExclusionReason, 64)

	if item.IdempotencyKey == "" || len(item.IdempotencyKey) > 180 {
		return nil, fmt.Errorf("reliability observation idempotency key is invalid")
	}
	if item.Source == "" || len(item.Source) > 64 || item.SourceID == "" || len(item.SourceID) > 180 {
		return nil, fmt.Errorf("reliability observation source is invalid")
	}
	switch item.FactType {
	case ReliabilityFactCustomerRequest, ReliabilityFactUpstreamAttempt, ReliabilityFactActiveProbe:
	default:
		return nil, fmt.Errorf("reliability observation fact type is invalid")
	}
	switch item.Outcome {
	case ReliabilityOutcomeSuccess, ReliabilityOutcomeFailure, ReliabilityOutcomeRecovered, ReliabilityOutcomeExcluded:
	default:
		return nil, fmt.Errorf("reliability observation outcome is invalid")
	}
	if item.RequestClass == "" {
		item.RequestClass = ReliabilityRequestClassOther
	}
	switch item.RequestClass {
	case ReliabilityRequestClassText, ReliabilityRequestClassImage, ReliabilityRequestClassVideo, ReliabilityRequestClassOther:
	default:
		return nil, fmt.Errorf("reliability observation request class is invalid")
	}
	if item.RouteFingerprint != "" && !reliabilityRouteFingerprintPattern.MatchString(item.RouteFingerprint) {
		return nil, fmt.Errorf("reliability observation route fingerprint is invalid")
	}
	if item.RoutingFingerprint != "" && !reliabilityRouteFingerprintPattern.MatchString(item.RoutingFingerprint) {
		return nil, fmt.Errorf("reliability observation routing fingerprint is invalid")
	}
	if item.AccessGroupID < 0 {
		return nil, fmt.Errorf("reliability observation access group is invalid")
	}
	if item.EndpointHash != "" && !reliabilityEndpointHashPattern.MatchString(item.EndpointHash) {
		return nil, fmt.Errorf("reliability observation endpoint hash is invalid")
	}
	if item.LatencyMs < 0 {
		return nil, fmt.Errorf("reliability observation latency is invalid")
	}
	if item.StatusCode != nil && (*item.StatusCode < 100 || *item.StatusCode > 599) {
		return nil, fmt.Errorf("reliability observation status code is invalid")
	}
	if item.CustomerImpact && (item.FactType != ReliabilityFactCustomerRequest || item.Outcome != ReliabilityOutcomeFailure) {
		return nil, fmt.Errorf("customer impact requires a failed customer request")
	}
	if item.CustomerImpact && item.ErrorOwner != "provider" && item.ErrorOwner != "platform" {
		return nil, fmt.Errorf("customer impact requires provider or platform ownership")
	}
	if item.ObservedAt.IsZero() {
		item.ObservedAt = time.Now()
	}
	return &item, nil
}
