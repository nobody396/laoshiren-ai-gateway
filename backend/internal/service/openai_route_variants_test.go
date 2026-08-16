package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandOpenAIRouteShadowVariantsDeduplicatesCurrentEndpointAndSharesFailureDomain(t *testing.T) {
	account := &Account{
		ID: 23, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{OpenAIRouteFailureDomainExtraKey: "pomoai:gpt-pro-0.2"},
	}
	source := OpenAIRouteShadowCandidate{
		Account: account, Endpoint: "https://hk.pomoai.xyz/v1/responses",
		Transport: string(OpenAIUpstreamTransportHTTPSSE), Priority: 1,
		HasReliabilitySample: true, SuccessLowerBound: 0.99, TTFTMilliseconds: 500,
	}

	expanded, audit, err := expandOpenAIRouteShadowVariants(
		[]OpenAIRouteShadowCandidate{source},
		[]openAIRouteRouteVariantConfig{
			{AccountID: 23, BaseURL: "https://jp.pomoai.xyz"},
			{AccountID: 23, BaseURL: "https://hk.pomoai.xyz"},
		},
	)

	require.NoError(t, err)
	require.Len(t, audit, 2, "the normalized policy records every declared route without raw URLs")
	require.Len(t, expanded, 2, "the current HK endpoint must not be duplicated")
	require.False(t, expanded[0].RouteVariant)
	require.True(t, expanded[1].RouteVariant)
	require.Same(t, account, expanded[1].Account, "variants reuse the credential/account failure domain without mutating it")
	require.False(t, expanded[1].HasReliabilitySample)
	require.Zero(t, expanded[1].SuccessLowerBound)
	require.Zero(t, expanded[1].TTFTMilliseconds)
	require.Equal(t, "https://jp.pomoai.xyz/v1/responses", expanded[1].Endpoint)

	baseKey, err := NewOpenAIRouteKey(account, 6, "gpt-5.6-sol", OpenAIRouteRequestClassText, expanded[0].Endpoint, expanded[0].Transport)
	require.NoError(t, err)
	variantKey, err := NewOpenAIRouteKey(account, 6, "gpt-5.6-sol", OpenAIRouteRequestClassText, expanded[1].Endpoint, expanded[1].Transport)
	require.NoError(t, err)
	require.Equal(t, baseKey.AccountID, variantKey.AccountID)
	require.Equal(t, baseKey.FailureDomain, variantKey.FailureDomain)
	require.NotEqual(t, baseKey.EndpointHash, variantKey.EndpointHash)
}

func TestExpandOpenAIRouteShadowVariantsRejectsUnsafeOrAmbiguousPolicy(t *testing.T) {
	source := OpenAIRouteShadowCandidate{
		Account:  &Account{ID: 23, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		Endpoint: "https://hk.pomoai.xyz/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
	}
	tests := map[string][]openAIRouteRouteVariantConfig{
		"plain http":      {{AccountID: 23, BaseURL: "http://jp.pomoai.xyz"}},
		"credential":      {{AccountID: 23, BaseURL: "https://user:secret@jp.pomoai.xyz"}},
		"private ip":      {{AccountID: 23, BaseURL: "https://127.0.0.1"}},
		"invalid account": {{AccountID: 0, BaseURL: "https://jp.pomoai.xyz"}},
		"duplicate endpoint": {
			{AccountID: 23, BaseURL: "https://jp.pomoai.xyz"},
			{AccountID: 23, BaseURL: "https://JP.POMOAI.XYZ/"},
		},
	}
	for name, configs := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := expandOpenAIRouteShadowVariants([]OpenAIRouteShadowCandidate{source}, configs)
			require.ErrorIs(t, err, ErrOpenAIRouteInvalidPolicy)
		})
	}
}

func TestExpandOpenAIRouteShadowVariantsNeverChangesNonResponsesOrOAuthRoutes(t *testing.T) {
	configs := []openAIRouteRouteVariantConfig{{AccountID: 23, BaseURL: "https://jp.pomoai.xyz"}}
	for _, source := range []OpenAIRouteShadowCandidate{
		{
			Account:  &Account{ID: 23, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
			Endpoint: "https://hk.pomoai.xyz/v1/chat/completions", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		},
		{
			Account:  &Account{ID: 23, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
			Endpoint: "https://chatgpt.com/backend-api/codex/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		},
	} {
		expanded, _, err := expandOpenAIRouteShadowVariants([]OpenAIRouteShadowCandidate{source}, configs)
		require.NoError(t, err)
		require.Len(t, expanded, 1)
		require.False(t, expanded[0].RouteVariant)
	}
}

func TestFinalizeOpenAIRouteShadowDecisionDetectsSameAccountEndpointDivergence(t *testing.T) {
	decision := &OpenAIRouteShadowDecision{
		Evaluated: true, SelectedAccountID: 23, SelectedEndpointHash: "jp-hash",
		Audit: &OpenAIRouteShadowAuditSnapshot{},
	}

	finalizeOpenAIRouteShadowDecision(decision, 23, "hk-hash")

	require.True(t, decision.Diverged)
	require.Equal(t, "hk-hash", decision.Audit.LegacySelectedEndpointHash)
}

func TestApplyOpenAIRouteShadowLegacyAuditTargetsOnlyExecutedAccountEndpoint(t *testing.T) {
	account := &Account{ID: 23}
	decision := &OpenAIRouteShadowDecision{Audit: &OpenAIRouteShadowAuditSnapshot{Candidates: []OpenAIRouteShadowAuditCandidate{
		{AccountID: 23, EndpointHash: "hk-hash", RouteFingerprint: "hk-route"},
		{AccountID: 23, EndpointHash: "jp-hash", RouteFingerprint: "jp-route", RouteVariant: true},
	}}}
	source := openAIAccountCandidateScore{account: account, score: 0.75, compactTier: 2}

	applyOpenAIRouteShadowLegacyAudit(decision, []openAIAccountCandidateScore{source}, []openAIAccountCandidateScore{source}, 1, 0)

	require.InDelta(t, 0.75, decision.Audit.Candidates[0].LegacyScore, 1e-12)
	require.Equal(t, 1, decision.Audit.Candidates[0].LegacyRank)
	require.Zero(t, decision.Audit.Candidates[1].LegacyScore)
	require.Zero(t, decision.Audit.Candidates[1].LegacyRank)
}
