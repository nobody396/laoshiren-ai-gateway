package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupOptionTestService(groupID int64, platform string, models []string) (*ClientSetupService, *APIKey, *clientSetupTicketCacheStub) {
	group := &Group{ID: groupID, Name: "test", Platform: platform, Status: StatusActive}
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
				svc, key, _ := setupOptionTestService(groupID, PlatformOpenAI, models)
				options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, osName)
				require.NoError(t, err)
				expected := []ClientSetupOption{{ClientID: "codex", Name: "Codex"}}
				if groupID == 58 {
					expected = append(expected, ClientSetupOption{ClientID: "grok-build", Name: "Grok Build"})
				}
				require.Equal(t, expected, options)
			})
		}
	}
}

func TestGrokBuildSetupOptionImportsAllEconomicGroupModels(t *testing.T) {
	models := []string{"gpt-6-astra", "gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.5", "gpt-5.4"}
	svc, key, cache := setupOptionTestService(58, PlatformOpenAI, models)
	ticket, err := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "grok-build", "windows")
	require.NoError(t, err)
	require.Equal(t, clientSetupOptionTicketPurpose, cache.data[ticket.Ticket].Purpose)
	require.Equal(t, ClientSetupTargetGrok, ticket.Target)
	require.Equal(t, "responses", ticket.Protocol)
	require.Equal(t, "gpt-5.6-sol", ticket.ModelID)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, "grok-build", credential.ClientID)
	require.Equal(t, ClientSetupTargetGrok, credential.Target)
}

func TestCodexSetupOptionsFailClosedForOtherGroupUnknownModelAndMultiGroup(t *testing.T) {
	svc, key, _ := setupOptionTestService(40, PlatformOpenAI, []string{"gpt-5.6-sol"})
	options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	require.Empty(t, options)

	svc, key, _ = setupOptionTestService(6, PlatformOpenAI, []string{"gpt-5.6-sol", "future-unverified-model"})
	options, err = svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	require.Empty(t, options)

	svc, key, _ = setupOptionTestService(6, PlatformOpenAI, []string{"gpt-5.6-sol"})
	key.GroupIDs = []int64{6, 58}
	_, err = svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.ErrorIs(t, err, ErrClientSetupSelectionUnavailable)
}

func TestCodexSetupOptionTicketRevalidatesFullCoverageOnExchange(t *testing.T) {
	svc, key, cache := setupOptionTestService(58, PlatformOpenAI, []string{"gpt-6-astra", "gpt-5.6-sol", "gpt-5.4"})
	ticket, err := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "codex", "windows")
	require.NoError(t, err)
	require.Equal(t, clientSetupOptionTicketPurpose, cache.data[ticket.Ticket].Purpose)
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

func TestClaudeSetupOptionsCoverAllModelsForThreeReleasedGroupsAndOSes(t *testing.T) {
	standard := []string{
		"claude-fable-5", "claude-fable-5-1", "claude-haiku-4-5", "claude-opus-4-6", "claude-opus-4-7",
		"claude-opus-4-8", "claude-opus-5", "claude-sonnet-4-6", "claude-sonnet-5",
	}
	economy := []string{
		"claude-haiku-4-5", "claude-opus-4-5", "claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8",
		"claude-opus-5", "claude-sonnet-4-6", "claude-sonnet-5",
	}
	groups := map[int64][]string{5: standard, 15: standard, 65: economy}
	for groupID, models := range groups {
		for _, osName := range []string{"macos", "linux", "windows"} {
			t.Run(osName, func(t *testing.T) {
				svc, key, _ := setupOptionTestService(groupID, PlatformAnthropic, models)
				options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, osName)
				require.NoError(t, err)
				require.Equal(t, []ClientSetupOption{
					{ClientID: "claude-code", Name: "Claude Code"},
					{ClientID: "opencode", Name: "OpenCode"},
				}, options)
			})
		}
	}
}

func TestOpenCodeSetupOptionsCoverEveryReleasedGroupModelOnAllOSes(t *testing.T) {
	cases := []struct {
		groupID  int64
		platform string
		models   []string
		protocol string
	}{
		{34, PlatformGrok, []string{"grok-4.6", "grok-4.5"}, "chat_completions"},
		{57, PlatformGemini, []string{"gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.1-pro"}, "generate_content"},
		{60, PlatformOpenAI, []string{"glm-5.3", "glm-5.2"}, "chat_completions"},
		{61, PlatformOpenAI, []string{"deepseek-v4-pro-0813", "deepseek-v4-flash-0731"}, "responses"},
		{62, PlatformOpenAI, []string{"kimi-k3", "kimi-k2.7-code"}, "chat_completions"},
		{63, PlatformOpenAI, []string{"minimax-m3"}, "chat_completions"},
		{64, PlatformOpenAI, []string{"qwen3.8-max", "qwen3.7-max", "qwen3.7-plus", "qwen3.7-flash", "qwen3.6-plus", "qwen3.6-flash"}, "responses"},
	}
	for _, tc := range cases {
		for _, osName := range []string{"macos", "linux", "windows"} {
			svc, key, _ := setupOptionTestService(tc.groupID, tc.platform, tc.models)
			options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, osName)
			require.NoError(t, err)
			require.Contains(t, options, ClientSetupOption{ClientID: "opencode", Name: "OpenCode"})

			ticket, err := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "opencode", osName)
			require.NoError(t, err)
			require.Equal(t, ClientSetupTargetOpenCode, ticket.Target)
			require.Equal(t, tc.protocol, ticket.Protocol)
			if tc.groupID == 64 {
				require.Contains(t, options, ClientSetupOption{ClientID: "zcode", Name: "ZCode"})
				zcodeTicket, zcodeErr := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "zcode", osName)
				require.NoError(t, zcodeErr)
				require.Equal(t, ClientSetupTargetZCode, zcodeTicket.Target)
				require.Equal(t, "responses", zcodeTicket.Protocol)
			}
		}
	}
}

func TestClaudeSetupOptionTicketUsesMessagesAndRevalidatesFullCoverage(t *testing.T) {
	models := []string{"claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5"}
	svc, key, cache := setupOptionTestService(65, PlatformAnthropic, models)
	ticket, err := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "claude-code", "macos")
	require.NoError(t, err)
	require.Equal(t, clientSetupOptionTicketPurpose, cache.data[ticket.Ticket].Purpose)
	require.Equal(t, "claude-opus-5", ticket.ModelID)
	require.Equal(t, "messages", ticket.Protocol)
	require.Equal(t, ClientSetupTargetClaude, ticket.Target)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, "claude-code", credential.ClientID)
	require.Equal(t, "messages", credential.Protocol)

	ticket, err = svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "claude-code", "macos")
	require.NoError(t, err)
	svc.models = &clientSetupModelsStub{models: append(models, "future-unverified-model")}
	_, err = svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.ErrorIs(t, err, ErrInvalidClientSetupTicket)
}
