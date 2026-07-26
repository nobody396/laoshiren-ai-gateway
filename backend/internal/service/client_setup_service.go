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

	clientSetupTicketPurpose = "client_setup"
	clientSetupTicketTTL     = 10 * time.Minute
	clientSetupAPIBaseURL    = "https://api.laoshirenai.com"
)

var (
	ErrInvalidClientSetupTarget = infraerrors.BadRequest("INVALID_CLIENT_SETUP_TARGET", "不支持的一键安装目标")
	ErrClientSetupGroupMissing  = infraerrors.Forbidden("CLIENT_SETUP_GROUP_MISSING", "当前账户没有可用于该客户端的分组")
	ErrInvalidClientSetupTicket = infraerrors.Unauthorized("INVALID_CLIENT_SETUP_TICKET", "一键安装凭证无效、已过期或已使用")
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

// ClientSetupService creates dedicated per-client API keys lazily and hides
// their raw values behind a short-lived, one-time setup ticket.
type ClientSetupService struct {
	apiKeys *APIKeyService
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
	apiKey := value.(*APIKey)

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
		if keys[i].Name == name && keys[i].Group != nil && clientSetupGroupCompatible(target, keys[i].Group) {
			return &keys[i], nil
		}
	}

	groups, err := s.apiKeys.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	group := selectClientSetupGroup(target, groups)
	if group == nil {
		return nil, ErrClientSetupGroupMissing.WithMetadata(map[string]string{"target": target})
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
	default:
		return "", ErrInvalidClientSetupTarget
	}
}

func clientSetupKeyName(target string) string {
	if target == ClientSetupTargetClaude {
		return "一键安装 · Claude Code"
	}
	return "一键安装 · Codex"
}

func clientSetupGroupCompatible(target string, group *Group) bool {
	if group == nil || !group.IsActive() {
		return false
	}
	if target == ClientSetupTargetClaude {
		return group.Platform == PlatformAnthropic || group.Platform == PlatformAntigravity
	}
	return target == ClientSetupTargetCodex && group.Platform == PlatformOpenAI
}

func selectClientSetupGroup(target string, groups []Group) *Group {
	// GetAvailableGroups preserves the administrator-defined SortOrder. Do not
	// silently override that product decision based on a guessed billing mode.
	for i := range groups {
		if clientSetupGroupCompatible(target, &groups[i]) {
			group := groups[i]
			return &group
		}
	}
	return nil
}
