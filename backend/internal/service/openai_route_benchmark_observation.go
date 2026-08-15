package service

import (
	"context"
	"math"
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
	mergeScaledOpenAIRouteObservationAggregate(&result, prior, evidence.Confidence)
	if result.ReliabilityCount == before {
		return passive, false
	}
	if evidence.LastObservedAt.After(result.LastObservedAt) {
		result.LastObservedAt = evidence.LastObservedAt
	}
	return result, true
}
