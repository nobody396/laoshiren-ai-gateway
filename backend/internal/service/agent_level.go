package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	AgentLevelLight    = "light"
	AgentLevelStandard = "standard"
	AgentLevelCore     = "core"
	AgentLevelSuper    = "super"
	AgentLevelManual   = "manual_base"

	agentLevelRateCap = 0.20
	agentLevelEpsilon = 0.00000001

	AgentLevelRateSourceLevel      = "agent_level"
	AgentLevelRateSourceMonthly    = "agent_monthly"
	AgentLevelRateSourceCumulative = "agent_cumulative"
	AgentLevelRateSourceManualBase = "agent_manual_base"
)

type AgentLevelRule struct {
	LevelKey                       string     `json:"level_key"`
	LevelName                      string     `json:"level_name"`
	Rate                           float64    `json:"rate"`
	MonthlyConsumptionThreshold    *float64   `json:"monthly_consumption_threshold,omitempty"`
	CumulativeConsumptionThreshold *float64   `json:"cumulative_consumption_threshold,omitempty"`
	SortOrder                      int        `json:"sort_order"`
	Enabled                        bool       `json:"enabled"`
	UpdatedAt                      *time.Time `json:"updated_at,omitempty"`
}

type AgentLevelState struct {
	AgentID              int64      `json:"agent_id"`
	BaseLevelKey         string     `json:"base_level_key"`
	BaseRate             float64    `json:"base_rate"`
	PermanentLevelKey    string     `json:"permanent_level_key"`
	TemporaryLevelKey    *string    `json:"temporary_level_key,omitempty"`
	CurrentLevelKey      string     `json:"current_level_key"`
	CurrentRate          float64    `json:"current_rate"`
	RateSource           string     `json:"rate_source"`
	LastEvaluatedPeriod  string     `json:"last_evaluated_period"`
	LastMonthConsumption float64    `json:"last_month_consumption"`
	TotalConsumption     float64    `json:"total_consumption"`
	NextLevelKey         *string    `json:"next_level_key,omitempty"`
	NextLevelGap         float64    `json:"next_level_gap"`
	EvaluatedAt          *time.Time `json:"evaluated_at,omitempty"`
	CreatedAt            *time.Time `json:"created_at,omitempty"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

type AgentLevelUsageStats struct {
	LastMonthConsumption float64
	TotalConsumption     float64
}

type AgentLevelEvaluationResult struct {
	AgentID int64           `json:"agent_id"`
	State   AgentLevelState `json:"state"`
}

type AgentLevelEvaluationRunResult struct {
	Period         string                       `json:"period"`
	EvaluatedCount int                          `json:"evaluated_count"`
	FailedCount    int                          `json:"failed_count"`
	Items          []AgentLevelEvaluationResult `json:"items,omitempty"`
}

type agentLevelDecision struct {
	baseLevel      AgentLevelRule
	permanentLevel AgentLevelRule
	temporaryLevel *AgentLevelRule
	currentLevel   AgentLevelRule
	currentRate    float64
	rateSource     string
	nextLevelKey   *string
	nextLevelGap   float64
}

func defaultAgentLevelRules() []AgentLevelRule {
	standardCumulative := 1000.0
	coreMonthly := 1500.0
	coreCumulative := 10000.0
	superMonthly := 3000.0
	superCumulative := 50000.0
	return []AgentLevelRule{
		{LevelKey: AgentLevelLight, LevelName: "轻代理", Rate: 0.05, SortOrder: 1, Enabled: true},
		{LevelKey: AgentLevelStandard, LevelName: "标准代理", Rate: 0.10, CumulativeConsumptionThreshold: &standardCumulative, SortOrder: 2, Enabled: true},
		{LevelKey: AgentLevelCore, LevelName: "核心代理", Rate: 0.15, MonthlyConsumptionThreshold: &coreMonthly, CumulativeConsumptionThreshold: &coreCumulative, SortOrder: 3, Enabled: true},
		{LevelKey: AgentLevelSuper, LevelName: "超级代理", Rate: 0.20, MonthlyConsumptionThreshold: &superMonthly, CumulativeConsumptionThreshold: &superCumulative, SortOrder: 4, Enabled: true},
	}
}

func (s *CommissionService) GetAgentLevelRules(ctx context.Context) ([]AgentLevelRule, error) {
	rules := defaultAgentLevelRules()
	if s.levelRepo == nil {
		return rules, nil
	}
	loaded, err := s.levelRepo.GetAgentLevelRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("get agent level rules: %w", err)
	}
	if len(loaded) == 0 {
		return rules, nil
	}
	return normalizeAgentLevelRules(loaded), nil
}

func (s *CommissionService) UpdateAgentLevelRules(ctx context.Context, rules []AgentLevelRule) ([]AgentLevelRule, error) {
	if s.levelRepo == nil {
		return nil, fmt.Errorf("agent level repository is not configured")
	}
	normalized := normalizeAgentLevelRules(rules)
	if err := validateAgentLevelRules(normalized); err != nil {
		return nil, err
	}
	if err := s.levelRepo.UpdateAgentLevelRules(ctx, normalized); err != nil {
		return nil, fmt.Errorf("update agent level rules: %w", err)
	}
	return s.GetAgentLevelRules(ctx)
}

func (s *CommissionService) RunAgentLevelEvaluation(ctx context.Context, agentID int64, now time.Time) (*AgentLevelEvaluationResult, error) {
	if s.levelRepo == nil {
		return nil, fmt.Errorf("agent level repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	rules, err := s.GetAgentLevelRules(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.runAgentLevelEvaluationWithRules(ctx, agentID, now, rules)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *CommissionService) RunAllAgentLevelEvaluations(ctx context.Context, now time.Time) (*AgentLevelEvaluationRunResult, error) {
	if s.levelRepo == nil {
		return nil, fmt.Errorf("agent level repository is not configured")
	}
	agentIDs, err := s.levelRepo.ListAgentIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents for level evaluation: %w", err)
	}
	rules, err := s.GetAgentLevelRules(ctx)
	if err != nil {
		return nil, err
	}
	periodStart, _, periodKey := previousMonthWindow(now)
	run := &AgentLevelEvaluationRunResult{
		Period: periodKey,
		Items:  make([]AgentLevelEvaluationResult, 0, len(agentIDs)),
	}
	_ = periodStart
	for _, agentID := range agentIDs {
		item, evalErr := s.runAgentLevelEvaluationWithRules(ctx, agentID, now, rules)
		if evalErr != nil {
			run.FailedCount++
			slog.Error("agent level evaluation failed", "agent_id", agentID, "error", evalErr)
			continue
		}
		run.EvaluatedCount++
		run.Items = append(run.Items, *item)
	}
	return run, nil
}

func (s *CommissionService) runAgentLevelEvaluationWithRules(ctx context.Context, agentID int64, now time.Time, rules []AgentLevelRule) (*AgentLevelEvaluationResult, error) {
	periodStart, periodEnd, periodKey := previousMonthWindow(now)
	stats, err := s.levelRepo.GetAgentLevelUsageStats(ctx, agentID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("get agent level usage stats: %w", err)
	}
	if stats == nil {
		stats = &AgentLevelUsageStats{}
	}
	baseRate, _, err := s.levelRepo.GetAgentManualBaseRate(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent manual base rate: %w", err)
	}
	decision := decideAgentLevel(normalizeAgentLevelRules(rules), baseRate, stats.LastMonthConsumption, stats.TotalConsumption)
	evaluatedAt := now
	state := &AgentLevelState{
		AgentID:              agentID,
		BaseLevelKey:         decision.baseLevel.LevelKey,
		BaseRate:             clampAgentLevelRate(baseRate),
		PermanentLevelKey:    decision.permanentLevel.LevelKey,
		CurrentLevelKey:      decision.currentLevel.LevelKey,
		CurrentRate:          decision.currentRate,
		RateSource:           decision.rateSource,
		LastEvaluatedPeriod:  periodKey,
		LastMonthConsumption: stats.LastMonthConsumption,
		TotalConsumption:     stats.TotalConsumption,
		NextLevelKey:         decision.nextLevelKey,
		NextLevelGap:         decision.nextLevelGap,
		EvaluatedAt:          &evaluatedAt,
	}
	if decision.temporaryLevel != nil {
		temp := decision.temporaryLevel.LevelKey
		state.TemporaryLevelKey = &temp
	}
	if err := s.levelRepo.UpsertAgentLevelState(ctx, state); err != nil {
		return nil, fmt.Errorf("upsert agent level state: %w", err)
	}
	return &AgentLevelEvaluationResult{AgentID: agentID, State: *state}, nil
}

func decideAgentLevel(rules []AgentLevelRule, baseRate, lastMonthConsumption, totalConsumption float64) agentLevelDecision {
	normalized := enabledAgentLevelRules(normalizeAgentLevelRules(rules))
	light := firstAgentLevelRule(normalized)
	baseRate = clampAgentLevelRate(baseRate)
	baseLevel := levelForRate(normalized, baseRate)
	if baseLevel.LevelKey == "" {
		baseLevel = light
	}
	permanent := light
	for _, rule := range normalized {
		if rule.CumulativeConsumptionThreshold != nil && totalConsumption+agentLevelEpsilon >= *rule.CumulativeConsumptionThreshold && rule.Rate >= permanent.Rate {
			permanent = rule
		}
	}
	var temporary *AgentLevelRule
	for i := range normalized {
		rule := normalized[i]
		if rule.MonthlyConsumptionThreshold != nil && lastMonthConsumption+agentLevelEpsilon >= *rule.MonthlyConsumptionThreshold {
			if temporary == nil || rule.Rate >= temporary.Rate {
				copied := rule
				temporary = &copied
			}
		}
	}

	current := permanent
	source := AgentLevelRateSourceCumulative
	currentRate := permanent.Rate
	if temporary != nil && temporary.Rate > currentRate+agentLevelEpsilon {
		current = *temporary
		currentRate = temporary.Rate
		source = AgentLevelRateSourceMonthly
	}
	if baseRate > currentRate+agentLevelEpsilon {
		current = baseLevel
		currentRate = baseRate
		source = AgentLevelRateSourceManualBase
		if !rateMatchesRule(baseLevel, baseRate) {
			current.LevelKey = AgentLevelManual
			current.LevelName = "手动基准"
			current.Rate = baseRate
		}
	}
	if current.LevelKey == AgentLevelLight && source == AgentLevelRateSourceCumulative {
		source = AgentLevelRateSourceLevel
	}

	nextKey, nextGap := nextCumulativeLevelGap(normalized, totalConsumption)
	return agentLevelDecision{
		baseLevel:      baseLevel,
		permanentLevel: permanent,
		temporaryLevel: temporary,
		currentLevel:   current,
		currentRate:    currentRate,
		rateSource:     source,
		nextLevelKey:   nextKey,
		nextLevelGap:   nextGap,
	}
}

func normalizeAgentLevelRules(rules []AgentLevelRule) []AgentLevelRule {
	if len(rules) == 0 {
		rules = defaultAgentLevelRules()
	}
	out := make([]AgentLevelRule, 0, len(rules))
	for _, rule := range rules {
		rule.LevelKey = strings.TrimSpace(rule.LevelKey)
		rule.LevelName = strings.TrimSpace(rule.LevelName)
		rule.Rate = clampAgentLevelRate(rule.Rate)
		if rule.LevelName == "" {
			rule.LevelName = rule.LevelKey
		}
		if rule.SortOrder == 0 {
			rule.SortOrder = len(out) + 1
		}
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].Rate < out[j].Rate
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func enabledAgentLevelRules(rules []AgentLevelRule) []AgentLevelRule {
	out := make([]AgentLevelRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Enabled {
			out = append(out, rule)
		}
	}
	if len(out) == 0 {
		return defaultAgentLevelRules()
	}
	return out
}

func validateAgentLevelRules(rules []AgentLevelRule) error {
	if len(rules) == 0 {
		return infraerrors.BadRequest("INVALID_AGENT_LEVEL_RULES", "agent level rules are required")
	}
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if strings.TrimSpace(rule.LevelKey) == "" {
			return infraerrors.BadRequest("INVALID_AGENT_LEVEL_RULE", "level_key is required")
		}
		if _, ok := seen[rule.LevelKey]; ok {
			return infraerrors.BadRequest("INVALID_AGENT_LEVEL_RULE", "duplicate level_key")
		}
		seen[rule.LevelKey] = struct{}{}
		if rule.Rate < 0 || rule.Rate-agentLevelRateCap > agentLevelEpsilon {
			return infraerrors.BadRequest("INVALID_AGENT_LEVEL_RATE", "agent level rate must be between 0 and 0.2")
		}
		if rule.MonthlyConsumptionThreshold != nil && *rule.MonthlyConsumptionThreshold < 0 {
			return infraerrors.BadRequest("INVALID_AGENT_LEVEL_THRESHOLD", "monthly threshold must be non-negative")
		}
		if rule.CumulativeConsumptionThreshold != nil && *rule.CumulativeConsumptionThreshold < 0 {
			return infraerrors.BadRequest("INVALID_AGENT_LEVEL_THRESHOLD", "cumulative threshold must be non-negative")
		}
	}
	return nil
}

func firstAgentLevelRule(rules []AgentLevelRule) AgentLevelRule {
	if len(rules) == 0 {
		return defaultAgentLevelRules()[0]
	}
	return rules[0]
}

func levelForRate(rules []AgentLevelRule, rate float64) AgentLevelRule {
	var selected AgentLevelRule
	for _, rule := range rules {
		if rule.Rate <= rate+agentLevelEpsilon {
			selected = rule
		}
	}
	if selected.LevelKey == "" {
		return firstAgentLevelRule(rules)
	}
	return selected
}

func rateMatchesRule(rule AgentLevelRule, rate float64) bool {
	return math.Abs(rule.Rate-rate) <= agentLevelEpsilon
}

func nextCumulativeLevelGap(rules []AgentLevelRule, totalConsumption float64) (*string, float64) {
	for _, rule := range rules {
		if rule.CumulativeConsumptionThreshold == nil {
			continue
		}
		if totalConsumption+agentLevelEpsilon < *rule.CumulativeConsumptionThreshold {
			key := rule.LevelKey
			return &key, *rule.CumulativeConsumptionThreshold - totalConsumption
		}
	}
	return nil, 0
}

func previousMonthWindow(now time.Time) (time.Time, time.Time, string) {
	if now.IsZero() {
		now = time.Now()
	}
	loc := now.Location()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	start := monthStart.AddDate(0, -1, 0)
	return start, monthStart, start.Format("2006-01")
}

func clampAgentLevelRate(rate float64) float64 {
	if rate < 0 {
		return 0
	}
	if rate > agentLevelRateCap {
		return agentLevelRateCap
	}
	return rate
}
