package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A multi-group key that asks for a model its groups do not declare, or that
// has no funding, is describing its own request — not a gateway fault. These
// used to be booked as error_owner=platform, which put customer model-picker
// mistakes into the platform error rate and paged on them.
func TestMultiGroupClientRejectionIsNotPlatformOwned(t *testing.T) {
	body := []byte(`{"error":{"message":"No authorized group declares model claude-opus-5 for the responses endpoint","type":"invalid_request_error"}}`)

	parsed := parseOpsErrorResponse(body)
	phase := classifyOpsPhase(normalizeOpsErrorType(parsed.ErrorType, parsed.Code), parsed.Message, parsed.Code)

	require.Equal(t, "request", phase)
	require.Equal(t, "client", classifyOpsErrorOwner(phase, parsed.Message))
	require.Equal(t, "client_request", classifyOpsErrorSource(phase, parsed.Message))
}

// A routing or catalog outage really is ours: it still counts against the
// platform and still pages.
func TestMultiGroupRoutingOutageStaysPlatformOwned(t *testing.T) {
	body := []byte(`{"error":{"message":"Model catalog is temporarily unavailable","type":"api_error"}}`)

	parsed := parseOpsErrorResponse(body)
	phase := classifyOpsPhase(normalizeOpsErrorType(parsed.ErrorType, parsed.Code), parsed.Message, parsed.Code)

	require.Equal(t, "internal", phase)
	require.Equal(t, "platform", classifyOpsErrorOwner(phase, parsed.Message))
}
