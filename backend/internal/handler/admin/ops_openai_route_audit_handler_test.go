package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminOpenAIRouteAuditRepoStub struct{}

var adminOpenAIRouteAuditNow = time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

func (adminOpenAIRouteAuditRepoStub) CheckOpenAIRouteShadowDecisionStorage(context.Context) error {
	return nil
}

func (adminOpenAIRouteAuditRepoStub) CreateOpenAIRouteShadowDecision(context.Context, *service.OpenAIRouteShadowDecisionRecord) error {
	return nil
}

func (adminOpenAIRouteAuditRepoStub) ListOpenAIRouteShadowDecisions(context.Context, *service.OpenAIRouteShadowDecisionFilter) (*service.OpenAIRouteShadowDecisionList, error) {
	return &service.OpenAIRouteShadowDecisionList{
		Decisions: []*service.OpenAIRouteShadowDecisionRecord{{DecisionID: "shadow:test", GroupID: 7, Model: "gpt-5.6-sol", RequestClass: service.OpenAIRouteRequestClassText}},
		Total:     1,
		Page:      1,
		PageSize:  20,
	}, nil
}

func (adminOpenAIRouteAuditRepoStub) GetOpenAIRouteShadowDecisionStats(context.Context, *service.OpenAIRouteShadowDecisionFilter) (*service.OpenAIRouteShadowDecisionStats, error) {
	return &service.OpenAIRouteShadowDecisionStats{
		Total:                          200,
		Evaluated:                      200,
		EvaluatedLinkedSuccessfulUsage: 200,
		PolicySnapshotVariants:         1,
		ActivationIDVariants:           1,
		ShadowStartedAtVariants:        1,
		ShadowStartedAt:                adminOpenAIRouteAuditNow.Add(-72 * time.Hour),
		PolicyMaxAccountShare:          0.8,
		PolicyMaxProviderShare:         0.9,
		CoveredHourBuckets:             72,
		FirstDecisionAt:                adminOpenAIRouteAuditNow.Add(-72 * time.Hour),
		LastDecisionAt:                 adminOpenAIRouteAuditNow,
		SelectedAccounts:               []service.OpenAIRouteShadowSelectedAccountStats{{AccountID: 23, SelectedPercent: 60}},
		SelectedProviders:              []service.OpenAIRouteShadowSelectedProviderStats{{ProviderKey: "pomelo", SelectedPercent: 60}},
	}, nil
}

func TestParseOpenAIRouteShadowDecisionFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?time_range=1h&group_id=7&model=gpt-5.6-sol&request_class=image&policy_mode=shadow&policy_version=3&activation_id=activation-3&evaluated=true&diverged=false&emergency=true&page=2&page_size=500", nil)

	filter, err := parseOpenAIRouteShadowDecisionFilter(c, true)
	require.NoError(t, err)
	require.NotNil(t, filter.GroupID)
	require.Equal(t, int64(7), *filter.GroupID)
	require.NotNil(t, filter.PolicyVersion)
	require.Equal(t, 3, *filter.PolicyVersion)
	require.Equal(t, "gpt-5.6-sol", filter.Model)
	require.Equal(t, service.OpenAIRouteRequestClassImage, filter.RequestClass)
	require.Equal(t, service.OpenAIRoutePolicyShadow, filter.PolicyMode)
	require.Equal(t, "activation-3", filter.ActivationID)
	require.Equal(t, 2, filter.Page)
	require.Equal(t, 200, filter.PageSize)
	require.Equal(t, true, *filter.Evaluated)
	require.Equal(t, false, *filter.Diverged)
	require.Equal(t, true, *filter.Emergency)
}

func TestParseOpenAIRouteShadowDecisionFilterRejectsInvalidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{
		"/?group_id=bad",
		"/?policy_version=-1",
		"/?evaluated=maybe",
		"/?request_class=audio",
		"/?policy_mode=invalid",
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", query, nil)
		_, err := parseOpenAIRouteShadowDecisionFilter(c, true)
		require.Error(t, err, query)
	}
}

func TestOpenAIRouteShadowAuditHandlersReturnPersistedEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsService := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	opsService.SetOpenAIRouteAuditService(service.NewOpenAIRouteAuditService(adminOpenAIRouteAuditRepoStub{}))
	handler := NewOpsHandler(opsService)
	router := gin.New()
	router.GET("/decisions", handler.ListOpenAIRouteShadowDecisions)
	router.GET("/stats", handler.GetOpenAIRouteShadowDecisionStats)
	router.GET("/health", handler.GetOpenAIRouteAuditHealth)
	router.GET("/assessment", handler.AssessOpenAIRouteShadowPromotion)

	for _, path := range []string{
		"/decisions?time_range=1h",
		"/stats?time_range=1h",
		"/health",
		"/assessment?start_time=2026-08-09T12:00:00Z&end_time=2026-08-12T12:00:00Z&group_id=7&model=gpt-5.6-sol&request_class=text&policy_version=4&activation_id=activation-4",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, path)
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope), path)
		require.Equal(t, float64(0), envelope["code"], path)
	}
}

func TestParseOpenAIRoutePromotionAssessmentFilterRequiresExactScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	valid := "/?start_time=2026-08-09T12:00:00Z&end_time=2026-08-12T12:00:00Z&group_id=7&model=gpt-5.6-sol&request_class=text&policy_version=4&activation_id=activation-4"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, valid, nil)
	filter, err := parseOpenAIRoutePromotionAssessmentFilter(c)
	require.NoError(t, err)
	require.Equal(t, service.OpenAIRoutePolicyShadow, filter.PolicyMode)

	for _, query := range []string{
		"/?time_range=72h&group_id=7&model=gpt-5.6-sol&request_class=text&policy_version=4",
		"/?start_time=2026-08-09T12:00:00Z&end_time=2026-08-12T12:00:00Z&model=gpt-5.6-sol&request_class=text&policy_version=4&activation_id=activation-4",
		valid + "&evaluated=true",
		valid + "&policy_mode=legacy",
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, query, nil)
		_, err := parseOpenAIRoutePromotionAssessmentFilter(c)
		require.Error(t, err, query)
	}
}

func TestOpenAIRoutePromotionAssessmentHandlerNeverAuthorizesTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsService := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	opsService.SetOpenAIRouteAuditService(service.NewOpenAIRouteAuditService(adminOpenAIRouteAuditRepoStub{}))
	handler := NewOpsHandler(opsService)
	router := gin.New()
	router.GET("/assessment", handler.AssessOpenAIRouteShadowPromotion)

	w := httptest.NewRecorder()
	path := "/assessment?start_time=2026-08-09T12:00:00Z&end_time=2026-08-12T12:00:00Z&group_id=7&model=gpt-5.6-sol&request_class=text&policy_version=4&activation_id=activation-4"
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, w.Code)
	var envelope struct {
		Data struct {
			AutomatedEvidenceReady bool   `json:"automated_evidence_ready"`
			EligibleNextStage      string `json:"eligible_next_stage"`
			ManualApprovalRequired bool   `json:"manual_approval_required"`
			EnforceAvailable       bool   `json:"enforce_available"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.False(t, envelope.Data.AutomatedEvidenceReady, "no observation collector is available in this test")
	require.Equal(t, "none", envelope.Data.EligibleNextStage)
	require.True(t, envelope.Data.ManualApprovalRequired)
	require.False(t, envelope.Data.EnforceAvailable)
}
