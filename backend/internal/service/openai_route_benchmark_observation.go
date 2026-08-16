package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	// Active probes are a weak prior, never a substitute for real passive
	// outcomes. The global window deliberately excludes the stale legacy sweep
	// once it is older than three days.
	OpenAIRouteBenchmarkGlobalWindow    = 72 * time.Hour
	OpenAIRouteBenchmarkRecentWindow    = 6 * time.Hour
	OpenAIRouteBenchmarkSeasonalWeeks   = 4
	OpenAIRouteBenchmarkRecencyHalfLife = 24 * time.Hour

	OpenAIRouteBenchmarkMinimumSamples = 12
	OpenAIRouteBenchmarkFullSamples    = 80
	OpenAIRouteBenchmarkMaxConfidence  = 0.25

	OpenAIRouteObservationSourceActiveBenchmark = "active_benchmark"
)

// OpenAIRouteBenchmarkEvidence makes the provenance and strength of an active
// probe prior auditable. It contains no URL, credential, prompt or response.
type OpenAIRouteBenchmarkEvidence struct {
	Source            string
	RawSamples        uint64
	EffectiveSamples  uint64
	RecentSamples     uint64
	HourOfWeekSamples uint64
	Confidence        float64
	RecencyWeight     float64
	LastObservedAt    time.Time
}

// OpenAIRouteBenchmarkObservationProfile mirrors the passive learner's three
// windows but remains a separate type so active probes cannot be accidentally
// counted as real customer outcomes or provider share.
type OpenAIRouteBenchmarkObservationProfile struct {
	Global     OpenAIRouteObservationAggregate
	Recent     OpenAIRouteObservationAggregate
	HourOfWeek OpenAIRouteObservationAggregate
	Evidence   OpenAIRouteBenchmarkEvidence
}

// OpenAIRouteBenchmarkObservationStore is optional. The controller calls it
// only for a policy that explicitly enables the prior; existing Shadow slices
// therefore retain byte-for-byte scoring behavior.
type OpenAIRouteBenchmarkObservationStore interface {
	GetBenchmarkBatch(
		ctx context.Context,
		keys []OpenAIRouteKey,
		now time.Time,
	) (map[string]OpenAIRouteBenchmarkObservationProfile, error)
}

func NewOpenAIRouteBenchmarkObservationProfile() OpenAIRouteBenchmarkObservationProfile {
	return OpenAIRouteBenchmarkObservationProfile{
		Global:     NewOpenAIRouteObservationAggregate(),
		Recent:     NewOpenAIRouteObservationAggregate(),
		HourOfWeek: NewOpenAIRouteObservationAggregate(),
		Evidence: OpenAIRouteBenchmarkEvidence{
			Source: OpenAIRouteObservationSourceActiveBenchmark,
		},
	}
}

// FinalizeOpenAIRouteBenchmarkObservationProfile applies a bounded confidence
// ramp and exponential recency decay. Fewer than twelve fresh samples have
// exactly zero scoring influence, preventing a one-sample time-slot winner.
func FinalizeOpenAIRouteBenchmarkObservationProfile(
	profile OpenAIRouteBenchmarkObservationProfile,
	now time.Time,
) OpenAIRouteBenchmarkObservationProfile {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	evidence := profile.Evidence
	evidence.Source = strings.TrimSpace(evidence.Source)
	if evidence.Source == "" {
		evidence.Source = OpenAIRouteObservationSourceActiveBenchmark
	}
	evidence.RawSamples = profile.Global.ReliabilityCount
	evidence.RecentSamples = profile.Recent.ReliabilityCount
	evidence.HourOfWeekSamples = profile.HourOfWeek.ReliabilityCount
	evidence.LastObservedAt = profile.Global.LastObservedAt.UTC()
	evidence.Confidence = 0
	evidence.RecencyWeight = 0
	evidence.EffectiveSamples = 0
	if evidence.RawSamples < OpenAIRouteBenchmarkMinimumSamples || evidence.LastObservedAt.IsZero() {
		profile.Evidence = evidence
		return profile
	}
	age := now.Sub(evidence.LastObservedAt)
	if age < 0 {
		age = 0
	}
	recency := math.Exp(-math.Ln2 * float64(age) / float64(OpenAIRouteBenchmarkRecencyHalfLife))
	if recency < 0 || math.IsNaN(recency) || math.IsInf(recency, 0) {
		recency = 0
	}
	if recency > 1 {
		recency = 1
	}
	sampleConfidence := math.Min(1, float64(evidence.RawSamples)/OpenAIRouteBenchmarkFullSamples)
	evidence.RecencyWeight = recency
	evidence.Confidence = OpenAIRouteBenchmarkMaxConfidence * sampleConfidence * recency
	evidence.EffectiveSamples = uint64(math.Round(float64(evidence.RawSamples) * evidence.Confidence))
	profile.Evidence = evidence
	return profile
}

// ApplyOpenAIRouteBenchmarkPrior adds only confidence-scaled pseudo-counts to
// an already blended passive aggregate. It never changes passive share/cost
// accounting and returns false when rounding leaves no effective evidence.
func ApplyOpenAIRouteBenchmarkPrior(
	passive OpenAIRouteObservationAggregate,
	profile OpenAIRouteBenchmarkObservationProfile,
) (OpenAIRouteObservationAggregate, bool) {
	evidence := profile.Evidence
	if evidence.Confidence <= 0 || evidence.EffectiveSamples == 0 || profile.Global.ReliabilityCount == 0 {
		return passive, false
	}
	prior := BlendOpenAIRouteObservationProfile(OpenAIRouteObservationProfile{
		Global:     profile.Global,
		Recent:     profile.Recent,
		HourOfWeek: profile.HourOfWeek,
	})
	if prior.ReliabilityCount == 0 {
		return passive, false
	}
	result := NewOpenAIRouteObservationAggregate()
	result.Merge(passive)
	before := result.ReliabilityCount
	mergeOpenAIRouteBenchmarkPrior(&result, prior, evidence.EffectiveSamples)
	if result.ReliabilityCount == before {
		return passive, false
	}
	if evidence.LastObservedAt.After(result.LastObservedAt) {
		result.LastObservedAt = evidence.LastObservedAt
	}
	return result, true
}

// mergeOpenAIRouteBenchmarkPrior converts a weak active-probe prior into an
// exact pseudo-sample budget. Generic profile blending rounds every histogram
// bucket independently. That is fine for well-populated passive windows, but a
// 16-sample benchmark at ~5% confidence has one effective sample and can have
// every latency bucket round to zero. The route would then retain reliability
// influence while losing the only signal that distinguishes one Base URL from
// another.
//
// Benchmark histograms are ordered distributions, so deterministic midpoint-
// quantile downsampling is a better contract than per-bucket rounding: one
// effective sample represents the median bucket; larger budgets preserve more
// of the distribution. Counts remain exact, active probes still cannot train
// customer cost, and the logic is isolated to explicitly enabled benchmark
// priors so existing passive-only Shadow treatments remain byte-for-byte
// unchanged.
func mergeOpenAIRouteBenchmarkPrior(
	target *OpenAIRouteObservationAggregate,
	source OpenAIRouteObservationAggregate,
	effectiveSamples uint64,
) {
	if target == nil || effectiveSamples == 0 || source.ReliabilityCount == 0 {
		return
	}
	scaled := NewOpenAIRouteObservationAggregate()
	scaled.ReliabilityCount = effectiveSamples
	scaled.AttemptCount = proportionalOpenAIRouteBenchmarkCount(
		source.AttemptCount,
		source.ReliabilityCount,
		effectiveSamples,
	)
	if scaled.AttemptCount < scaled.ReliabilityCount {
		scaled.AttemptCount = scaled.ReliabilityCount
	}

	outcomes := apportionOpenAIRouteBenchmarkCounts(
		[]uint64{source.SuccessCount, source.FailureCount},
		effectiveSamples,
	)
	scaled.SuccessCount = outcomes[0]
	scaled.FailureCount = outcomes[1]
	scaled.PartialStreams = proportionalOpenAIRouteBenchmarkCount(
		source.PartialStreams,
		source.ReliabilityCount,
		effectiveSamples,
	)
	if scaled.PartialStreams > scaled.FailureCount {
		scaled.PartialStreams = scaled.FailureCount
	}

	classes := make([]OpenAIRouteFailureClass, 0, len(source.FailureCounts))
	for class := range source.FailureCounts {
		classes = append(classes, class)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })
	failureCounts := make([]uint64, len(classes))
	for idx, class := range classes {
		failureCounts[idx] = source.FailureCounts[class]
	}
	for idx, count := range apportionOpenAIRouteBenchmarkCounts(failureCounts, scaled.FailureCount) {
		if count > 0 {
			scaled.FailureCounts[classes[idx]] = count
		}
	}

	ttftSamples := proportionalOpenAIRouteBenchmarkCount(
		sumOpenAIRouteHistogram(source.TTFTHistogram),
		source.ReliabilityCount,
		effectiveSamples,
	)
	scaled.TTFTHistogram = downsampleOpenAIRouteBenchmarkHistogram(source.TTFTHistogram, ttftSamples)
	scaled.TTFTSampleCount = sumOpenAIRouteHistogram(scaled.TTFTHistogram)
	latencySamples := proportionalOpenAIRouteBenchmarkCount(
		sumOpenAIRouteHistogram(source.LatencyHistogram),
		source.ReliabilityCount,
		effectiveSamples,
	)
	scaled.LatencyHistogram = downsampleOpenAIRouteBenchmarkHistogram(source.LatencyHistogram, latencySamples)
	scaled.LatencySampleCount = sumOpenAIRouteHistogram(scaled.LatencyHistogram)
	scaled.LastObservedAt = source.LastObservedAt

	// Deliberately leave ActualCostSamples and both cost totals at zero. Active
	// probes are admission evidence, never customer cost observations.
	target.Merge(scaled)
}

func proportionalOpenAIRouteBenchmarkCount(value, sourceTotal, targetTotal uint64) uint64 {
	if value == 0 || sourceTotal == 0 || targetTotal == 0 {
		return 0
	}
	result := uint64(math.Round(float64(value) * float64(targetTotal) / float64(sourceTotal)))
	if result > targetTotal {
		return targetTotal
	}
	return result
}

func apportionOpenAIRouteBenchmarkCounts(values []uint64, targetTotal uint64) []uint64 {
	result := make([]uint64, len(values))
	if len(values) == 0 || targetTotal == 0 {
		return result
	}
	var sourceTotal uint64
	for _, value := range values {
		sourceTotal += value
	}
	if sourceTotal == 0 {
		return result
	}
	type remainder struct {
		idx   int
		value float64
	}
	remainders := make([]remainder, len(values))
	var assigned uint64
	for idx, value := range values {
		exact := float64(value) * float64(targetTotal) / float64(sourceTotal)
		floor := uint64(math.Floor(exact))
		result[idx] = floor
		assigned += floor
		remainders[idx] = remainder{idx: idx, value: exact - float64(floor)}
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		return remainders[i].value > remainders[j].value
	})
	for idx := uint64(0); assigned < targetTotal; idx++ {
		result[remainders[idx%uint64(len(remainders))].idx]++
		assigned++
	}
	return result
}

func downsampleOpenAIRouteBenchmarkHistogram(source []uint64, targetTotal uint64) []uint64 {
	result := make([]uint64, len(source))
	sourceTotal := sumOpenAIRouteHistogram(source)
	if sourceTotal == 0 || targetTotal == 0 {
		return result
	}
	for sample := uint64(0); sample < targetTotal; sample++ {
		// Pick the midpoint of each equal-probability stratum. For one pseudo
		// sample this is the median, not whichever sparse bucket happens to win
		// an arbitrary rounding tie.
		rank := uint64((float64(sample) + 0.5) * float64(sourceTotal) / float64(targetTotal))
		if rank >= sourceTotal {
			rank = sourceTotal - 1
		}
		var cumulative uint64
		for idx, count := range source {
			cumulative += count
			if cumulative > rank {
				result[idx]++
				break
			}
		}
	}
	return result
}
