package handler

import (
	"testing"

	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/stretchr/testify/require"
)

// A multi-group key that asks for a model its groups do not declare, or that
// has no funding, is describing its own request — not a gateway fault. These
// used to be booked as error_owner=platform, which put customer model-picker
// mistakes into the platform error rate and paged on them.
func TestMultiGroupClientRejectionIsNotPlatformOwned(t *testing.T) {
	body := []byte(`{"error":{"message":"No authorized group declares model claude-opus-5 for the responses endpoint; this key serves it on /v1/messages instead","type":"` + middleware2.MultiGroupRequestRejectedCode + `"}}`)

	parsed := parseOpsErrorResponse(body)
	normalizedType := normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
	phase := classifyOpsPhase(normalizedType, parsed.Message, parsed.Code)

	require.Equal(t, "invalid_request_error", normalizedType)
	require.Equal(t, "request", phase)
	require.Equal(t, "client", classifyOpsErrorOwner(phase, parsed.Message))
	require.Equal(t, "client_request", classifyOpsErrorSource(phase, parsed.Message))
}

// A routing or catalog outage really is ours: it keeps its own code so it still
// counts against the platform and still pages.
func TestMultiGroupRoutingOutageStaysPlatformOwned(t *testing.T) {
	body := []byte(`{"error":{"message":"Model catalog is temporarily unavailable","type":"` + middleware2.MultiGroupRoutingUnavailableCode + `"}}`)

	parsed := parseOpsErrorResponse(body)
	normalizedType := normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
	phase := classifyOpsPhase(normalizedType, parsed.Message, parsed.Code)

	require.Equal(t, "api_error", normalizedType)
	require.Equal(t, "internal", phase)
	require.Equal(t, "platform", classifyOpsErrorOwner(phase, parsed.Message))
}
