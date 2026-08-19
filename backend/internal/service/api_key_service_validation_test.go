package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCreateAPIKeyRequestRejectsInvalidLimitsAndExpiry(t *testing.T) {
	zero := 0
	for name, request := range map[string]CreateAPIKeyRequest{
		"negative quota": {Quota: -1},
		"nan rate":       {RateLimit5h: math.NaN()},
		"infinite rate":  {RateLimit1d: math.Inf(1)},
		"zero expiry":    {ExpiresInDays: &zero},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, validateCreateAPIKeyRequest(request))
		})
	}
	require.NoError(t, validateCreateAPIKeyRequest(CreateAPIKeyRequest{Quota: 0, RateLimit5h: 1, RateLimit1d: 2, RateLimit7d: 3}))
}

func TestValidateUpdateAPIKeyRequestRejectsInvalidLimits(t *testing.T) {
	negative := -1.0
	valid := 0.0
	require.Error(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{Quota: &negative}))
	require.NoError(t, validateUpdateAPIKeyRequest(UpdateAPIKeyRequest{Quota: &valid}))
}
