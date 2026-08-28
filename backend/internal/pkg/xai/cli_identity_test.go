package xai

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveCLIVersionDefaultsToPinnedVersion(t *testing.T) {
	t.Setenv(CLIVersionEnv, "")
	require.Equal(t, CLIClientVersion, ResolveCLIVersion())
	require.True(t, IsSupportedCLIVersion(CLIClientVersion))
}

func TestResolveCLIVersionRejectsUnsafeOrOldOverride(t *testing.T) {
	for _, version := range []string{"0.2.92", "0.2.93-beta.1", "0.2.95\r\nX-Injected: true", "0.2.093", "0.3", "1"} {
		t.Run(version, func(t *testing.T) {
			t.Setenv(CLIVersionEnv, version)
			require.Equal(t, CLIClientVersion, ResolveCLIVersion())
		})
	}
}

func TestApplyCLIProxyHeadersScopesIdentityToCLIHost(t *testing.T) {
	t.Setenv(CLIVersionEnv, "")
	cliReq, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	cliReq.Header.Set("User-Agent", "legacy-client/1.0")

	ApplyCLIProxyHeaders(cliReq)

	require.Equal(t, CLIClientVersion, cliReq.Header.Get("x-grok-client-version"))
	require.Equal(t, CLIClientIdentifier, cliReq.Header.Get("x-grok-client-identifier"))
	require.Equal(t, CLITokenAuth, cliReq.Header.Get("X-XAI-Token-Auth"))
	require.Equal(t, CLIUserAgent(CLIClientVersion), cliReq.Header.Get("User-Agent"))

	apiReq, err := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	apiReq.Header.Set("User-Agent", "direct-api-client/1.0")
	ApplyCLIProxyHeaders(apiReq)
	require.Equal(t, "direct-api-client/1.0", apiReq.Header.Get("User-Agent"))
	require.Empty(t, apiReq.Header.Get("x-grok-client-version"))
}
