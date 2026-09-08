package handler

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSetupSelectionFromRequestPreservesLegacyRequests(t *testing.T) {
	apiKeyID := int64(42)
	_, explicit, err := setupSelectionFromRequest(clientSetupTicketRequest{APIKeyID: &apiKeyID})
	require.NoError(t, err)
	require.False(t, explicit)

	_, explicit, err = setupSelectionFromRequest(clientSetupTicketRequest{Target: "codex"})
	require.NoError(t, err)
	require.False(t, explicit)
}

func TestSetupSelectionFromRequestRequiresEveryExactField(t *testing.T) {
	apiKeyID := int64(42)
	req := clientSetupTicketRequest{
		APIKeyID: &apiKeyID, ClientID: "codex", ClientVersionKey: "cli:0.151.0",
		Protocol: "responses", ModelID: "gpt-5.6-sol", OS: "macos",
	}
	selection, explicit, err := setupSelectionFromRequest(req)
	require.NoError(t, err)
	require.True(t, explicit)
	require.Equal(t, service.ClientSetupSelection{
		ClientID: "codex", ClientVersionKey: "cli:0.151.0",
		Protocol: "responses", ModelID: "gpt-5.6-sol", OS: "macos",
	}, selection)

	req.OS = ""
	_, _, err = setupSelectionFromRequest(req)
	require.ErrorIs(t, err, service.ErrInvalidClientSetupSelection)
}

func TestSimpleSetupOptionRequestContainsOnlyKeyClientAndOS(t *testing.T) {
	apiKeyID := int64(42)
	req := clientSetupTicketRequest{APIKeyID: &apiKeyID, ClientID: "codex", OS: "macos"}
	require.True(t, isSimpleSetupOptionRequest(req))

	req.ModelID = "gpt-5.6-sol"
	require.False(t, isSimpleSetupOptionRequest(req))
	req.ModelID = ""
	req.Target = "codex"
	require.False(t, isSimpleSetupOptionRequest(req))
}
