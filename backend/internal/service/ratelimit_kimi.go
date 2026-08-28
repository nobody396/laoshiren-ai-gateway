package service

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

const kimiConcurrentRequestLimitMessage = "You've reached your concurrent request limit. Please wait for your ongoing requests to finish and try again."
const kimiConcurrencyLimitReasonPrefix = "kimi_concurrency_limit"

func isKimiConcurrencyLimit403(account *Account, upstreamMsg string) bool {
	return isKimiCompatibleAccount(account) && strings.TrimSpace(upstreamMsg) == kimiConcurrentRequestLimitMessage
}

func isKimiCompatibleAccount(account *Account) bool {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(account.GetOpenAIBaseURL()))
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "api.kimi.com", "api.moonshot.cn", "api.moonshot.ai":
		return true
	default:
		return false
	}
}

func (s *RateLimitService) handleKimiConcurrencyLimit403(ctx context.Context, account *Account) {
	until := time.Now().Add(time.Duration(openAI403CooldownMinutesDefault) * time.Minute)
	reason := kimiConcurrencyLimitReasonPrefix + ": " + kimiConcurrentRequestLimitMessage
	if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
		slog.Warn("kimi_concurrency_limit_set_temp_unschedulable_failed", "account_id", account.ID, "error", err)
		return
	}
	slog.Info("kimi_concurrency_limited", "account_id", account.ID, "until", until.UTC())
}
