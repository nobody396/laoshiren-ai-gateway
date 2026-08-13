package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	OpenAIRouteCostUnitsPerUSD int64 = 1_000_000_000
	OpenAIRouteMultiplierScale int64 = 1_000_000_000
	// OpenAIRouteMaxExactInteger keeps fixed-point values exactly representable
	// in Redis Lua, whose number type is IEEE-754 double precision.
	OpenAIRouteMaxExactInteger int64 = 9_007_199_254_740_991
)

type OpenAIRouteBudgetScope struct {
	GroupID      int64
	Model        string
	RequestClass OpenAIRouteRequestClass
	Window       string
	Epoch        string
}

func (s OpenAIRouteBudgetScope) Valid() bool {
	return s.GroupID > 0 && strings.TrimSpace(s.Model) != "" && s.RequestClass.Valid() && strings.TrimSpace(s.Window) != "" && strings.TrimSpace(s.Epoch) != ""
}

func (s OpenAIRouteBudgetScope) Fingerprint() string {
	canonical := fmt.Sprintf("%d|%s|%s|%s|%s", s.GroupID, strings.TrimSpace(s.Model), s.RequestClass, strings.TrimSpace(s.Window), strings.TrimSpace(s.Epoch))
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:16])
}

type OpenAIRouteBudgetWindowConfig struct {
	Scope OpenAIRouteBudgetScope

	TargetAverageMultiplier float64
	HardAverageMultiplier   float64
	EmergencyDebtLimitUSD   float64
	MaxCreditUSD            float64
	TTL                     time.Duration
}

func (c OpenAIRouteBudgetWindowConfig) Validate() error {
	if !c.Scope.Valid() || c.TTL <= 0 {
		return ErrOpenAIRouteInvalidPolicy
	}
	if !isFiniteNonNegative(c.TargetAverageMultiplier) || !isFiniteNonNegative(c.HardAverageMultiplier) || c.HardAverageMultiplier < c.TargetAverageMultiplier {
		return ErrOpenAIRouteInvalidPolicy
	}
	if !isFiniteNonNegative(c.EmergencyDebtLimitUSD) || !isFiniteNonNegative(c.MaxCreditUSD) {
		return ErrOpenAIRouteInvalidPolicy
	}
	return nil
}

func (c OpenAIRouteBudgetWindowConfig) EmptyLedger() OpenAIRouteBudgetLedger {
	return OpenAIRouteBudgetLedger{
		TargetAverageMultiplier: c.TargetAverageMultiplier,
		HardAverageMultiplier:   c.HardAverageMultiplier,
		EmergencyDebtLimitUSD:   c.EmergencyDebtLimitUSD,
		MaxCreditUSD:            c.MaxCreditUSD,
	}
}

type OpenAIRouteBudgetStoreReserveRequest struct {
	ReservationID        string
	RouteKey             OpenAIRouteKey
	RateMultiplier       float64
	EstimatedBaseCostUSD float64
	Windows              []OpenAIRouteBudgetWindowConfig
	ReservationTTL       time.Duration
}

type OpenAIRouteBudgetStoreReservation struct {
	ReservationID  string
	RouteKey       OpenAIRouteKey
	Allowed        bool
	Emergency      bool
	RejectedWindow int
}

type OpenAIRouteBudgetStoreSettlement struct {
	ReservationID        string
	RouteKey             OpenAIRouteKey
	ActualBaseCostUSD    float64
	ActualAccountCostUSD float64
	Windows              []OpenAIRouteBudgetWindowConfig
	AuditTTL             time.Duration
}

type OpenAIRouteBudgetStore interface {
	GetLedgers(ctx context.Context, windows []OpenAIRouteBudgetWindowConfig) ([]OpenAIRouteBudgetLedger, error)
	Reserve(ctx context.Context, req OpenAIRouteBudgetStoreReserveRequest) (OpenAIRouteBudgetStoreReservation, error)
	Settle(ctx context.Context, settlement OpenAIRouteBudgetStoreSettlement) error
	Cancel(ctx context.Context, settlement OpenAIRouteBudgetStoreSettlement) error
}

func OpenAIRouteUSDToCostUnits(value float64) (int64, error) {
	return openAIRouteFloatToScaledUnits(value, OpenAIRouteCostUnitsPerUSD)
}

func OpenAIRouteCostUnitsToUSD(value int64) float64 {
	return float64(value) / float64(OpenAIRouteCostUnitsPerUSD)
}

func OpenAIRouteMultiplierToUnits(value float64) (int64, error) {
	return openAIRouteFloatToScaledUnits(value, OpenAIRouteMultiplierScale)
}

func OpenAIRouteMultiplierUnitsToFloat(value int64) float64 {
	return float64(value) / float64(OpenAIRouteMultiplierScale)
}

func openAIRouteFloatToScaledUnits(value float64, scale int64) (int64, error) {
	if !isFiniteNonNegative(value) || value > float64(OpenAIRouteMaxExactInteger)/float64(scale) {
		return 0, ErrOpenAIRouteInvalidCost
	}
	return int64(math.Round(value * float64(scale))), nil
}
