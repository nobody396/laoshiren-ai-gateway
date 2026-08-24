package service

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompensableDurationClipsLateArrivalAndMonitoringGap(t *testing.T) {
	start := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	segments := []CompensationImpactSegment{{ID: 1, StartedAt: start, EndedAt: start.Add(time.Hour)}, {ID: 2, StartedAt: start.Add(2 * time.Hour), EndedAt: start.Add(3 * time.Hour)}}
	duration, ids, err := CompensableDurationMillis(segments, start.Add(30*time.Minute), start.Add(4*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(90*60*1000), duration)
	require.Equal(t, []int64{1, 2}, ids)
}

func TestCompensationRoundingAndAggregationFailClosedAtInt64Boundary(t *testing.T) {
	require.Equal(t, int64(922337203685478), roundMicrosToFen(math.MaxInt64))
	_, err := checkedAddCompensation(math.MaxInt64, 1)
	require.ErrorContains(t, err, "exceeds supported range")
}

func TestCompensationFormulaUsesDurationProductGroupAndTier(t *testing.T) {
	raw, err := CalculateCompensationRawMicros(30*60*1000, 3_000, 10_000, 10_000)
	require.NoError(t, err)
	require.Equal(t, int64(15_000_000), raw)
	priority, err := CalculateCompensationRawMicros(30*60*1000, 3_000, 20_000, 12_500)
	require.NoError(t, err)
	require.Equal(t, int64(37_500_000), priority)
}

func TestCompensationCapsRelationshipTierAndRollingExposure(t *testing.T) {
	policy := DefaultCompensationPolicy()
	tests := []struct {
		name  string
		input CompensationCapInput
		want  int64
		cap   string
	}{
		{"standard tier cap", CompensationCapInput{RawValueMicros: 30_000_000, Tier: CustomerTierStandard, TierCapCNYFen: 1_000, VerifiedPaidValueCNYFen: 50_000}, 1_000, "tier"},
		{"relationship cap", CompensationCapInput{RawValueMicros: 30_000_000, Tier: CustomerTierPriority, TierCapCNYFen: 5_000, VerifiedPaidValueCNYFen: 20_000}, 2_000, "relationship"},
		{"standard rolling remaining", CompensationCapInput{RawValueMicros: 30_000_000, Tier: CustomerTierStandard, TierCapCNYFen: 1_000, VerifiedPaidValueCNYFen: 50_000, RollingExecutedCNYFen: 1_500}, 500, "rolling"},
		{"priority rolling remaining", CompensationCapInput{RawValueMicros: 100_000_000, Tier: CustomerTierPriority, TierCapCNYFen: 5_000, VerifiedPaidValueCNYFen: 30_000, RollingExecutedCNYFen: 7_000}, 2_000, "rolling"},
		{"strategic has no rolling cap", CompensationCapInput{RawValueMicros: 500_000_000, Tier: CustomerTierStrategic, TierCapCNYFen: 20_000, VerifiedPaidValueCNYFen: 200_000, RollingExecutedCNYFen: 999_999}, 20_000, "tier"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ApplyCompensationCaps(test.input, policy)
			require.Equal(t, test.want, result.FinalCNYFen)
			require.Equal(t, test.cap, result.LimitingCap)
		})
	}
}

func TestCompensationAllocationRoundsToCentsAndPreservesTotal(t *testing.T) {
	allocation := AllocateCompensationFen(3_000, []int64{15_000_000, 30_000_000})
	require.Equal(t, []int64{1_000, 2_000}, allocation)
	oneSecond, err := CalculateCompensationRawMicros(1_000, 3_000, 10_000, 10_000)
	require.NoError(t, err)
	require.Equal(t, int64(8_333), oneSecond)
	require.Equal(t, []int64{1}, AllocateCompensationFen(1, []int64{oneSecond}))
}

func TestHighValueDraftRequiresRedesignAboveTenPercent(t *testing.T) {
	require.False(t, IsHighValueCompensationDraft(3_000, 30_000, 1_000))
	require.True(t, IsHighValueCompensationDraft(3_001, 30_000, 1_000))
}

func TestCompensationSnapshotReproductionKeepsExcludedItemsAtZero(t *testing.T) {
	start := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	facts := []CompensationEvidenceFact{}
	for i := 0; i < 3; i++ {
		facts = append(facts, CompensationEvidenceFact{ObservationID: int64(i + 1), RequestID: fmt.Sprintf("request-%d", i), FactType: "customer_request", Outcome: "failure", ErrorOwner: "provider", CustomerImpact: true, Qualified: true, QualificationReason: "qualified_final_failure"})
	}
	snapshot := CompensationUserEvidenceSnapshot{Policy: DefaultCompensationPolicy(), User: CompensationDraftUser{UserID: 1, Tier: CustomerTierStandard, Eligible: false, Included: false, EvidenceComplete: true, FinalFailureCount: 3, RawValueCNYMicros: 0, TierCapCNYFen: 1000, VerifiedPaidValueCNYFen: 10000, ProposedTotalCNYFen: 0}, Items: []CompensationDraftItem{{UserID: 1, ProductID: 1, BenefitChannel: "balance", CompensableDurationMS: 30 * 60 * 1000, ProductRateCNYFenPerHour: 3000, GroupWeightBPS: 10000, TierMultiplierBPS: 10000, RawValueCNYMicros: 0, ExclusionReason: "fewer_than_four_final_failures", SourceFacts: facts, Segments: []CompensationEvidenceSegment{{SegmentID: 1, ProductID: 1, StartedAt: start, EndedAt: start.Add(time.Hour), ClippedStartedAt: start, ClippedEndedAt: start.Add(30 * time.Minute), DurationMS: 30 * 60 * 1000}}}}}
	payload, err := canonicalCompensationJSON(snapshot)
	require.NoError(t, err)
	reproduced, err := ReproduceCompensationUserFromEvidence(payload)
	require.NoError(t, err)
	require.False(t, reproduced.PolicyEligible)
	require.Zero(t, reproduced.RawValueCNYMicros)
	require.Zero(t, reproduced.ProposedTotalCNYFen)
}

func TestCompensationSnapshotReproductionRejectsUnexcludedZeroRaw(t *testing.T) {
	start := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	facts := []CompensationEvidenceFact{}
	for i := 0; i < 4; i++ {
		facts = append(facts, CompensationEvidenceFact{ObservationID: int64(i + 1), RequestID: fmt.Sprintf("request-%d", i), FactType: "customer_request", Outcome: "failure", ErrorOwner: "provider", CustomerImpact: true, Qualified: true, QualificationReason: "qualified_final_failure"})
	}
	snapshot := CompensationUserEvidenceSnapshot{Policy: DefaultCompensationPolicy(), User: CompensationDraftUser{UserID: 1, Tier: CustomerTierStandard, Eligible: true, Included: true, EvidenceComplete: true, FinalFailureCount: 4, RawValueCNYMicros: 0, TierCapCNYFen: 1000, VerifiedPaidValueCNYFen: 10000, ProposedTotalCNYFen: 0}, Items: []CompensationDraftItem{{UserID: 1, ProductID: 1, BenefitChannel: "balance", CompensableDurationMS: 30 * 60 * 1000, ProductRateCNYFenPerHour: 3000, GroupWeightBPS: 10000, TierMultiplierBPS: 10000, RawValueCNYMicros: 0, SourceFacts: facts, Segments: []CompensationEvidenceSegment{{SegmentID: 1, ProductID: 1, StartedAt: start, EndedAt: start.Add(time.Hour), ClippedStartedAt: start, ClippedEndedAt: start.Add(30 * time.Minute), DurationMS: 30 * 60 * 1000}}}}}
	payload, err := canonicalCompensationJSON(snapshot)
	require.NoError(t, err)
	_, err = ReproduceCompensationUserFromEvidence(payload)
	require.ErrorContains(t, err, "raw value does not reproduce")
}
