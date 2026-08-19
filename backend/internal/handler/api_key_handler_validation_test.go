package handler

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAPIKeyCreateRequest(t *testing.T) {
	negative := -1.0
	zeroDays := 0
	require.Error(t, validateAPIKeyCreateRequest(CreateAPIKeyRequest{Quota: &negative}))
	require.Error(t, validateAPIKeyCreateRequest(CreateAPIKeyRequest{ExpiresInDays: &zeroDays}))
	nan := math.NaN()
	require.Error(t, validateAPIKeyCreateRequest(CreateAPIKeyRequest{RateLimit7d: &nan}))
	require.NoError(t, validateAPIKeyCreateRequest(CreateAPIKeyRequest{}))
}

func TestValidateAPIKeyUpdateRequest(t *testing.T) {
	infinite := math.Inf(1)
	valid := 0.0
	require.Error(t, validateAPIKeyUpdateRequest(UpdateAPIKeyRequest{RateLimit1d: &infinite}))
	require.NoError(t, validateAPIKeyUpdateRequest(UpdateAPIKeyRequest{RateLimit1d: &valid}))
}
