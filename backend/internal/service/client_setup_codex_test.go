package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func codexSetupTestService(groupID int64, models []string) (*ClientSetupService, *APIKey, *clientSetupTicketCacheStub) {
	group := &Group{ID: groupID, Name: "test", Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{ID: 41, UserID: 9, Key: "secret", Name: "selected", Status: StatusActive, Group: group}
	cache := newClientSetupTicketCacheStub()
	return &ClientSetupService{
		apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{key.ID: key}},
		tickets: cache,
		models:  &clientSetupModelsStub{models: models},
	}, key, cache
}

func TestCodexSetupOptionsCoverAllModelsForThreeReleasedGroupsAndOSes(t *testing.T) {
	groups := map[int64][]string{
		6:  {"gpt-6-astra", "gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.5", "gpt-5.4", "gpt-5.3-codex-spark"},
		58: {"gpt-6-astra", "gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.5", "gpt-5.4"},
		59: {"gpt-6-astra", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex-spark"},
	}
	for groupID, models := range groups {
		for _, osName := range []string{"macos", "linux", "windows"} {
			t.Run(osName, func(t *testing.T) {
				svc, key, _ := codexSetupTestService(groupID, models)
				options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, osName)
				require.NoError(t, err)
				require.Equal(t, []ClientSetupOption{{ClientID: "codex", Name: "Codex"}}, options)
			})
		}
	}
}

func TestCodexSetupOptionsFailClosedForOtherGroupUnknownModelAndMultiGroup(t *testing.T) {
	svc, key, _ := codexSetupTestService(40, []string{"gpt-5.6-sol"})
	options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	require.Empty(t, options)

	svc, key, _ = codexSetupTestService(6, []string{"gpt-5.6-sol", "future-unverified-model"})
	options, err = svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	require.Empty(t, options)

	svc, key, _ = codexSetupTestService(6, []string{"gpt-5.6-sol"})
	key.GroupIDs = []int64{6, 58}
	_, err = svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.ErrorIs(t, err, ErrClientSetupSelectionUnavailable)
}

func TestCodexSetupOptionTicketRevalidatesFullCoverageOnExchange(t *testing.T) {
	svc, key, cache := codexSetupTestService(58, []string{"gpt-6-astra", "gpt-5.6-sol", "gpt-5.4"})
	ticket, err := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "codex", "windows")
	require.NoError(t, err)
	require.Equal(t, clientSetupCodexOptionTicketPurpose, cache.data[ticket.Ticket].Purpose)
	require.Equal(t, "gpt-5.6-sol", ticket.ModelID)
	require.Equal(t, "responses", ticket.Protocol)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, key.Key, credential.APIKey)
	require.Equal(t, "codex", credential.ClientID)
	require.Equal(t, "windows", credential.OS)

	ticket, err = svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "codex", "windows")
	require.NoError(t, err)
	svc.models = &clientSetupModelsStub{models: []string{"gpt-5.6-sol", "future-unverified-model"}}
	_, err = svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.ErrorIs(t, err, ErrInvalidClientSetupTicket)
}
