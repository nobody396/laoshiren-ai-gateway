package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const MaxAPIKeyGroups = 100

func (k *APIKey) IsMultiGroup() bool { return k != nil && len(k.GroupIDs) > 0 }

func validateAPIKeyGroupIDs(groupID *int64, ids []int64) error {
	if ids == nil {
		return nil
	}
	if groupID != nil || len(ids) == 0 || len(ids) > MaxAPIKeyGroups {
		return infraerrors.BadRequest("INVALID_KEY_GROUPS", "Select 1–100 groups, without group_id")
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return infraerrors.BadRequest("INVALID_KEY_GROUPS", "Group IDs must be positive and unique")
		}
		seen[id] = true
	}
	return nil
}

func (s *APIKeyService) validateMultiGroupAccess(ctx context.Context, user *User, groupID *int64, ids []int64) error {
	if err := validateAPIKeyGroupIDs(groupID, ids); err != nil {
		return err
	}
	if ids == nil {
		return nil
	}
	if user == nil {
		return ErrUserNotFound
	}
	for _, id := range ids {
		g, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("get authorized group: %w", err)
		}
		if g == nil || !g.IsActive() || g.IsUniversal() || !s.canUserBindGroup(ctx, user, g) {
			return ErrGroupNotAllowed
		}
	}
	return nil
}

// AuthorizeMultiGroupTarget re-reads the payer: cached identity snapshots do not
// contain exclusive-group grants. Expired subscriptions fail here, never falling
// through to another group's wallet merely because that wallet has funds.
func (s *APIKeyService) AuthorizeMultiGroupTarget(ctx context.Context, key *APIKey, group *Group) (*User, error) {
	if key == nil || key.User == nil || group == nil || !slices.Contains(key.GroupIDs, group.ID) || !group.IsActive() || group.IsUniversal() {
		return nil, ErrGroupNotAllowed
	}
	user, err := s.userRepo.GetByID(ctx, key.User.ID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() || !s.canUserBindGroup(ctx, user, group) {
		return nil, ErrGroupNotAllowed
	}
	return user, nil
}

// MultiGroupModels uses declared rate cards, not transient account health. A
// failing first route must not silently move a request to a differently-priced
// group. Groups without a declared model catalog are deliberately not guessed.
func (s *GatewayService) MultiGroupModels(ctx context.Context, group *Group) ([]string, error) {
	if s.channelService == nil || group == nil {
		return nil, infraerrors.ServiceUnavailable("MULTI_GROUP_CATALOG_UNAVAILABLE", "Multi-group model catalog is unavailable")
	}
	cache, err := s.channelService.loadCache(ctx)
	if err != nil {
		return nil, err
	}
	channel := cache.channelByGroupID[group.ID]
	if channel == nil || !channel.IsActive() {
		// A missing/disabled catalog is unknown, not proof the first group does
		// not serve this model. Never skip it and silently switch funding sources.
		return nil, infraerrors.ServiceUnavailable("MULTI_GROUP_CATALOG_UNAVAILABLE", "An authorized group has no active declared model catalog")
	}
	var models []string
	for _, pricing := range channel.ModelPricing {
		if !slices.Contains(matchingPlatforms(group.Platform), pricing.Platform) {
			continue
		}
		for _, model := range pricing.Models {
			if model != "" {
				models = append(models, model)
			}
		}
	}
	// Keep wildcard declarations for request admission; expand only discovery
	// names from current account mappings, never from a hard-coded model list.
	if slices.ContainsFunc(models, func(m string) bool { return strings.HasSuffix(m, "*") }) && s.accountRepo != nil {
		declared := append([]string(nil), models...)
		for _, model := range s.GetAvailableModels(ctx, &group.ID, group.Platform) {
			if MultiGroupModelMatches(declared, model) {
				models = append(models, model)
			}
		}
	}
	slices.Sort(models)
	return slices.Compact(models), nil
}

// MultiGroupProtocolSupported preserves existing explicit protocol admission;
// it does not create a new adapter or infer compatibility from a model name.
func MultiGroupProtocolSupported(g *Group, protocol string) bool {
	if g == nil || g.IsUniversal() {
		return false
	}
	switch protocol {
	case "messages":
		return g.Platform == PlatformAnthropic || g.Platform == PlatformAntigravity || g.Platform == PlatformGrok || (g.Platform == PlatformOpenAI && g.AllowMessagesDispatch)
	case "responses":
		return g.Platform == PlatformOpenAI || g.Platform == PlatformGrok
	case "chat_completions":
		return g.Platform == PlatformOpenAI || g.Platform == PlatformGrok || g.Platform == PlatformGemini
	case "generate_content":
		return g.Platform == PlatformGemini || g.Platform == PlatformAntigravity
	case "":
		return g.Platform == PlatformOpenAI || g.Platform == PlatformAnthropic || g.Platform == PlatformGemini || g.Platform == PlatformAntigravity || g.Platform == PlatformGrok
	default:
		return false
	}
}

// MultiGroupModelMatches follows the same exact/prefix declarations as the
// channel rate card, while future model names remain data rather than code.
func MultiGroupModelMatches(patterns []string, model string) bool {
	for _, pattern := range patterns {
		if strings.EqualFold(pattern, model) {
			return true
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(strings.ToLower(model), strings.ToLower(strings.TrimSuffix(pattern, "*"))) {
			return true
		}
	}
	return false
}
