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
	Ready         bool      `json:"ready"`
	Submitted     uint64    `json:"submitted"`
	Written       uint64    `json:"written"`
	Failed        uint64    `json:"failed"`
	Dropped       uint64    `json:"dropped"`
	Rejected      uint64    `json:"rejected"`
	InFlight      uint64    `json:"in_flight"`
	Waiting       uint64    `json:"waiting"`
	Running       int64     `json:"running"`
	Completeness  float64   `json:"completeness"`
	StorageChecks uint64    `json:"storage_checks"`
	StorageFailed uint64    `json:"storage_failed"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastFailureAt time.Time `json:"last_failure_at,omitempty"`
	LastError     string    `json:"last_error"`
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

	lastSuccessNS atomic.Int64
	lastFailureNS atomic.Int64
	lastError     atomic.Value
	stopOnce      sync.Once
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
	return c.submit(func(ctx context.Context) error {
		if err := c.store.Record(ctx, observation); err != nil {
			return err
		}
		if c.outcomeRecorder != nil {
			return c.outcomeRecorder.RecordOpenAIRouteOutcome(ctx, observation)
		}
		return nil
	}, observation.Key)
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
	return c.submit(func(ctx context.Context) error { return c.store.RecordCost(ctx, observation) }, observation.Key)
}

func (c *OpenAIRouteObservationCollector) submit(write func(context.Context) error, key OpenAIRouteKey) bool {
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
	})
	if !ok {
		c.dropped.Add(1)
		c.recordFailure(errOpenAIRouteObservationQueueFull)
	}
	return ok
}

func (c *OpenAIRouteObservationCollector) Stats() OpenAIRouteObservationCollectorStats {
	if c == nil {
		return OpenAIRouteObservationCollectorStats{}
	}
	stats := OpenAIRouteObservationCollectorStats{
		Submitted:     c.submitted.Load(),
		Written:       c.written.Load(),
		Failed:        c.failed.Load(),
		Dropped:       c.dropped.Load(),
		Rejected:      c.rejected.Load(),
		StorageChecks: c.checks.Load(),
		StorageFailed: c.checkFail.Load(),
	}
	terminal := stats.Written + stats.Failed + stats.Dropped
	if stats.Submitted > terminal {
		stats.InFlight = stats.Submitted - terminal
	}
	totalEvidence := terminal + stats.Rejected
	stats.Completeness = 1
	if totalEvidence > 0 {
		stats.Completeness = float64(stats.Written) / float64(totalEvidence)
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
	storageHealthy := !stats.LastSuccessAt.IsZero() && (stats.LastFailureAt.IsZero() || stats.LastSuccessAt.After(stats.LastFailureAt))
	stats.Ready = c.store != nil && storageHealthy && stats.Completeness >= 0.99
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

func (c *OpenAIRouteObservationCollector) Stop() {
	if c == nil || c.pool == nil {
		return
	}
	c.stopOnce.Do(func() { c.pool.StopAndWait() })
}
