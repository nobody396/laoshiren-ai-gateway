package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultOpenAIRouteObservationWorkers = 8
	defaultOpenAIRouteObservationQueue   = 4096
	openAIRouteObservationWriteTimeout   = 2 * time.Second
)

var errOpenAIRouteObservationQueueFull = errors.New("OpenAI route observation queue full")

type OpenAIRouteObservationCollectorStats struct {
	Ready                bool      `json:"ready"`
	Submitted            uint64    `json:"submitted"`
	Written              uint64    `json:"written"`
	Failed               uint64    `json:"failed"`
	Dropped              uint64    `json:"dropped"`
	Rejected             uint64    `json:"rejected"`
	InFlight             uint64    `json:"in_flight"`
	Waiting              uint64    `json:"waiting"`
	Running              int64     `json:"running"`
	Completeness         float64   `json:"completeness"`
	StorageChecks        uint64    `json:"storage_checks"`
	StorageFailed        uint64    `json:"storage_failed"`
	LastSuccessAt        time.Time `json:"last_success_at,omitempty"`
	LastFailureAt        time.Time `json:"last_failure_at,omitempty"`
	LastError            string    `json:"last_error"`
	OutcomeApplied       uint64    `json:"outcome_applied"`
	OutcomeFailed        uint64    `json:"outcome_failed"`
	OutcomeInFlight      uint64    `json:"outcome_in_flight"`
	OutcomeCompleteness  float64   `json:"outcome_completeness"`
	OutcomeLastSuccessAt time.Time `json:"outcome_last_success_at,omitempty"`
	OutcomeLastFailureAt time.Time `json:"outcome_last_failure_at,omitempty"`
	OutcomeLastError     string    `json:"outcome_last_error"`
}

// OpenAIRouteObservationCollector keeps Redis observation writes off the user
// hot path. Overflow drops learner evidence rather than delaying a customer;
// the drop counter makes that evidence loss explicit at the readiness gate.
type OpenAIRouteObservationCollector struct {
	store           OpenAIRouteObservationStore
	outcomeRecorder OpenAIRouteOutcomeRecorder
	pool            pond.Pool

	submitted atomic.Uint64
	written   atomic.Uint64
	failed    atomic.Uint64
	dropped   atomic.Uint64
	rejected  atomic.Uint64
	checks    atomic.Uint64
	checkFail atomic.Uint64

	lastSuccessNS        atomic.Int64
	lastFailureNS        atomic.Int64
	lastError            atomic.Value
	outcomeApplied       atomic.Uint64
	outcomeFailed        atomic.Uint64
	outcomeExpected      atomic.Uint64
	outcomeLastSuccessNS atomic.Int64
	outcomeLastFailureNS atomic.Int64
	outcomeLastError     atomic.Value
	stopOnce             sync.Once
}

func NewOpenAIRouteObservationCollector(store OpenAIRouteObservationStore, recorder OpenAIRouteOutcomeRecorder) *OpenAIRouteObservationCollector {
	return NewOpenAIRouteObservationCollectorWithOptions(store, defaultOpenAIRouteObservationWorkers, defaultOpenAIRouteObservationQueue, recorder)
}

func NewOpenAIRouteObservationCollectorWithOptions(store OpenAIRouteObservationStore, workers, queue int, recorders ...OpenAIRouteOutcomeRecorder) *OpenAIRouteObservationCollector {
	if workers <= 0 {
		workers = defaultOpenAIRouteObservationWorkers
	}
	if queue <= 0 {
		queue = defaultOpenAIRouteObservationQueue
	}
	collector := &OpenAIRouteObservationCollector{
		store: store,
		pool:  pond.NewPool(workers, pond.WithQueueSize(queue)),
	}
	collector.lastError.Store("")
	collector.outcomeLastError.Store("")
	if len(recorders) > 0 {
		collector.outcomeRecorder = recorders[0]
	}
	return collector
}

func (c *OpenAIRouteObservationCollector) Start() {
	if c == nil || c.store == nil {
		return
	}
	c.checks.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), openAIRouteObservationWriteTimeout)
	defer cancel()
	if err := c.store.Check(ctx); err != nil {
		c.checkFail.Add(1)
		c.recordFailure(err)
		logger.L().Warn("openai.route_observation_storage_check_failed",
			zap.String("component", "routing.observation"),
			zap.Error(err),
		)
		return
	}
	c.recordSuccess()
}

func (c *OpenAIRouteObservationCollector) TryRecord(observation OpenAIRouteObservation) bool {
	if c == nil || c.store == nil || c.pool == nil || c.pool.Stopped() {
		return false
	}
	if err := observation.Validate(); err != nil {
		c.rejected.Add(1)
		c.recordFailure(err)
		return false
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	}
	var applyOutcome func(context.Context) error
	if c.outcomeRecorder != nil {
		c.outcomeExpected.Add(1)
		applyOutcome = func(ctx context.Context) error {
			return c.outcomeRecorder.RecordOpenAIRouteOutcome(ctx, observation)
		}
	}
	return c.submit(
		func(ctx context.Context) error { return c.store.Record(ctx, observation) },
		applyOutcome,
		observation.Key,
	)
}

func (c *OpenAIRouteObservationCollector) TryRecordCost(observation OpenAIRouteActualCostObservation) bool {
	if c == nil || c.store == nil || c.pool == nil || c.pool.Stopped() {
		return false
	}
	if err := observation.Validate(); err != nil {
		c.rejected.Add(1)
		c.recordFailure(err)
		return false
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	}
	return c.submit(func(ctx context.Context) error { return c.store.RecordCost(ctx, observation) }, nil, observation.Key)
}

func (c *OpenAIRouteObservationCollector) submit(write func(context.Context) error, applyOutcome func(context.Context) error, key OpenAIRouteKey) bool {
	if c == nil || c.pool == nil || write == nil {
		return false
	}
	c.submitted.Add(1)
	_, ok := c.pool.TrySubmit(func() {
		ctx, cancel := context.WithTimeout(context.Background(), openAIRouteObservationWriteTimeout)
		defer cancel()
		if err := write(ctx); err != nil {
			c.failed.Add(1)
			c.recordFailure(err)
			if applyOutcome != nil {
				c.outcomeFailed.Add(1)
				c.recordOutcomeFailure(err)
			}
			logger.L().Warn("openai.route_observation_write_failed",
				zap.String("component", "routing.observation"),
				zap.Int64("account_id", key.AccountID),
				zap.String("route_fingerprint", OpenAIRouteObservationFingerprint(key)),
				zap.Error(err),
			)
			return
		}
		c.written.Add(1)
		c.recordSuccess()
		if applyOutcome == nil {
			return
		}
		outcomeCtx, outcomeCancel := context.WithTimeout(context.Background(), openAIRouteObservationWriteTimeout)
		defer outcomeCancel()
		if err := applyOutcome(outcomeCtx); err != nil {
			c.outcomeFailed.Add(1)
			c.recordOutcomeFailure(err)
			logger.L().Warn("openai.route_outcome_apply_failed",
				zap.String("component", "routing.health"),
				zap.Int64("account_id", key.AccountID),
				zap.String("route_fingerprint", OpenAIRouteObservationFingerprint(key)),
				zap.Error(err),
			)
			return
		}
		c.outcomeApplied.Add(1)
		c.recordOutcomeSuccess()
	})
	if !ok {
		c.dropped.Add(1)
		c.recordFailure(errOpenAIRouteObservationQueueFull)
		if applyOutcome != nil {
			c.outcomeFailed.Add(1)
			c.recordOutcomeFailure(errOpenAIRouteObservationQueueFull)
		}
	}
	return ok
}

func (c *OpenAIRouteObservationCollector) Stats() OpenAIRouteObservationCollectorStats {
	if c == nil {
		return OpenAIRouteObservationCollectorStats{}
	}
	stats := OpenAIRouteObservationCollectorStats{
		Submitted:      c.submitted.Load(),
		Written:        c.written.Load(),
		Failed:         c.failed.Load(),
		Dropped:        c.dropped.Load(),
		Rejected:       c.rejected.Load(),
		StorageChecks:  c.checks.Load(),
		StorageFailed:  c.checkFail.Load(),
		OutcomeApplied: c.outcomeApplied.Load(),
		OutcomeFailed:  c.outcomeFailed.Load(),
	}
	terminal := stats.Written + stats.Failed + stats.Dropped
	if stats.Submitted > terminal {
		stats.InFlight = stats.Submitted - terminal
	}
	totalEvidence := stats.Submitted + stats.Rejected
	stats.Completeness = 1
	if totalEvidence > 0 {
		stats.Completeness = float64(stats.Written) / float64(totalEvidence)
	}
	outcomeExpected := c.outcomeExpected.Load()
	outcomeTerminal := stats.OutcomeApplied + stats.OutcomeFailed
	if outcomeExpected > outcomeTerminal {
		stats.OutcomeInFlight = outcomeExpected - outcomeTerminal
	}
	stats.OutcomeCompleteness = 1
	if outcomeExpected > 0 {
		stats.OutcomeCompleteness = float64(stats.OutcomeApplied) / float64(outcomeExpected)
	}
	if ns := c.lastSuccessNS.Load(); ns > 0 {
		stats.LastSuccessAt = time.Unix(0, ns).UTC()
	}
	if ns := c.lastFailureNS.Load(); ns > 0 {
		stats.LastFailureAt = time.Unix(0, ns).UTC()
	}
	if value, ok := c.lastError.Load().(string); ok {
		stats.LastError = strings.TrimSpace(value)
	}
	if ns := c.outcomeLastSuccessNS.Load(); ns > 0 {
		stats.OutcomeLastSuccessAt = time.Unix(0, ns).UTC()
	}
	if ns := c.outcomeLastFailureNS.Load(); ns > 0 {
		stats.OutcomeLastFailureAt = time.Unix(0, ns).UTC()
	}
	if value, ok := c.outcomeLastError.Load().(string); ok {
		stats.OutcomeLastError = strings.TrimSpace(value)
	}
	storageHealthy := !stats.LastSuccessAt.IsZero() && (stats.LastFailureAt.IsZero() || stats.LastSuccessAt.After(stats.LastFailureAt))
	outcomeHealthy := stats.OutcomeLastFailureAt.IsZero() || stats.OutcomeLastSuccessAt.After(stats.OutcomeLastFailureAt)
	stats.Ready = c.store != nil && storageHealthy && outcomeHealthy && stats.Completeness >= 0.99 && stats.OutcomeCompleteness >= 0.99
	if c.pool != nil {
		stats.Waiting = c.pool.WaitingTasks()
		stats.Running = c.pool.RunningWorkers()
	}
	return stats
}

func (c *OpenAIRouteObservationCollector) recordFailure(err error) {
	if c == nil {
		return
	}
	c.lastFailureNS.Store(time.Now().UTC().UnixNano())
	if err != nil {
		c.lastError.Store(err.Error())
	}
}

func (c *OpenAIRouteObservationCollector) recordSuccess() {
	if c == nil {
		return
	}
	c.lastSuccessNS.Store(time.Now().UTC().UnixNano())
	c.lastError.Store("")
}

func (c *OpenAIRouteObservationCollector) recordOutcomeFailure(err error) {
	if c == nil {
		return
	}
	c.outcomeLastFailureNS.Store(time.Now().UTC().UnixNano())
	if err != nil {
		c.outcomeLastError.Store(err.Error())
	}
}

func (c *OpenAIRouteObservationCollector) recordOutcomeSuccess() {
	if c == nil {
		return
	}
	c.outcomeLastSuccessNS.Store(time.Now().UTC().UnixNano())
	c.outcomeLastError.Store("")
}

func (c *OpenAIRouteObservationCollector) Stop() {
	if c == nil || c.pool == nil {
		return
	}
	c.stopOnce.Do(func() { c.pool.StopAndWait() })
}
