package service

import (
	"context"
	"log/slog"
	"strings"
)

const (
	AntigravityPrivacySet    = "privacy_set"
	AntigravityPrivacyFailed = "privacy_set_failed"
)

func setAntigravityPrivacy(ctx context.Context, accessToken, projectID, proxyURL string) string {
	_ = ctx
	_ = projectID
	_ = proxyURL
	if accessToken == "" {
		return ""
	}
	slog.Warn("antigravity_privacy_set_unsupported")
	return AntigravityPrivacyFailed
}

func applyAntigravityPrivacyMode(account *Account, mode string) {
	if account == nil || strings.TrimSpace(mode) == "" {
		return
	}
	extra := make(map[string]any, len(account.Extra)+1)
	for k, v := range account.Extra {
		extra[k] = v
	}
	extra["privacy_mode"] = mode
	account.Extra = extra
}
