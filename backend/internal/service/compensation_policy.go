package service

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"
)

type CompensationPolicy struct {
	Version                       int   `json:"version"`
	FinalFailureThreshold         int   `json:"final_failure_threshold"`
	CustomerRelationshipCapBPS    int64 `json:"customer_relationship_cap_bps"`
	HighValueThresholdBPS         int64 `json:"high_value_threshold_bps"`
	RollingWindowDays             int   `json:"rolling_window_days"`
	StandardRollingFixedCapCNYFen int64 `json:"standard_rolling_fixed_cap_cny_fen"`
	StandardRollingPaidValueBPS   int64 `json:"standard_rolling_paid_value_bps"`
	PriorityRollingFixedCapCNYFen int64 `json:"priority_rolling_fixed_cap_cny_fen"`
	PriorityRollingPaidValueBPS   int64 `json:"priority_rolling_paid_value_bps"`
	ShadowMinimumDays             int   `json:"shadow_minimum_days"`
	ShadowMinimumIncidents        int   `json:"shadow_minimum_incidents"`
}

func DefaultCompensationPolicy() CompensationPolicy {
	return CompensationPolicy{Version: 1, FinalFailureThreshold: 4, CustomerRelationshipCapBPS: 1_000, HighValueThresholdBPS: 1_000, RollingWindowDays: 30, StandardRollingFixedCapCNYFen: 2_000, StandardRollingPaidValueBPS: 2_000, PriorityRollingFixedCapCNYFen: 15_000, PriorityRollingPaidValueBPS: 3_000, ShadowMinimumDays: 30, ShadowMinimumIncidents: 3}
}

func (p CompensationPolicy) Valid() bool {
	return p.Version > 0 && p.FinalFailureThreshold > 0 && p.CustomerRelationshipCapBPS > 0 && p.CustomerRelationshipCapBPS <= 10_000 && p.HighValueThresholdBPS > 0 && p.HighValueThresholdBPS <= 10_000 && p.RollingWindowDays > 0 && p.StandardRollingFixedCapCNYFen >= 0 && p.PriorityRollingFixedCapCNYFen >= 0 && p.ShadowMinimumDays >= 30 && p.ShadowMinimumIncidents >= 3
}

func CalculateCompensationRawMicros(durationMillis, rateCNYFenPerHour int64, groupWeightBPS, tierMultiplierBPS int64) (int64, error) {
	if durationMillis < 0 || rateCNYFenPerHour < 0 || groupWeightBPS < 0 || tierMultiplierBPS < 0 {
		return 0, fmt.Errorf("compensation formula input cannot be negative")
	}
	numerator := new(big.Int).Mul(big.NewInt(durationMillis), big.NewInt(rateCNYFenPerHour))
	numerator.Mul(numerator, big.NewInt(10_000))
	numerator.Mul(numerator, big.NewInt(groupWeightBPS))
	numerator.Mul(numerator, big.NewInt(tierMultiplierBPS))
	denominator := new(big.Int).Mul(big.NewInt(3_600_000), big.NewInt(100_000_000))
	return roundedPositiveFraction(numerator, denominator)
}

func roundedPositiveFraction(numerator, denominator *big.Int) (int64, error) {
	if denominator.Sign() <= 0 || numerator.Sign() < 0 {
		return 0, fmt.Errorf("invalid positive fraction")
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("compensation value exceeds supported range")
	}
	return quotient.Int64(), nil
}

type CompensationCapInput struct {
	RawValueMicros          int64
	Tier                    CustomerTier
	TierCapCNYFen           int64
	VerifiedPaidValueCNYFen int64
	RollingExecutedCNYFen   int64
}
type CompensationCapResult struct {
	FinalCNYFen            int64  `json:"final_cny_fen"`
	RawRoundedCNYFen       int64  `json:"raw_rounded_cny_fen"`
	RelationshipCapCNYFen  int64  `json:"relationship_cap_cny_fen"`
	TierCapCNYFen          int64  `json:"tier_cap_cny_fen"`
	RollingCapCNYFen       *int64 `json:"rolling_cap_cny_fen,omitempty"`
	RollingRemainingCNYFen *int64 `json:"rolling_remaining_cny_fen,omitempty"`
	LimitingCap            string `json:"limiting_cap"`
}

func ApplyCompensationCaps(input CompensationCapInput, policy CompensationPolicy) CompensationCapResult {
	rawFen := roundMicrosToFen(input.RawValueMicros)
	relationship := mulBPS(input.VerifiedPaidValueCNYFen, policy.CustomerRelationshipCapBPS)
	final := rawFen
	limit := "raw"
	apply := func(value int64, name string) {
		if value < 0 {
			value = 0
		}
		if value < final {
			final = value
			limit = name
		}
	}
	apply(input.TierCapCNYFen, "tier")
	apply(relationship, "relationship")
	result := CompensationCapResult{RawRoundedCNYFen: rawFen, RelationshipCapCNYFen: relationship, TierCapCNYFen: input.TierCapCNYFen}
	var rollingLimit int64
	hasRolling := true
	switch input.Tier {
	case CustomerTierStandard:
		rollingLimit = minCompensationInt64(policy.StandardRollingFixedCapCNYFen, mulBPS(input.VerifiedPaidValueCNYFen, policy.StandardRollingPaidValueBPS))
	case CustomerTierPriority:
		rollingLimit = minCompensationInt64(policy.PriorityRollingFixedCapCNYFen, mulBPS(input.VerifiedPaidValueCNYFen, policy.PriorityRollingPaidValueBPS))
	default:
		hasRolling = false
	}
	if hasRolling {
		remaining := rollingLimit - input.RollingExecutedCNYFen
		if remaining < 0 {
			remaining = 0
		}
		result.RollingCapCNYFen = &rollingLimit
		result.RollingRemainingCNYFen = &remaining
		apply(remaining, "rolling")
	}
	result.FinalCNYFen = final
	result.LimitingCap = limit
	return result
}

func roundMicrosToFen(value int64) int64 {
	if value <= 0 {
		return 0
	}
	result := value / 10_000
	if value%10_000 >= 5_000 {
		result++
	}
	return result
}

func checkedAddCompensation(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > math.MaxInt64-b {
		return 0, fmt.Errorf("compensation aggregate exceeds supported range")
	}
	return a + b, nil
}
func mulBPS(value, bps int64) int64 {
	if value <= 0 || bps <= 0 {
		return 0
	}
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(bps))
	product.Quo(product, big.NewInt(10_000))
	if !product.IsInt64() {
		return math.MaxInt64
	}
	return product.Int64()
}
func minCompensationInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func AllocateCompensationFen(total int64, rawMicros []int64) []int64 {
	result := make([]int64, len(rawMicros))
	if total <= 0 || len(rawMicros) == 0 {
		return result
	}
	sum := big.NewInt(0)
	for _, value := range rawMicros {
		if value > 0 {
			sum.Add(sum, big.NewInt(value))
		}
	}
	if sum.Sign() == 0 {
		return result
	}
	type remainder struct {
		index int
		value *big.Int
	}
	remainders := make([]remainder, 0, len(rawMicros))
	allocated := int64(0)
	for i, value := range rawMicros {
		if value <= 0 {
			remainders = append(remainders, remainder{i, big.NewInt(0)})
			continue
		}
		numerator := new(big.Int).Mul(big.NewInt(total), big.NewInt(value))
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(numerator, sum, r)
		result[i] = q.Int64()
		allocated += result[i]
		remainders = append(remainders, remainder{i, new(big.Int).Set(r)})
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		cmp := remainders[i].value.Cmp(remainders[j].value)
		if cmp == 0 {
			return remainders[i].index < remainders[j].index
		}
		return cmp > 0
	})
	left := total - allocated
	for i := int64(0); i < left; i++ {
		result[remainders[int(i%int64(len(remainders)))].index]++
	}
	return result
}

func IsHighValueCompensationDraft(proposedTotalCNYFen, productRollingPaidValueCNYFen, thresholdBPS int64) bool {
	return proposedTotalCNYFen > mulBPS(productRollingPaidValueCNYFen, thresholdBPS)
}

type CompensationImpactSegment struct {
	ID        int64
	StartedAt time.Time
	EndedAt   time.Time
}

func CompensableDurationMillis(segments []CompensationImpactSegment, firstFailure, impactEnd time.Time) (int64, []int64, error) {
	var total int64
	ids := []int64{}
	for _, segment := range segments {
		start := segment.StartedAt
		if firstFailure.After(start) {
			start = firstFailure
		}
		end := segment.EndedAt
		if end.IsZero() || impactEnd.Before(end) {
			end = impactEnd
		}
		if end.After(start) {
			var err error
			total, err = checkedAddCompensation(total, end.Sub(start).Milliseconds())
			if err != nil {
				return 0, nil, err
			}
			ids = append(ids, segment.ID)
		}
	}
	return total, ids, nil
}

func ReproduceCompensationUserFromEvidence(payload []byte) (CompensationReproduction, error) {
	var snapshot CompensationUserEvidenceSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return CompensationReproduction{}, err
	}
	if !snapshot.Policy.Valid() {
		return CompensationReproduction{}, fmt.Errorf("frozen compensation policy is invalid")
	}
	result := CompensationReproduction{Included: snapshot.User.Included}
	requests := map[string]bool{}
	raws := make([]int64, len(snapshot.Items))
	var rawTotal int64
	for i, item := range snapshot.Items {
		for _, fact := range item.SourceFacts {
			if fact.Qualified && fact.RequestID != "" {
				requests[fact.RequestID] = true
			}
		}
		var duration int64
		for _, segment := range item.Segments {
			if segment.ClippedEndedAt.Before(segment.ClippedStartedAt) || segment.DurationMS != segment.ClippedEndedAt.Sub(segment.ClippedStartedAt).Milliseconds() {
				return result, fmt.Errorf("frozen segment evidence is inconsistent")
			}
			var err error
			duration, err = checkedAddCompensation(duration, segment.DurationMS)
			if err != nil {
				return result, err
			}
		}
		if duration != item.CompensableDurationMS {
			return result, fmt.Errorf("frozen duration evidence is inconsistent")
		}
		if item.ExclusionReason == "" {
			if item.ProductRateCNYFenPerHour <= 0 || item.GroupWeightBPS <= 0 || item.TierMultiplierBPS <= 0 || duration <= 0 {
				return result, fmt.Errorf("frozen positive item is missing calculation inputs")
			}
			raw, err := CalculateCompensationRawMicros(duration, item.ProductRateCNYFenPerHour, item.GroupWeightBPS, item.TierMultiplierBPS)
			if err != nil {
				return result, err
			}
			if raw != item.RawValueCNYMicros {
				return result, fmt.Errorf("frozen item raw value does not reproduce")
			}
			raws[i] = raw
			rawTotal, err = checkedAddCompensation(rawTotal, raw)
			if err != nil {
				return result, err
			}
		} else if item.RawValueCNYMicros != 0 || item.ProposedCNYFen != 0 {
			return result, fmt.Errorf("frozen excluded item contains a benefit value")
		}
	}
	result.DistinctFinalFailures = len(requests)
	result.PolicyEligible = len(requests) >= snapshot.Policy.FinalFailureThreshold
	if result.DistinctFinalFailures != snapshot.User.FinalFailureCount || result.PolicyEligible != snapshot.User.Eligible {
		return result, fmt.Errorf("frozen eligibility evidence does not reproduce")
	}
	result.RawValueCNYMicros = rawTotal
	if rawTotal != snapshot.User.RawValueCNYMicros {
		return result, fmt.Errorf("frozen user raw value does not reproduce")
	}
	capResult := ApplyCompensationCaps(CompensationCapInput{RawValueMicros: rawTotal, Tier: snapshot.User.Tier, TierCapCNYFen: snapshot.User.TierCapCNYFen, VerifiedPaidValueCNYFen: snapshot.User.VerifiedPaidValueCNYFen, RollingExecutedCNYFen: snapshot.User.RollingGoodwillExecutedCNYFen}, snapshot.Policy)
	if snapshot.User.ProposedTotalCNYFen < capResult.FinalCNYFen {
		capResult.FinalCNYFen = snapshot.User.ProposedTotalCNYFen
	}
	if !snapshot.User.Included {
		capResult.FinalCNYFen = 0
	}
	result.ProposedTotalCNYFen = capResult.FinalCNYFen
	allocation := AllocateCompensationFen(result.ProposedTotalCNYFen, raws)
	for i, item := range snapshot.Items {
		var err error
		if item.BenefitChannel == "builder_pass" {
			result.BuilderPassBenefitCNYFen, err = checkedAddCompensation(result.BuilderPassBenefitCNYFen, allocation[i])
		} else {
			result.BalanceBenefitCNYFen, err = checkedAddCompensation(result.BalanceBenefitCNYFen, allocation[i])
		}
		if err != nil {
			return result, err
		}
	}
	if result.ProposedTotalCNYFen != snapshot.User.ProposedTotalCNYFen || result.BalanceBenefitCNYFen != snapshot.User.BalanceBenefitCNYFen || result.BuilderPassBenefitCNYFen != snapshot.User.BuilderPassBenefitCNYFen {
		return result, fmt.Errorf("frozen benefit allocation does not reproduce")
	}
	return result, nil
}
