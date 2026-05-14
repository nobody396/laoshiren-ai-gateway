package service

import "strings"

func optionalTrimmedStringPtr(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalInt64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

// optionalNonEqualStringPtr stores value only when it differs from compare.
func optionalNonEqualStringPtr(value, compare string) *string {
	value = strings.TrimSpace(value)
	compare = strings.TrimSpace(compare)
	if value == "" || value == compare {
		return nil
	}
	return &value
}

func forwardResultBillingModel(requestedModel, upstreamModel string) string {
	if trimmed := strings.TrimSpace(requestedModel); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(upstreamModel)
}
