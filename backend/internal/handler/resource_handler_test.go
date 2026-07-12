package handler

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildCodexWindowsAppInstallerEnablesNonBlockingUpdates(t *testing.T) {
	xml, err := buildCodexWindowsAppInstaller(service.CachedDownloadAsset{
		ID:   "openai.codex_26.707.3748.0_x64_2p2nqsd0c76g0.msix",
		Name: "OpenAI.Codex_26.707.3748.0_x64__2p2nqsd0c76g0.Msix",
	})
	require.NoError(t, err)
	require.Contains(t, xml, `Name="OpenAI.Codex"`)
	require.Contains(t, xml, `Publisher="CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B"`)
	require.Contains(t, xml, `Version="26.707.3748.0"`)
	require.Contains(t, xml, `HoursBetweenUpdateChecks="24"`)
	require.Contains(t, xml, `ShowPrompt="false" UpdateBlocksActivation="false"`)
	require.Contains(t, xml, `<AutomaticBackgroundTask />`)
	require.Contains(t, xml, `/packages/openai.codex_26.707.3748.0_x64_2p2nqsd0c76g0.msix`)
}

func TestBuildCodexWindowsAppInstallerRejectsUnexpectedFilename(t *testing.T) {
	_, err := buildCodexWindowsAppInstaller(service.CachedDownloadAsset{ID: "bad", Name: "bad.msix"})
	require.Error(t, err)
}
