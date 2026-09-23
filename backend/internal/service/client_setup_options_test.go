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

}

// A key authorized for several groups reaches each client through the groups
// that can serve it. Codex is configured from the GPT groups; the Claude and
// Grok models the same key can use belong to their own clients and are not a
// Codex coverage failure.
func TestSetupOptionsScopeEachClientToItsOwnGroups(t *testing.T) {
	svc, key := multiGroupSetupService(map[int64][]string{
		5:  {"claude-opus-5", "claude-sonnet-5"},
		6:  {"gpt-6-astra", "gpt-5.6-sol"},
		34: {"grok-4.6"},
		58: {"gpt-5.5", "gpt-5.4"},
	})

	options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	// Ordered by the key's own group priority: Claude group, GPT standard,
	// Grok, GPT economy. OpenCode is released on both the Claude and Grok
	// groups and is configured for the higher-priority one.
	require.Equal(t, []ClientSetupOption{
		{ClientID: "claude-code", Name: "Claude Code"},
		{ClientID: "opencode", Name: "OpenCode"},
		{ClientID: "codex", Name: "Codex"},
		{ClientID: "workbuddy", Name: "WorkBuddy"},
		{ClientID: "grok-build", Name: "Grok Build"},
	}, options)

	// Codex bills through the first GPT group in the key's own priority order.
	ticket, err := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "codex", "macos")
	require.NoError(t, err)
	require.Equal(t, "responses", ticket.Protocol)
	require.Equal(t, "gpt-5.6-sol", ticket.ModelID)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, key.Key, credential.APIKey)
	require.Equal(t, "codex", credential.ClientID)
}

// Scoping must not weaken the fail-closed rule: an unverified model inside a
// client's own scope still removes that client, even though the same key has
// other groups that are fine.
func TestMultiGroupSetupOptionStillFailsClosedInsideItsOwnScope(t *testing.T) {
	svc, key := multiGroupSetupService(map[int64][]string{
		5:  {"claude-opus-5"},
		6:  {"gpt-5.6-sol", "future-unverified-model"},
		34: {"grok-4.6"},
		58: {"gpt-5.5"},
	})

	options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	for _, option := range options {
		require.NotEqual(t, "codex", option.ClientID)
	}
	require.Contains(t, options, ClientSetupOption{ClientID: "claude-code", Name: "Claude Code"})
	// Grok Build is released on the economy group only, so the unverified model
	// sitting in the standard group is outside its scope and does not remove it.
	require.Contains(t, options, ClientSetupOption{ClientID: "grok-build", Name: "Grok Build"})

	_, err = svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "codex", "macos")
	require.ErrorIs(t, err, ErrClientSetupSelectionUnavailable)
}

// A group whose live platform no longer matches its release entry is a
// configuration fault, not something to route around.
func TestMultiGroupSetupOptionRejectsPlatformDrift(t *testing.T) {
	svc, key := multiGroupSetupService(map[int64][]string{6: {"gpt-5.6-sol"}})
	stub, ok := svc.apiKeys.(*clientSetupAPIKeysStub)
	require.True(t, ok)
	stub.groups = []Group{{ID: 6, Platform: PlatformAnthropic, Status: StatusActive}}

	options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, "macos")
	require.NoError(t, err)
	require.Empty(t, options)
}

func multiGroupSetupService(modelsByGroup map[int64][]string) (*ClientSetupService, *APIKey) {
	platforms := map[int64]string{5: PlatformAnthropic, 6: PlatformOpenAI, 34: PlatformGrok, 58: PlatformOpenAI}
	groupIDs := []int64{5, 6, 34, 58}
	ordered := make([]int64, 0, len(modelsByGroup))
	for _, id := range groupIDs {
		if _, ok := modelsByGroup[id]; ok {
			ordered = append(ordered, id)
		}
	}
	groups := make([]Group, 0, len(ordered))
	for _, id := range ordered {
		groups = append(groups, Group{ID: id, Platform: platforms[id], Status: StatusActive})
	}
	key := &APIKey{ID: 41, UserID: 9, Key: "secret", Name: "multi", Status: StatusActive, GroupIDs: ordered}
	return &ClientSetupService{
		apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{key.ID: key}, groups: groups},
		tickets: newClientSetupTicketCacheStub(),
		models:  &clientSetupModelsStub{byGroup: modelsByGroup},
	}, key
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
			if tc.groupID == 34 {
				if osName == "linux" {
					require.NotContains(t, options, ClientSetupOption{ClientID: "workbuddy", Name: "WorkBuddy"})
				} else {
					require.Contains(t, options, ClientSetupOption{ClientID: "workbuddy", Name: "WorkBuddy"})
					workBuddyTicket, workBuddyErr := svc.IssueTicketForOption(context.Background(), key.UserID, key.ID, "workbuddy", osName)
					require.NoError(t, workBuddyErr)
					require.Equal(t, ClientSetupTargetWorkBuddy, workBuddyTicket.Target)
					require.Equal(t, "chat_completions", workBuddyTicket.Protocol)
				}
			}
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

func TestNewModelClientSelectionsAndTickets(t *testing.T) {
	for _, tc := range []struct {
		model, client, protocol, platform string
		group                             int64
	}{
		{"gpt-6-sol", "codex", "responses", PlatformOpenAI, 6},
		{"gpt-6-luna", "codex", "responses", PlatformOpenAI, 59},
		{"claude-opus-5-5", "claude-code", "messages", PlatformAnthropic, 5},
	} {
		for _, osName := range []string{"macos", "linux", "windows"} {
			t.Run(tc.model+"/"+osName, func(t *testing.T) {
				svc, key, _ := setupOptionTestService(tc.group, tc.platform, []string{tc.model})
				options, err := svc.SetupOptions(context.Background(), key.UserID, key.ID, osName)
				require.NoError(t, err)
				require.Contains(t, options, ClientSetupOption{ClientID: tc.client, Name: map[string]string{"codex": "Codex", "claude-code": "Claude Code"}[tc.client]})
				selection := ClientSetupSelection{ClientID: tc.client, ClientVersionKey: generatedClientSetupContracts[tc.client].VersionKey, ModelID: tc.model, Protocol: tc.protocol, OS: osName}
				ticket, err := svc.IssueTicketForSelection(context.Background(), key.UserID, key.ID, selection)
				require.NoError(t, err)
				credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
				require.NoError(t, err)
				require.Equal(t, tc.model, credential.ModelID)
				require.Equal(t, tc.protocol, credential.Protocol)
				_, err = svc.ExchangeTicket(context.Background(), ticket.Ticket)
				require.Error(t, err)
			})
		}
	}
}
