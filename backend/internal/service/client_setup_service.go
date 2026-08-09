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
	ClientSetupTargetClaude = "claude"
	ClientSetupTargetCodex  = "codex"
	ClientSetupTargetGrok   = "grok"

	clientSetupTicketPurpose = "client_setup"
	clientSetupTicketTTL     = 10 * time.Minute
	clientSetupAPIBaseURL    = "https://api.laoshirenai.com"

	clientSetupClaudeGroupName = "MAX 20X"
	clientSetupCodexGroupName  = "Pro 20X"
	clientSetupGrokGroupName   = "Grok 4.5"
)

var (
	ErrInvalidClientSetupTarget  = infraerrors.BadRequest("INVALID_CLIENT_SETUP_TARGET", "不支持的一键安装目标")
	ErrClientSetupGroupMissing   = infraerrors.Forbidden("CLIENT_SETUP_GROUP_MISSING", "当前账户没有可用于该客户端的分组")
	ErrClientSetupKeyUnavailable = infraerrors.Forbidden("CLIENT_SETUP_KEY_UNAVAILABLE", "当前 API 密钥无法用于一键配置")
	ErrInvalidClientSetupTicket  = infraerrors.Unauthorized("INVALID_CLIENT_SETUP_TICKET", "一键安装凭证无效、已过期或已使用")
)

type ClientSetupTicket struct {
	Ticket    string
	ExpiresIn int
	Target    string
	KeyName   string
	GroupName string
}

type ClientSetupCredential struct {
	Target  string
	APIKey  string
	BaseURL string
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
	apiKeys clientSetupAPIKeyService
	tickets SSOTicketCache
	ensure  singleflight.Group
}

func NewClientSetupService(apiKeys *APIKeyService, tickets SSOTicketCache) *ClientSetupService {
	return &ClientSetupService{apiKeys: apiKeys, tickets: tickets}
}

func (s *ClientSetupService) IssueTicket(ctx context.Context, userID int64, target string) (*ClientSetupTicket, error) {
	target, err := normalizeClientSetupTarget(target)
	if err != nil {
		return nil, err
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

	return s.issueTicketForAPIKey(ctx, userID, target, apiKey)
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

	return s.issueTicketForAPIKey(ctx, userID, target, apiKey)
}

func (s *ClientSetupService) issueTicketForAPIKey(ctx context.Context, userID int64, target string, apiKey *APIKey) (*ClientSetupTicket, error) {
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
	if err := s.tickets.StoreSSOTicket(ctx, ticket, data, clientSetupTicketTTL); err != nil {
		return nil, fmt.Errorf("store client setup ticket: %w", err)
	}

	groupName := ""
	if apiKey.Group != nil {
		groupName = apiKey.Group.Name
	}
	return &ClientSetupTicket{
		Ticket:    ticket,
		ExpiresIn: int(clientSetupTicketTTL.Seconds()),
		Target:    target,
		KeyName:   apiKey.Name,
		GroupName: groupName,
	}, nil
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
	if data.Purpose != clientSetupTicketPurpose ||
		data.APIKeyID == nil ||
		data.CreatedAt.IsZero() ||
		data.CreatedAt.After(now.Add(5*time.Second)) ||
		now.Sub(data.CreatedAt) > clientSetupTicketTTL {
		return nil, ErrInvalidClientSetupTicket
	}
	target, err := normalizeClientSetupTarget(data.TargetKind)
	if err != nil {
		return nil, ErrInvalidClientSetupTicket
	}

	apiKey, err := s.apiKeys.GetByID(ctx, *data.APIKeyID)
	if err != nil {
		return nil, ErrInvalidClientSetupTicket
	}
	if apiKey.UserID != data.UserID ||
		apiKey.Status != StatusActive ||
		apiKey.Group == nil ||
		!clientSetupGroupCompatible(target, apiKey.Group) {
		return nil, ErrInvalidClientSetupTicket
	}

	baseURL := clientSetupAPIBaseURL
	if apiKey.Group.Platform == PlatformAntigravity {
		baseURL += "/antigravity"
	}
	return &ClientSetupCredential{
		Target:  target,
		APIKey:  apiKey.Key,
		BaseURL: baseURL,
	}, nil
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
	groupName := strings.ToLower(strings.Join(strings.Fields(group.Name), " "))
	requiredName := strings.ToLower(clientSetupRequiredGroupName(target))
	return groupName == requiredName ||
		groupName == requiredName+" 分组" ||
		strings.Contains(groupName, requiredName)
}

func selectClientSetupGroup(target string, groups []Group) *Group {
	// One-click onboarding is a fixed product rule: Claude Code keys use MAX
	// 20X, Codex keys use Pro 20X, and Grok Build keys use the public Grok 4.5
	// balance group. Subscription groups are deliberately not selected here.
	for i := range groups {
		if clientSetupGroupMatchesTarget(target, &groups[i]) {
			group := groups[i]
			return &group
		}
	}
	return nil
}
