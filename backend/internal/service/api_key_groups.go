package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/claude"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/gemini"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai"
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

// GroupModelDeclaration keeps discovery candidates and exact request admission
// in one immutable snapshot. Prefix arrays alone cannot represent mapped-price
// exceptions (e.g. family-* allowed but family-private denied).
type GroupModelDeclaration struct {
	Models  []string
	matches func(string, string) bool
}

// Matches reports whether any supported native protocol admits this name.
func (d *GroupModelDeclaration) Matches(group *Group, model string) bool {
	for _, protocol := range MultiGroupTextProtocols {
		if d.MatchesProtocol(group, protocol, model) {
			return true
		}
	}
	return false
}

var MultiGroupTextProtocols = []string{"messages", "responses", "chat_completions", "generate_content"}

func (d *GroupModelDeclaration) MatchesProtocol(group *Group, protocol, model string) bool {
	if d == nil || group == nil || !MultiGroupProtocolSupported(group, protocol) || IsDisabledPublicModelForGroup(model, group.ID) {
		return false
	}
	if d.matches != nil {
		return d.matches(protocol, model)
	}
	return MultiGroupRequestModelMatches(group, d.Models, model)
}

func (s *GatewayService) MultiGroupCatalog(ctx context.Context, group *Group) (*GroupModelDeclaration, error) {
	reader, _ := s.accountRepo.(GroupModelInventoryReader)
	return NewGroupModelCatalog(s.channelService, reader).Declaration(ctx, group)
}

func (s *GatewayService) MultiGroupModels(ctx context.Context, group *Group) ([]string, error) {
	d, err := s.MultiGroupCatalog(ctx, group)
	if err != nil {
		return nil, err
	}
	return d.Models, nil
}

type GroupModelCatalog struct {
	channelService *ChannelService
	reader         GroupModelInventoryReader
}

func NewGroupModelCatalog(channels *ChannelService, reader GroupModelInventoryReader) *GroupModelCatalog {
	return &GroupModelCatalog{channelService: channels, reader: reader}
}
func (catalog *GroupModelCatalog) Models(ctx context.Context, group *Group) ([]string, error) {
	d, err := catalog.Declaration(ctx, group)
	if err != nil {
		return nil, err
	}
	return d.Models, nil
}

func (catalog *GroupModelCatalog) Declaration(ctx context.Context, group *Group) (*GroupModelDeclaration, error) {
	unavailable := func() error {
		return infraerrors.ServiceUnavailable("MULTI_GROUP_CATALOG_UNAVAILABLE", "Multi-group model catalog is unavailable")
	}
	if group == nil || catalog.channelService == nil || catalog.reader == nil {
		return nil, unavailable()
	}
	cache, err := catalog.channelService.loadCache(ctx)
	if err != nil {
		return nil, err
	}
	if cache.failed {
		return nil, unavailable()
	}
	channel := cache.channelByGroupID[group.ID]
	if channel != nil && !channel.IsActive() {
		return nil, unavailable()
	}
	inventory, err := catalog.reader.ListGroupModelInventory(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	var candidates []string
	wildcard := false
	for _, row := range inventory {
		if row.Platform != group.Platform {
			continue
		}
		account := Account{Platform: row.Platform, Type: row.Type, Credentials: map[string]any{"model_mapping": row.ModelMapping}}
		accounts = append(accounts, account)
		mapping := account.GetModelMapping()
		// Bedrock retains its built-in fallback mapping even with custom keys;
		// OAuth short-name aliases are existing selector capabilities too.
		if account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
			wildcard = true
		}
		if len(mapping) == 0 {
			candidates = append(candidates, "*")
			wildcard = true
		}
		for name := range mapping {
			candidates = append(candidates, name)
			wildcard = wildcard || strings.Contains(name, "*")
		}
	}
	if wildcard {
		switch group.Platform {
		case PlatformOpenAI:
			candidates = append(candidates, openai.DefaultModelIDs()...)
		case PlatformAnthropic:
			candidates = append(candidates, claude.DefaultModelIDs()...)
			for alias := range claude.ModelIDOverrides {
				candidates = append(candidates, alias)
			}
		case PlatformGemini:
			for _, m := range gemini.DefaultModels() {
				candidates = append(candidates, strings.TrimPrefix(m.Name, "models/"))
			}
		}
	}
	var lookup *channelLookup
	if channel != nil {
		lookup = &channelLookup{cache: cache, channel: channel, platform: group.Platform}
		for name := range channel.ModelMapping[group.Platform] {
			candidates = append(candidates, name)
		}
		for _, price := range channel.ModelPricing {
			if price.Platform == group.Platform {
				candidates = append(candidates, price.Models...)
			}
		}
	}
	d := &GroupModelDeclaration{}
	d.matches = func(protocol, model string) bool {
		if protocol == "messages" && group.Platform == PlatformOpenAI {
			requested := model
			model = NormalizeOpenAICompatRequestedModel(requested)
			if mapped := group.ResolveMessagesDispatchModel(requested); mapped != "" {
				model = mapped
			}
		}
		if IsDisabledPublicModelForGroup(model, group.ID) {
			return false
		}
		for _, account := range accounts {
			// Match the same requested name the actual scheduler tests, not a channel
			// alias target. Channel-only aliases do not add account capabilities.
			supported := account.IsModelSupported(model)
			if account.Platform != PlatformOpenAI && account.Platform != PlatformGrok {
				supported = accountSupportsDeclaredModel(&account, model)
			}
			if !supported {
				continue
			}
			if lookup == nil || !channel.RestrictModels {
				return true
			}
			// The non-OpenAI selector does not enforce upstream-price names
			// before forwarding; OAuth/Bedrock can transform them later. Do
			// not guess a price name here and silently select another payer.
			if account.Platform != PlatformOpenAI && channel.BillingModelSource == BillingModelSourceUpstream {
				return true
			}
			mapped := resolveMapping(lookup, group.ID, model)
			billingModel := billingModelForRestriction(mapped.BillingModelSource, model, mapped.MappedModel)
			if billingModel == "" {
				billingModel = account.GetMappedModel(model)
				if account.Platform == PlatformOpenAI {
					billingModel = resolveOpenAIAccountUpstreamModelForRequest(&account, model, false)
				}
			}
			if !checkRestricted(lookup, group.ID, billingModel) {
				return true
			}
		}
		return false
	}
	slices.Sort(candidates)
	for _, name := range slices.Compact(candidates) {
		// Only concrete, currently admitted candidates are exposed. Wildcard
		// admission remains in the predicate, including exact exclusion holes.
		if !strings.Contains(name, "*") && d.Matches(group, name) {
			d.Models = append(d.Models, name)
		}
	}
	return d, nil
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

// MultiGroupRequestModelMatches reuses existing provider alias normalization;
// it does not invent a new model or protocol bridge.
func MultiGroupRequestModelMatches(group *Group, patterns []string, model string) bool {
	if group == nil || IsDisabledPublicModelForGroup(model, group.ID) {
		return false
	}
	candidates := modelLookupCandidatesForMapping(group.Platform, strings.ToLower(model))
	if group.Platform == PlatformOpenAI {
		if normalized, ok := normalizeKnownCodexModel(model); ok {
			candidates = append(candidates, normalized)
		}
	}
	for _, candidate := range candidates {
		if MultiGroupModelMatches(patterns, candidate) {
			return true
		}
	}
	return false
}
