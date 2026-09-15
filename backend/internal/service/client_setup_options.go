package service

import (
	"context"
	"strings"
)

const clientSetupOptionTicketPurpose = "client_setup_option_v1"

type releasedSetupGroup struct {
	clientID       string
	clientName     string
	platform       string
	protocol       string
	preferredModel string
}

var releasedSetupGroups = map[int64][]releasedSetupGroup{
	5: {
		{clientID: "claude-code", clientName: "Claude Code", platform: PlatformAnthropic, protocol: "messages", preferredModel: "claude-opus-5"},
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformAnthropic, protocol: "messages", preferredModel: "claude-sonnet-5"},
	},
	15: {
		{clientID: "claude-code", clientName: "Claude Code", platform: PlatformAnthropic, protocol: "messages", preferredModel: "claude-opus-5"},
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformAnthropic, protocol: "messages", preferredModel: "claude-sonnet-5"},
	},
	65: {
		{clientID: "claude-code", clientName: "Claude Code", platform: PlatformAnthropic, protocol: "messages", preferredModel: "claude-opus-5"},
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformAnthropic, protocol: "messages", preferredModel: "claude-sonnet-5"},
	},
	6: {
		{clientID: "codex", clientName: "Codex", platform: PlatformOpenAI, protocol: "responses", preferredModel: "gpt-5.6-sol"},
	},
	58: {
		{clientID: "codex", clientName: "Codex", platform: PlatformOpenAI, protocol: "responses", preferredModel: "gpt-5.6-sol"},
		{clientID: "grok-build", clientName: "Grok Build", platform: PlatformOpenAI, protocol: "responses", preferredModel: "gpt-5.6-sol"},
	},
	59: {
		{clientID: "codex", clientName: "Codex", platform: PlatformOpenAI, protocol: "responses", preferredModel: "gpt-5.6-sol"},
	},
	34: {
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformGrok, protocol: "chat_completions", preferredModel: "grok-4.6"},
		{clientID: "workbuddy", clientName: "WorkBuddy", platform: PlatformGrok, protocol: "chat_completions", preferredModel: "grok-4.6"},
	},
	57: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformGemini, protocol: "generate_content", preferredModel: "gemini-3.7-flash"}},
	60: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "glm-5.3"}},
	61: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "responses", preferredModel: "deepseek-v4-pro-0813"}},
	62: {
		{clientID: "kimi-code", clientName: "Kimi Code", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "kimi-k3"},
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "kimi-k3"},
	},
	63: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "minimax-m3"}},
	64: {
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "responses", preferredModel: "qwen3.8-max"},
		{clientID: "zcode", clientName: "ZCode", platform: PlatformOpenAI, protocol: "responses", preferredModel: "qwen3.8-max"},
	},
}

type ClientSetupOption struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
}

// SetupOptions returns only fully covered choices. Partial model coverage is
// an internal failure state and never becomes a customer-visible option.
//
// Coverage is judged inside the client's own scope: the key's groups that are
// released for that client. A key authorized for several groups reaches each
// client through the groups that can actually serve it, and the models its
// other groups serve are out of scope rather than a coverage failure.
func (s *ClientSetupService) SetupOptions(ctx context.Context, userID, apiKeyID int64, osName string) ([]ClientSetupOption, error) {
	key, err := s.setupAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	options := []ClientSetupOption{}
	seen := map[string]bool{}
	for _, groupID := range setupKeyGroupIDs(key) {
		for _, config := range releasedSetupGroups[groupID] {
			if seen[config.clientID] {
				continue
			}
			seen[config.clientID] = true
			if _, _, ok := s.setupSelection(ctx, key, osName, config.clientID); ok {
				options = append(options, ClientSetupOption{ClientID: config.clientID, Name: config.clientName})
			}
		}
	}
	return options, nil
}

// setupKeyGroupIDs lists the key's groups in the same priority order the
// gateway routes them, so the group that would pay is also the group whose
// release entry configures the client.
func setupKeyGroupIDs(key *APIKey) []int64 {
	if key == nil {
		return nil
	}
	if key.IsMultiGroup() {
		return key.GroupIDs
	}
	if key.Group == nil {
		return nil
	}
	return []int64{key.Group.ID}
}

func (s *ClientSetupService) IssueTicketForOption(ctx context.Context, userID, apiKeyID int64, clientID, osName string) (*ClientSetupTicket, error) {
	key, err := s.setupAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	clientID = strings.ToLower(strings.TrimSpace(clientID))
	selection, _, ok := s.setupSelection(ctx, key, osName, clientID)
	if !ok {
		return nil, ErrClientSetupSelectionUnavailable
	}
	target := clientSetupInstallerTarget(selection.ClientID)
	if target == "" {
		return nil, ErrClientSetupSelectionUnavailable
	}
	ticket, err := s.issueTicketForAPIKeyWithPurpose(ctx, userID, selection.ClientID, key, &selection, clientSetupOptionTicketPurpose)
	if err != nil {
		return nil, err
	}
	// The stored target remains the exact client ID; the copied command only
	// needs the installer's compact target name.
	ticket.Target = target
	return ticket, nil
}

func (s *ClientSetupService) setupAPIKey(ctx context.Context, userID, apiKeyID int64) (*APIKey, error) {
	if s == nil || s.apiKeys == nil || s.tickets == nil || s.models == nil || userID <= 0 || apiKeyID <= 0 {
		return nil, ErrClientSetupKeyUnavailable
	}
	key, err := s.apiKeys.GetByID(ctx, apiKeyID)
	if err != nil || key == nil || key.UserID != userID || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return nil, ErrClientSetupKeyUnavailable
	}
	if len(setupKeyGroupIDs(key)) == 0 {
		return nil, ErrClientSetupKeyUnavailable
	}
	return key, nil
}

func (s *ClientSetupService) setupSelection(ctx context.Context, key *APIKey, osName, clientID string) (ClientSetupSelection, releasedSetupGroup, bool) {
	scope, config, ok := s.setupScope(ctx, key, clientID)
	if !ok {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}
	osName = strings.ToLower(strings.TrimSpace(osName))
	contract, okContract := generatedClientSetupContracts[config.clientID]
	if !okContract || contract.OneClickStatus != "ready" || !contract.Protocols[config.protocol] || !contract.OSReady[osName] {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}

	var visible []string
	seen := map[string]bool{}
	for _, groupID := range scope {
		for _, raw := range s.models.GetAvailableModels(ctx, &groupID, "") {
			model := strings.ToLower(strings.TrimSpace(raw))
			if model == "" || model == "codex-auto-review" || seen[model] || s.models.IsModelRestricted(ctx, groupID, model) {
				continue
			}
			// Every model inside this client's scope must be verified for it.
			// Partial coverage is an internal failure state, never a
			// customer-visible option.
			if !setupClientSupportsModel(config.clientID, config.protocol, model) {
				return ClientSetupSelection{}, releasedSetupGroup{}, false
			}
			seen[model] = true
			visible = append(visible, model)
		}
	}
	if len(visible) == 0 {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}

	defaultModel := visible[0]
	for _, model := range visible {
		if model == config.preferredModel {
			defaultModel = model
			break
		}
	}
	return ClientSetupSelection{
		ClientID:         config.clientID,
		ClientVersionKey: contract.VersionKey,
		Protocol:         config.protocol,
		ModelID:          defaultModel,
		OS:               osName,
	}, config, true
}

// setupScope returns the key's groups that are released for this client, in the
// order the gateway would route them, together with the release entry of the
// first such group. Everything this client is offered comes from that scope;
// models the key's other groups serve belong to other clients.
func (s *ClientSetupService) setupScope(ctx context.Context, key *APIKey, clientID string) ([]int64, releasedSetupGroup, bool) {
	groupIDs := setupKeyGroupIDs(key)
	if len(groupIDs) == 0 {
		return nil, releasedSetupGroup{}, false
	}
	platforms, err := s.setupGroupPlatforms(ctx, key)
	if err != nil {
		return nil, releasedSetupGroup{}, false
	}
	var scope []int64
	var config releasedSetupGroup
	for _, groupID := range groupIDs {
		for _, candidate := range releasedSetupGroups[groupID] {
			if candidate.clientID != clientID {
				continue
			}
			// A release entry names the platform its group must have. Drift
			// between that entry and the live group is a configuration fault,
			// not something to route around.
			if platforms[groupID] != candidate.platform {
				return nil, releasedSetupGroup{}, false
			}
			// A client that speaks several protocols is configured for the
			// one its highest-priority group uses; groups reached over a
			// different protocol belong to a different configuration.
			if config.clientID == "" {
				config = candidate
			} else if candidate.protocol != config.protocol {
				continue
			}
			scope = append(scope, groupID)
		}
	}
	if config.clientID == "" {
		return nil, releasedSetupGroup{}, false
	}
	return scope, config, true
}

func (s *ClientSetupService) setupGroupPlatforms(ctx context.Context, key *APIKey) (map[int64]string, error) {
	if !key.IsMultiGroup() {
		return map[int64]string{key.Group.ID: key.Group.Platform}, nil
	}
	groups, err := s.apiKeys.GetAvailableGroups(ctx, key.UserID)
	if err != nil {
		return nil, err
	}
	platforms := make(map[int64]string, len(groups))
	for i := range groups {
		if groups[i].IsActive() {
			platforms[groups[i].ID] = groups[i].Platform
		}
	}
	return platforms, nil
}

func setupClientSupportsModel(_ string, protocol, model string) bool {
	if protocol == "responses" {
		return generatedCodexSetupModels[model]
	}
	return generatedClientSetupModelProtocols[model][protocol]
}
