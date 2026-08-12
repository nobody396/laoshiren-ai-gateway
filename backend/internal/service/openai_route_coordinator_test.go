package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeOpenAIRouteBudgetStore struct {
	ledgers      []OpenAIRouteBudgetLedger
	reserveCalls []OpenAIRouteBudgetStoreReserveRequest
	rejectFirst  bool
}

func (f *fakeOpenAIRouteBudgetStore) GetLedgers(context.Context, []OpenAIRouteBudgetWindowConfig) ([]OpenAIRouteBudgetLedger, error) {
	return append([]OpenAIRouteBudgetLedger(nil), f.ledgers...), nil
}

func (f *fakeOpenAIRouteBudgetStore) Reserve(_ context.Context, req OpenAIRouteBudgetStoreReserveRequest) (OpenAIRouteBudgetStoreReservation, error) {
	f.reserveCalls = append(f.reserveCalls, req)
	allowed := !f.rejectFirst || len(f.reserveCalls) != 1
	return OpenAIRouteBudgetStoreReservation{
		ReservationID:  req.ReservationID,
		RouteKey:       req.RouteKey,
		Allowed:        allowed,
		Emergency:      allowed && req.RateMultiplier > req.Windows[0].TargetAverageMultiplier,
		RejectedWindow: map[bool]int{true: -1, false: 0}[allowed],
	}, nil
}

func (f *fakeOpenAIRouteBudgetStore) Settle(context.Context, OpenAIRouteBudgetStoreSettlement) error {
	return nil
}

func (f *fakeOpenAIRouteBudgetStore) Cancel(context.Context, OpenAIRouteBudgetStoreSettlement) error {
	return nil
}

func TestAllocateAndReserveOpenAIRoute_TriesNextRankedCandidateAfterAtomicRace(t *testing.T) {
	allocation := testOpenAIRouteAllocationRequest(
		testOpenAIRouteCandidate(1, "provider-a", 0.15),
		testOpenAIRouteCandidate(2, "provider-b", 0.20),
	)
	window := testOpenAIRouteBudgetWindowConfig(allocation.Policy)
	store := &fakeOpenAIRouteBudgetStore{
		ledgers:     []OpenAIRouteBudgetLedger{allocation.Budget},
		rejectFirst: true,
	}

	plan, reservation, err := AllocateAndReserveOpenAIRoute(
		context.Background(),
		store,
		allocation,
		[]OpenAIRouteBudgetWindowConfig{window},
		"request-123",
		time.Minute,
	)
	require.NoError(t, err)
	require.True(t, reservation.Allowed)
	require.Len(t, store.reserveCalls, 2)
	require.Equal(t, store.reserveCalls[1].RateMultiplier, plan.Selected.Candidate.RateMultiplier)
	require.Contains(t, plan.Excluded, OpenAIRouteExclusion{
		AccountID: plan.Ranked[0].Candidate.Key.AccountID,
		Reason:    OpenAIRouteExcludedCost,
	})
}

func TestAllocateAndReserveOpenAIRoute_UsesEverySharedBudgetWindowBeforeReserve(t *testing.T) {
	allocation := testOpenAIRouteAllocationRequest(testOpenAIRouteCandidate(2, "provider", 0.20))
	allocation.Policy.EmergencyDebtLimitUSD = 0
	short := allocation.Budget
	short.CreditUSD = 1
	long := allocation.Budget
	long.CreditUSD = 0
	store := &fakeOpenAIRouteBudgetStore{ledgers: []OpenAIRouteBudgetLedger{short, long}}
	windows := []OpenAIRouteBudgetWindowConfig{
		testOpenAIRouteBudgetWindowConfig(allocation.Policy),
		testOpenAIRouteBudgetWindowConfig(allocation.Policy),
	}
	windows[1].Scope.Window = "1h"
	windows[1].Scope.Epoch = "2026-08-08T12:00Z"

	_, _, err := AllocateAndReserveOpenAIRoute(
		context.Background(), store, allocation, windows, "request-456", time.Minute,
	)
	require.ErrorIs(t, err, ErrOpenAIRouteBudgetExhausted)
	require.Empty(t, store.reserveCalls)
}

func TestAllocateAndReserveOpenAIRoute_RejectsPolicyWindowMismatch(t *testing.T) {
	allocation := testOpenAIRouteAllocationRequest(testOpenAIRouteCandidate(1, "provider", 0.15))
	ledger := allocation.Budget
	ledger.TargetAverageMultiplier = 0.16
	store := &fakeOpenAIRouteBudgetStore{ledgers: []OpenAIRouteBudgetLedger{ledger}}

	_, _, err := AllocateAndReserveOpenAIRoute(
		context.Background(),
		store,
		allocation,
		[]OpenAIRouteBudgetWindowConfig{testOpenAIRouteBudgetWindowConfig(allocation.Policy)},
		"request-789",
		time.Minute,
	)
	require.ErrorIs(t, err, ErrOpenAIRouteInvalidPolicy)
	require.Empty(t, store.reserveCalls)
}

func testOpenAIRouteBudgetWindowConfig(policy OpenAIRoutePolicy) OpenAIRouteBudgetWindowConfig {
	return OpenAIRouteBudgetWindowConfig{
		Scope: OpenAIRouteBudgetScope{
			GroupID:      7,
			Model:        "gpt-5.6-sol",
			RequestClass: OpenAIRouteRequestClassText,
			Window:       "5m",
			Epoch:        "2026-08-08T12:00Z",
		},
		TargetAverageMultiplier: policy.TargetAverageMultiplier,
		HardAverageMultiplier:   policy.HardAverageMultiplier,
		EmergencyDebtLimitUSD:   policy.EmergencyDebtLimitUSD,
		MaxCreditUSD:            policy.MaxCreditUSD,
		TTL:                     10 * time.Minute,
	}
}
