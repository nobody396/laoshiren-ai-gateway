//go:build unit

package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveAPIProtocolForRequestPrefersNativeEndpoint(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Credentials: map[string]any{
		"api_protocol": APIProtocolAdaptive,
		"api_base_urls": map[string]any{
			APIProtocolChatCompletions: "https://provider.example/chat",
			APIProtocolResponses:       "https://provider.example/responses",
		},
	}}

	decision, err := ResolveAPIProtocolForRequest(account, APIProtocolResponses)
	require.NoError(t, err)
	require.Equal(t, APIProtocolResponses, decision.UpstreamProtocol)
	require.Equal(t, "https://provider.example/responses", decision.BaseURL)
	require.True(t, decision.Native)
	require.False(t, decision.RequiresBridge)
}

func TestResolveAPIProtocolForRequestFallsBackToSingleBridge(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Credentials: map[string]any{
		"api_protocol":  APIProtocolAdaptive,
		"api_base_urls": map[string]string{APIProtocolChatCompletions: "https://provider.example/v1"},
	}}

	decision, err := ResolveAPIProtocolForRequest(account, APIProtocolResponses)
	require.NoError(t, err)
	require.Equal(t, APIProtocolChatCompletions, decision.UpstreamProtocol)
	require.False(t, decision.Native)
	require.True(t, decision.RequiresBridge)
}

func TestResolveAPIProtocolForRequestRejectsOAuthAndMissingEndpoints(t *testing.T) {
	_, err := ResolveAPIProtocolForRequest(&Account{Type: AccountTypeOAuth}, APIProtocolResponses)
	require.True(t, errors.Is(err, ErrNoCompatibleAPIProtocol))

	_, err = ResolveAPIProtocolForRequest(&Account{Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_protocol": APIProtocolAdaptive,
	}}, APIProtocolAnthropic)
	require.True(t, errors.Is(err, ErrNoCompatibleAPIProtocol))
}
