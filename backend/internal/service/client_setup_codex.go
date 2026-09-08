package service

import (
	"context"
	"strings"
)

const clientSetupCodexOptionTicketPurpose = "client_setup_codex_option_v1"

var codexSetupGroupIDs = map[int64]struct{}{
	6:  {}, // GPT 标准线路
	58: {}, // GPT 经济线路
	59: {}, // GPT 企业高速线路
}

type ClientSetupOption struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
}

// SetupOptions intentionally returns only fully ready choices. Partial model
// coverage is an internal QA state and must never become a customer option.
func (s *ClientSetupService) SetupOptions(ctx context.Context, userID, apiKeyID int64, osName string) ([]ClientSetupOption, error) {
	key, err := s.setupAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	if _, ok := s.codexSetupSelection(ctx, key, osName); !ok {
		return []ClientSetupOption{}, nil
	}
	return []ClientSetupOption{{ClientID: "codex", Name: "Codex"}}, nil
}

func (s *ClientSetupService) IssueTicketForOption(ctx context.Context, userID, apiKeyID int64, clientID, osName string) (*ClientSetupTicket, error) {
	if strings.ToLower(strings.TrimSpace(clientID)) != "codex" {
		return nil, ErrClientSetupSelectionUnavailable
	}
	key, err := s.setupAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	selection, ok := s.codexSetupSelection(ctx, key, osName)
	if !ok {
		return nil, ErrClientSetupSelectionUnavailable
	}
	return s.issueTicketForAPIKeyWithPurpose(ctx, userID, ClientSetupTargetCodex, key, &selection, clientSetupCodexOptionTicketPurpose)
}

func (s *ClientSetupService) setupAPIKey(ctx context.Context, userID, apiKeyID int64) (*APIKey, error) {
	if s == nil || s.apiKeys == nil || s.tickets == nil || s.models == nil || userID <= 0 || apiKeyID <= 0 {
		return nil, ErrClientSetupKeyUnavailable
	}
	key, err := s.apiKeys.GetByID(ctx, apiKeyID)
	if err != nil || key == nil || key.UserID != userID || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.Group == nil {
		return nil, ErrClientSetupKeyUnavailable
	}
	// The first release is deliberately single-group. A multi-group Key needs
	// an explicit group selector before it can safely own one client Provider.
	if key.IsMultiGroup() {
		return nil, ErrClientSetupSelectionUnavailable
	}
	return key, nil
}

func (s *ClientSetupService) codexSetupSelection(ctx context.Context, key *APIKey, osName string) (ClientSetupSelection, bool) {
	osName = strings.ToLower(strings.TrimSpace(osName))
	if key == nil || key.Group == nil || key.Group.Platform != PlatformOpenAI {
		return ClientSetupSelection{}, false
	}
	if _, ok := codexSetupGroupIDs[key.Group.ID]; !ok {
		return ClientSetupSelection{}, false
	}
	contract, ok := generatedClientSetupContracts["codex"]
	if !ok || contract.OneClickStatus != "ready" || !contract.Protocols["responses"] || !contract.OSReady[osName] {
		return ClientSetupSelection{}, false
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
		return ClientSetupSelection{}, false
	}
	for _, model := range visible {
		if !generatedCodexSetupModels[model] {
			return ClientSetupSelection{}, false
		}
	}

	defaultModel := visible[0]
	for _, model := range visible {
		if model == "gpt-5.6-sol" {
			defaultModel = model
			break
		}
	}
	return ClientSetupSelection{
		ClientID:         "codex",
		ClientVersionKey: contract.VersionKey,
		Protocol:         "responses",
		ModelID:          defaultModel,
		OS:               osName,
	}, true
}
