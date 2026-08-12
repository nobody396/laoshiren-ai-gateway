package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	// OpenAIRouteObservationRecentWindow is deliberately short enough to react
	// to a regional/Base-URL incident without letting one transient request
	// erase the longer reliability history.
	OpenAIRouteObservationRecentWindow  = time.Hour
	OpenAIRouteObservationGlobalWindow  = 7 * 24 * time.Hour
	OpenAIRouteObservationSeasonalWeeks = 8
)

// OpenAIRouteLatencyHistogramUpperBoundsMS is shared by the service and Redis
// repository so observations remain mergeable across instances and releases.
// The final bucket is represented by MaxInt64.
var OpenAIRouteLatencyHistogramUpperBoundsMS = [...]int64{
	100, 250, 500, 1_000, 2_000, 3_000, 5_000, 8_000,
	12_000, 20_000, 30_000, 60_000, math.MaxInt64,
}

// OpenAIRouteObservation is one completed upstream attempt, not one end-user
// request. Covered failovers therefore remain visible to the learner.
type OpenAIRouteObservation struct {
	Key        OpenAIRouteKey
	ObservedAt time.Time

	Success       bool
	FailureClass  OpenAIRouteFailureClass
	PenalizeRoute bool
	PartialStream bool

	TTFTMilliseconds        int64
	CompletionLatencyMS     int64
	ActualBaseCostUSD       float64
	ActualAccountCostUSD    float64
	ActualCostAuthoritative bool
}

type OpenAIRouteActualCostObservation struct {
	Key                  OpenAIRouteKey
	ObservedAt           time.Time
	ActualBaseCostUSD    float64
	ActualAccountCostUSD float64
}

func (o OpenAIRouteActualCostObservation) Validate() error {
	if !o.Key.Valid() {
		return ErrOpenAIRouteNoCandidate
	}
	if !isFiniteNonNegative(o.ActualBaseCostUSD) || !isFiniteNonNegative(o.ActualAccountCostUSD) {
		return ErrOpenAIRouteInvalidCost
	}
	return nil
}

func (o OpenAIRouteObservation) Validate() error {
	if !o.Key.Valid() {
		return ErrOpenAIRouteNoCandidate
	}
	if o.TTFTMilliseconds < 0 || o.CompletionLatencyMS < 0 {
		return fmt.Errorf("%w: negative route latency", ErrOpenAIRouteInvalidCost)
	}
	if !isFiniteNonNegative(o.ActualBaseCostUSD) || !isFiniteNonNegative(o.ActualAccountCostUSD) {
		return ErrOpenAIRouteInvalidCost
	}
	if o.Success {
		if o.FailureClass != "" && o.FailureClass != OpenAIRouteFailureNone {
			return fmt.Errorf("%w: successful observation has failure class", ErrOpenAIRouteInvalidPolicy)
		}
		return nil
	}
	if o.FailureClass == "" || o.FailureClass == OpenAIRouteFailureNone {
		return fmt.Errorf("%w: failed observation is missing failure class", ErrOpenAIRouteInvalidPolicy)
	}
	return nil
}

func OpenAIRouteLatencyHistogramBucket(milliseconds int64) int {
	if milliseconds <= 0 {
		return -1
	}
	for idx, upper := range OpenAIRouteLatencyHistogramUpperBoundsMS {
		if milliseconds <= upper {
			return idx
		}
	}
	return len(OpenAIRouteLatencyHistogramUpperBoundsMS) - 1
}

// OpenAIRouteObservationAggregate is a mergeable, non-sensitive projection of
// route outcomes. No URL, request body, credential, or user identity is stored.
type OpenAIRouteObservationAggregate struct {
	AttemptCount     uint64
	ReliabilityCount uint64
	SuccessCount     uint64
	FailureCount     uint64
	PartialStreams   uint64

	FailureCounts map[OpenAIRouteFailureClass]uint64

	TTFTSampleCount    uint64
	TTFTHistogram      []uint64
	LatencySampleCount uint64
	LatencyHistogram   []uint64

	ActualCostSamples    uint64
	ActualBaseCostUSD    float64
	ActualAccountCostUSD float64

	LastObservedAt time.Time
}

func NewOpenAIRouteObservationAggregate() OpenAIRouteObservationAggregate {
	return OpenAIRouteObservationAggregate{
		FailureCounts:    make(map[OpenAIRouteFailureClass]uint64),
		TTFTHistogram:    make([]uint64, len(OpenAIRouteLatencyHistogramUpperBoundsMS)),
		LatencyHistogram: make([]uint64, len(OpenAIRouteLatencyHistogramUpperBoundsMS)),
	}
}

func (a *OpenAIRouteObservationAggregate) Merge(other OpenAIRouteObservationAggregate) {
	if a == nil {
		return
	}
	if a.FailureCounts == nil {
		a.FailureCounts = make(map[OpenAIRouteFailureClass]uint64)
	}
	if len(a.TTFTHistogram) != len(OpenAIRouteLatencyHistogramUpperBoundsMS) {
		a.TTFTHistogram = make([]uint64, len(OpenAIRouteLatencyHistogramUpperBoundsMS))
	}
	if len(a.LatencyHistogram) != len(OpenAIRouteLatencyHistogramUpperBoundsMS) {
		a.LatencyHistogram = make([]uint64, len(OpenAIRouteLatencyHistogramUpperBoundsMS))
	}
	a.AttemptCount += other.AttemptCount
	a.ReliabilityCount += other.ReliabilityCount
	a.SuccessCount += other.SuccessCount
	a.FailureCount += other.FailureCount
	a.PartialStreams += other.PartialStreams
	a.TTFTSampleCount += other.TTFTSampleCount
	a.LatencySampleCount += other.LatencySampleCount
	a.ActualCostSamples += other.ActualCostSamples
	a.ActualBaseCostUSD += other.ActualBaseCostUSD
	a.ActualAccountCostUSD += other.ActualAccountCostUSD
	for class, count := range other.FailureCounts {
		a.FailureCounts[class] += count
	}
	for idx, count := range other.TTFTHistogram {
		if idx < len(a.TTFTHistogram) {
			a.TTFTHistogram[idx] += count
		}
	}
	for idx, count := range other.LatencyHistogram {
		if idx < len(a.LatencyHistogram) {
			a.LatencyHistogram[idx] += count
		}
	}
	if other.LastObservedAt.After(a.LastObservedAt) {
		a.LastObservedAt = other.LastObservedAt
	}
}

func (a OpenAIRouteObservationAggregate) SuccessLowerBound() float64 {
	return OpenAIRouteWilsonLowerBound(a.SuccessCount, a.ReliabilityCount, 1.96)
}

func (a OpenAIRouteObservationAggregate) TTFTPercentile(percentile float64) float64 {
	return OpenAIRouteHistogramPercentile(a.TTFTHistogram, a.TTFTSampleCount, percentile)
}

func (a OpenAIRouteObservationAggregate) LatencyPercentile(percentile float64) float64 {
	return OpenAIRouteHistogramPercentile(a.LatencyHistogram, a.LatencySampleCount, percentile)
}

func (a OpenAIRouteObservationAggregate) PartialStreamRate() float64 {
	// ReliabilityCount contains successful and route-penalizing outcomes only.
	// AttemptCount additionally contains neutral user/client outcomes and would
	// let cancellations make a route's stream-integrity score look better.
	if a.ReliabilityCount == 0 || a.PartialStreams == 0 {
		return 0
	}
	rate := float64(a.PartialStreams) / float64(a.ReliabilityCount)
	if rate > 1 {
		return 1
	}
	return rate
}

func OpenAIRouteHistogramPercentile(histogram []uint64, sampleCount uint64, percentile float64) float64 {
	if sampleCount == 0 || len(histogram) == 0 {
		return 0
	}
	if percentile <= 0 || math.IsNaN(percentile) {
		percentile = 0.5
	}
	if percentile > 1 {
		percentile = 1
	}
	target := uint64(math.Ceil(percentile * float64(sampleCount)))
	if target == 0 {
		target = 1
	}
	var cumulative uint64
	for idx, count := range histogram {
		cumulative += count
		if cumulative >= target {
			if idx >= len(OpenAIRouteLatencyHistogramUpperBoundsMS) {
				idx = len(OpenAIRouteLatencyHistogramUpperBoundsMS) - 1
			}
			upper := OpenAIRouteLatencyHistogramUpperBoundsMS[idx]
			if upper == math.MaxInt64 {
				// Keep an outlier finite so it cannot poison scoring or JSON.
				return float64(OpenAIRouteLatencyHistogramUpperBoundsMS[len(OpenAIRouteLatencyHistogramUpperBoundsMS)-2] * 2)
			}
			return float64(upper)
		}
	}
	return 0
}

type OpenAIRouteObservationProfile struct {
	Global     OpenAIRouteObservationAggregate
	Recent     OpenAIRouteObservationAggregate
	HourOfWeek OpenAIRouteObservationAggregate
}

// OpenAIRouteObservationStore is the cross-instance learner boundary. Batch
// reads are required because Shadow evaluation has a strict hot-path deadline.
type OpenAIRouteObservationStore interface {
	Check(ctx context.Context) error
	Record(ctx context.Context, observation OpenAIRouteObservation) error
	RecordCost(ctx context.Context, observation OpenAIRouteActualCostObservation) error
	GetBatch(ctx context.Context, keys []OpenAIRouteKey, now time.Time) (map[string]OpenAIRouteObservationProfile, error)
}

type OpenAIRouteOutcomeRecorder interface {
	RecordOpenAIRouteOutcome(ctx context.Context, observation OpenAIRouteObservation) error
}

func OpenAIRouteObservationFingerprint(key OpenAIRouteKey) string {
	return OpenAIRouteHealthStoreKeyForRoute(key).Fingerprint()
}

// BlendOpenAIRouteObservationProfile combines long-term, recent and Beijing
// hour-of-week evidence. Sparse seasonal data cannot override the global view;
// recent evidence ramps in gradually rather than causing a one-sample switch.
func BlendOpenAIRouteObservationProfile(profile OpenAIRouteObservationProfile) OpenAIRouteObservationAggregate {
	global := profile.Global
	if global.ReliabilityCount == 0 {
		return global
	}

	weighted := []struct {
		aggregate OpenAIRouteObservationAggregate
		weight    float64
	}{
		{aggregate: global, weight: 1},
	}
	if profile.Recent.ReliabilityCount > 0 {
		confidence := math.Min(1, float64(profile.Recent.ReliabilityCount)/20)
		weighted = append(weighted, struct {
			aggregate OpenAIRouteObservationAggregate
			weight    float64
		}{aggregate: profile.Recent, weight: 1.5 * confidence})
	}
	if profile.HourOfWeek.ReliabilityCount >= 20 {
		confidence := math.Min(1, float64(profile.HourOfWeek.ReliabilityCount)/80)
		weighted = append(weighted, struct {
			aggregate OpenAIRouteObservationAggregate
			weight    float64
		}{aggregate: profile.HourOfWeek, weight: 0.75 * confidence})
	}

	// Produce a synthetic aggregate whose Wilson and percentile projections are
	// deterministic. Count weights are rounded only after all weights are known.
	result := NewOpenAIRouteObservationAggregate()
	var totalWeight float64
	for _, item := range weighted {
		totalWeight += item.weight
	}
	for _, item := range weighted {
		if item.weight <= 0 || totalWeight <= 0 {
			continue
		}
		scale := item.weight / totalWeight
		mergeScaledOpenAIRouteObservationAggregate(&result, item.aggregate, scale)
	}
	result.LastObservedAt = newestOpenAIRouteObservationTime(profile.Global.LastObservedAt, profile.Recent.LastObservedAt, profile.HourOfWeek.LastObservedAt)
	return result
}

func mergeScaledOpenAIRouteObservationAggregate(target *OpenAIRouteObservationAggregate, source OpenAIRouteObservationAggregate, scale float64) {
	if target == nil || scale <= 0 {
		return
	}
	scaled := NewOpenAIRouteObservationAggregate()
	scaled.AttemptCount = uint64(math.Round(float64(source.AttemptCount) * scale))
	scaled.ReliabilityCount = uint64(math.Round(float64(source.ReliabilityCount) * scale))
	scaled.SuccessCount = uint64(math.Round(float64(source.SuccessCount) * scale))
	if scaled.SuccessCount > scaled.ReliabilityCount {
		scaled.SuccessCount = scaled.ReliabilityCount
	}
	scaled.FailureCount = uint64(math.Round(float64(source.FailureCount) * scale))
	scaled.PartialStreams = uint64(math.Round(float64(source.PartialStreams) * scale))
	scaled.TTFTSampleCount = uint64(math.Round(float64(source.TTFTSampleCount) * scale))
	scaled.LatencySampleCount = uint64(math.Round(float64(source.LatencySampleCount) * scale))
	scaled.ActualCostSamples = uint64(math.Round(float64(source.ActualCostSamples) * scale))
	scaled.ActualBaseCostUSD = source.ActualBaseCostUSD * scale
	scaled.ActualAccountCostUSD = source.ActualAccountCostUSD * scale
	for class, count := range source.FailureCounts {
		scaled.FailureCounts[class] = uint64(math.Round(float64(count) * scale))
	}
	for idx, count := range source.TTFTHistogram {
		if idx < len(scaled.TTFTHistogram) {
			scaled.TTFTHistogram[idx] = uint64(math.Round(float64(count) * scale))
		}
	}
	for idx, count := range source.LatencyHistogram {
		if idx < len(scaled.LatencyHistogram) {
			scaled.LatencyHistogram[idx] = uint64(math.Round(float64(count) * scale))
		}
	}
	// Histogram rounding may differ from the rounded sample count. Derive the
	// count from buckets so percentile rank is always internally consistent.
	scaled.TTFTSampleCount = sumOpenAIRouteHistogram(scaled.TTFTHistogram)
	scaled.LatencySampleCount = sumOpenAIRouteHistogram(scaled.LatencyHistogram)
	target.Merge(scaled)
}

func sumOpenAIRouteHistogram(values []uint64) uint64 {
	var total uint64
	for _, value := range values {
		total += value
	}
	return total
}

func newestOpenAIRouteObservationTime(values ...time.Time) time.Time {
	sort.Slice(values, func(i, j int) bool { return values[i].After(values[j]) })
	if len(values) == 0 {
		return time.Time{}
	}
	return values[0]
}
