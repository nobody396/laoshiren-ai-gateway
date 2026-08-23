package handler

import (
	"context"
	"database/sql/driver"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type handlerReliabilitySettings struct {
	service.SettingRepository
	disableOps bool
}

func (s handlerReliabilitySettings) GetValue(_ context.Context, key string) (string, error) {
	switch key {
	case service.SettingKeyReliabilityObservationEnabled:
		return "true", nil
	case service.SettingKeyOpsMonitoringEnabled:
		if s.disableOps {
			return "false", nil
		}
		return "true", nil
	default:
		return "", service.ErrSettingNotFound
	}
}

func (handlerReliabilitySettings) Set(_ context.Context, _, _ string) error { return nil }

func newHandlerReliabilityEvidence(t *testing.T, settings handlerReliabilitySettings) (*service.ReliabilityEvidenceService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	evidence := service.NewReliabilityEvidenceService(db, settings)
	require.NoError(t, evidence.Start(context.Background()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, evidence.Stop(ctx))
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	return evidence, mock
}

func expectOneReliabilityObservationInsert(mock sqlmock.Sqlmock) {
	args := make([]driver.Value, 25)
	for index := range args {
		args[index] = sqlmock.AnyArg()
	}
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO reliability_observations")).
		ExpectExec().WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func TestBuildFinalReliabilityObservationSeparatesCustomerOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID: 8, UserID: 9, GroupID: int64Pointer(7),
		User:  &service.User{ID: 9},
		Group: &service.Group{ID: 7, Platform: service.PlatformOpenAI},
	})
	c.Set(opsModelKey, "gpt-5.6-sol")
	c.Writer.Header().Set("X-Request-Id", "req-1")
	c.Writer.WriteHeader(200)

	observation := buildFinalReliabilityObservation(c, time.Now().Add(-250*time.Millisecond), nil)

	require.NotNil(t, observation)
	require.Equal(t, "req-1", observation.RequestIdentity)
	require.Equal(t, service.ReliabilityOutcomeSuccess, observation.Outcome)
	require.Equal(t, int64(9), *observation.UserID)
	require.Equal(t, int64(7), *observation.GroupID)
	require.Equal(t, "responses", observation.Protocol)
	require.Equal(t, service.ReliabilityRequestClassText, observation.RequestClass)
	require.GreaterOrEqual(t, observation.LatencyMs, int64(200))
}

func TestBuildFinalReliabilityObservationPreservesWebSocketTransport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportWS)
	c.Writer.Header().Set("X-Request-Id", "req-ws")
	c.Writer.WriteHeader(200)

	observation := buildFinalReliabilityObservation(c, time.Now(), nil)
	require.NotNil(t, observation)
	require.Equal(t, "websocket_responses", observation.Protocol)
}

func TestReliabilityCorrelationPrefersGatewayOwnedClientRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest("POST", "/v1/responses", nil)
	request = request.WithContext(context.WithValue(request.Context(), ctxkey.ClientRequestID, "gateway-client-1"))
	c.Request = request
	c.Writer.Header().Set("X-Request-Id", "reused-upstream-id")
	c.Writer.WriteHeader(200)

	observation := buildFinalReliabilityObservation(c, time.Now(), nil)

	require.Equal(t, "gateway-client-1", observation.RequestIdentity)
	require.Equal(t, "reused-upstream-id", observation.RequestID)
	require.Equal(t, "gateway-client-1", observation.ClientRequestID)
}

func TestBuildFinalReliabilityObservationMarksSuccessfulFailoverAsRecovered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Writer.Header().Set("X-Request-Id", "req-recovered")
	c.Writer.WriteHeader(200)
	c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{AccountID: 53, UpstreamStatusCode: 503}})

	observation := buildFinalReliabilityObservation(c, time.Now().Add(-time.Second), nil)

	require.NotNil(t, observation)
	require.Equal(t, service.ReliabilityOutcomeRecovered, observation.Outcome)
}

func TestBuildFailedReliabilityObservationUsesFinalOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Writer.Header().Set("X-Request-Id", "req-2")
	c.Writer.WriteHeader(503)
	entry := &service.OpsInsertErrorLogInput{
		RequestID: "req-2", UserID: int64Pointer(9), GroupID: int64Pointer(7),
		AccountID: int64Pointer(53), Platform: service.PlatformOpenAI, Model: "gpt-5.6-sol",
		StatusCode: 503, ErrorOwner: "provider", ErrorSource: "upstream_http",
	}

	observation := buildFinalReliabilityObservation(c, time.Now().Add(-time.Second), entry)

	require.NotNil(t, observation)
	require.Equal(t, service.ReliabilityOutcomeFailure, observation.Outcome)
	require.Equal(t, "provider", observation.ErrorOwner)
}

func TestBuildFinalReliabilityObservationExcludesClientCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest("POST", "/v1/responses", nil)
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	c.Request = request.WithContext(ctx)
	c.Writer.Header().Set("X-Request-Id", "req-cancelled")
	c.Writer.WriteHeader(500)
	entry := &service.OpsInsertErrorLogInput{
		RequestID: "req-cancelled", StatusCode: 500, ErrorOwner: "platform", ErrorMessage: "context canceled",
	}

	outcome := buildFinalReliabilityObservation(c, time.Now().Add(-time.Second), entry)

	require.Equal(t, service.ReliabilityOutcomeExcluded, outcome.Outcome)
	require.Equal(t, "client_cancelled", outcome.ExclusionReason)
}

func TestBuildAttemptReliabilityObservationsNeverCountAsCustomerImpact(t *testing.T) {
	groupID := int64(7)
	entry := &service.OpsInsertErrorLogInput{
		RequestID: "req-3", GroupID: &groupID, Platform: service.PlatformOpenAI, Model: "gpt-5.6-sol",
		UpstreamErrors: []*service.OpsUpstreamErrorEvent{
			{AtUnixMs: time.Now().UnixMilli(), AccountID: 53, Platform: service.PlatformOpenAI, UpstreamStatusCode: 503, AccessGroupID: 90, EndpointHash: "0123456789abcdef", RoutingFingerprint: "0123456789abcdef0123456789abcdef", UpstreamTransport: "http_sse"},
			{AtUnixMs: time.Now().UnixMilli(), AccountID: 33, Platform: service.PlatformOpenAI, UpstreamStatusCode: 429},
		},
	}

	observations := buildAttemptReliabilityObservations(entry, "responses", service.ReliabilityRequestClassText)

	require.Len(t, observations, 2)
	require.NotEqual(t, observations[0].AttemptIdentity, observations[1].AttemptIdentity)
	require.Equal(t, int64(90), observations[0].AccessGroupID)
	require.Equal(t, "0123456789abcdef", observations[0].EndpointHash)
	require.Equal(t, "0123456789abcdef0123456789abcdef", observations[0].RoutingFingerprint)
	require.Equal(t, "http_sse", observations[0].Transport)
	for _, observation := range observations {
		require.Equal(t, service.ReliabilityOutcomeFailure, observation.Outcome)
		require.Equal(t, "req-3", observation.RequestIdentity)
	}
}

func TestBuildSuccessfulAttemptReliabilityObservationCapturesSelectedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	groupID := int64(7)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID: 8, UserID: 9, GroupID: &groupID,
		User: &service.User{ID: 9}, Group: &service.Group{ID: 7, Platform: service.PlatformOpenAI},
	})
	c.Set(opsModelKey, "gpt-5.6-sol")
	c.Set(opsAccountIDKey, int64(53))
	service.SetOpsReliabilityRouteIdentity(c, service.OpsReliabilityRouteIdentity{AccountID: 53, AccessGroupID: 90, EndpointHash: "0123456789abcdef", RoutingFingerprint: "0123456789abcdef0123456789abcdef", UpstreamTransport: "http_sse"})
	c.Writer.Header().Set("X-Request-Id", "req-success-attempt")

	observation := buildSuccessfulAttemptReliabilityObservation(c)

	require.NotNil(t, observation)
	require.Equal(t, service.ReliabilityOutcomeSuccess, observation.Outcome)
	require.Equal(t, int64(53), *observation.AccountID)
	require.Equal(t, "success", observation.AttemptIdentity)
	require.Equal(t, int64(90), observation.AccessGroupID)
	require.Equal(t, "0123456789abcdef", observation.EndpointHash)
	require.Equal(t, "0123456789abcdef0123456789abcdef", observation.RoutingFingerprint)
	require.Equal(t, "http_sse", observation.Transport)
}

func TestOpsErrorLoggerMiddlewareQueuesOneFinalCustomerOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := handlerReliabilitySettings{}
	ops := service.NewOpsService(nil, settings, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	evidence, mock := newHandlerReliabilityEvidence(t, settings)
	expectOneReliabilityObservationInsert(mock)
	router := gin.New()
	router.POST("/v1/responses", OpsErrorLoggerMiddleware(ops, evidence), func(c *gin.Context) {
		groupID := int64(7)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 8, UserID: 9, GroupID: &groupID,
			User: &service.User{ID: 9}, Group: &service.Group{ID: 7, Platform: service.PlatformOpenAI},
		})
		setOpsRequestContext(c, "gpt-5.6-sol", true, nil)
		c.Header("X-Request-Id", "req-middleware-1")
		c.Status(200)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("POST", "/v1/responses", nil))

	require.Eventually(t, func() bool { return evidence.Completeness().Written == 1 }, 2*time.Second, 10*time.Millisecond)
}

func TestReliabilityEvidenceRemainsEnabledWhenLegacyOpsMonitoringIsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := handlerReliabilitySettings{disableOps: true}
	ops := service.NewOpsService(nil, settings, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	evidence, mock := newHandlerReliabilityEvidence(t, settings)
	expectOneReliabilityObservationInsert(mock)
	router := gin.New()
	router.POST("/v1/responses", OpsErrorLoggerMiddleware(ops, evidence), func(c *gin.Context) {
		groupID := int64(7)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 8, UserID: 9, GroupID: &groupID,
			User: &service.User{ID: 9}, Group: &service.Group{ID: 7, Platform: service.PlatformOpenAI},
		})
		setOpsRequestContext(c, "gpt-5.6-sol", true, nil)
		c.Header("X-Request-Id", "req-ops-disabled")
		c.Status(200)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/v1/responses", nil))

	require.Eventually(t, func() bool { return evidence.Completeness().Written == 1 }, 2*time.Second, 10*time.Millisecond)
}

func int64Pointer(value int64) *int64 { return &value }
