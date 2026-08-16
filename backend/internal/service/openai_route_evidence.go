package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	OpenAIRouteEvidenceComponentAudit       = "audit"
	OpenAIRouteEvidenceComponentObservation = "observation"

	openAIRouteEvidenceHeartbeatInterval = 15 * time.Second
	openAIRouteEvidenceMaximumGap        = 90 * time.Second
	openAIRouteEvidenceWriteTimeout      = 2 * time.Second
)

// OpenAIRouteEvidenceCounters is a non-sensitive cumulative snapshot for one
// process epoch. Counts only increase inside an epoch. A clean shutdown seals
// the final values, allowing a later process to continue the same experiment
// without pretending that its counters started at zero.
type OpenAIRouteEvidenceCounters struct {
	Attempted       uint64
	Written         uint64
	Failed          uint64
	Dropped         uint64
	Rejected        uint64
	OutcomeExpected uint64
	OutcomeApplied  uint64
	OutcomeFailed   uint64
	StorageChecks   uint64
	StorageFailed   uint64
	LastError       string
}

type OpenAIRouteEvidenceEpoch struct {
	EpochID       string
	InstanceID    string
	Component     string
	StartedAt     time.Time
	HeartbeatAt   time.Time
	StoppedAt     time.Time
	CleanShutdown bool
	Counters      OpenAIRouteEvidenceCounters
}

// OpenAIRouteEvidenceEpochStore is optional so focused unit tests and older
// adapters remain source-compatible. Production PostgreSQL implementations
// provide it for both the audit writer and observation collector.
type OpenAIRouteEvidenceEpochStore interface {
	BeginOpenAIRouteEvidenceEpoch(ctx context.Context, epoch OpenAIRouteEvidenceEpoch) error
	CheckpointOpenAIRouteEvidenceEpoch(ctx context.Context, epoch OpenAIRouteEvidenceEpoch) error
	ListOpenAIRouteEvidenceEpochs(ctx context.Context, start, end time.Time) ([]OpenAIRouteEvidenceEpoch, error)
}

type OpenAIRouteEvidenceComponentHealth struct {
	Component           string    `json:"component"`
	Ready               bool      `json:"ready"`
	Epochs              int       `json:"epochs"`
	CounterStartedAt    time.Time `json:"counter_started_at,omitempty"`
	LastCoveredAt       time.Time `json:"last_covered_at,omitempty"`
	MaximumGapSeconds   float64   `json:"maximum_gap_seconds"`
	UncleanEpochs       int       `json:"unclean_epochs"`
	Attempted           uint64    `json:"attempted"`
	Written             uint64    `json:"written"`
	Failed              uint64    `json:"failed"`
	Dropped             uint64    `json:"dropped"`
	Rejected            uint64    `json:"rejected"`
	InFlight            uint64    `json:"in_flight"`
	Completeness        float64   `json:"completeness"`
	OutcomeExpected     uint64    `json:"outcome_expected"`
	OutcomeApplied      uint64    `json:"outcome_applied"`
	OutcomeFailed       uint64    `json:"outcome_failed"`
	OutcomeInFlight     uint64    `json:"outcome_in_flight"`
	OutcomeCompleteness float64   `json:"outcome_completeness"`
	StorageChecks       uint64    `json:"storage_checks"`
	StorageFailed       uint64    `json:"storage_failed"`
}

type OpenAIRouteEvidenceWindowHealth struct {
	Available      bool                               `json:"available"`
	Ready          bool                               `json:"ready"`
	Scope          string                             `json:"scope"`
	WindowStart    time.Time                          `json:"window_start"`
	WindowEnd      time.Time                          `json:"window_end"`
	MaximumGapSecs float64                            `json:"maximum_gap_seconds"`
	Audit          OpenAIRouteEvidenceComponentHealth `json:"audit"`
	Observation    OpenAIRouteEvidenceComponentHealth `json:"observation"`
}

// BuildOpenAIRouteEvidenceWindowHealth deliberately treats an unclean stale
// epoch as evidence loss even when the next process starts quickly. Graceful
// releases are continuous; crashes remain visible and cannot be hidden by a
// fresh process with perfect counters.
func BuildOpenAIRouteEvidenceWindowHealth(
	start time.Time,
	end time.Time,
	epochs []OpenAIRouteEvidenceEpoch,
) OpenAIRouteEvidenceWindowHealth {
	start = start.UTC()
	end = end.UTC()
	window := OpenAIRouteEvidenceWindowHealth{
		Scope:          "durable_process_epochs_overlap_conservative",
		WindowStart:    start,
		WindowEnd:      end,
		MaximumGapSecs: openAIRouteEvidenceMaximumGap.Seconds(),
	}
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return window
	}
	window.Audit = buildOpenAIRouteEvidenceComponentHealth(OpenAIRouteEvidenceComponentAudit, start, end, epochs)
	window.Observation = buildOpenAIRouteEvidenceComponentHealth(OpenAIRouteEvidenceComponentObservation, start, end, epochs)
	window.Available = window.Audit.Epochs > 0 && window.Observation.Epochs > 0
	window.Ready = window.Available && window.Audit.Ready && window.Observation.Ready
	return window
}

func buildOpenAIRouteEvidenceComponentHealth(
	component string,
	start time.Time,
	end time.Time,
	epochs []OpenAIRouteEvidenceEpoch,
) OpenAIRouteEvidenceComponentHealth {
	selected := make([]OpenAIRouteEvidenceEpoch, 0, len(epochs))
	for _, epoch := range epochs {
		if epoch.Component != component || epoch.StartedAt.IsZero() || epoch.HeartbeatAt.IsZero() {
			continue
		}
		coverageEnd := epoch.HeartbeatAt.UTC()
		if epoch.CleanShutdown && !epoch.StoppedAt.IsZero() {
			coverageEnd = epoch.StoppedAt.UTC()
		}
		if epoch.StartedAt.UTC().Before(end) && coverageEnd.After(start) {
			selected = append(selected, epoch)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].StartedAt.Equal(selected[j].StartedAt) {
			return selected[i].EpochID < selected[j].EpochID
		}
		return selected[i].StartedAt.Before(selected[j].StartedAt)
	})

	health := OpenAIRouteEvidenceComponentHealth{
		Component:           component,
		Epochs:              len(selected),
		Completeness:        1,
		OutcomeCompleteness: 1,
	}
	if len(selected) == 0 {
		health.Ready = false
		return health
	}

	cursor := start
	var maximumGap time.Duration
	for _, epoch := range selected {
		epochStart := epoch.StartedAt.UTC()
		coverageEnd := epoch.HeartbeatAt.UTC()
		if epoch.CleanShutdown && !epoch.StoppedAt.IsZero() {
			coverageEnd = epoch.StoppedAt.UTC()
		}
		if health.CounterStartedAt.IsZero() || epochStart.Before(health.CounterStartedAt) {
			health.CounterStartedAt = epochStart
		}
		if epochStart.After(cursor) {
			gap := epochStart.Sub(cursor)
			if gap > maximumGap {
				maximumGap = gap
			}
		}
		if coverageEnd.After(cursor) {
			cursor = coverageEnd
		}
		if !epoch.CleanShutdown && end.Sub(coverageEnd) > openAIRouteEvidenceMaximumGap {
			health.UncleanEpochs++
		}
		mergeOpenAIRouteEvidenceCounters(&health, epoch.Counters)
	}
	if cursor.Before(end) {
		gap := end.Sub(cursor)
		if gap > maximumGap {
			maximumGap = gap
		}
	}
	health.LastCoveredAt = cursor
	health.MaximumGapSeconds = maximumGap.Seconds()
	terminal := health.Written + health.Failed
	if component == OpenAIRouteEvidenceComponentObservation {
		terminal += health.Dropped
	}
	if health.Attempted > terminal {
		health.InFlight = health.Attempted - terminal
	}
	denominator := health.Attempted
	if component == OpenAIRouteEvidenceComponentObservation {
		denominator += health.Rejected
	}
	if denominator > 0 {
		health.Completeness = float64(health.Written) / float64(denominator)
	}
	outcomeTerminal := health.OutcomeApplied + health.OutcomeFailed
	if health.OutcomeExpected > outcomeTerminal {
		health.OutcomeInFlight = health.OutcomeExpected - outcomeTerminal
	}
	if health.OutcomeExpected > 0 {
		health.OutcomeCompleteness = float64(health.OutcomeApplied) / float64(health.OutcomeExpected)
	}
	continuous := maximumGap <= openAIRouteEvidenceMaximumGap && !health.CounterStartedAt.After(start)
	noLoss := health.Failed == 0 && health.Dropped == 0 && health.Rejected == 0 &&
		health.StorageFailed == 0 && health.OutcomeFailed == 0
	health.Ready = continuous && health.UncleanEpochs == 0 && noLoss &&
		health.InFlight == 0 && health.OutcomeInFlight == 0 &&
		health.Completeness >= openAIRoutePromotionMinimumCompleteness &&
		health.OutcomeCompleteness >= openAIRoutePromotionMinimumCompleteness
	return health
}

func mergeOpenAIRouteEvidenceCounters(target *OpenAIRouteEvidenceComponentHealth, source OpenAIRouteEvidenceCounters) {
	if target == nil {
		return
	}
	target.Attempted += source.Attempted
	target.Written += source.Written
	target.Failed += source.Failed
	target.Dropped += source.Dropped
	target.Rejected += source.Rejected
	target.OutcomeExpected += source.OutcomeExpected
	target.OutcomeApplied += source.OutcomeApplied
	target.OutcomeFailed += source.OutcomeFailed
	target.StorageChecks += source.StorageChecks
	target.StorageFailed += source.StorageFailed
}

type openAIRouteEvidenceTracker struct {
	store      OpenAIRouteEvidenceEpochStore
	component  string
	instanceID string
	epochID    string
	startedAt  time.Time
	counters   func() OpenAIRouteEvidenceCounters

	mu        sync.Mutex
	started   bool
	stopped   bool
	stopCh    chan struct{}
	doneCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

func newOpenAIRouteEvidenceTracker(
	store OpenAIRouteEvidenceEpochStore,
	component string,
	counters func() OpenAIRouteEvidenceCounters,
) *openAIRouteEvidenceTracker {
	instanceID := uuid.NewString()
	return &openAIRouteEvidenceTracker{
		store:      store,
		component:  component,
		instanceID: instanceID,
		epochID:    "route-evidence:" + component + ":" + instanceID,
		startedAt:  time.Now().UTC(),
		counters:   counters,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

func (t *openAIRouteEvidenceTracker) Start() {
	if t == nil || t.store == nil {
		return
	}
	t.startOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), openAIRouteEvidenceWriteTimeout)
		_ = t.flush(ctx, false)
		cancel()
		go t.run()
	})
}

func (t *openAIRouteEvidenceTracker) run() {
	defer close(t.doneCh)
	ticker := time.NewTicker(openAIRouteEvidenceHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), openAIRouteEvidenceWriteTimeout)
			_ = t.flush(ctx, false)
			cancel()
		case <-t.stopCh:
			return
		}
	}
}

func (t *openAIRouteEvidenceTracker) Flush(ctx context.Context) error {
	if t == nil || t.store == nil {
		return errors.New("OpenAI route durable evidence store unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return t.flush(ctx, false)
}

func (t *openAIRouteEvidenceTracker) Stop() {
	if t == nil || t.store == nil {
		return
	}
	t.stopOnce.Do(func() {
		close(t.stopCh)
		select {
		case <-t.doneCh:
		case <-time.After(openAIRouteEvidenceWriteTimeout):
		}
		ctx, cancel := context.WithTimeout(context.Background(), openAIRouteEvidenceWriteTimeout)
		_ = t.flush(ctx, true)
		cancel()
	})
}

func (t *openAIRouteEvidenceTracker) flush(ctx context.Context, cleanShutdown bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return nil
	}
	now := time.Now().UTC()
	epoch := OpenAIRouteEvidenceEpoch{
		EpochID:       t.epochID,
		InstanceID:    t.instanceID,
		Component:     t.component,
		StartedAt:     t.startedAt,
		HeartbeatAt:   now,
		CleanShutdown: cleanShutdown,
	}
	if t.counters != nil {
		epoch.Counters = t.counters()
		epoch.Counters.LastError = truncateOpenAIRouteEvidenceError(epoch.Counters.LastError)
	}
	if cleanShutdown {
		epoch.StoppedAt = now
	}
	if !t.started {
		if err := t.store.BeginOpenAIRouteEvidenceEpoch(ctx, epoch); err != nil {
			return err
		}
		t.started = true
	}
	if err := t.store.CheckpointOpenAIRouteEvidenceEpoch(ctx, epoch); err != nil {
		return err
	}
	if cleanShutdown {
		t.stopped = true
	}
	return nil
}

func truncateOpenAIRouteEvidenceError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		return value[:512]
	}
	return value
}
