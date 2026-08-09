package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	monthlyUpstreamProbeInterval      = 2 * time.Minute
	monthlyUpstreamProbeTimeout       = 25 * time.Second
	monthlyUpstreamProbeGrokTimeout   = 45 * time.Second
	monthlyUpstreamProbeRunnerTimeout = 2 * time.Minute

	monthlyOpenAIProbeEstimatedInputTokens     = 18
	monthlyOpenAIProbeEstimatedOutputTokens    = 1
	monthlyAnthropicProbeEstimatedInputTokens  = 236
	monthlyAnthropicProbeEstimatedOutputTokens = 32

	monthlyOpenAIGPT54MiniInputCostPerToken  = 8e-7
	monthlyOpenAIGPT54MiniOutputCostPerToken = 3.2e-6
	monthlyClaudeHaiku45InputCostPerToken    = 1e-6
	monthlyClaudeHaiku45OutputCostPerToken   = 5e-6
	monthlyUpstreamProbeCostCurrency         = "USD"

	MonthlyUpstreamProbePathGateway        = "gateway"
	MonthlyUpstreamProbePathDirectUpstream = "direct_upstream"
)

type monthlyUpstreamProbeTargetSpec struct {
	Role     string
	Platform string
	Model    string
}

type monthlyUpstreamProbeResolvedTarget struct {
	Role        string
	AccountName string
	Platform    string
	Model       string
	GroupID     int64
	GroupName   string
	Account     *Account
	Accounts    []Account
}

var monthlyUpstreamProbeTargetSpecs = []monthlyUpstreamProbeTargetSpec{
	{Role: "gpt", Platform: PlatformOpenAI, Model: "gpt-5.4-mini"},
	{Role: "claude", Platform: PlatformAnthropic, Model: "claude-haiku-4-5"},
	{Role: "grok", Platform: PlatformGrok, Model: "grok-4.5"},
}

func monthlyUpstreamProbeTimeoutForModel(model string) time.Duration {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "grok-") {
		return monthlyUpstreamProbeGrokTimeout
	}
	return monthlyUpstreamProbeTimeout
}

func monthlyUpstreamProbeSpecForRole(role string) (monthlyUpstreamProbeTargetSpec, bool) {
	for _, spec := range monthlyUpstreamProbeTargetSpecs {
		if spec.Role == role {
			return spec, true
		}
	}
	return monthlyUpstreamProbeTargetSpec{}, false
}

func monthlyUpstreamProbeSpecForAccount(account Account) (monthlyUpstreamProbeTargetSpec, bool) {
	name := strings.ToLower(strings.TrimSpace(account.Name))
	if strings.Contains(name, "grok") {
		return monthlyUpstreamProbeSpecForRole("grok")
	}
	switch account.Platform {
	case PlatformOpenAI:
		return monthlyUpstreamProbeSpecForRole("gpt")
	case PlatformAnthropic:
		return monthlyUpstreamProbeSpecForRole("claude")
	case PlatformGrok:
		return monthlyUpstreamProbeSpecForRole("grok")
	default:
		return monthlyUpstreamProbeTargetSpec{}, false
	}
}

func monthlyUpstreamProbeTargetByAccountName(targets []monthlyUpstreamProbeResolvedTarget, accountName string) (monthlyUpstreamProbeResolvedTarget, bool) {
	for _, target := range targets {
		if target.AccountName == accountName {
			return target, true
		}
	}
	return monthlyUpstreamProbeResolvedTarget{}, false
}

func monthlyUpstreamProbeTargetNameSet(targets []monthlyUpstreamProbeResolvedTarget) map[string]struct{} {
	out := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.AccountName != "" {
			out[target.AccountName] = struct{}{}
		}
	}
	return out
}

func monthlyUpstreamProbeTargetFromAccount(account Account, spec monthlyUpstreamProbeTargetSpec) monthlyUpstreamProbeResolvedTarget {
	accountCopy := account
	return monthlyUpstreamProbeResolvedTarget{
		Role:        spec.Role,
		AccountName: account.Name,
		Platform:    spec.Platform,
		Model:       spec.Model,
		Account:     &accountCopy,
		Accounts:    []Account{accountCopy},
	}
}

func monthlyUpstreamProbeDedupKey(account Account, spec monthlyUpstreamProbeTargetSpec) string {
	if account.ID > 0 {
		return spec.Role + ":" + strconv.FormatInt(account.ID, 10)
	}
	return spec.Role + ":" + account.Name
}

func monthlyUpstreamProbeChannelAccountName(role string) string {
	switch role {
	case "gpt":
		return "monthly-codex-gateway"
	case "claude":
		return "monthly-claude-gateway"
	case "grok":
		return "monthly-grok-gateway"
	default:
		return "monthly-" + strings.ToLower(strings.TrimSpace(role)) + "-gateway"
	}
}

func monthlyUpstreamProbeTargetFromGroup(group *MonthlyCardPublicPlanGroup, spec monthlyUpstreamProbeTargetSpec, accounts []Account) monthlyUpstreamProbeResolvedTarget {
	target := monthlyUpstreamProbeResolvedTarget{
		Role:        spec.Role,
		AccountName: monthlyUpstreamProbeChannelAccountName(spec.Role),
		Platform:    spec.Platform,
		Model:       spec.Model,
		Accounts:    append([]Account(nil), accounts...),
	}
	if group != nil {
		target.GroupID = group.ID
		target.GroupName = group.Name
	}
	for i := range accounts {
		if accounts[i].Platform != spec.Platform {
			continue
		}
		accountCopy := accounts[i]
		target.Account = &accountCopy
		break
	}
	return target
}

func shouldKeepMonthlyUpstreamProbePoint(point MonthlyUpstreamProbePoint, targetNames map[string]struct{}) bool {
	if len(targetNames) == 0 || point.AccountName == "" {
		return true
	}
	if _, ok := targetNames[point.AccountName]; ok {
		return true
	}
	// When the monitor is configured from monthly-card groups, stale account-level
	// rows should not create extra cards beside the channel-level probe rows.
	return false
}

type MonthlyUpstreamProbeCostEstimate struct {
	Currency             string  `json:"currency"`
	RateMultiplier       float64 `json:"rate_multiplier"`
	InputTokens          int     `json:"input_tokens"`
	OutputTokens         int     `json:"output_tokens"`
	InputCostPerToken    float64 `json:"input_cost_per_token"`
	OutputCostPerToken   float64 `json:"output_cost_per_token"`
	StandardCostPerProbe float64 `json:"standard_cost_per_probe"`
	ActualCostPerProbe   float64 `json:"actual_cost_per_probe"`
	ActualCostPerMinute  float64 `json:"actual_cost_per_minute"`
	ActualCostPerHour    float64 `json:"actual_cost_per_hour"`
	ActualCostPerDay     float64 `json:"actual_cost_per_day"`
	ProbeIntervalSeconds int     `json:"probe_interval_seconds"`
	EstimateNote         string  `json:"estimate_note"`
}

type MonthlyUpstreamProbePoint struct {
	AccountID    int64     `json:"account_id"`
	AccountName  string    `json:"account_name"`
	Platform     string    `json:"platform"`
	Model        string    `json:"model"`
	ProbePath    string    `json:"probe_path"`
	Status       string    `json:"status"`
	HTTPStatus   *int      `json:"http_status"`
	LatencyMs    int64     `json:"latency_ms"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	CheckedAt    time.Time `json:"checked_at"`
}

type MonthlyUpstreamProbeDiagnostic struct {
	ProbePath    string     `json:"probe_path"`
	Status       string     `json:"status"`
	HTTPStatus   *int       `json:"http_status"`
	LatencyMs    int64      `json:"latency_ms"`
	ErrorCode    string     `json:"error_code"`
	ErrorMessage string     `json:"error_message"`
	CheckedAt    *time.Time `json:"checked_at"`
}

type MonthlyUpstreamProbeAccount struct {
	AccountID        int64                             `json:"account_id"`
	AccountName      string                            `json:"account_name"`
	Platform         string                            `json:"platform"`
	Model            string                            `json:"model"`
	LatestStatus     string                            `json:"latest_status"`
	LatestHTTPStatus *int                              `json:"latest_http_status"`
	LatestLatencyMs  int64                             `json:"latest_latency_ms"`
	LatestErrorCode  string                            `json:"latest_error_code"`
	LatestError      string                            `json:"latest_error"`
	LatestCheckedAt  *time.Time                        `json:"latest_checked_at"`
	Uptime           float64                           `json:"uptime"`
	SuccessCount     int                               `json:"success_count"`
	TotalCount       int                               `json:"total_count"`
	CostEstimate     *MonthlyUpstreamProbeCostEstimate `json:"cost_estimate,omitempty"`
	DirectUpstream   *MonthlyUpstreamProbeDiagnostic   `json:"latest_direct_upstream,omitempty"`
	Points           []MonthlyUpstreamProbePoint       `json:"points"`
}

type MonthlyUpstreamProbeSnapshot struct {
	Enabled             bool                          `json:"enabled"`
	PublicStatusEnabled bool                          `json:"public_status_enabled"`
	WindowMinutes       int                           `json:"window_minutes"`
	GeneratedAt         time.Time                     `json:"generated_at"`
	Accounts            []MonthlyUpstreamProbeAccount `json:"accounts"`
}

type MonthlyCardPublicStatusPoint struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

type MonthlyCardPublicStatusAccount struct {
	DisplayName     string                         `json:"display_name"`
	Channel         string                         `json:"channel"`
	Status          string                         `json:"status"`
	LatestCheckedAt *time.Time                     `json:"latest_checked_at"`
	Uptime          float64                        `json:"uptime"`
	Points          []MonthlyCardPublicStatusPoint `json:"points"`
}

type MonthlyCardPublicPlanGroup struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	Platform        string   `json:"platform"`
	RateMultiplier  float64  `json:"rate_multiplier"`
	WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
}

type MonthlyCardPublicPlan struct {
	ID          string                      `json:"id"`
	Name        string                      `json:"name"`
	GPTGroup    *MonthlyCardPublicPlanGroup `json:"gpt_group,omitempty"`
	ClaudeGroup *MonthlyCardPublicPlanGroup `json:"claude_group,omitempty"`
	GrokGroup   *MonthlyCardPublicPlanGroup `json:"grok_group,omitempty"`
}

type MonthlyCardPublicStatusSnapshot struct {
	Enabled              bool                             `json:"enabled"`
	VisibleToUsers       bool                             `json:"visible_to_users"`
	WindowMinutes        int                              `json:"window_minutes"`
	ProbeIntervalSeconds int                              `json:"probe_interval_seconds"`
	GeneratedAt          time.Time                        `json:"generated_at"`
	Plans                []MonthlyCardPublicPlan          `json:"plans"`
	Accounts             []MonthlyCardPublicStatusAccount `json:"accounts"`
}

type MonthlyUpstreamProbeSettings struct {
	Enabled             bool `json:"enabled"`
	PublicStatusEnabled bool `json:"public_status_enabled"`
}

type MonthlyUpstreamProbeSettingsUpdate struct {
	Enabled             *bool `json:"enabled"`
	PublicStatusEnabled *bool `json:"public_status_enabled"`
}

func (s *OpsService) startMonthlyUpstreamProbeRunner() {
	if s == nil || s.opsRepo == nil || s.accountRepo == nil || s.settingRepo == nil {
		return
	}
	go func() {
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		for {
			<-timer.C
			ctx, cancel := context.WithTimeout(context.Background(), monthlyUpstreamProbeRunnerTimeout)
			if s.IsMonthlyUpstreamProbeEnabled(ctx) {
				_ = s.RunMonthlyUpstreamProbeOnce(ctx)
			}
			cancel()
			timer.Reset(monthlyUpstreamProbeInterval)
		}
	}()
}

func (s *OpsService) IsMonthlyUpstreamProbeEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyMonthlyUpstreamProbeEnabled)
	if err != nil {
		return false
	}
	return isTruthyMonthlyCardSettingValue(value)
}

func (s *OpsService) IsMonthlyCardPublicStatusEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyMonthlyCardPublicStatusEnabled)
	if err != nil {
		return false
	}
	return isTruthyMonthlyCardSettingValue(value)
}

func isTruthyMonthlyCardSettingValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "enabled":
		return true
	default:
		return false
	}
}

func normalizeMonthlyUpstreamProbeWindow(windowMinutes int) int {
	if windowMinutes <= 0 {
		return 60
	}
	if windowMinutes > 24*60 {
		return 24 * 60
	}
	return windowMinutes
}

func (s *OpsService) GetMonthlyUpstreamProbeSettings(ctx context.Context) *MonthlyUpstreamProbeSettings {
	return &MonthlyUpstreamProbeSettings{
		Enabled:             s.IsMonthlyUpstreamProbeEnabled(ctx),
		PublicStatusEnabled: s.IsMonthlyCardPublicStatusEnabled(ctx),
	}
}

func (s *OpsService) UpdateMonthlyUpstreamProbeSettings(ctx context.Context, req *MonthlyUpstreamProbeSettingsUpdate) (*MonthlyUpstreamProbeSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("settings repository is not available")
	}
	if req == nil {
		return s.GetMonthlyUpstreamProbeSettings(ctx), nil
	}
	if req.Enabled != nil {
		if err := s.settingRepo.Set(ctx, SettingKeyMonthlyUpstreamProbeEnabled, strconv.FormatBool(*req.Enabled)); err != nil {
			return nil, err
		}
	}
	if req.PublicStatusEnabled != nil {
		if err := s.settingRepo.Set(ctx, SettingKeyMonthlyCardPublicStatusEnabled, strconv.FormatBool(*req.PublicStatusEnabled)); err != nil {
			return nil, err
		}
	}
	return s.GetMonthlyUpstreamProbeSettings(ctx), nil
}

func (s *OpsService) GetMonthlyUpstreamProbeSnapshot(ctx context.Context, windowMinutes int) (*MonthlyUpstreamProbeSnapshot, error) {
	windowMinutes = normalizeMonthlyUpstreamProbeWindow(windowMinutes)
	if s == nil || s.opsRepo == nil {
		return nil, fmt.Errorf("ops repository is not available")
	}

	now := time.Now()
	enabled := s.IsMonthlyUpstreamProbeEnabled(ctx)
	publicStatusEnabled := s.IsMonthlyCardPublicStatusEnabled(ctx)
	reference := now.Truncate(time.Minute)
	expectedSlots := monthlyUpstreamProbeExpectedSlotCount(windowMinutes)
	points, err := s.opsRepo.ListMonthlyUpstreamProbeResults(ctx, now.Add(-time.Duration(windowMinutes)*time.Minute))
	if err != nil {
		return nil, err
	}
	probeTargets := []monthlyUpstreamProbeResolvedTarget{}
	probeAccountsByName := map[string]*Account{}
	probeAccountsByID := map[int64]*Account{}
	if s.accountRepo != nil {
		if loadedTargets, loadErr := s.loadMonthlyUpstreamProbeTargets(ctx); loadErr == nil {
			probeTargets = loadedTargets
			for _, target := range loadedTargets {
				if target.Account != nil && target.AccountName != "" {
					probeAccountsByName[target.AccountName] = target.Account
				}
				for i := range target.Accounts {
					accountCopy := target.Accounts[i]
					if accountCopy.ID > 0 {
						probeAccountsByID[accountCopy.ID] = &accountCopy
					}
				}
			}
		}
	}
	probeTargetNames := monthlyUpstreamProbeTargetNameSet(probeTargets)

	byAccount := make(map[string]*MonthlyUpstreamProbeAccount)
	gatewayPointsBySlot := make(map[string]map[int]MonthlyUpstreamProbePoint)
	targetInitialized := make(map[string]bool)
	ensureAccount := func(accountID int64, accountName, platform, model string) (*MonthlyUpstreamProbeAccount, string) {
		key := accountName
		if key == "" {
			key = platform + ":" + model
		}
		item := byAccount[key]
		if item == nil {
			item = &MonthlyUpstreamProbeAccount{
				AccountID:   accountID,
				AccountName: accountName,
				Platform:    platform,
				Model:       model,
				CostEstimate: buildMonthlyUpstreamProbeCostEstimate(
					accountName,
					platform,
					model,
					monthlyProbeAccountForCost(accountID, accountName, probeAccountsByID, probeAccountsByName),
				),
				Points: make([]MonthlyUpstreamProbePoint, 0, expectedSlots),
			}
			byAccount[key] = item
		}
		if item.AccountID == 0 && accountID != 0 {
			item.AccountID = accountID
		}
		if item.AccountName == "" && accountName != "" {
			item.AccountName = accountName
		}
		if item.Platform == "" && platform != "" {
			item.Platform = platform
		}
		if item.Model == "" && model != "" {
			item.Model = model
		}
		return item, key
	}
	if enabled && len(probeTargets) > 0 {
		for _, target := range probeTargets {
			accountID := int64(0)
			if target.Account != nil {
				accountID = target.Account.ID
			}
			_, key := ensureAccount(accountID, target.AccountName, target.Platform, target.Model)
			targetInitialized[key] = true
		}
	}
	for _, point := range points {
		if !shouldKeepMonthlyUpstreamProbePoint(point, probeTargetNames) {
			continue
		}
		point.ProbePath = normalizeMonthlyUpstreamProbePath(point.ProbePath)
		if target, ok := monthlyUpstreamProbeTargetByAccountName(probeTargets, point.AccountName); ok {
			point.Platform = target.Platform
			point.Model = target.Model
		}
		item, key := ensureAccount(point.AccountID, point.AccountName, point.Platform, point.Model)
		if point.ProbePath == MonthlyUpstreamProbePathDirectUpstream {
			if item.DirectUpstream == nil || point.CheckedAt.After(diagnosticCheckedAt(item.DirectUpstream)) {
				checkedAt := point.CheckedAt
				item.DirectUpstream = &MonthlyUpstreamProbeDiagnostic{
					ProbePath:    point.ProbePath,
					Status:       point.Status,
					HTTPStatus:   point.HTTPStatus,
					LatencyMs:    point.LatencyMs,
					ErrorCode:    point.ErrorCode,
					ErrorMessage: point.ErrorMessage,
					CheckedAt:    &checkedAt,
				}
			}
			continue
		}
		item.Points = append(item.Points, point)
		if slot, ok := monthlyUpstreamProbeSlotDistance(reference, point.CheckedAt, expectedSlots); ok {
			slots := gatewayPointsBySlot[key]
			if slots == nil {
				slots = make(map[int]MonthlyUpstreamProbePoint)
				gatewayPointsBySlot[key] = slots
			}
			if existing, exists := slots[slot]; !exists || point.CheckedAt.After(existing.CheckedAt) {
				slots[slot] = point
			}
		}
		if item.LatestCheckedAt == nil || point.CheckedAt.After(*item.LatestCheckedAt) {
			checkedAt := point.CheckedAt
			item.LatestCheckedAt = &checkedAt
			item.LatestStatus = point.Status
			item.LatestHTTPStatus = point.HTTPStatus
			item.LatestLatencyMs = point.LatencyMs
			item.LatestErrorCode = point.ErrorCode
			item.LatestError = point.ErrorMessage
			item.Model = point.Model
			item.CostEstimate = buildMonthlyUpstreamProbeCostEstimate(
				point.AccountName,
				point.Platform,
				point.Model,
				monthlyProbeAccountForCost(point.AccountID, point.AccountName, probeAccountsByID, probeAccountsByName),
			)
		}
	}

	accounts := make([]MonthlyUpstreamProbeAccount, 0, len(byAccount))
	for key, item := range byAccount {
		slots := gatewayPointsBySlot[key]
		if len(slots) == 0 && item.DirectUpstream != nil && !targetInitialized[key] {
			continue
		}
		sort.Slice(item.Points, func(i, j int) bool {
			return item.Points[i].CheckedAt.Before(item.Points[j].CheckedAt)
		})
		if len(slots) > 0 || targetInitialized[key] {
			item.SuccessCount, item.TotalCount, item.Uptime = monthlyUpstreamProbeSlotHealth(slots)
		}
		if item.LatestCheckedAt == nil && targetInitialized[key] {
			item.LatestStatus = "missing"
		}
		accounts = append(accounts, *item)
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Platform == accounts[j].Platform {
			return accounts[i].AccountName < accounts[j].AccountName
		}
		return accounts[i].Platform < accounts[j].Platform
	})

	return &MonthlyUpstreamProbeSnapshot{
		Enabled:             enabled,
		PublicStatusEnabled: publicStatusEnabled,
		WindowMinutes:       windowMinutes,
		GeneratedAt:         now,
		Accounts:            accounts,
	}, nil
}

func (s *OpsService) GetMonthlyCardPublicStatusSnapshot(ctx context.Context, windowMinutes int) (*MonthlyCardPublicStatusSnapshot, error) {
	windowMinutes = normalizeMonthlyUpstreamProbeWindow(windowMinutes)
	plans := s.loadMonthlyCardPublicPlans(ctx)
	if !s.IsMonthlyCardPublicStatusEnabled(ctx) {
		return monthlyCardHiddenPublicStatusSnapshot(windowMinutes, plans), nil
	}

	snapshot, err := s.GetMonthlyUpstreamProbeSnapshot(ctx, windowMinutes)
	if err != nil {
		return nil, err
	}
	if !snapshot.PublicStatusEnabled {
		return monthlyCardHiddenPublicStatusSnapshot(windowMinutes, plans), nil
	}

	accounts := make([]MonthlyCardPublicStatusAccount, 0, len(snapshot.Accounts))
	for _, account := range snapshot.Accounts {
		points := make([]MonthlyCardPublicStatusPoint, 0, len(account.Points))
		for _, point := range account.Points {
			points = append(points, MonthlyCardPublicStatusPoint{
				Status:    point.Status,
				CheckedAt: point.CheckedAt,
			})
		}

		accounts = append(accounts, MonthlyCardPublicStatusAccount{
			DisplayName:     monthlyCardPublicDisplayName(account.AccountName, account.Model, account.Platform),
			Channel:         monthlyCardPublicChannelName(account.AccountName, account.Model, account.Platform),
			Status:          account.LatestStatus,
			LatestCheckedAt: account.LatestCheckedAt,
			Uptime:          account.Uptime,
			Points:          points,
		})
	}
	sort.Slice(accounts, func(i, j int) bool {
		leftRank := monthlyCardPublicSortRank(accounts[i].Channel)
		rightRank := monthlyCardPublicSortRank(accounts[j].Channel)
		if leftRank == rightRank {
			return accounts[i].DisplayName < accounts[j].DisplayName
		}
		return leftRank < rightRank
	})

	return &MonthlyCardPublicStatusSnapshot{
		Enabled:              snapshot.Enabled,
		VisibleToUsers:       snapshot.PublicStatusEnabled,
		WindowMinutes:        snapshot.WindowMinutes,
		ProbeIntervalSeconds: int(monthlyUpstreamProbeInterval / time.Second),
		GeneratedAt:          snapshot.GeneratedAt,
		Plans:                plans,
		Accounts:             accounts,
	}, nil
}

func monthlyCardHiddenPublicStatusSnapshot(windowMinutes int, plans []MonthlyCardPublicPlan) *MonthlyCardPublicStatusSnapshot {
	return &MonthlyCardPublicStatusSnapshot{
		Enabled:              false,
		VisibleToUsers:       false,
		WindowMinutes:        windowMinutes,
		ProbeIntervalSeconds: int(monthlyUpstreamProbeInterval / time.Second),
		GeneratedAt:          time.Now(),
		Plans:                plans,
		Accounts:             []MonthlyCardPublicStatusAccount{},
	}
}

var monthlyCardPublicPlanDefinitions = []struct {
	ID              string
	Name            string
	GPTGroupName    string
	ClaudeGroupName string
	GrokGroupName   string
	GPTGroupID      int64
	ClaudeGroupID   int64
	GrokGroupID     int64
}{
	{ID: "plus", Name: "Plus", GPTGroupName: "GPT Plus 月卡组", ClaudeGroupName: "Claude Plus 月卡组", GrokGroupName: "Grok Plus 月卡组"},
	{ID: "pro", Name: "Pro", GPTGroupName: "GPT Pro V3 月卡组", ClaudeGroupName: "Claude Pro V3 月卡组", GrokGroupName: "Grok Pro V3 月卡组"},
	{ID: "max", Name: "Max", GPTGroupName: "GPT Max V3 月卡组", ClaudeGroupName: "Claude Max V3 月卡组", GrokGroupName: "Grok Max V3 月卡组"},
}

// monthlyUpstreamProbeSupplementalGroupDefinitions keeps status monitoring
// independent from the current sale catalog and preserves a fallback lookup
// for installations that have not provisioned the current Grok groups yet.
var monthlyUpstreamProbeSupplementalGroupDefinitions = []struct {
	Role      string
	GroupName string
	GroupID   int64
}{
	{Role: "grok", GroupName: "Grok Lite 月卡组", GroupID: 35},
}

func (s *OpsService) loadMonthlyCardPublicPlans(ctx context.Context) []MonthlyCardPublicPlan {
	plans := make([]MonthlyCardPublicPlan, 0, len(monthlyCardPublicPlanDefinitions))
	if s == nil || s.groupRepo == nil {
		return plans
	}
	groupsByName := s.activeMonthlyCardGroupsByName(ctx)
	for _, def := range monthlyCardPublicPlanDefinitions {
		plans = append(plans, MonthlyCardPublicPlan{
			ID:          def.ID,
			Name:        def.Name,
			GPTGroup:    s.monthlyCardPublicPlanGroupByNameOrID(ctx, groupsByName, def.GPTGroupName, def.GPTGroupID),
			ClaudeGroup: s.monthlyCardPublicPlanGroupByNameOrID(ctx, groupsByName, def.ClaudeGroupName, def.ClaudeGroupID),
			GrokGroup:   s.monthlyCardPublicPlanGroupByNameOrID(ctx, groupsByName, def.GrokGroupName, def.GrokGroupID),
		})
	}
	return plans
}

func (s *OpsService) activeMonthlyCardGroupsByName(ctx context.Context) map[string]*Group {
	groupsByName := make(map[string]*Group)
	if s == nil || s.groupRepo == nil {
		return groupsByName
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return groupsByName
	}
	for i := range groups {
		g := &groups[i]
		if g.SubscriptionType == SubscriptionTypeCredit && g.Name != "" {
			groupsByName[g.Name] = g
		}
	}
	return groupsByName
}

func (s *OpsService) monthlyCardPublicPlanGroupByNameOrID(ctx context.Context, groupsByName map[string]*Group, groupName string, fallbackGroupID int64) *MonthlyCardPublicPlanGroup {
	if groupName != "" {
		if group := groupsByName[groupName]; group != nil {
			return monthlyCardPublicPlanGroupFromService(group)
		}
	}
	return s.monthlyCardPublicPlanGroup(ctx, fallbackGroupID)
}

func (s *OpsService) monthlyCardPublicPlanGroup(ctx context.Context, groupID int64) *MonthlyCardPublicPlanGroup {
	if s == nil || s.groupRepo == nil || groupID <= 0 {
		return nil
	}
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil || group == nil || group.Status != StatusActive {
		return nil
	}
	return monthlyCardPublicPlanGroupFromService(group)
}

func monthlyCardPublicPlanGroupFromService(group *Group) *MonthlyCardPublicPlanGroup {
	if group == nil || group.Status != StatusActive {
		return nil
	}
	return &MonthlyCardPublicPlanGroup{
		ID:              group.ID,
		Name:            group.Name,
		Platform:        group.Platform,
		RateMultiplier:  group.RateMultiplier,
		WeeklyLimitUSD:  group.WeeklyLimitUSD,
		MonthlyLimitUSD: group.MonthlyLimitUSD,
	}
}

func monthlyCardPublicSortRank(channel string) int {
	switch channel {
	case "Codex":
		return 1
	case "Claude":
		return 2
	case "Grok":
		return 3
	default:
		return 99
	}
}

func monthlyCardPublicDisplayName(accountName, model, platform string) string {
	channel := monthlyCardPublicChannelName(accountName, model, platform)
	if channel == "" {
		return "月卡通道"
	}
	return channel + " 月卡"
}

func monthlyCardPublicChannelName(accountName, model, platform string) string {
	identity := strings.ToLower(strings.TrimSpace(accountName + " " + model))
	if strings.Contains(identity, "grok") {
		return "Grok"
	}
	if strings.Contains(identity, "codex") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-") {
		return "Codex"
	}
	if strings.Contains(identity, "claude") {
		return "Claude"
	}
	switch platform {
	case PlatformOpenAI:
		return "Codex"
	case PlatformAnthropic:
		return "Claude"
	case PlatformGrok:
		return "Grok"
	default:
		return platform
	}
}

func (s *OpsService) RunMonthlyUpstreamProbeOnce(ctx context.Context) error {
	if s == nil || s.accountRepo == nil || s.opsRepo == nil {
		return fmt.Errorf("monthly upstream probe dependencies are not available")
	}

	targets, err := s.loadMonthlyUpstreamProbeTargets(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, target := range targets {
		point, account := s.probeMonthlyGatewayTarget(ctx, target)
		if point.AccountName == "" {
			point.AccountName = target.AccountName
		}
		if point.Platform == "" {
			point.Platform = target.Platform
		}
		if point.Model == "" {
			point.Model = target.Model
		}
		point.ProbePath = MonthlyUpstreamProbePathGateway
		if err := s.opsRepo.InsertMonthlyUpstreamProbeResult(ctx, &point); err != nil {
			errs = append(errs, err)
		}
		if !isMonthlyUpstreamProbeHealthy(point.Status) {
			s.recordMonthlyUpstreamProbeError(ctx, &point)
		}
		if account != nil && point.Status != "not_schedulable" && !isMonthlyUpstreamProbeHealthy(point.Status) {
			diagnostic := probeMonthlyDirectUpstreamAccount(ctx, account, target.Model)
			diagnostic.AccountName = target.AccountName
			diagnostic.Platform = target.Platform
			diagnostic.Model = target.Model
			diagnostic.ProbePath = MonthlyUpstreamProbePathDirectUpstream
			if err := s.opsRepo.InsertMonthlyUpstreamProbeResult(ctx, &diagnostic); err != nil {
				errs = append(errs, err)
			}
			if !isMonthlyUpstreamProbeHealthy(diagnostic.Status) {
				s.recordMonthlyUpstreamProbeError(ctx, &diagnostic)
			}
		}
	}
	return errors.Join(errs...)
}

func (s *OpsService) probeMonthlyGatewayTarget(ctx context.Context, target monthlyUpstreamProbeResolvedTarget) (MonthlyUpstreamProbePoint, *Account) {
	if target.GroupID <= 0 {
		account := target.Account
		var point MonthlyUpstreamProbePoint
		if account == nil {
			point = MonthlyUpstreamProbePoint{
				AccountName:  target.AccountName,
				Platform:     target.Platform,
				Model:        target.Model,
				Status:       "failed",
				ErrorCode:    "missing_account",
				ErrorMessage: "monthly upstream account was not found",
				CheckedAt:    time.Now(),
			}
		} else {
			point = s.probeMonthlyGatewayAccount(ctx, account, target.Model)
		}
		point.AccountName = target.AccountName
		point.Platform = target.Platform
		point.Model = target.Model
		return point, account
	}

	switch target.Platform {
	case PlatformOpenAI:
		return s.probeMonthlyOpenAIGroupThroughGateway(ctx, target)
	case PlatformAnthropic:
		return s.probeMonthlyAnthropicGroupThroughGateway(ctx, target)
	case PlatformGrok:
		return s.probeMonthlyGrokGroupThroughGateway(ctx, target)
	default:
		return monthlyProbeTargetLocalFailure(target, "unsupported_platform", "unsupported platform for monthly upstream probe"), nil
	}
}

func (s *OpsService) probeMonthlyGrokGroupThroughGateway(ctx context.Context, target monthlyUpstreamProbeResolvedTarget) (MonthlyUpstreamProbePoint, *Account) {
	if s == nil || s.openAIGatewayService == nil {
		return monthlyProbeTargetLocalFailure(target, "gateway_service_unavailable", "grok gateway service is not available"), nil
	}
	groupID := target.GroupID
	sessionHash := monthlyGatewayProbeSessionHash(target)
	selection, _, err := s.openAIGatewayService.SelectGrokAccountWithScheduler(
		ctx,
		&groupID,
		sessionHash,
		target.Model,
		nil,
		false,
	)
	if err != nil {
		return monthlyProbeTargetLocalFailure(target, "account_select_failed", err.Error()), nil
	}
	if selection == nil || selection.Account == nil {
		return monthlyProbeTargetLocalFailure(target, "missing_account", "monthly Grok account was not selected"), nil
	}
	if selection.Acquired && selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	} else if !selection.Acquired {
		return monthlyProbeSelectedAccountBusy(target, selection.Account), selection.Account
	}

	point := s.probeMonthlyGrokThroughGateway(ctx, selection.Account, target.Model, sessionHash)
	point.AccountName = target.AccountName
	point.Platform = target.Platform
	point.Model = target.Model
	return point, selection.Account
}

func (s *OpsService) probeMonthlyOpenAIGroupThroughGateway(ctx context.Context, target monthlyUpstreamProbeResolvedTarget) (MonthlyUpstreamProbePoint, *Account) {
	if s == nil || s.openAIGatewayService == nil {
		return monthlyProbeTargetLocalFailure(target, "gateway_service_unavailable", "openai gateway service is not available"), nil
	}
	groupID := target.GroupID
	sessionHash := monthlyGatewayProbeSessionHash(target)
	selection, _, err := s.openAIGatewayService.SelectAccountWithSchedulerForRequest(
		ctx,
		&groupID,
		"",
		sessionHash,
		target.Model,
		nil,
		OpenAIUpstreamTransportAny,
		false,
	)
	if err != nil {
		return monthlyProbeTargetLocalFailure(target, "account_select_failed", err.Error()), nil
	}
	if selection == nil || selection.Account == nil {
		return monthlyProbeTargetLocalFailure(target, "missing_account", "monthly upstream account was not selected"), nil
	}
	if selection.Acquired && selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	} else if !selection.Acquired && selection.WaitPlan != nil {
		return monthlyProbeSelectedAccountBusy(target, selection.Account), selection.Account
	} else if !selection.Acquired {
		return monthlyProbeSelectedAccountBusy(target, selection.Account), selection.Account
	}

	body, _ := json.Marshal(createOpenAICompactProbePayload(target.Model))
	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeoutForModel(target.Model))
	defer cancel()
	c, recorder := newMonthlyProbeGinContext(probeCtx, "/v1/responses", body)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")
	c.Request.Header.Set("Originator", "codex_cli_rs")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.0.0 monthly-gateway-probe")
	c.Request.Header.Set("Version", "0.0.0")
	c.Request.Header.Set("Session_ID", sessionHash)
	c.Request.Header.Set("Conversation_ID", sessionHash)

	started := time.Now()
	result, forwardErr := s.openAIGatewayService.Forward(c.Request.Context(), c, selection.Account, body)
	point := monthlyGatewayProbePoint(selection.Account, target.Model, recorder, started, forwardErr, func() time.Duration {
		if result != nil {
			return result.Duration
		}
		return 0
	})
	point.AccountName = target.AccountName
	point.Platform = target.Platform
	point.Model = target.Model
	return point, selection.Account
}

func (s *OpsService) probeMonthlyAnthropicGroupThroughGateway(ctx context.Context, target monthlyUpstreamProbeResolvedTarget) (MonthlyUpstreamProbePoint, *Account) {
	if s == nil || s.gatewayService == nil {
		return monthlyProbeTargetLocalFailure(target, "gateway_service_unavailable", "anthropic gateway service is not available"), nil
	}
	groupID := target.GroupID
	sessionHash := monthlyGatewayProbeSessionHash(target)
	selection, err := s.gatewayService.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, target.Model, nil, "")
	if err != nil {
		return monthlyProbeTargetLocalFailure(target, "account_select_failed", err.Error()), nil
	}
	if selection == nil || selection.Account == nil {
		return monthlyProbeTargetLocalFailure(target, "missing_account", "monthly upstream account was not selected"), nil
	}
	if selection.Acquired && selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	} else if !selection.Acquired && selection.WaitPlan != nil {
		return monthlyProbeSelectedAccountBusy(target, selection.Account), selection.Account
	} else if !selection.Acquired {
		return monthlyProbeSelectedAccountBusy(target, selection.Account), selection.Account
	}

	body, _ := json.Marshal(map[string]any{
		"model":      target.Model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "OK"}},
		"metadata":   map[string]string{"user_id": "monthly-gateway-probe"},
		"stream":     false,
	})
	parsed, err := ParseGatewayRequest(body, PlatformAnthropic)
	if err != nil {
		return monthlyProbeTargetLocalFailure(target, "build_gateway_request_failed", err.Error()), nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeoutForModel(target.Model))
	defer cancel()
	c, recorder := newMonthlyProbeGinContext(probeCtx, "/v1/messages", body)
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.84 (external, cli) monthly-gateway-probe")
	c.Request.Header.Set("Anthropic-Version", "2023-06-01")
	c.Request.Header.Set("Anthropic-Beta", "oauth-2025-04-20,interleaved-thinking-2025-05-14")
	c.Request.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
	c.Request.Header.Set("X-App", "cli")

	started := time.Now()
	result, forwardErr := s.gatewayService.Forward(c.Request.Context(), c, selection.Account, parsed)
	point := monthlyGatewayProbePoint(selection.Account, target.Model, recorder, started, forwardErr, func() time.Duration {
		if result != nil {
			return result.Duration
		}
		return 0
	})
	point.AccountName = target.AccountName
	point.Platform = target.Platform
	point.Model = target.Model
	return point, selection.Account
}

func normalizeMonthlyUpstreamProbePath(path string) string {
	switch strings.ToLower(strings.TrimSpace(path)) {
	case MonthlyUpstreamProbePathGateway:
		return MonthlyUpstreamProbePathGateway
	case MonthlyUpstreamProbePathDirectUpstream:
		return MonthlyUpstreamProbePathDirectUpstream
	default:
		return MonthlyUpstreamProbePathDirectUpstream
	}
}

func diagnosticCheckedAt(diagnostic *MonthlyUpstreamProbeDiagnostic) time.Time {
	if diagnostic == nil || diagnostic.CheckedAt == nil {
		return time.Time{}
	}
	return *diagnostic.CheckedAt
}

func (s *OpsService) loadMonthlyUpstreamProbeTargets(ctx context.Context) ([]monthlyUpstreamProbeResolvedTarget, error) {
	if s == nil || s.accountRepo == nil {
		return nil, nil
	}
	if s.groupRepo == nil {
		return s.loadMonthlyUpstreamProbeTargetsByNameSearch(ctx)
	}
	return s.loadMonthlyUpstreamProbeTargetsFromGroups(ctx)
}

func (s *OpsService) loadMonthlyUpstreamProbeTargetsFromGroups(ctx context.Context) ([]monthlyUpstreamProbeResolvedTarget, error) {
	plans := s.loadMonthlyCardPublicPlans(ctx)
	targets := make([]monthlyUpstreamProbeResolvedTarget, 0, len(monthlyUpstreamProbeTargetSpecs))
	seen := make(map[string]struct{})

	appendTarget := func(role string, group *MonthlyCardPublicPlanGroup) error {
		if group == nil {
			return nil
		}
		spec, ok := monthlyUpstreamProbeSpecForRole(role)
		if !ok {
			return nil
		}
		if _, exists := seen[spec.Role]; exists {
			return nil
		}
		accounts, err := s.accountRepo.ListByGroup(ctx, group.ID)
		if err != nil {
			return err
		}
		seen[spec.Role] = struct{}{}
		targets = append(targets, monthlyUpstreamProbeTargetFromGroup(group, spec, accounts))
		return nil
	}

	for _, plan := range plans {
		groupsByRole := []struct {
			role  string
			group *MonthlyCardPublicPlanGroup
		}{
			{role: "gpt", group: plan.GPTGroup},
			{role: "claude", group: plan.ClaudeGroup},
			{role: "grok", group: plan.GrokGroup},
		}
		for _, candidate := range groupsByRole {
			if err := appendTarget(candidate.role, candidate.group); err != nil {
				return nil, err
			}
		}
	}

	groupsByName := s.activeMonthlyCardGroupsByName(ctx)
	for _, def := range monthlyUpstreamProbeSupplementalGroupDefinitions {
		group := s.monthlyCardPublicPlanGroupByNameOrID(ctx, groupsByName, def.GroupName, def.GroupID)
		if err := appendTarget(def.Role, group); err != nil {
			return nil, err
		}
	}

	sortMonthlyUpstreamProbeTargets(targets)
	return targets, nil
}

func (s *OpsService) loadMonthlyUpstreamProbeTargetsByNameSearch(ctx context.Context) ([]monthlyUpstreamProbeResolvedTarget, error) {
	accounts, _, err := s.accountRepo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 100}, "", "", "", "pomoai-monthly", 0)
	if err != nil {
		return nil, err
	}
	targets := make([]monthlyUpstreamProbeResolvedTarget, 0, len(accounts))
	seen := make(map[string]struct{})
	for _, account := range accounts {
		spec, ok := monthlyUpstreamProbeSpecForAccount(account)
		if !ok || account.Name == "" {
			continue
		}
		key := monthlyUpstreamProbeDedupKey(account, spec)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, monthlyUpstreamProbeTargetFromAccount(account, spec))
	}
	sortMonthlyUpstreamProbeTargets(targets)
	return targets, nil
}

func sortMonthlyUpstreamProbeTargets(targets []monthlyUpstreamProbeResolvedTarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		leftRank := monthlyCardPublicSortRank(monthlyCardPublicChannelName(targets[i].AccountName, targets[i].Model, targets[i].Platform))
		rightRank := monthlyCardPublicSortRank(monthlyCardPublicChannelName(targets[j].AccountName, targets[j].Model, targets[j].Platform))
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if targets[i].GroupID != targets[j].GroupID {
			return targets[i].GroupID < targets[j].GroupID
		}
		if targets[i].AccountName != targets[j].AccountName {
			return targets[i].AccountName < targets[j].AccountName
		}
		if targets[i].Account == nil || targets[j].Account == nil {
			return targets[i].Account != nil
		}
		return targets[i].Account.ID < targets[j].Account.ID
	})
}

func monthlyProbeAccountForCost(accountID int64, accountName string, byID map[int64]*Account, byName map[string]*Account) *Account {
	if accountID > 0 {
		if account := byID[accountID]; account != nil {
			return account
		}
	}
	if accountName != "" {
		return byName[accountName]
	}
	return nil
}

func monthlyGatewayProbeSessionHash(target monthlyUpstreamProbeResolvedTarget) string {
	slot := time.Now().Unix() / int64(monthlyUpstreamProbeInterval/time.Second)
	return fmt.Sprintf("monthly_gateway_probe_%s_%d_%d", strings.ToLower(target.Platform), target.GroupID, slot)
}

func monthlyProbeTargetLocalFailure(target monthlyUpstreamProbeResolvedTarget, code, message string) MonthlyUpstreamProbePoint {
	return MonthlyUpstreamProbePoint{
		AccountName:  target.AccountName,
		Platform:     target.Platform,
		Model:        target.Model,
		ProbePath:    MonthlyUpstreamProbePathGateway,
		Status:       "failed",
		ErrorCode:    code,
		ErrorMessage: message,
		CheckedAt:    time.Now(),
	}
}

func monthlyProbeSelectedAccountBusy(target monthlyUpstreamProbeResolvedTarget, account *Account) MonthlyUpstreamProbePoint {
	point := monthlyProbeTargetLocalFailure(target, "account_slot_busy", "selected monthly upstream account has no free slot")
	point.Status = "rate_limited"
	if account != nil {
		point.AccountID = account.ID
	}
	status := http.StatusTooManyRequests
	point.HTTPStatus = &status
	return point
}

func (s *OpsService) probeMonthlyGatewayAccount(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	if account == nil {
		return MonthlyUpstreamProbePoint{Status: "failed", ErrorCode: "nil_account", CheckedAt: time.Now()}
	}
	if !account.IsSchedulable() {
		return MonthlyUpstreamProbePoint{
			AccountID:    account.ID,
			AccountName:  account.Name,
			Platform:     account.Platform,
			Model:        model,
			Status:       "not_schedulable",
			ErrorMessage: fmt.Sprintf("account status=%s schedulable=%t", account.Status, account.Schedulable),
			CheckedAt:    time.Now(),
		}
	}

	switch account.Platform {
	case PlatformOpenAI:
		if s == nil || s.openAIGatewayService == nil {
			return monthlyProbeLocalFailure(account, model, "gateway_service_unavailable", "openai gateway service is not available")
		}
		return s.probeMonthlyOpenAIThroughGateway(ctx, account, model)
	case PlatformAnthropic:
		if s == nil || s.gatewayService == nil {
			return monthlyProbeLocalFailure(account, model, "gateway_service_unavailable", "anthropic gateway service is not available")
		}
		return s.probeMonthlyAnthropicThroughGateway(ctx, account, model)
	case PlatformGrok:
		if s == nil || s.openAIGatewayService == nil {
			return monthlyProbeLocalFailure(account, model, "gateway_service_unavailable", "grok gateway service is not available")
		}
		return s.probeMonthlyGrokThroughGateway(ctx, account, model, fmt.Sprintf("monthly_gateway_probe_%d", account.ID))
	default:
		return MonthlyUpstreamProbePoint{
			AccountID:    account.ID,
			AccountName:  account.Name,
			Platform:     account.Platform,
			Model:        model,
			Status:       "failed",
			ErrorCode:    "unsupported_platform",
			ErrorMessage: "unsupported platform for monthly upstream probe",
			CheckedAt:    time.Now(),
		}
	}
}

func probeMonthlyDirectUpstreamAccount(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	if account == nil {
		return MonthlyUpstreamProbePoint{Status: "failed", ErrorCode: "nil_account", CheckedAt: time.Now()}
	}
	if !account.IsSchedulable() {
		return MonthlyUpstreamProbePoint{
			AccountID:    account.ID,
			AccountName:  account.Name,
			Platform:     account.Platform,
			Model:        model,
			Status:       "not_schedulable",
			ErrorMessage: fmt.Sprintf("account status=%s schedulable=%t", account.Status, account.Schedulable),
			CheckedAt:    time.Now(),
		}
	}

	switch account.Platform {
	case PlatformOpenAI:
		return probeMonthlyOpenAIUpstream(ctx, account, model)
	case PlatformAnthropic:
		return probeMonthlyAnthropicUpstream(ctx, account, model)
	case PlatformGrok:
		return probeMonthlyGrokUpstream(ctx, account, model)
	default:
		return MonthlyUpstreamProbePoint{
			AccountID:    account.ID,
			AccountName:  account.Name,
			Platform:     account.Platform,
			Model:        model,
			Status:       "failed",
			ErrorCode:    "unsupported_platform",
			ErrorMessage: "unsupported platform for monthly upstream probe",
			CheckedAt:    time.Now(),
		}
	}
}

func (s *OpsService) probeMonthlyGrokThroughGateway(ctx context.Context, account *Account, model, sessionHash string) MonthlyUpstreamProbePoint {
	body, _ := json.Marshal(createGrokMonthlyProbePayload(model))
	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeoutForModel(model))
	defer cancel()
	c, recorder := newMonthlyProbeGinContext(probeCtx, "/v1/responses", body)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")
	c.Request.Header.Set("Originator", "grok_cli_rs")
	c.Request.Header.Set("User-Agent", "grok-cli monthly-gateway-probe")
	c.Request.Header.Set("Session_ID", sessionHash)
	c.Request.Header.Set("Conversation_ID", sessionHash)

	started := time.Now()
	result, err := s.openAIGatewayService.Forward(c.Request.Context(), c, account, body)
	return monthlyGatewayProbePoint(account, model, recorder, started, err, func() time.Duration {
		if result != nil {
			return result.Duration
		}
		return 0
	})
}

func (s *OpsService) probeMonthlyOpenAIThroughGateway(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	body, _ := json.Marshal(createOpenAICompactProbePayload(model))
	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeoutForModel(model))
	defer cancel()
	c, recorder := newMonthlyProbeGinContext(probeCtx, "/v1/responses", body)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")
	c.Request.Header.Set("Originator", "codex_cli_rs")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.0.0 monthly-gateway-probe")
	c.Request.Header.Set("Version", "0.0.0")
	c.Request.Header.Set("Session_ID", fmt.Sprintf("monthly_gateway_probe_%d", account.ID))
	c.Request.Header.Set("Conversation_ID", fmt.Sprintf("monthly_gateway_probe_%d", account.ID))

	started := time.Now()
	result, err := s.openAIGatewayService.Forward(c.Request.Context(), c, account, body)
	return monthlyGatewayProbePoint(account, model, recorder, started, err, func() time.Duration {
		if result != nil {
			return result.Duration
		}
		return 0
	})
}

func (s *OpsService) probeMonthlyAnthropicThroughGateway(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "OK"}},
		"metadata":   map[string]string{"user_id": fmt.Sprintf("monthly-gateway-probe-%d", account.ID)},
		"stream":     false,
	})
	parsed, err := ParseGatewayRequest(body, PlatformAnthropic)
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "build_gateway_request_failed", err.Error())
	}

	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeoutForModel(model))
	defer cancel()
	c, recorder := newMonthlyProbeGinContext(probeCtx, "/v1/messages", body)
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.84 (external, cli) monthly-gateway-probe")
	c.Request.Header.Set("Anthropic-Version", "2023-06-01")
	c.Request.Header.Set("Anthropic-Beta", "oauth-2025-04-20,interleaved-thinking-2025-05-14")
	c.Request.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
	c.Request.Header.Set("X-App", "cli")

	started := time.Now()
	result, err := s.gatewayService.Forward(c.Request.Context(), c, account, parsed)
	return monthlyGatewayProbePoint(account, model, recorder, started, err, func() time.Duration {
		if result != nil {
			return result.Duration
		}
		return 0
	})
}

func newMonthlyProbeGinContext(ctx context.Context, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.Request = req
	return c, recorder
}

func monthlyGatewayProbePoint(account *Account, model string, recorder *httptest.ResponseRecorder, started time.Time, forwardErr error, resultDuration func() time.Duration) MonthlyUpstreamProbePoint {
	latency := time.Since(started)
	if resultDuration != nil {
		if duration := resultDuration(); duration > 0 {
			latency = duration
		}
	}
	var httpStatus *int
	statusCode := 0
	if recorder != nil {
		statusCode = recorder.Code
		if statusCode == 0 && forwardErr == nil {
			statusCode = http.StatusOK
		}
		if statusCode > 0 {
			httpStatus = &statusCode
		}
	}

	point := MonthlyUpstreamProbePoint{
		AccountID:   account.ID,
		AccountName: account.Name,
		Platform:    account.Platform,
		Model:       model,
		ProbePath:   MonthlyUpstreamProbePathGateway,
		HTTPStatus:  httpStatus,
		LatencyMs:   latency.Milliseconds(),
		CheckedAt:   time.Now(),
	}
	body := []byte(nil)
	if recorder != nil && recorder.Body != nil {
		body = recorder.Body.Bytes()
	}
	point.ErrorCode, point.ErrorMessage = extractMonthlyProbeError(body)
	var failoverErr *UpstreamFailoverError
	if errors.As(forwardErr, &failoverErr) {
		// A streaming response can commit HTTP 200 before the upstream later
		// fails. Surface the actual upstream status and message instead of the
		// misleading "HTTP 200 + failed" combination.
		if failoverErr.StatusCode > 0 {
			statusCode = failoverErr.StatusCode
			httpStatus = &statusCode
			point.HTTPStatus = httpStatus
		}
		if code, message := extractMonthlyProbeError(failoverErr.ResponseBody); message != "" {
			point.ErrorCode = code
			point.ErrorMessage = message
		}
	}
	shouldCaptureResponseError := forwardErr != nil || statusCode == 0 || statusCode < 200 || statusCode >= 300
	if shouldCaptureResponseError && isMonthlyProbeGenericClientUnavailable(point.ErrorCode, point.ErrorMessage) {
		point.ErrorCode = "gateway_forward_failed"
		if forwardErr != nil {
			point.ErrorMessage = forwardErr.Error()
		} else {
			point.ErrorMessage = "gateway forwarding failed before a successful upstream response"
		}
	}
	if point.ErrorMessage == "" && shouldCaptureResponseError {
		point.ErrorMessage = monthlyGatewayProbeContextError(recorder)
	}
	if point.ErrorMessage == "" && forwardErr != nil {
		point.ErrorMessage = forwardErr.Error()
	}
	switch {
	case forwardErr == nil && statusCode >= 200 && statusCode < 300:
		if point.LatencyMs > 5000 {
			point.Status = "slow"
		} else {
			point.Status = "ok"
		}
	case statusCode == http.StatusTooManyRequests:
		point.Status = "rate_limited"
	default:
		point.Status = "failed"
		if point.ErrorCode == "" {
			point.ErrorCode = "gateway_probe_failed"
		}
	}
	point.ErrorMessage = sanitizeMonthlyProbeError(point.ErrorMessage, account)
	return point
}

func isMonthlyProbeGenericClientUnavailable(code, message string) bool {
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	normalizedMessage := strings.ToLower(strings.TrimSpace(message))
	if normalizedCode != "api_error" {
		return false
	}
	return strings.Contains(normalizedMessage, "temporarily unavailable") ||
		strings.Contains(normalizedMessage, "please try again later")
}

func monthlyGatewayProbeContextError(recorder *httptest.ResponseRecorder) string {
	if recorder == nil {
		return ""
	}
	if recorder.Body == nil {
		return ""
	}
	raw := strings.TrimSpace(recorder.Body.String())
	if raw == "" {
		return ""
	}
	return raw
}

func probeMonthlyOpenAIUpstream(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	apiKey := strings.TrimSpace(account.GetOpenAIApiKey())
	if apiKey == "" {
		return monthlyProbeLocalFailure(account, model, "missing_api_key", "missing upstream api key")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.GetOpenAIBaseURL()), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	url := buildOpenAIResponsesURL(baseURL)
	body, _ := json.Marshal(createOpenAICompactProbePayload(model))
	return executeMonthlyProbeHTTP(ctx, account, model, url, map[string]string{
		"Authorization":   "Bearer " + apiKey,
		"Content-Type":    "application/json",
		"Accept":          "application/json",
		"OpenAI-Beta":     "responses=experimental",
		"Originator":      "codex_cli_rs",
		"User-Agent":      "codex_cli_rs/0.0.0 monthly-upstream-probe",
		"Version":         "0.0.0",
		"Session_ID":      fmt.Sprintf("monthly_probe_%d", account.ID),
		"Conversation_ID": fmt.Sprintf("monthly_probe_%d", account.ID),
	}, body)
}

func probeMonthlyGrokUpstream(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	token := monthlyProbeGrokCredential(account)
	if token == "" {
		return monthlyProbeLocalFailure(account, model, "missing_api_key", "missing upstream Grok credential")
	}
	targetURL, err := buildGrokResponsesURL(account, nil)
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "invalid_base_url", err.Error())
	}
	body, _ := json.Marshal(createGrokMonthlyProbePayload(model))
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
		"Accept":        "text/event-stream",
		"User-Agent":    "grok-cli monthly-upstream-probe",
	}
	if account.IsGrokOAuth() {
		cliHeaders := make(http.Header)
		applyGrokCLIHeaders(cliHeaders)
		for key, values := range cliHeaders {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}
	return executeMonthlyProbeHTTP(ctx, account, model, targetURL, headers, body)
}

// createGrokMonthlyProbePayload mirrors the native Grok CLI Responses shape.
// The generic OpenAI probe is non-streaming, but the production Grok clients
// use streaming Responses and some compatible pools route the two shapes
// differently. Monitoring must exercise the same transport as real traffic.
func createGrokMonthlyProbePayload(model string) map[string]any {
	payload := createOpenAICompactProbePayload(model)
	payload["stream"] = true
	return payload
}

func monthlyProbeGrokCredential(account *Account) string {
	if account == nil || !account.IsGrok() {
		return ""
	}
	if account.IsGrokOAuth() {
		return strings.TrimSpace(account.GetGrokAccessToken())
	}
	return strings.TrimSpace(account.GetCredential("api_key"))
}

func buildMonthlyUpstreamProbeCostEstimate(accountName, platform, model string, account *Account) *MonthlyUpstreamProbeCostEstimate {
	rateMultiplier := monthlyProbeRateMultiplier(account, accountName)
	var inputTokens, outputTokens int
	var inputPrice, outputPrice float64

	switch {
	case platform == PlatformOpenAI && model == "gpt-5.4-mini":
		inputTokens = monthlyOpenAIProbeEstimatedInputTokens
		outputTokens = monthlyOpenAIProbeEstimatedOutputTokens
		inputPrice = monthlyOpenAIGPT54MiniInputCostPerToken
		outputPrice = monthlyOpenAIGPT54MiniOutputCostPerToken
	case platform == PlatformAnthropic && model == "claude-haiku-4-5":
		inputTokens = monthlyAnthropicProbeEstimatedInputTokens
		outputTokens = monthlyAnthropicProbeEstimatedOutputTokens
		inputPrice = monthlyClaudeHaiku45InputCostPerToken
		outputPrice = monthlyClaudeHaiku45OutputCostPerToken
	default:
		return nil
	}

	standard := float64(inputTokens)*inputPrice + float64(outputTokens)*outputPrice
	actual := standard * rateMultiplier
	perMinute := actual * float64(time.Minute) / float64(monthlyUpstreamProbeInterval)
	return &MonthlyUpstreamProbeCostEstimate{
		Currency:             monthlyUpstreamProbeCostCurrency,
		RateMultiplier:       rateMultiplier,
		InputTokens:          inputTokens,
		OutputTokens:         outputTokens,
		InputCostPerToken:    inputPrice,
		OutputCostPerToken:   outputPrice,
		StandardCostPerProbe: standard,
		ActualCostPerProbe:   actual,
		ActualCostPerMinute:  perMinute,
		ActualCostPerHour:    perMinute * 60,
		ActualCostPerDay:     perMinute * 60 * 24,
		ProbeIntervalSeconds: int(monthlyUpstreamProbeInterval / time.Second),
		EstimateNote:         "按近期月卡健康探针实测均值估算，实际账单以 usage_logs 为准",
	}
}

func monthlyProbeRateMultiplier(account *Account, accountName string) float64 {
	if account != nil {
		return account.BillingRateMultiplier()
	}
	parts := strings.Split(strings.TrimSpace(accountName), "-")
	if len(parts) == 0 {
		return 1
	}
	parsed, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil || parsed < 0 {
		return 1
	}
	return parsed
}

func probeMonthlyAnthropicUpstream(ctx context.Context, account *Account, model string) MonthlyUpstreamProbePoint {
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return monthlyProbeLocalFailure(account, model, "missing_api_key", "missing upstream api key")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.GetBaseURL()), "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "OK"}},
		"metadata":   map[string]string{"user_id": fmt.Sprintf("monthly-probe-%d", account.ID)},
		"stream":     false,
	})
	return executeMonthlyProbeHTTP(ctx, account, model, buildSupplierAnthropicMessagesURL(baseURL), map[string]string{
		"Authorization":     "Bearer " + apiKey,
		"Content-Type":      "application/json",
		"Accept":            "application/json",
		"User-Agent":        "claude-cli/2.1.84 (external, cli)",
		"Anthropic-Version": "2023-06-01",
		"Anthropic-Beta":    "oauth-2025-04-20,interleaved-thinking-2025-05-14",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
		"X-App": "cli",
	}, body)
}

func executeMonthlyProbeHTTP(ctx context.Context, account *Account, model string, url string, headers map[string]string, body []byte) MonthlyUpstreamProbePoint {
	probeCtx, cancel := context.WithTimeout(ctx, monthlyUpstreamProbeTimeoutForModel(model))
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "build_request_failed", err.Error())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "request_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return monthlyProbeLocalFailure(account, model, "read_response_failed", err.Error())
	}

	httpStatus := resp.StatusCode
	point := MonthlyUpstreamProbePoint{
		AccountID:   account.ID,
		AccountName: account.Name,
		Platform:    account.Platform,
		Model:       model,
		HTTPStatus:  &httpStatus,
		LatencyMs:   time.Since(started).Milliseconds(),
		CheckedAt:   time.Now(),
	}
	point.ErrorCode, point.ErrorMessage = extractMonthlyProbeError(raw)
	switch {
	case httpStatus >= 200 && httpStatus < 300:
		if point.LatencyMs > 5000 {
			point.Status = "slow"
		} else {
			point.Status = "ok"
		}
	case httpStatus == http.StatusTooManyRequests:
		point.Status = "rate_limited"
	default:
		point.Status = "failed"
	}
	point.ErrorMessage = sanitizeMonthlyProbeError(point.ErrorMessage, account)
	return point
}

func monthlyProbeLocalFailure(account *Account, model, code, message string) MonthlyUpstreamProbePoint {
	point := MonthlyUpstreamProbePoint{
		Model:        model,
		Status:       "failed",
		ErrorCode:    code,
		ErrorMessage: message,
		CheckedAt:    time.Now(),
	}
	if account != nil {
		point.AccountID = account.ID
		point.AccountName = account.Name
		point.Platform = account.Platform
		point.ErrorMessage = sanitizeMonthlyProbeError(point.ErrorMessage, account)
	}
	return point
}

func extractMonthlyProbeError(raw []byte) (string, string) {
	var parsed struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", strings.TrimSpace(string(raw))
	}
	if obj, ok := parsed.Error.(map[string]any); ok {
		code, _ := obj["code"].(string)
		if code == "" {
			code, _ = obj["type"].(string)
		}
		message, _ := obj["message"].(string)
		return code, message
	}
	return "", ""
}

func sanitizeMonthlyProbeError(message string, account *Account) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if account != nil {
		for _, key := range []string{"api_key", "access_token", "refresh_token"} {
			if value := account.GetCredential(key); value != "" {
				message = strings.ReplaceAll(message, value, "[redacted]")
			}
		}
	}
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func isMonthlyUpstreamProbeHealthy(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok", "slow", "rate_limited":
		return true
	default:
		return false
	}
}

func monthlyUpstreamProbeExpectedSlotCount(windowMinutes int) int {
	window := time.Duration(normalizeMonthlyUpstreamProbeWindow(windowMinutes)) * time.Minute
	count := int(window / monthlyUpstreamProbeInterval)
	if window%monthlyUpstreamProbeInterval != 0 {
		count++
	}
	if count < 1 {
		return 1
	}
	return count
}

func monthlyUpstreamProbeSlotDistance(reference time.Time, checkedAt time.Time, expectedSlots int) (int, bool) {
	if expectedSlots <= 0 || checkedAt.IsZero() {
		return 0, false
	}
	delta := reference.Sub(checkedAt)
	halfInterval := monthlyUpstreamProbeInterval / 2
	if delta < -halfInterval {
		return 0, false
	}
	if delta < 0 {
		delta = 0
	}
	slot := int((delta + halfInterval) / monthlyUpstreamProbeInterval)
	if slot < 0 || slot >= expectedSlots {
		return 0, false
	}
	return slot, true
}

func monthlyUpstreamProbeSlotHealth(slots map[int]MonthlyUpstreamProbePoint) (int, int, float64) {
	totalCount := len(slots)
	if totalCount <= 0 {
		return 0, 0, 0
	}
	successCount := 0
	score := 0.0
	for _, point := range slots {
		switch strings.ToLower(strings.TrimSpace(point.Status)) {
		case "ok":
			successCount++
			score += 1
		case "slow", "rate_limited":
			score += 0.5
		}
	}
	return successCount, totalCount, score / float64(totalCount)
}

func (s *OpsService) recordMonthlyUpstreamProbeError(ctx context.Context, point *MonthlyUpstreamProbePoint) {
	if s == nil || point == nil || s.opsRepo == nil {
		return
	}
	statusCode := http.StatusBadGateway
	if point.HTTPStatus != nil && *point.HTTPStatus > 0 {
		statusCode = *point.HTTPStatus
	}
	errorType := strings.TrimSpace(point.ErrorCode)
	if errorType == "" {
		errorType = "monthly_probe_failed"
	}
	errorMessage := strings.TrimSpace(point.ErrorMessage)
	if errorMessage == "" {
		errorMessage = strings.TrimSpace(point.Status)
	}
	accountID := monthlyProbeInt64Ptr(point.AccountID)
	upstreamMessage := monthlyProbeStringPtr(errorMessage)
	upstreamLatencyMs := monthlyProbeInt64Ptr(point.LatencyMs)
	entry := &OpsInsertErrorLogInput{
		RequestID:            "monthly-probe-" + uuid.NewString(),
		AccountID:            accountID,
		Platform:             point.Platform,
		Model:                point.Model,
		RequestPath:          monthlyProbeRequestPath(point),
		InboundEndpoint:      monthlyProbeRequestPath(point),
		UpstreamEndpoint:     monthlyProbeRequestPath(point),
		RequestedModel:       point.Model,
		UpstreamModel:        point.Model,
		UserAgent:            "laoshirenai-monthly-upstream-probe",
		ErrorPhase:           monthlyProbeErrorPhase(point),
		ErrorType:            errorType,
		Severity:             "warning",
		StatusCode:           statusCode,
		ErrorMessage:         errorMessage,
		ErrorSource:          "monthly_upstream_probe",
		ErrorOwner:           "ops",
		UpstreamStatusCode:   point.HTTPStatus,
		UpstreamErrorMessage: upstreamMessage,
		UpstreamLatencyMs:    upstreamLatencyMs,
		IsRetryable:          true,
		CreatedAt:            point.CheckedAt,
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	_ = s.RecordError(ctx, entry, nil)
}

func monthlyProbeRequestPath(point *MonthlyUpstreamProbePoint) string {
	if point == nil {
		return "/monthly-upstream-probe"
	}
	switch point.Platform {
	case PlatformOpenAI:
		return "/v1/responses"
	case PlatformAnthropic:
		return "/v1/messages"
	default:
		return "/monthly-upstream-probe"
	}
}

func monthlyProbeErrorPhase(point *MonthlyUpstreamProbePoint) string {
	if point != nil && point.ProbePath == MonthlyUpstreamProbePathDirectUpstream {
		return "upstream"
	}
	return "internal"
}

func monthlyProbeInt64Ptr(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func monthlyProbeStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
