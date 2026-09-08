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
	34: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformGrok, protocol: "chat_completions", preferredModel: "grok-4.6"}},
	57: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformGemini, protocol: "generate_content", preferredModel: "gemini-3.7-flash"}},
	60: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "glm-5.3"}},
	61: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "responses", preferredModel: "deepseek-v4-pro-0813"}},
	62: {
		{clientID: "kimi-code", clientName: "Kimi Code", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "kimi-k3"},
		{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "kimi-k3"},
	},
	63: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "chat_completions", preferredModel: "minimax-m3"}},
	64: {{clientID: "opencode", clientName: "OpenCode", platform: PlatformOpenAI, protocol: "responses", preferredModel: "qwen3.8-max"}},
}

type ClientSetupOption struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
}

// SetupOptions returns only fully covered choices. Partial model coverage is
// an internal failure state and never becomes a customer-visible option.
func (s *ClientSetupService) SetupOptions(ctx context.Context, userID, apiKeyID int64, osName string) ([]ClientSetupOption, error) {
	key, err := s.setupAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	options := make([]ClientSetupOption, 0, len(releasedSetupGroups[key.Group.ID]))
	for _, config := range releasedSetupGroups[key.Group.ID] {
		if _, _, ok := s.setupSelection(ctx, key, osName, config.clientID); ok {
			options = append(options, ClientSetupOption{ClientID: config.clientID, Name: config.clientName})
		}
	}
	return options, nil
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
	if err != nil || key == nil || key.UserID != userID || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.Group == nil {
		return nil, ErrClientSetupKeyUnavailable
	}
	if key.IsMultiGroup() {
		return nil, ErrClientSetupSelectionUnavailable
	}
	return key, nil
}

func (s *ClientSetupService) setupSelection(ctx context.Context, key *APIKey, osName, clientID string) (ClientSetupSelection, releasedSetupGroup, bool) {
	if key == nil || key.Group == nil {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}
	var config releasedSetupGroup
	for _, candidate := range releasedSetupGroups[key.Group.ID] {
		if candidate.clientID == clientID {
			config = candidate
			break
		}
	}
	if config.clientID == "" || key.Group.Platform != config.platform {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}
	osName = strings.ToLower(strings.TrimSpace(osName))
	contract, ok := generatedClientSetupContracts[config.clientID]
	if !ok || contract.OneClickStatus != "ready" || !contract.Protocols[config.protocol] || !contract.OSReady[osName] {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}

	groupID := key.Group.ID
	models := s.models.GetAvailableModels(ctx, &groupID, "")
	visible := make([]string, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, raw := range models {
		model := strings.ToLower(strings.TrimSpace(raw))
		if model == "" || model == "codex-auto-review" || seen[model] || s.models.IsModelRestricted(ctx, groupID, model) {
			continue
		}
		seen[model] = true
		visible = append(visible, model)
	}
	if len(visible) == 0 {
		return ClientSetupSelection{}, releasedSetupGroup{}, false
	}
	for _, model := range visible {
		if !setupClientSupportsModel(config.clientID, config.protocol, model) {
			return ClientSetupSelection{}, releasedSetupGroup{}, false
		}
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

func setupClientSupportsModel(_ string, protocol, model string) bool {
	if protocol == "responses" {
		return generatedCodexSetupModels[model]
	}
	return generatedClientSetupModelProtocols[model][protocol]
}
