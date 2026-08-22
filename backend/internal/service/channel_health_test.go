//go:build unit

package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyChannelProbeResultDoesNotMislabelProviderFailuresAsCredentialErrors(t *testing.T) {
	tests := []struct {
		status    int
		err       error
		class     ChannelHealthClass
		invalid   bool
		retryable bool
	}{
		{http.StatusUnauthorized, nil, ChannelHealthCredentialInvalid, true, false},
		{http.StatusForbidden, nil, ChannelHealthCredentialInvalid, true, false},
		{http.StatusTooManyRequests, nil, ChannelHealthRateLimited, false, true},
		{http.StatusBadGateway, nil, ChannelHealthTransient, false, true},
		{http.StatusBadRequest, nil, ChannelHealthBusinessError, false, false},
		{0, nil, ChannelHealthTransient, false, true},
		{0, errors.New("dial timeout"), ChannelHealthTransient, false, true},
	}
	for _, tt := range tests {
		verdict := ClassifyChannelProbeResult(tt.status, tt.err)
		require.Equal(t, tt.class, verdict.Class)
		require.Equal(t, tt.invalid, verdict.CredentialInvalid)
		require.Equal(t, tt.retryable, verdict.Retryable)
	}
}

func TestEvaluateChannelBalanceUsesSchedulingThreshold(t *testing.T) {
	require.Equal(t, ChannelBalanceUnknown, EvaluateChannelBalance(-1, 10))
	require.Equal(t, ChannelBalanceExhausted, EvaluateChannelBalance(0, 10))
	require.Equal(t, ChannelBalanceLow, EvaluateChannelBalance(10, 10))
	require.Equal(t, ChannelBalanceAvailable, EvaluateChannelBalance(10.01, 10))
}
