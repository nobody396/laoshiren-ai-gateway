package service

import (
	"context"
	"fmt"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
)

const geminiChatCompletionsEndpoint = "/v1/chat/completions"

// SelectGeminiAccountWithScheduler 是 Gemini 桥接调度的刻意窄化入口，与 Grok
// 调度同构：复用网关的粘性会话、并发与 wait-plan 原语，但只接受
// platform=gemini 且 type=apikey 的账号（其上游为 OpenAI 兼容协议），
// 避免 Gemini OAuth（code assist）账号或 antigravity 混合调度账号
// 泄漏到 OpenAI 格式转发路径。
func (s *OpenAIGatewayService) SelectGeminiAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	decision := OpenAIAccountScheduleDecision{Layer: openAIAccountScheduleLayerLoadBalance}
	accounts, err := s.listSchedulableGeminiAccounts(ctx, groupID)
	if err != nil {
		return nil, decision, err
	}

	valid := func(account *Account) bool {
		return geminiChatBridgeAccountEligible(account, requestedModel, excludedIDs)
	}

	// Sticky first. Re-read the account so a recently disabled or rate-limited
	// credential cannot be selected from a stale cache entry.
	if sessionHash != "" && s.cache != nil {
		if accountID, stickyErr := s.getStickySessionAccountID(ctx, groupID, sessionHash); stickyErr == nil && accountID > 0 {
			if account, getErr := s.getSchedulableAccount(ctx, accountID); getErr == nil && valid(account) && geminiAccountMatchesGroup(account, groupID) {
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
		if geminiChatBridgeAllCandidatesModelUnsupported(accounts, requestedModel, excludedIDs) {
			return nil, decision, &ModelNotSupportedError{RequestedModel: requestedModel, Platform: PlatformGemini}
		}
		return nil, decision, fmt.Errorf("%w: no available Gemini accounts supporting model %q", ErrNoAvailableAccounts, requestedModel)
	}

	sortAccountsByPriorityAndLastUsed(candidates, false, groupID)
	var waitCandidate *Account
	for _, candidate := range candidates {
		latest, latestErr := s.accountRepo.GetByID(ctx, candidate.ID)
		if latestErr != nil || !valid(latest) || !geminiAccountMatchesGroup(latest, groupID) {
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
		return nil, decision, fmt.Errorf("%w: no schedulable Gemini accounts", ErrNoAvailableAccounts)
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

// geminiChatBridgeAccountEligible 判定账号能否在 /v1/chat/completions 桥接路径
// 上服务：仅 platform=gemini 的 apikey 账号（上游为 OpenAI 兼容协议）。
// Gemini OAuth（code assist）账号的上游不是 OpenAI 格式，不能进入该路径。
func geminiChatBridgeAccountEligible(
	account *Account,
	requestedModel string,
	excludedIDs map[int64]struct{},
) bool {
	if account == nil || account.Platform != PlatformGemini || account.Type != AccountTypeAPIKey || !account.IsSchedulable() {
		return false
	}
	if excludedIDs != nil {
		if _, excluded := excludedIDs[account.ID]; excluded {
			return false
		}
	}
	if requestedModel != "" && !account.IsModelSupported(requestedModel) {
		return false
	}
	return true
}

// geminiChatBridgeAllCandidatesModelUnsupported 判断排除 excluded 之后，剩余的
// 可桥接 Gemini 账号是否全部因 model_mapping 不支持请求模型而被过滤
// （无映射的账号视为支持任意模型）。用于把“未知模型”归类为客户端 4xx
// 而不是 503。
func geminiChatBridgeAllCandidatesModelUnsupported(accounts []Account, requestedModel string, excludedIDs map[int64]struct{}) bool {
	if requestedModel == "" || len(accounts) == 0 {
		return false
	}
	total, unsupported := 0, 0
	for i := range accounts {
		acc := &accounts[i]
		if excludedIDs != nil {
			if _, excluded := excludedIDs[acc.ID]; excluded {
				continue
			}
		}
		if acc.Platform != PlatformGemini || acc.Type != AccountTypeAPIKey || !acc.IsSchedulable() {
			continue
		}
		total++
		if !acc.IsModelSupported(requestedModel) {
			unsupported++
		}
	}
	return total > 0 && unsupported == total
}

func geminiAccountMatchesGroup(account *Account, groupID *int64) bool {
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

func (s *OpenAIGatewayService) listSchedulableGeminiAccounts(ctx context.Context, groupID *int64) ([]Account, error) {
	if s.schedulerSnapshot != nil {
		// hasForcePlatform=true：强制单平台快照，排除 antigravity 混合调度
		// 账号；桥接路径只能由纯 Gemini 账号服务。
		accounts, _, err := s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformGemini, true)
		return accounts, err
	}
	if s.accountRepo == nil {
		return nil, fmt.Errorf("account repository is unavailable")
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		return s.accountRepo.ListSchedulableByPlatform(ctx, PlatformGemini)
	}
	if groupID != nil {
		return s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformGemini)
	}
	return s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformGemini)
}
