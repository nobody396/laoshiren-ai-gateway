//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUniversalGroupResolveRouteUsesProtocolThenPriority(t *testing.T) {
	group := &Group{ID: 90, Platform: PlatformUniversal, UniversalRoutes: []UniversalRouteConfig{
		{PublicModel: "gpt-5.6-sol", MatchType: UniversalRouteMatchExact, InboundProtocol: APIProtocolResponses, TargetGroupID: 6, Priority: 20, Enabled: true},
		{PublicModel: "gpt-5.6-sol", MatchType: UniversalRouteMatchExact, InboundProtocol: APIProtocolResponses, TargetGroupID: 7, Priority: 10, Enabled: true},
		{PublicModel: "gpt-", MatchType: UniversalRouteMatchPrefix, InboundProtocol: APIProtocolChatCompletions, TargetGroupID: 8, Priority: 1, Enabled: true},
	}}

	decision, err := group.ResolveUniversalRoute("gpt-5.6-sol", APIProtocolResponses)
	require.NoError(t, err)
	require.Equal(t, int64(7), decision.TargetGroupID)

	decision, err = group.ResolveUniversalRoute("gpt-5.4", APIProtocolChatCompletions)
	require.NoError(t, err)
	require.Equal(t, int64(8), decision.TargetGroupID)

	_, err = group.ResolveUniversalRoute("claude-opus-5", APIProtocolResponses)
	require.ErrorIs(t, err, ErrUniversalRouteNotFound)
}

func TestUniversalPublicModelsHidesPrefixesAndDeduplicates(t *testing.T) {
	group := &Group{Platform: PlatformUniversal, UniversalRoutes: []UniversalRouteConfig{
		{PublicModel: "gpt-5.6-sol", MatchType: UniversalRouteMatchExact, TargetGroupID: 6, Enabled: true},
		{PublicModel: "GPT-5.6-SOL", MatchType: UniversalRouteMatchExact, TargetGroupID: 7, Enabled: true},
		{PublicModel: "gpt-", MatchType: UniversalRouteMatchPrefix, TargetGroupID: 8, Enabled: true},
	}}
	require.Equal(t, []string{"GPT-5.6-SOL"}, group.UniversalPublicModels())
}

func TestUniversalTargetProtocolMatrix(t *testing.T) {
	require.True(t, universalTargetSupportsProtocol(PlatformAnthropic, APIProtocolAnthropic))
	require.True(t, universalTargetSupportsProtocol(PlatformOpenAI, APIProtocolResponses))
	require.True(t, universalTargetSupportsProtocol(PlatformGemini, APIProtocolChatCompletions))
	require.False(t, universalTargetSupportsProtocol(PlatformAnthropic, APIProtocolResponses))
	require.False(t, universalTargetSupportsProtocol(PlatformGemini, APIProtocolAnthropic))
}
