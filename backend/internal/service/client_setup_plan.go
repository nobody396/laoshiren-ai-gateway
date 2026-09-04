package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

const clientSetupPlanPurpose = "client_setup_plan_v1"

type ClientSetupPlanModel struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
}
type ClientSetupPlan struct {
	ClientID         string                 `json:"client_id"`
	ClientVersionKey string                 `json:"client_version_key"`
	OS               string                 `json:"os"`
	Target           string                 `json:"target"`
	BaseURL          string                 `json:"base_url"`
	GroupIDs         []int64                `json:"group_ids"`
	Models           []ClientSetupPlanModel `json:"models"`
	DefaultModel     string                 `json:"default_model"`
	Available        bool                   `json:"available"`
	Reason           string                 `json:"reason,omitempty"`
	Fingerprint      string                 `json:"fingerprint"`
}
type setupGroupDiscovery interface {
	MultiGroupCatalog(context.Context, *Group) (*GroupModelDeclaration, error)
}

// SetupPlans is a credential-free view. Authorization and catalog are re-read
// both when a command is issued and when its one-time ticket is redeemed.
func (s *ClientSetupService) SetupPlans(ctx context.Context, userID, keyID int64, os string) ([]ClientSetupPlan, error) {
	if s == nil || s.apiKeys == nil || s.models == nil || (os != "macos" && os != "linux" && os != "windows") {
		return nil, ErrInvalidClientSetupSelection
	}
	key, err := s.apiKeys.GetByID(ctx, keyID)
	if err != nil || key == nil || key.UserID != userID || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return nil, ErrClientSetupKeyUnavailable
	}
	payerID := userID
	if key.User != nil {
		if !key.User.IsActive() {
			return nil, ErrClientSetupKeyUnavailable
		}
		payerID = key.User.ID
	}
	available, err := s.apiKeys.GetAvailableGroups(ctx, payerID)
	if err != nil {
		return nil, err
	}
	ids := append([]int64(nil), key.GroupIDs...)
	if len(ids) == 0 && key.Group != nil {
		ids = []int64{key.Group.ID}
	}
	if len(ids) == 0 {
		return nil, ErrClientSetupGroupMissing
	}
	byID := map[int64]*Group{}
	for i := range available {
		byID[available[i].ID] = &available[i]
	}
	reader, ok := s.models.(setupGroupDiscovery)
	if !ok {
		return nil, ErrClientSetupSelectionUnavailable
	}
	declarations := map[int64]*GroupModelDeclaration{}
	for _, id := range ids {
		group := byID[id]
		if group == nil || !group.IsActive() || group.IsUniversal() {
			return nil, ErrClientSetupGroupMissing
		}
		declaration, err := reader.MultiGroupCatalog(ctx, group)
		if err != nil {
			return nil, err
		}
		declarations[id] = declaration
	}
	clients := make([]string, 0, len(generatedClientSetupContracts))
	for id := range generatedClientSetupContracts {
		clients = append(clients, id)
	}
	sort.Strings(clients)
	plans := make([]ClientSetupPlan, 0, len(clients))
	for _, id := range clients {
		contract := generatedClientSetupContracts[id]
		plan := ClientSetupPlan{ClientID: id, ClientVersionKey: contract.VersionKey, OS: os, Target: clientSetupInstallerTarget(id), BaseURL: clientSetupAPIBaseURL, GroupIDs: ids, Models: []ClientSetupPlanModel{}}
		if !key.IsMultiGroup() && byID[ids[0]].Platform == PlatformAntigravity {
			plan.BaseURL += "/antigravity"
		}
		seen := map[string]bool{}
		for _, groupID := range ids {
			declaration := declarations[groupID]
			if declaration == nil {
				continue
			}
			models := append([]string(nil), declaration.Models...)
			sort.Strings(models)
			for _, model := range models {
				if seen[model] || strings.ContainsAny(model, "*\r\n\x00") {
					continue
				}
				for _, protocol := range []string{"responses", "messages", "generate_content", "chat_completions"} {
					if contract.Protocols[protocol] && generatedClientSetupModelProtocols[model][protocol] && declaration.MatchesProtocol(byID[groupID], protocol, model) {
						plan.Models = append(plan.Models, ClientSetupPlanModel{ID: model, Protocol: protocol})
						seen[model] = true
						break
					}
				}
			}
		}
		switch {
		case contract.OneClickStatus != "ready" || !contract.OSReady[os] || plan.Target == "":
			plan.Reason = "此客户端版本与系统的自动安装、配置和验收适配尚未开放"
		case len(plan.Models) == 0:
			plan.Reason = "这把 Key 的授权分组没有与该客户端匹配的已验证模型"
		default:
			plan.Available = true
			plan.DefaultModel = plan.Models[0].ID
		}
		// Fingerprint includes the exact ordered authority, OS, version and model plan.
		raw, _ := json.Marshal(plan)
		hash := sha256.Sum256(raw)
		plan.Fingerprint = hex.EncodeToString(hash[:])
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s *ClientSetupService) setupPlan(ctx context.Context, userID, keyID int64, clientID, os, fingerprint string) (*ClientSetupPlan, error) {
	plans, err := s.SetupPlans(ctx, userID, keyID, os)
	if err != nil {
		return nil, err
	}
	for i := range plans {
		p := &plans[i]
		if p.ClientID == clientID && p.Available && p.Fingerprint == fingerprint {
			return p, nil
		}
	}
	return nil, ErrClientSetupSelectionUnavailable
}
func (s *ClientSetupService) IssueTicketForPlan(ctx context.Context, userID, keyID int64, clientID, os, fingerprint string) (*ClientSetupTicket, error) {
	if s == nil || s.tickets == nil {
		return nil, ErrInvalidClientSetupTicket
	}
	plan, err := s.setupPlan(ctx, userID, keyID, clientID, os, fingerprint)
	if err != nil {
		return nil, err
	}
	key, err := s.apiKeys.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	selection := ClientSetupSelection{ClientID: plan.ClientID, ClientVersionKey: plan.ClientVersionKey, OS: plan.OS, ModelID: plan.DefaultModel, Protocol: plan.Models[0].Protocol, PlanFingerprint: plan.Fingerprint}
	return s.issueTicketForAPIKey(ctx, userID, plan.Target, key, &selection)
}
