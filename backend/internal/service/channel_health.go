package service

import "net/http"

type ChannelHealthClass string

const (
	ChannelHealthHealthy           ChannelHealthClass = "healthy"
	ChannelHealthCredentialInvalid ChannelHealthClass = "credential_invalid"
	ChannelHealthRateLimited       ChannelHealthClass = "rate_limited"
	ChannelHealthTransient         ChannelHealthClass = "transient_upstream"
	ChannelHealthBusinessError     ChannelHealthClass = "business_error"
)

type ChannelHealthVerdict struct {
	Class             ChannelHealthClass
	CredentialInvalid bool
	Retryable         bool
}

// ClassifyChannelProbeResult is the single source of truth shared by future
// channel-monitor UI and routing admission. Only 401/403 invalidate an API key;
// rate limits and provider failures remain recoverable operational states.
func ClassifyChannelProbeResult(statusCode int, err error) ChannelHealthVerdict {
	if err != nil {
		return ChannelHealthVerdict{
			Class:     ChannelHealthTransient,
			Retryable: true,
		}
	}
	switch {
	case statusCode <= 0:
		return ChannelHealthVerdict{Class: ChannelHealthTransient, Retryable: true}
	case statusCode >= 200 && statusCode < 400:
		return ChannelHealthVerdict{Class: ChannelHealthHealthy}
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return ChannelHealthVerdict{Class: ChannelHealthCredentialInvalid, CredentialInvalid: true}
	case statusCode == http.StatusTooManyRequests:
		return ChannelHealthVerdict{Class: ChannelHealthRateLimited, Retryable: true}
	case statusCode >= 500:
		return ChannelHealthVerdict{Class: ChannelHealthTransient, Retryable: true}
	default:
		return ChannelHealthVerdict{Class: ChannelHealthBusinessError}
	}
}

type ChannelBalanceState string

const (
	ChannelBalanceUnknown   ChannelBalanceState = "unknown"
	ChannelBalanceAvailable ChannelBalanceState = "available"
	ChannelBalanceLow       ChannelBalanceState = "low"
	ChannelBalanceExhausted ChannelBalanceState = "exhausted"
)

// EvaluateChannelBalance applies the scheduler admission threshold instead of
// treating every positive balance as healthy. A negative balance means the
// supplier did not return a usable value.
func EvaluateChannelBalance(balance, schedulingThreshold float64) ChannelBalanceState {
	if balance < 0 {
		return ChannelBalanceUnknown
	}
	if balance == 0 {
		return ChannelBalanceExhausted
	}
	if schedulingThreshold > 0 && balance <= schedulingThreshold {
		return ChannelBalanceLow
	}
	return ChannelBalanceAvailable
}
