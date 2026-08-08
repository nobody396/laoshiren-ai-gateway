package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPrepareOpenAIRouteBudgetWindows_CanonicalizesOrderAndRedisSlot(t *testing.T) {
	windows := []service.OpenAIRouteBudgetWindowConfig{
		openAIRouteBudgetUnitWindow("1h", "2026-08-08T12:00Z"),
		openAIRouteBudgetUnitWindow("5m", "2026-08-08T12:05Z"),
	}
	prepared, err := prepareOpenAIRouteBudgetWindows(windows)
	require.NoError(t, err)
	require.Len(t, prepared, 2)

	tag := "{" + prepared[0].domainFingerprint + "}"
	require.Contains(t, prepared[0].redisKey, tag)
	require.Contains(t, prepared[1].redisKey, tag)
	reservationKey := openAIRouteBudgetReservationRedisKey(prepared[0].domainFingerprint, "request-id")
	require.Contains(t, reservationKey, tag)
	routeFingerprint, err := validateOpenAIRouteBudgetRoute(openAIRouteBudgetUnitRoute(28), prepared)
	require.NoError(t, err)

	reversed, err := prepareOpenAIRouteBudgetWindows([]service.OpenAIRouteBudgetWindowConfig{windows[1], windows[0]})
	require.NoError(t, err)
	require.Equal(t, prepared[0].scopeFingerprint, reversed[0].scopeFingerprint)
	require.Equal(t, prepared[1].scopeFingerprint, reversed[1].scopeFingerprint)
	require.Equal(t,
		openAIRouteBudgetReservationSignature("request-id", routeFingerprint, 1, 2, prepared),
		openAIRouteBudgetReservationSignature("request-id", routeFingerprint, 1, 2, reversed),
	)
}

func TestOpenAIRouteBudgetReservationSignature_BindsExactRoute(t *testing.T) {
	prepared, err := prepareOpenAIRouteBudgetWindows([]service.OpenAIRouteBudgetWindowConfig{
		openAIRouteBudgetUnitWindow("5m", "2026-08-08T12:00Z"),
	})
	require.NoError(t, err)
	firstRoute, err := validateOpenAIRouteBudgetRoute(openAIRouteBudgetUnitRoute(28), prepared)
	require.NoError(t, err)
	secondRoute, err := validateOpenAIRouteBudgetRoute(openAIRouteBudgetUnitRoute(29), prepared)
	require.NoError(t, err)
	require.NotEqual(t,
		openAIRouteBudgetReservationSignature("request-id", firstRoute, 200_000_000, 100_000_000, prepared),
		openAIRouteBudgetReservationSignature("request-id", secondRoute, 200_000_000, 100_000_000, prepared),
	)
}

func TestPrepareOpenAIRouteBudgetWindows_RejectsCrossDomainAndDuplicates(t *testing.T) {
	first := openAIRouteBudgetUnitWindow("5m", "2026-08-08T12:00Z")
	second := openAIRouteBudgetUnitWindow("1h", "2026-08-08T12:00Z")
	second.Scope.Model = "gpt-5.6-luna"
	_, err := prepareOpenAIRouteBudgetWindows([]service.OpenAIRouteBudgetWindowConfig{first, second})
	require.ErrorIs(t, err, service.ErrOpenAIRouteInvalidPolicy)

	_, err = prepareOpenAIRouteBudgetWindows([]service.OpenAIRouteBudgetWindowConfig{first, first})
	require.ErrorIs(t, err, service.ErrOpenAIRouteInvalidPolicy)
}

func TestOpenAIRouteBudgetKeysDoNotExposeModelOrReservationID(t *testing.T) {
	domain := openAIRouteBudgetDomainFingerprint(7, "gpt-5.6-sol")
	budgetKey := openAIRouteBudgetRedisKey(domain, strings.Repeat("a", 32))
	reservationKey := openAIRouteBudgetReservationRedisKey(domain, "customer-visible-request-id")
	require.NotContains(t, budgetKey, "gpt-5.6-sol")
	require.NotContains(t, reservationKey, "customer-visible-request-id")
}

func openAIRouteBudgetUnitWindow(window, epoch string) service.OpenAIRouteBudgetWindowConfig {
	return service.OpenAIRouteBudgetWindowConfig{
		Scope: service.OpenAIRouteBudgetScope{
			GroupID: 7,
			Model:   "gpt-5.6-sol",
			Window:  window,
			Epoch:   epoch,
		},
		TargetAverageMultiplier: 0.155,
		HardAverageMultiplier:   0.18,
		EmergencyDebtLimitUSD:   0.01,
		MaxCreditUSD:            1,
		TTL:                     time.Hour,
	}
}

func openAIRouteBudgetUnitRoute(accountID int64) service.OpenAIRouteKey {
	return service.OpenAIRouteKey{
		GroupID:       7,
		AccountID:     accountID,
		Model:         "gpt-5.6-sol",
		EndpointHash:  "endpoint",
		Transport:     "sse",
		FailureDomain: "test-provider",
	}
}
