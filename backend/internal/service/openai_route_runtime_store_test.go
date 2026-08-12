package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteHealthStoreKey_FingerprintsEveryHealthDimension(t *testing.T) {
	base := OpenAIRouteHealthStoreKey{
		Scope:         OpenAIRouteHealthScopeRoute,
		GroupID:       7,
		AccountID:     28,
		FailureDomain: "anyroute",
		Model:         "gpt-5.6-sol",
		RequestClass:  OpenAIRouteRequestClassText,
		EndpointHash:  "endpoint-a",
		Transport:     "sse",
	}
	require.True(t, base.Valid())
	require.Len(t, base.Fingerprint(), 32)

	variants := []OpenAIRouteHealthStoreKey{
		func() OpenAIRouteHealthStoreKey { value := base; value.GroupID++; return value }(),
		func() OpenAIRouteHealthStoreKey { value := base; value.AccountID++; return value }(),
		func() OpenAIRouteHealthStoreKey { value := base; value.Model = "gpt-5.4-mini"; return value }(),
		func() OpenAIRouteHealthStoreKey {
			value := base
			value.RequestClass = OpenAIRouteRequestClassImage
			return value
		}(),
		func() OpenAIRouteHealthStoreKey { value := base; value.EndpointHash = "endpoint-b"; return value }(),
		func() OpenAIRouteHealthStoreKey { value := base; value.Transport = "websocket"; return value }(),
	}
	for _, variant := range variants {
		require.NotEqual(t, base.Fingerprint(), variant.Fingerprint())
	}

	provider := OpenAIRouteHealthStoreKeyForProvider(OpenAIRouteKey{
		GroupID:       7,
		AccountID:     28,
		FailureDomain: "anyroute",
		Model:         "gpt-5.6-sol",
		RequestClass:  OpenAIRouteRequestClassText,
	})
	require.True(t, provider.Valid())
	require.Equal(t, OpenAIRouteHealthScopeProvider, provider.Scope)
	require.NotEqual(t, base.Fingerprint(), provider.Fingerprint())
}
