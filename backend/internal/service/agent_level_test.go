package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDecideAgentLevel_CumulativeStandardPermanent(t *testing.T) {
	decision := decideAgentLevel(defaultAgentLevelRules(), 0.05, 0, 1000)

	require.Equal(t, AgentLevelStandard, decision.permanentLevel.LevelKey)
	require.Equal(t, AgentLevelStandard, decision.currentLevel.LevelKey)
	require.Equal(t, 0.10, decision.currentRate)
	require.Equal(t, AgentLevelRateSourceCumulative, decision.rateSource)
}

func TestDecideAgentLevel_MonthlyCoreTemporaryAndDowngradeToPermanent(t *testing.T) {
	upgraded := decideAgentLevel(defaultAgentLevelRules(), 0.10, 1500, 2000)
	require.Equal(t, AgentLevelStandard, upgraded.permanentLevel.LevelKey)
	require.NotNil(t, upgraded.temporaryLevel)
	require.Equal(t, AgentLevelCore, upgraded.temporaryLevel.LevelKey)
	require.Equal(t, AgentLevelCore, upgraded.currentLevel.LevelKey)
	require.Equal(t, 0.15, upgraded.currentRate)
	require.Equal(t, AgentLevelRateSourceMonthly, upgraded.rateSource)

	downgraded := decideAgentLevel(defaultAgentLevelRules(), 0.10, 1499.99, 2000)
	require.Equal(t, AgentLevelStandard, downgraded.permanentLevel.LevelKey)
	require.Nil(t, downgraded.temporaryLevel)
	require.Equal(t, AgentLevelStandard, downgraded.currentLevel.LevelKey)
	require.Equal(t, 0.10, downgraded.currentRate)
}

func TestDecideAgentLevel_CumulativeCoreAndSuper(t *testing.T) {
	core := decideAgentLevel(defaultAgentLevelRules(), 0.05, 0, 10000)
	require.Equal(t, AgentLevelCore, core.permanentLevel.LevelKey)
	require.Equal(t, AgentLevelCore, core.currentLevel.LevelKey)
	require.Equal(t, 0.15, core.currentRate)

	super := decideAgentLevel(defaultAgentLevelRules(), 0.05, 0, 50000)
	require.Equal(t, AgentLevelSuper, super.permanentLevel.LevelKey)
	require.Equal(t, AgentLevelSuper, super.currentLevel.LevelKey)
	require.Equal(t, 0.20, super.currentRate)
	require.Nil(t, super.nextLevelKey)
	require.Equal(t, 0.0, super.nextLevelGap)
}

func TestDecideAgentLevel_MonthlySuperBeatsPermanentCore(t *testing.T) {
	decision := decideAgentLevel(defaultAgentLevelRules(), 0.05, 3000, 10000)

	require.Equal(t, AgentLevelCore, decision.permanentLevel.LevelKey)
	require.NotNil(t, decision.temporaryLevel)
	require.Equal(t, AgentLevelSuper, decision.temporaryLevel.LevelKey)
	require.Equal(t, AgentLevelSuper, decision.currentLevel.LevelKey)
	require.Equal(t, 0.20, decision.currentRate)
	require.Equal(t, AgentLevelRateSourceMonthly, decision.rateSource)
}

func TestDecideAgentLevel_ManualBaseDoesNotBlockUpgrade(t *testing.T) {
	decision := decideAgentLevel(defaultAgentLevelRules(), 0.05, 0, 1000)
	require.Equal(t, AgentLevelStandard, decision.currentLevel.LevelKey)
	require.Equal(t, 0.10, decision.currentRate)

	manualBase := decideAgentLevel(defaultAgentLevelRules(), 0.15, 0, 1000)
	require.Equal(t, AgentLevelCore, manualBase.currentLevel.LevelKey)
	require.Equal(t, 0.15, manualBase.currentRate)
	require.Equal(t, AgentLevelRateSourceManualBase, manualBase.rateSource)
}

func TestNextAgentLevelProgressMonthlyReachedTarget(t *testing.T) {
	progress := nextAgentLevelProgress(defaultAgentLevelRules(), 1600, 0.10, true)

	require.NotNil(t, progress)
	require.Equal(t, AgentLevelCore, progress.LevelKey)
	require.Equal(t, 1500.0, progress.Threshold)
	require.Equal(t, 0.0, progress.Gap)
	require.Equal(t, 1.0, progress.Progress)
}

func TestNextAgentLevelProgressCumulativeUsesPermanentFloor(t *testing.T) {
	progress := nextAgentLevelProgress(defaultAgentLevelRules(), 500, 0.05, false)

	require.NotNil(t, progress)
	require.Equal(t, AgentLevelStandard, progress.LevelKey)
	require.Equal(t, 1000.0, progress.Threshold)
	require.Equal(t, 500.0, progress.Gap)
	require.Equal(t, 0.5, progress.Progress)
}

func TestPreviousMonthWindowUsesNaturalMonth(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	start, end, key := previousMonthWindow(time.Date(2026, 6, 1, 0, 0, 0, 0, loc))

	require.Equal(t, "2026-05", key)
	require.Equal(t, time.Date(2026, 5, 1, 0, 0, 0, 0, loc), start)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, loc), end)
}
