package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectClientSetupGroupPreservesAdminOrder(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "Codex 钱包", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: 2, Name: "Codex 月卡 A", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, SortOrder: 2},
		{ID: 3, Name: "Codex 月卡 B", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeCredit, SortOrder: 3},
	}

	selected := selectClientSetupGroup(ClientSetupTargetCodex, groups)
	require.NotNil(t, selected)
	require.Equal(t, int64(1), selected.ID)
}

func TestClientSetupGroupCompatibility(t *testing.T) {
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetClaude, &Group{
		Platform: PlatformAnthropic,
		Status:   StatusActive,
	}))
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetClaude, &Group{
		Platform: PlatformAntigravity,
		Status:   StatusActive,
	}))
	require.False(t, clientSetupGroupCompatible(ClientSetupTargetClaude, &Group{
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}))
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetCodex, &Group{
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}))
	require.False(t, clientSetupGroupCompatible(ClientSetupTargetCodex, &Group{
		Platform: PlatformOpenAI,
		Status:   "inactive",
	}))
}

func TestNormalizeClientSetupTargetRejectsUnknownTarget(t *testing.T) {
	_, err := normalizeClientSetupTarget("gemini")
	require.ErrorIs(t, err, ErrInvalidClientSetupTarget)
}
