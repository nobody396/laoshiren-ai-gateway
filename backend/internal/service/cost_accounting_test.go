package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCostAccountingCurrentMonthlyCatalogContainsOnlyV3Plans(t *testing.T) {
	require.Equal(t, []string{"plus", "pro", "max"}, costAccountingMonthlyCardPlanOrder)
	require.ElementsMatch(t, []string{"plus", "pro", "max"}, mapKeys(costAccountingMonthlyCardGroupNames))
	require.ElementsMatch(t, []string{"plus", "pro", "max"}, mapKeys(costAccountingPlanPricing))
}

func TestCostAccountingPayAsYouGoTargetsDiscoversAllPublicStandardGroups(t *testing.T) {
	groups := []Group{
		{ID: 34, Name: "Grok", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, SortOrder: 0},
		{ID: 7, Name: "Lite GPT", Status: StatusActive, SubscriptionType: SubscriptionTypeCredit, IsExclusive: true},
		{ID: 15, Name: "Claude 外接", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, SortOrder: 20},
		{ID: 21, Name: "Claude Bedrock", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, SortOrder: 10},
		{ID: 99, Name: "停用公开组", Status: StatusDisabled, SubscriptionType: SubscriptionTypeStandard},
		{ID: 88, Name: "公开但独享", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsExclusive: true},
	}

	targets := costAccountingPayAsYouGoTargets(groups)

	require.Equal(t, []int64{34, 21, 15}, []int64{targets[0].ID, targets[1].ID, targets[2].ID})
}

func TestCostAccountingProductForGroup(t *testing.T) {
	require.Equal(t, "glm", costAccountingProductForGroup(Group{Name: "GLM 5.2", Platform: PlatformAnthropic}))
	require.Equal(t, "grok", costAccountingProductForGroup(Group{Name: "Grok", Platform: PlatformAnthropic}))
	require.Equal(t, "gpt", costAccountingProductForGroup(Group{Name: "Pro 20X", Platform: PlatformOpenAI}))
	require.Equal(t, "claude", costAccountingProductForGroup(Group{Name: "Claude 官转", Platform: PlatformAnthropic}))
	require.Equal(t, "gemini", costAccountingProductForGroup(Group{Name: "Gemini", Platform: "gemini"}))
}

func mapKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
