package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteFailureDomainID_UsesExplicitMetadataAndSafeFallback(t *testing.T) {
	require.Equal(t, "", OpenAIRouteFailureDomainID(nil))
	require.Equal(t, "account:28", OpenAIRouteFailureDomainID(&Account{ID: 28}))
	require.Equal(t, "anyroute", OpenAIRouteFailureDomainID(&Account{
		ID:    28,
		Extra: map[string]any{openAIRouteFailureDomainExtraKey: " anyroute "},
	}))
	require.Equal(t, "pomo", OpenAIRouteFailureDomainID(&Account{
		ID: 24,
		Extra: map[string]any{
			"routing": map[string]any{"failure_domain_id": " pomo "},
		},
	}))
}

func TestNewOpenAIRouteKey_DimensionsAccountModelRequestClassEndpointTransport(t *testing.T) {
	account := &Account{ID: 28, Extra: map[string]any{openAIRouteFailureDomainExtraKey: "anyroute"}}
	key, err := NewOpenAIRouteKey(account, 7, " gpt-5.6-sol ", OpenAIRouteRequestClassText, "https://us.example.invalid/v1/responses/", "sse")
	require.NoError(t, err)
	require.Equal(t, int64(7), key.GroupID)
	require.Equal(t, int64(28), key.AccountID)
	require.Equal(t, "gpt-5.6-sol", key.Model)
	require.Equal(t, OpenAIRouteRequestClassText, key.RequestClass)
	require.Equal(t, "sse", key.Transport)
	require.Equal(t, "anyroute", key.FailureDomain)
	require.Len(t, key.EndpointHash, 16)

	same, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://us.example.invalid/v1/responses", "sse")
	require.NoError(t, err)
	require.Equal(t, key.EndpointHash, same.EndpointHash)
	credentialBearing, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "HTTPS://user:secret@US.EXAMPLE.INVALID/v1/responses/?token=secret#fragment", "sse")
	require.NoError(t, err)
	require.Equal(t, key.EndpointHash, credentialBearing.EndpointHash)

	_, err = NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "", "sse")
	require.ErrorIs(t, err, ErrOpenAIRouteNoCandidate)
	_, err = NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://us.example.invalid/v1/responses", "")
	require.ErrorIs(t, err, ErrOpenAIRouteNoCandidate)

	_, err = NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassUnknown, "https://us.example.invalid/v1/responses", "sse")
	require.ErrorIs(t, err, ErrOpenAIRouteNoCandidate)
}

func TestOpenAIRouteEndpointForAccountAcceptsAbsoluteObservedEndpoint(t *testing.T) {
	account := &Account{
		ID:       28,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://configured.example.invalid/v1",
		},
	}
	require.Equal(t,
		"https://observed.example.invalid/v1/chat/completions",
		openAIRouteEndpointForAccount(account, "https://observed.example.invalid/v1/chat/completions?token=secret"),
	)
}

func TestOpenAIRouteWilsonLowerBound_IsConservativeForSmallSamples(t *testing.T) {
	require.Equal(t, 0.0, OpenAIRouteWilsonLowerBound(0, 0, 1.96))
	require.Less(t, OpenAIRouteWilsonLowerBound(1, 1, 1.96), 0.30)
	require.Greater(t, OpenAIRouteWilsonLowerBound(990, 1000, 1.96), 0.97)
	require.Less(t, OpenAIRouteWilsonLowerBound(900, 1000, 1.96), 0.90)
}
