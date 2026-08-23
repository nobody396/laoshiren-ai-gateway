package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAppendOpsUpstreamError_UsesRequestBodyBytesFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	setOpsUpstreamRequestBody(c, []byte(`{"model":"gpt-5"}`))
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Kind:    "http_error",
		Message: "upstream failed",
	})

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, `{"model":"gpt-5"}`, events[0].UpstreamRequestBody)
}

func TestAppendOpsUpstreamError_UsesRequestBodyStringFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	c.Set(OpsUpstreamRequestBodyKey, `{"model":"gpt-4"}`)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Kind:    "request_error",
		Message: "dial timeout",
	})

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, `{"model":"gpt-4"}`, events[0].UpstreamRequestBody)
}

func TestAppendOpsUpstreamErrorAttachesOnlyMatchingSelectedRouteIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpsReliabilityRouteIdentity(c, OpsReliabilityRouteIdentity{
		AccountID: 53, AccessGroupID: 90, EndpointHash: "0123456789abcdef",
		RoutingFingerprint: "0123456789abcdef0123456789abcdef", UpstreamTransport: "http_sse",
	})
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: 53, UpstreamStatusCode: 503})
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{AccountID: 33, UpstreamStatusCode: 503})

	raw, _ := c.Get(OpsUpstreamErrorsKey)
	events, ok := raw.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Equal(t, int64(90), events[0].AccessGroupID)
	require.Equal(t, "0123456789abcdef", events[0].EndpointHash)
	require.Equal(t, "0123456789abcdef0123456789abcdef", events[0].RoutingFingerprint)
	require.Equal(t, "http_sse", events[0].UpstreamTransport)
	require.Empty(t, events[1].EndpointHash)
}
