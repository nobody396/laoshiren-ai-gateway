package service

import "math"

// OpenAIRouteWilsonLowerBound returns a conservative success-rate estimate for
// small samples. z=1.96 represents an approximate 95% confidence interval.
func OpenAIRouteWilsonLowerBound(successes, total uint64, z float64) float64 {
	if total == 0 {
		return 0
	}
	if successes > total {
		successes = total
	}
	if z <= 0 || math.IsNaN(z) || math.IsInf(z, 0) {
		z = 1.96
	}
	n := float64(total)
	p := float64(successes) / n
	z2 := z * z
	center := p + z2/(2*n)
	margin := z * math.Sqrt((p*(1-p)+z2/(4*n))/n)
	bound := (center - margin) / (1 + z2/n)
	return math.Max(0, math.Min(1, bound))
}

// openAIRouteExplorationBoost is a bounded UCB-style uncertainty bonus. It
// encourages evidence collection without allowing exploration to overpower
// health, circuit, share, or cost gates.
func openAIRouteExplorationBoost(samples, totalSamples uint64) float64 {
	if totalSamples == 0 {
		return 1
	}
	uncertainty := math.Sqrt(math.Log(float64(totalSamples)+2) / (float64(samples) + 1))
	return 1 + math.Min(0.25, 0.08*uncertainty)
}
