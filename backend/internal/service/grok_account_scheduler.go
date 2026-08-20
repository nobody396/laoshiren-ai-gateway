package service

import (
	"context"
	"errors"
	"fmt"
)

// SelectOpenAICompatibleAccountWithScheduler dispatches a protocol-compatible
// request to a platform-isolated scheduler.
func (s *OpenAIGatewayService) SelectOpenAICompatibleAccountWithScheduler(
	ctx context.Context,
	platform string,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.SelectOpenAICompatibleAccountWithSchedulerForRouting(
		ctx, platform, groupID, previousResponseID, sessionHash, requestedModel,
		excludedIDs, requiredTransport, requireCompact, false,
	)
}

// SelectOpenAICompatibleAccountWithSchedulerForRouting adds request-intent
// routing without changing the legacy entry point used by text traffic.
func (s *OpenAIGatewayService) SelectOpenAICompatibleAccountWithSchedulerForRouting(
	ctx context.Context,
	platform string,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
	preferImageGeneration bool,
	routeEndpoints ...string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if platform == PlatformGrok {
		selection, decision, err := s.SelectGrokAccountWithScheduler(ctx, groupID, sessionHash, requestedModel, excludedIDs, false)
		return selection, decision, s.classifyNoServableSelectionError(ctx, platform, groupID, err)
	}
	selection, decision, err := s.selectAccountWithSchedulerForRouting(
		ctx, groupID, previousResponseID, sessionHash, requestedModel,
		excludedIDs, requiredTransport, requireCompact, preferImageGeneration, routeEndpoints...,
	)
	return selection, decision, s.classifyNoServableSelectionError(ctx, platform, groupID, err)
}

// classifyNoServableSelectionError 在“零候选账号”失败时区分结构性不可服务
// （分组在该端点的请求平台下没有任何账号，例如 gemini 分组误调
// /v1/chat/completions）与暂时性不可用（账号存在但限流/过载/排队）。
// 前者返回 *NoServableAccountsError，由 handler 映射为客户端 4xx；
// 后者保持原错误（handler 返回 503，继续计入平台侧告警）。
func (s *OpenAIGatewayService) classifyNoServableSelectionError(ctx context.Context, platform string, groupID *int64, err error) error {
	if err == nil {
		return nil
	}
	var modelErr *ModelNotSupportedError
	if errors.As(err, &modelErr) {
		return err
	}
	if !errors.Is(err, ErrNoAvailableAccounts) {
		return err
	}
	if s.groupHasAccountsForPlatform(ctx, groupID, platform) {
		return err
	}
	return &NoServableAccountsError{Platform: platform}
}

// SelectGrokAccountWithScheduler is the deliberately narrow Grok scheduling
// entry point. It reuses the gateway's sticky-session, concurrency and wait-plan
// primitives without broadening the OpenAI scheduler's platform assumptions.
// This keeps the upstream integration isolated and prevents Grok accounts from
// ever leaking into an OpenAI group (or vice versa).
func (s *OpenAIGatewayService) SelectGrokAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requireMediaGeneration bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	decision := OpenAIAccountScheduleDecision{Layer: openAIAccountScheduleLayerLoadBalance}
	accounts, err := s.listSchedulableGrokAccounts(ctx, groupID)
	if err != nil {
		return nil, decision, err
	}

	valid := func(account *Account) bool {
		return grokAccountEligibleForScheduling(account, requestedModel, excludedIDs, requireMediaGeneration)
	}

	// Sticky first. Re-read the account so a recently disabled or rate-limited
	// credential cannot be selected from a stale cache entry.
	if sessionHash != "" && s.cache != nil {
		if accountID, stickyErr := s.getStickySessionAccountID(ctx, groupID, sessionHash); stickyErr == nil && accountID > 0 {
			if account, getErr := s.getSchedulableAccount(ctx, accountID); getErr == nil && valid(account) && grokAccountMatchesGroup(account, groupID) {
				if latest, latestErr := s.accountRepo.GetByID(ctx, account.ID); latestErr == nil && valid(latest) {
					result, acquireErr := s.tryAcquireAccountSlot(ctx, latest.ID, latest.Concurrency)
					if acquireErr == nil && result != nil && result.Acquired {
						decision.Layer = openAIAccountScheduleLayerSessionSticky
						decision.StickySessionHit = true
						decision.SelectedAccountID = latest.ID
						decision.SelectedAccountType = latest.Type
						selection, buildErr := s.newSelectionResult(ctx, latest, true, result.ReleaseFunc, nil)
						return selection, decision, buildErr
					}
				}
			}
			_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		}
	}

	candidates := make([]*Account, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if valid(account) {
			candidates = append(candidates, account)
		}
	}
	decision.CandidateCount = len(candidates)
	decision.TopK = len(candidates)
	if len(candidates) == 0 {
		return nil, decision, fmt.Errorf("%w: no available Grok accounts supporting model %q", ErrNoAvailableAccounts, requestedModel)
	}

	sortAccountsByPriorityAndLastUsed(candidates, false, groupID)
	var waitCandidate *Account
	for _, candidate := range candidates {
		latest, latestErr := s.accountRepo.GetByID(ctx, candidate.ID)
		if latestErr != nil || !valid(latest) || !grokAccountMatchesGroup(latest, groupID) {
			continue
		}
		if waitCandidate == nil {
			waitCandidate = latest
		}
		result, acquireErr := s.tryAcquireAccountSlot(ctx, latest.ID, latest.Concurrency)
		if acquireErr != nil || result == nil || !result.Acquired {
			continue
		}
		if sessionHash != "" {
			_ = s.BindStickySession(ctx, groupID, sessionHash, latest.ID)
		}
		decision.SelectedAccountID = latest.ID
		decision.SelectedAccountType = latest.Type
		selection, buildErr := s.newSelectionResult(ctx, latest, true, result.ReleaseFunc, nil)
		return selection, decision, buildErr
	}

	if waitCandidate == nil {
		return nil, decision, fmt.Errorf("%w: no schedulable Grok accounts", ErrNoAvailableAccounts)
	}
	cfg := s.schedulingConfig()
	decision.SelectedAccountID = waitCandidate.ID
	decision.SelectedAccountType = waitCandidate.Type
	selection, buildErr := s.newSelectionResult(ctx, waitCandidate, false, nil, &AccountWaitPlan{
		AccountID:      waitCandidate.ID,
		MaxConcurrency: waitCandidate.Concurrency,
		Timeout:        cfg.FallbackWaitTimeout,
		MaxWaiting:     cfg.FallbackMaxWaiting,
	})
	return selection, decision, buildErr
}

func grokAccountEligibleForScheduling(
	account *Account,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requireMediaGeneration bool,
) bool {
	if account == nil || account.Platform != PlatformGrok || !account.IsSchedulable() {
		return false
	}
	if excludedIDs != nil {
		if _, excluded := excludedIDs[account.ID]; excluded {
			return false
		}
	}
	if paused, _ := shouldAutoPauseGrokAccountByQuota(account); paused {
		return false
	}
	if requestedModel != "" && !account.IsModelSupported(requestedModel) {
		return false
	}
	if requireMediaGeneration && !supportsGrokMediaGenerationScheduling(account) {
		return false
	}
	return true
}

func supportsGrokMediaGenerationScheduling(account *Account) bool {
	if account == nil || !account.IsGrok() {
		return false
	}
	eligible, reason := account.GrokMediaGenerationEligibility()
	// An unobserved OAuth account remains a candidate so the request path can
	// perform the billing probe before forwarding. Known Free, forbidden or
	// explicitly disabled accounts are excluded before consuming switch budget.
	return eligible || reason == "billing_unobserved"
}

func grokAccountMatchesGroup(account *Account, groupID *int64) bool {
	if account == nil {
		return false
	}
	if groupID == nil {
		return true
	}
	for _, membership := range account.AccountGroups {
		if membership.GroupID == *groupID {
			return true
		}
	}
	return false
}

func (s *OpenAIGatewayService) listSchedulableGrokAccounts(ctx context.Context, groupID *int64) ([]Account, error) {
	if s.schedulerSnapshot != nil {
		accounts, _, err := s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformGrok, false)
		return accounts, err
	}
	if s.accountRepo == nil {
		return nil, fmt.Errorf("account repository is unavailable")
	}
	if s.cfg != nil && s.cfg.RunMode == "simple" {
		return s.accountRepo.ListSchedulableByPlatform(ctx, PlatformGrok)
	}
	if groupID != nil {
		return s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformGrok)
	}
	return s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformGrok)
}
