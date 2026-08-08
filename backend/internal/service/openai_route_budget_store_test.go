package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteBudgetScope_RequiresExplicitEpoch(t *testing.T) {
	scope := OpenAIRouteBudgetScope{
		GroupID: 7,
		Model:   "gpt-5.6-sol",
		Window:  "5m",
	}
	require.False(t, scope.Valid())

	scope.Epoch = "2026-08-08T12:00Z"
	require.True(t, scope.Valid())
	require.Equal(t, scope.Fingerprint(), scope.Fingerprint())

	changed := scope
	changed.Epoch = "2026-08-08T12:05Z"
	require.NotEqual(t, scope.Fingerprint(), changed.Fingerprint())
}

func TestOpenAIRouteBudgetWindowConfig_ValidatesPolicyAndTTL(t *testing.T) {
	config := OpenAIRouteBudgetWindowConfig{
		Scope: OpenAIRouteBudgetScope{
			GroupID: 7,
			Model:   "gpt-5.6-sol",
			Window:  "5m",
			Epoch:   "2026-08-08T12:00Z",
		},
		TargetAverageMultiplier: 0.155,
		HardAverageMultiplier:   0.18,
		EmergencyDebtLimitUSD:   1,
		MaxCreditUSD:            10,
		TTL:                     10 * time.Minute,
	}
	require.NoError(t, config.Validate())

	config.HardAverageMultiplier = 0.15
	require.ErrorIs(t, config.Validate(), ErrOpenAIRouteInvalidPolicy)
	config.HardAverageMultiplier = 0.18
	config.TTL = 0
	require.ErrorIs(t, config.Validate(), ErrOpenAIRouteInvalidPolicy)
}

func TestOpenAIRouteFixedPointConversions_AreExactAndLuaSafe(t *testing.T) {
	units, err := OpenAIRouteUSDToCostUnits(0.155)
	require.NoError(t, err)
	require.Equal(t, int64(155_000_000), units)
	require.InDelta(t, 0.155, OpenAIRouteCostUnitsToUSD(units), 1e-12)

	multiplier, err := OpenAIRouteMultiplierToUnits(0.25)
	require.NoError(t, err)
	require.Equal(t, int64(250_000_000), multiplier)
	require.InDelta(t, 0.25, OpenAIRouteMultiplierUnitsToFloat(multiplier), 1e-12)

	_, err = OpenAIRouteUSDToCostUnits(math.Inf(1))
	require.ErrorIs(t, err, ErrOpenAIRouteInvalidCost)
	_, err = OpenAIRouteUSDToCostUnits(float64(OpenAIRouteMaxExactInteger)/float64(OpenAIRouteCostUnitsPerUSD) + 1)
	require.ErrorIs(t, err, ErrOpenAIRouteInvalidCost)
}
