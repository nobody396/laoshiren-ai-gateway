package service

import (
	"net/http"
	"time"
)

type openAIOAuth429Disposition uint8

const (
	openAIOAuth429Transient openAIOAuth429Disposition = iota
	openAIOAuth429Quota5h
	openAIOAuth429Quota7d
	openAIOAuth429QuotaReset
)

func classifyOpenAIOAuth429(headers http.Header, responseBody []byte) (openAIOAuth429Disposition, *time.Time) {
	if snapshot := ParseCodexRateLimitHeaders(headers); snapshot != nil {
		if normalized := snapshot.Normalize(); normalized != nil {
			if normalized.Used7dPercent != nil && *normalized.Used7dPercent >= 100 {
				return openAIOAuth429Quota7d, resetTimeFromSeconds(normalized.Reset7dSeconds)
			}
			if normalized.Used5hPercent != nil && *normalized.Used5hPercent >= 100 {
				return openAIOAuth429Quota5h, resetTimeFromSeconds(normalized.Reset5hSeconds)
			}
		}
	}
	if resetAt := openAI429ResetTimeFromHeaders(headers); resetAt != nil {
		return openAIOAuth429QuotaReset, resetAt
	}
	if resetUnix := parseOpenAIRateLimitResetTime(responseBody); resetUnix != nil {
		resetAt := time.Unix(*resetUnix, 0)
		return openAIOAuth429QuotaReset, &resetAt
	}
	return openAIOAuth429Transient, nil
}

func openAI429ResetTimeFromHeaders(headers http.Header) *time.Time {
	snapshot := ParseCodexRateLimitHeaders(headers)
	if snapshot == nil {
		return nil
	}
	normalized := snapshot.Normalize()
	if normalized == nil {
		return nil
	}
	maxSeconds := 0
	for _, seconds := range []*int{normalized.Reset5hSeconds, normalized.Reset7dSeconds} {
		if seconds != nil && *seconds > maxSeconds {
			maxSeconds = *seconds
		}
	}
	if maxSeconds <= 0 {
		return nil
	}
	resetAt := time.Now().Add(time.Duration(maxSeconds) * time.Second)
	return &resetAt
}

func resetTimeFromSeconds(seconds *int) *time.Time {
	if seconds == nil || *seconds <= 0 {
		return nil
	}
	resetAt := time.Now().Add(time.Duration(*seconds) * time.Second)
	return &resetAt
}

func isTransientOpenAIOAuth429(account *Account, statusCode int, headers http.Header, responseBody []byte) bool {
	if account == nil || !account.IsOpenAIOAuth() || statusCode != http.StatusTooManyRequests {
		return false
	}
	disposition, _ := classifyOpenAIOAuth429(headers, responseBody)
	return disposition == openAIOAuth429Transient
}
