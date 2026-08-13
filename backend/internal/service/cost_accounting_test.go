package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCostAccountingCurrentMonthlyCatalogContainsOnlyV3Plans(t *testing.T) {
	require.Equal(t, []string{"plus", "pro", "max"}, costAccountingMonthlyCardPlanOrder)
	require.ElementsMatch(t, []string{"plus", "pro", "max"}, mapKeys(costAccountingMonthlyCardGroupNames))
	require.ElementsMatch(t, []string{"plus", "pro", "max"}, mapKeys(costAccountingPlanPricing))
	for _, products := range costAccountingMonthlyCardGroupNames {
		require.ElementsMatch(t, []string{"gpt", "claude", "grok"}, mapKeys(products))
	}
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

func TestPayAsYouGoTopupScenarioUsesObservedBlendedCost(t *testing.T) {
	// Production-shaped regression: the group contained a 1.2 image account,
	// but the real mixed cost was only 1053.1318 / 2644.6014 charged credits.
	// The image account multiplier must not turn this into a false -147.4% loss.
	scenario, basis := payAsYouGoTopupScenario(2644.6014, 1053.1318)

	require.NotNil(t, scenario)
	require.Equal(t, CostAccountingScenarioBasisObservedUsage, basis)
	require.Equal(t, 39.82, scenario.CostCNY)
	require.Equal(t, 57.18, scenario.ProfitCNY)
	require.Equal(t, 58.95, scenario.MarginPercent)
}

func TestPayAsYouGoTopupScenarioRequiresObservedCustomerCharges(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		rawCredits  float64
		realCostCNY float64
	}{
		{name: "no usage", rawCredits: 0, realCostCNY: 0},
		{name: "cost without billed credits", rawCredits: 0, realCostCNY: 12},
		{name: "invalid negative cost", rawCredits: 100, realCostCNY: -1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			scenario, basis := payAsYouGoTopupScenario(testCase.rawCredits, testCase.realCostCNY)
			require.Nil(t, scenario)
			require.Empty(t, basis)
		})
	}
}

func mapKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}
