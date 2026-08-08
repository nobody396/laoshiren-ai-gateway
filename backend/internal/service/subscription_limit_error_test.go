package service

import (
	"net/http"
	"strings"
	"testing"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionLimitErrorsExposeCorrectRetrySemantics(t *testing.T) {
	require.Equal(t, http.StatusTooManyRequests, infraerrors.Code(ErrDailyLimitExceeded))
	require.Equal(t, http.StatusTooManyRequests, infraerrors.Code(ErrWeeklyLimitExceeded))

	require.Equal(t, http.StatusForbidden, infraerrors.Code(ErrMonthlyLimitExceeded))
	require.Equal(t, "MONTHLY_LIMIT_EXCEEDED", infraerrors.Reason(ErrMonthlyLimitExceeded))

	message := infraerrors.Message(ErrMonthlyLimitExceeded)
	for _, phrase := range []string{
		"monthly plan quota has been exhausted",
		"account quota limit",
		"not a service outage",
		"Retrying will not help",
		"new monthly plan",
		"pay-as-you-go API key",
	} {
		require.True(t, strings.Contains(message, phrase), "missing customer guidance %q in %q", phrase, message)
	}
}
