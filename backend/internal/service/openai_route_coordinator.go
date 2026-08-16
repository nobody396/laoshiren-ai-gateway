package service

import (
	"context"
	"math"
	"strings"
	"time"
)

// AllocateAndReserveOpenAIRoute composes the pure allocator with the shared
// multi-window budget store. It is intentionally independent of the existing
// scheduler and gateway so Phase 2 can wire it after the concurrent gateway
// branch stabilizes.
//
// A plan is computed from a point-in-time ledger snapshot, then Redis performs
// the authoritative atomic reservation. If another instance consumes budget
// first, this function tries the next already-ranked candidate. Transport,
// half-open, session ownership, and account concurrency remain hard filters
// supplied by the scheduler integration layer.
func AllocateAndReserveOpenAIRoute(
	ctx context.Context,
	store OpenAIRouteBudgetStore,
	allocation OpenAIRouteAllocationRequest,
	windows []OpenAIRouteBudgetWindowConfig,
	reservationID string,
	reservationTTL time.Duration,
) (OpenAIRouteAllocationPlan, OpenAIRouteBudgetStoreReservation, error) {
	plan, reservation, _, err := allocateAndReserveOpenAIRouteWithLedgers(
		ctx, store, allocation, windows, reservationID, reservationTTL,
	)
	return plan, reservation, err
}

// allocateAndReserveOpenAIRouteWithLedgers returns the exact pre-reservation
// ledger snapshot used by the allocator. The audit path needs this evidence;
// the public coordinator keeps its original narrow signature.
func allocateAndReserveOpenAIRouteWithLedgers(
	ctx context.Context,
	store OpenAIRouteBudgetStore,
	allocation OpenAIRouteAllocationRequest,
	windows []OpenAIRouteBudgetWindowConfig,
	reservationID string,
	reservationTTL time.Duration,
) (OpenAIRouteAllocationPlan, OpenAIRouteBudgetStoreReservation, []OpenAIRouteBudgetLedger, error) {
	emptyReservation := OpenAIRouteBudgetStoreReservation{
		ReservationID:  strings.TrimSpace(reservationID),
		RejectedWindow: -1,
	}
	if store == nil || emptyReservation.ReservationID == "" || reservationTTL <= 0 {
		return OpenAIRouteAllocationPlan{}, emptyReservation, nil, ErrOpenAIRouteInvalidCost
	}

	policy, err := NormalizeOpenAIRoutePolicy(allocation.Policy)
	if err != nil {
		return OpenAIRouteAllocationPlan{}, emptyReservation, nil, err
	}
	ledgers, err := store.GetLedgers(ctx, windows)
	if err != nil {
		return OpenAIRouteAllocationPlan{}, emptyReservation, nil, err
	}
	if len(ledgers) == 0 || len(ledgers) != len(windows) {
		return OpenAIRouteAllocationPlan{}, emptyReservation, ledgers, ErrOpenAIRouteInvalidPolicy
	}
	for _, ledger := range ledgers {
		if math.Abs(ledger.TargetAverageMultiplier-policy.TargetAverageMultiplier) > 1e-12 ||
			math.Abs(ledger.HardAverageMultiplier-policy.HardAverageMultiplier) > 1e-12 {
			return OpenAIRouteAllocationPlan{}, emptyReservation, ledgers, ErrOpenAIRouteInvalidPolicy
		}
	}

	allocation.Policy = policy
	allocation.Budgets = ledgers
	plan, err := BuildOpenAIRouteAllocationPlan(allocation)
	if err != nil {
		return plan, emptyReservation, ledgers, err
	}

	lastReservation := emptyReservation
	for _, ranked := range plan.Ranked {
		estimatedBaseCostUSD := openAIRouteCandidateEstimatedBaseCost(ranked.Candidate, allocation.EstimatedBaseCostUSD)
		reservation, reserveErr := store.Reserve(ctx, OpenAIRouteBudgetStoreReserveRequest{
			ReservationID:        emptyReservation.ReservationID,
			RouteKey:             ranked.Candidate.Key,
			RateMultiplier:       ranked.Candidate.RateMultiplier,
			EstimatedBaseCostUSD: estimatedBaseCostUSD,
			Windows:              windows,
			ReservationTTL:       reservationTTL,
		})
		if reserveErr != nil {
			return plan, reservation, ledgers, reserveErr
		}
		lastReservation = reservation
		if !reservation.Allowed {
			plan.Excluded = append(plan.Excluded, newOpenAIRouteExclusion(ranked.Candidate, OpenAIRouteExcludedCost))
			continue
		}
		plan.Selected = ranked
		plan.Emergency = reservation.Emergency
		return plan, reservation, ledgers, nil
	}

	return plan, lastReservation, ledgers, ErrOpenAIRouteBudgetExhausted
}
