package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"golang.org/x/sync/singleflight"
)

const (
	ClientSetupTargetClaude    = "claude"
	ClientSetupTargetCodex     = "codex"
	ClientSetupTargetGrok      = "grok"
	ClientSetupTargetGemini    = "gemini"
	ClientSetupTargetKimi      = "kimi"
	ClientSetupTargetOpenCode  = "opencode"
	ClientSetupTargetZCode     = "zcode"
	ClientSetupTargetWorkBuddy = "workbuddy"

	clientSetupTicketPurpose          = "client_setup"
	clientSetupSelectionTicketPurpose = "client_setup_selection_v1"
	clientSetupTicketTTL              = 10 * time.Minute
	clientSetupAPIBaseURL             = "https://api.laoshirenai.com"

	clientSetupDefaultClaudeGroupName = "MAX 20X"
	clientSetupDefaultCodexGroupName  = "Pro 20X"
)

var (
	clientSetupGrokGroupPolicy  = generatedCatalogGroupPolicyFor(PlatformGrok)
	clientSetupGrokGroupName    = clientSetupGrokGroupPolicy.Preferred
	clientSetupLegacyGrokGroups = clientSetupGrokGroupPolicy.Legacy
	clientSetupClaudeGroupName  = preferredCatalogGroupOrDefault(PlatformAnthropic, clientSetupDefaultClaudeGroupName)
	clientSetupCodexGroupName   = preferredCatalogGroupOrDefault(PlatformOpenAI, clientSetupDefaultCodexGroupName)

	ErrInvalidClientSetupTarget        = infraerrors.BadRequest("INVALID_CLIENT_SETUP_TARGET", "不支持的一键安装目标")
	ErrClientSetupGroupMissing         = infraerrors.Forbidden("CLIENT_SETUP_GROUP_MISSING", "当前账户没有可用于该客户端的分组")
	ErrClientSetupKeyUnavailable       = infraerrors.Forbidden("CLIENT_SETUP_KEY_UNAVAILABLE", "当前 API 密钥无法用于一键配置")
	ErrInvalidClientSetupSelection     = infraerrors.BadRequest("INVALID_CLIENT_SETUP_SELECTION", "一键配置选择不完整或不受支持")
	ErrClientSetupSelectionUnavailable = infraerrors.Forbidden("CLIENT_SETUP_SELECTION_UNAVAILABLE", "该客户端、版本、协议、模型与系统组合尚未开放一键配置")
	ErrInvalidClientSetupTicket        = infraerrors.Unauthorized("INVALID_CLIENT_SETUP_TICKET", "一键安装凭证无效、已过期或已使用")
)

func preferredCatalogGroupOrDefault(platform, fallback string) string {
	if preferred := generatedCatalogGroupPolicyFor(platform).Preferred; preferred != "" {
		return preferred
	}
	return fallback
}

type ClientSetupTicket struct {
	Ticket           string
	ExpiresIn        int
	Target           string
	KeyName          string
	GroupName        string
	ClientID         string
	ClientVersionKey string
	Protocol         string
	ModelID          string
	OS               string
}

type ClientSetupCredential struct {
	Target           string
	APIKey           string
	BaseURL          string
	ClientID         string
	ClientVersionKey string
	Protocol         string
	ModelID          string
	OS               string
}

type ClientSetupSelection struct {
	ClientID         string
	ClientVersionKey string
	Protocol         string
	ModelID          string
	OS               string
}

type ClientSetupModelDiscovery interface {
	GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string
	IsModelRestricted(ctx context.Context, groupID int64, model string) bool
}

type clientSetupAPIKeyService interface {
	Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error)
	GetByID(ctx context.Context, id int64) (*APIKey, error)
	GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error)
	List(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error)
}

// ClientSetupService binds either a lazily-created install key or a user-selected
// existing key to a short-lived, one-time setup ticket.
type ClientSetupService struct {
	apiKeys        clientSetupAPIKeyService
	tickets        SSOTicketCache
	models         ClientSetupModelDiscovery
	selectionReady func(ClientSetupSelection) bool
	ensure         singleflight.Group
}

func NewClientSetupService(apiKeys *APIKeyService, tickets SSOTicketCache, models ClientSetupModelDiscovery) *ClientSetupService {
	return &ClientSetupService{apiKeys: apiKeys, tickets: tickets, models: models}
}

func (s *ClientSetupService) IssueTicket(ctx context.Context, userID int64, target string) (*ClientSetupTicket, error) {
	target, err := normalizeClientSetupTarget(target)
	if err != nil {
		return nil, err
	}
	// Gemini one-click setup is only available for an existing Gemini-group key
	// (IssueTicketForAPIKey). The lazy create-key flow must never auto-provision
	// a Gemini group.
	if target == ClientSetupTargetGemini {
		return nil, ErrClientSetupKeyUnavailable
	}
	if s == nil || s.apiKeys == nil || s.tickets == nil || userID <= 0 {
		return nil, ErrInvalidClientSetupTicket
	}

	value, err, _ := s.ensure.Do(fmt.Sprintf("%d:%s", userID, target), func() (any, error) {
		return s.ensureAPIKey(ctx, userID, target)
	})
	if err != nil {
		return nil, err
	}
	apiKey, ok := value.(*APIKey)
	if !ok || apiKey == nil {
		return nil, fmt.Errorf("ensure client setup API key returned an unexpected value")
	}

	return s.issueTicketForAPIKey(ctx, userID, target, apiKey, nil)
}

// IssueTicketForAPIKey creates a short-lived, one-time setup ticket for an
// existing key owned by the current user. The target is derived from the key's
// group so callers cannot pair an OpenAI key with Claude Code, or vice versa.
func (s *ClientSetupService) IssueTicketForAPIKey(ctx context.Context, userID, apiKeyID int64) (*ClientSetupTicket, error) {
	if s == nil || s.apiKeys == nil || s.tickets == nil || userID <= 0 || apiKeyID <= 0 {
		return nil, ErrClientSetupKeyUnavailable
	}

	apiKey, err := s.apiKeys.GetByID(ctx, apiKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID || apiKey.Status != StatusActive || apiKey.Group == nil {
		return nil, ErrClientSetupKeyUnavailable
	}
	target := clientSetupTargetForGroup(apiKey.Group)
	if target == "" {
		return nil, ErrClientSetupKeyUnavailable
	}

	return s.issueTicketForAPIKey(ctx, userID, target, apiKey, nil)
}

func (s *ClientSetupService) IssueTicketForSelection(ctx context.Context, userID, apiKeyID int64, selection ClientSetupSelection) (*ClientSetupTicket, error) {
	if s == nil || s.apiKeys == nil || s.tickets == nil || userID <= 0 || apiKeyID <= 0 {
		return nil, ErrClientSetupKeyUnavailable
	}
	selection, err := normalizeClientSetupSelection(selection)
	if err != nil {
		return nil, err
	}
	if !s.isSelectionReady(selection) {
		return nil, ErrClientSetupSelectionUnavailable
	}
	apiKey, err := s.apiKeys.GetByID(ctx, apiKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != userID || apiKey.Status != StatusActive || apiKey.Group == nil {
		return nil, ErrClientSetupKeyUnavailable
	}
	if !s.apiKeyExposesModel(ctx, apiKey, selection.ModelID) {
		return nil, ErrClientSetupSelectionUnavailable
	}
	return s.issueTicketForAPIKey(ctx, userID, selection.ClientID, apiKey, &selection)
}

func (s *ClientSetupService) issueTicketForAPIKey(ctx context.Context, userID int64, target string, apiKey *APIKey, selection *ClientSetupSelection) (*ClientSetupTicket, error) {
	return s.issueTicketForAPIKeyWithPurpose(ctx, userID, target, apiKey, selection, "")
}

func (s *ClientSetupService) issueTicketForAPIKeyWithPurpose(ctx context.Context, userID int64, target string, apiKey *APIKey, selection *ClientSetupSelection, selectionPurpose string) (*ClientSetupTicket, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate client setup ticket: %w", err)
	}
	ticket := hex.EncodeToString(tokenBytes)
	apiKeyID := apiKey.ID
	data := SSOTicketData{
		Purpose:    clientSetupTicketPurpose,
		UserID:     userID,
		APIKeyID:   &apiKeyID,
		TargetKind: target,
		CreatedAt:  time.Now().UTC(),
	}
	if selection != nil {
		data.Purpose = clientSetupSelectionTicketPurpose
		if selectionPurpose != "" {
			data.Purpose = selectionPurpose
		}
		data.ClientID = selection.ClientID
		data.ClientVersionKey = selection.ClientVersionKey
		data.Protocol = selection.Protocol
		data.ModelID = selection.ModelID
		data.OS = selection.OS
	}
	if err := s.tickets.StoreSSOTicket(ctx, ticket, data, clientSetupTicketTTL); err != nil {
		return nil, fmt.Errorf("store client setup ticket: %w", err)
	}

	groupName := ""
	if apiKey.Group != nil {
		groupName = apiKey.Group.Name
	}
	result := &ClientSetupTicket{
		Ticket:    ticket,
		ExpiresIn: int(clientSetupTicketTTL.Seconds()),
		Target:    target,
		KeyName:   apiKey.Name,
		GroupName: groupName,
	}
	if selection != nil {
		result.ClientID = selection.ClientID
		result.ClientVersionKey = selection.ClientVersionKey
		result.Protocol = selection.Protocol
		result.ModelID = selection.ModelID
		result.OS = selection.OS
	}
	return result, nil
}

func (s *ClientSetupService) ExchangeTicket(ctx context.Context, ticket string) (*ClientSetupCredential, error) {
	if s == nil || s.apiKeys == nil || s.tickets == nil {
		return nil, ErrInvalidClientSetupTicket
	}
	ticket = strings.TrimSpace(ticket)
	if len(ticket) != 64 {
		return nil, ErrInvalidClientSetupTicket
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		return nil, ErrInvalidClientSetupTicket
	}

	data, err := s.tickets.ConsumeSSOTicket(ctx, ticket)
	if err != nil {
		if errors.Is(err, ErrSSOTicketNotFound) {
			return nil, ErrInvalidClientSetupTicket
		}
		return nil, fmt.Errorf("consume client setup ticket: %w", err)
	}
	now := time.Now().UTC()
	if (data.Purpose != clientSetupTicketPurpose && data.Purpose != clientSetupSelectionTicketPurpose && data.Purpose != clientSetupOptionTicketPurpose) ||
		data.APIKeyID == nil ||
		data.CreatedAt.IsZero() ||
		data.CreatedAt.After(now.Add(5*time.Second)) ||
		now.Sub(data.CreatedAt) > clientSetupTicketTTL {
		return nil, ErrInvalidClientSetupTicket
	}
	apiKey, err := s.apiKeys.GetByID(ctx, *data.APIKeyID)
	if err != nil {
		return nil, ErrInvalidClientSetupTicket
	}
	if apiKey.UserID != data.UserID || apiKey.Status != StatusActive || len(setupKeyGroupIDs(apiKey)) == 0 {
		return nil, ErrInvalidClientSetupTicket
	}

	selection := ClientSetupSelection{
		ClientID: data.ClientID, ClientVersionKey: data.ClientVersionKey,
		Protocol: data.Protocol, ModelID: data.ModelID, OS: data.OS,
	}
	explicit := selectionHasAnyField(selection)
	target := ""
	if explicit {
		selection, err = normalizeClientSetupSelection(selection)
		validPurpose := data.Purpose == clientSetupSelectionTicketPurpose || data.Purpose == clientSetupOptionTicketPurpose
		if err != nil || !validPurpose || data.TargetKind != selection.ClientID ||
			!s.isSelectionReady(selection) || !s.apiKeyExposesModel(ctx, apiKey, selection.ModelID) {
			return nil, ErrInvalidClientSetupTicket
		}
		if data.Purpose == clientSetupOptionTicketPurpose {
			expected, _, ok := s.setupSelection(ctx, apiKey, selection.OS, selection.ClientID)
			if !ok || expected != selection {
				return nil, ErrInvalidClientSetupTicket
			}
		}
		target = clientSetupInstallerTarget(selection.ClientID)
		if target == "" {
			return nil, ErrInvalidClientSetupTicket
		}
	} else {
		target, err = normalizeClientSetupTarget(data.TargetKind)
		if err != nil || data.Purpose != clientSetupTicketPurpose || !clientSetupGroupCompatible(target, apiKey.Group) {
			return nil, ErrInvalidClientSetupTicket
		}
	}

	baseURL := clientSetupAPIBaseURL
	if apiKey.Group != nil && apiKey.Group.Platform == PlatformAntigravity {
		baseURL += "/antigravity"
	}
	credential := &ClientSetupCredential{
		Target:  target,
		APIKey:  apiKey.Key,
		BaseURL: baseURL,
	}
	if explicit {
		credential.ClientID = selection.ClientID
		credential.ClientVersionKey = selection.ClientVersionKey
		credential.Protocol = selection.Protocol
		credential.ModelID = selection.ModelID
		credential.OS = selection.OS
	}
	return credential, nil
}

func clientSetupInstallerTarget(clientID string) string {
	switch clientID {
	case "claude-code":
		return ClientSetupTargetClaude
	case "codex":
		return ClientSetupTargetCodex
	case "grok-build":
		return ClientSetupTargetGrok
	case "kimi-code":
		return ClientSetupTargetKimi
	case "opencode":
		return ClientSetupTargetOpenCode
	case "zcode":
		return ClientSetupTargetZCode
	case "workbuddy":
		return ClientSetupTargetWorkBuddy
	case "gemini-cli":
		return ClientSetupTargetGemini
	default:
		return ""
	}
}

func selectionHasAnyField(selection ClientSetupSelection) bool {
	return strings.TrimSpace(selection.ClientID) != "" || strings.TrimSpace(selection.ClientVersionKey) != "" ||
		strings.TrimSpace(selection.Protocol) != "" || strings.TrimSpace(selection.ModelID) != "" || strings.TrimSpace(selection.OS) != ""
}

func normalizeClientSetupSelection(selection ClientSetupSelection) (ClientSetupSelection, error) {
	selection.ClientID = strings.ToLower(strings.TrimSpace(selection.ClientID))
	selection.ClientVersionKey = strings.TrimSpace(selection.ClientVersionKey)
	selection.Protocol = strings.ToLower(strings.TrimSpace(selection.Protocol))
	selection.ModelID = strings.ToLower(strings.TrimSpace(selection.ModelID))
	selection.OS = strings.ToLower(strings.TrimSpace(selection.OS))
	if selection.ClientID == "" || selection.ClientVersionKey == "" || selection.Protocol == "" || selection.ModelID == "" || selection.OS == "" {
		return ClientSetupSelection{}, ErrInvalidClientSetupSelection
	}
	return selection, nil
}

func (s *ClientSetupService) isSelectionReady(selection ClientSetupSelection) bool {
	if !generatedClientSetupModelProtocols[selection.ModelID][selection.Protocol] {
		return false
	}
	if s != nil && s.selectionReady != nil {
		return s.selectionReady(selection)
	}
	contract, ok := generatedClientSetupContracts[selection.ClientID]
	if !ok || contract.OneClickStatus != "ready" || contract.VersionKey != selection.ClientVersionKey ||
		!contract.Protocols[selection.Protocol] || !contract.OSReady[selection.OS] {
		return false
	}
	return true
}

func (s *ClientSetupService) apiKeyExposesModel(ctx context.Context, apiKey *APIKey, modelID string) bool {
	if s == nil || s.models == nil || apiKey == nil {
		return false
	}
	if apiKey.Group != nil && apiKey.Group.Platform == PlatformUniversal {
		return containsModelName(apiKey.Group.UniversalPublicModels(), modelID)
	}
	// One authorized group offering the model unrestricted is enough: that is
	// the same group the gateway would route the request to.
	for _, groupID := range setupKeyGroupIDs(apiKey) {
		if s.models.IsModelRestricted(ctx, groupID, modelID) {
			continue
		}
		if containsModelName(s.models.GetAvailableModels(ctx, &groupID, ""), modelID) {
			return true
		}
	}
	return false
}

func containsModelName(models []string, modelID string) bool {
	for _, model := range models {
		if strings.EqualFold(strings.TrimSpace(model), modelID) {
			return true
		}
	}
	return false
}

func clientSetupTargetForGroup(group *Group) string {
	if group == nil || !group.IsActive() {
		return ""
	}
	switch group.Platform {
	case PlatformOpenAI:
		return ClientSetupTargetCodex
	case PlatformAnthropic, PlatformAntigravity:
		return ClientSetupTargetClaude
	case PlatformGrok:
		return ClientSetupTargetGrok
	case PlatformGemini:
		return ClientSetupTargetGemini
	default:
		return ""
	}
}

func (s *ClientSetupService) ensureAPIKey(ctx context.Context, userID int64, target string) (*APIKey, error) {
	name := clientSetupKeyName(target)
	keys, _, err := s.apiKeys.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100}, APIKeyListFilters{
		Search: name,
		Status: StatusActive,
	})
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].Name == name && keys[i].Group != nil && clientSetupGroupMatchesTarget(target, keys[i].Group) {
			return &keys[i], nil
		}
	}

	groups, err := s.apiKeys.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	group := selectClientSetupGroup(target, groups)
	if group == nil {
		return nil, ErrClientSetupGroupMissing.WithMetadata(map[string]string{
			"target":         target,
			"required_group": clientSetupRequiredGroupDescription(target),
		})
	}
	apiKey, err := s.apiKeys.Create(ctx, userID, CreateAPIKeyRequest{
		Name:    name,
		GroupID: &group.ID,
	})
	if err != nil {
		return nil, err
	}
	apiKey.Group = group
	return apiKey, nil
}

func normalizeClientSetupTarget(target string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case ClientSetupTargetClaude:
		return ClientSetupTargetClaude, nil
	case ClientSetupTargetCodex:
		return ClientSetupTargetCodex, nil
	case ClientSetupTargetGrok:
		return ClientSetupTargetGrok, nil
	case ClientSetupTargetGemini:
		return ClientSetupTargetGemini, nil
	default:
		return "", ErrInvalidClientSetupTarget
	}
}

func clientSetupKeyName(target string) string {
	switch target {
	case ClientSetupTargetClaude:
		return "一键安装 · Claude Code"
	case ClientSetupTargetGrok:
		return "一键安装 · Grok Build"
	case ClientSetupTargetGemini:
		return "一键安装 · Gemini CLI"
	default:
		return "一键安装 · Codex"
	}
}

func clientSetupGroupCompatible(target string, group *Group) bool {
	if group == nil || !group.IsActive() {
		return false
	}
	if target == ClientSetupTargetClaude {
		return group.Platform == PlatformAnthropic || group.Platform == PlatformAntigravity
	}
	if target == ClientSetupTargetCodex {
		return group.Platform == PlatformOpenAI
	}
	if target == ClientSetupTargetGemini {
		return group.Platform == PlatformGemini
	}
	return target == ClientSetupTargetGrok && group.Platform == PlatformGrok
}

func clientSetupRequiredGroupName(target string) string {
	if target == ClientSetupTargetClaude {
		return clientSetupClaudeGroupName
	}
	if target == ClientSetupTargetGrok {
		return clientSetupGrokGroupName
	}
	return clientSetupCodexGroupName
}

func clientSetupRequiredGroupDescription(target string) string {
	return clientSetupRequiredGroupName(target)
}

func clientSetupGroupMatchesTarget(target string, group *Group) bool {
	if !clientSetupGroupCompatible(target, group) || group.IsSubscriptionType() {
		return false
	}
	if target == ClientSetupTargetGrok {
		if clientSetupGrokGroupNameMatches(group.Name, clientSetupGrokGroupName) {
			return true
		}
		for _, legacyName := range clientSetupLegacyGrokGroups {
			if clientSetupGrokGroupNameMatches(group.Name, legacyName) {
				return true
			}
		}
		return false
	}
	return clientSetupGroupNameMatches(group.Name, clientSetupRequiredGroupName(target))
}

func clientSetupGroupNameMatches(name, required string) bool {
	groupName := strings.ToLower(strings.Join(strings.Fields(name), " "))
	requiredName := strings.ToLower(required)
	return groupName == requiredName ||
		groupName == requiredName+" 分组" ||
		strings.Contains(groupName, requiredName)
}

func clientSetupGrokGroupNameMatches(name, required string) bool {
	groupName := strings.ToLower(strings.Join(strings.Fields(name), " "))
	requiredName := strings.ToLower(strings.Join(strings.Fields(required), " "))
	return groupName == requiredName || groupName == requiredName+" 分组"
}

func clientSetupGrokGroupNameIsVersioned(name string) bool {
	return strings.ContainsAny(name, "0123456789")
}

func selectClientSetupGroup(target string, groups []Group) *Group {
	// One-click onboarding is a fixed product rule: Claude Code keys use MAX
	// 20X, Codex keys use Pro 20X, and Grok Build keys prefer the version-neutral
	// additive balance group. Version-named groups remain rollout-safe fallbacks
	// for existing installations; subscription groups are deliberately excluded.
	if target == ClientSetupTargetGrok {
		for i := range groups {
			if clientSetupGroupCompatible(target, &groups[i]) &&
				!groups[i].IsSubscriptionType() &&
				clientSetupGrokGroupNameMatches(groups[i].Name, clientSetupGrokGroupName) {
				group := groups[i]
				return &group
			}
		}
		// A version-neutral legacy group remains a safer fallback than a
		// model-version group: it survives catalog upgrades without silently
		// pinning newly created setup keys to the previous release. Preserve the
		// catalog order within each class so the newest versioned fallback still
		// wins when no neutral group exists.
		for _, versioned := range []bool{false, true} {
			for _, legacyName := range clientSetupLegacyGrokGroups {
				if clientSetupGrokGroupNameIsVersioned(legacyName) != versioned {
					continue
				}
				for i := range groups {
					if clientSetupGroupCompatible(target, &groups[i]) &&
						!groups[i].IsSubscriptionType() &&
						clientSetupGrokGroupNameMatches(groups[i].Name, legacyName) {
						group := groups[i]
						return &group
					}
				}
			}
		}
		return nil
	}
	for i := range groups {
		if clientSetupGroupMatchesTarget(target, &groups[i]) {
			group := groups[i]
			return &group
		}
	}
	return nil
}
