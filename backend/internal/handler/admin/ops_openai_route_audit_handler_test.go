package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminOpenAIRouteAuditRepoStub struct{}

func (adminOpenAIRouteAuditRepoStub) CheckOpenAIRouteShadowDecisionStorage(context.Context) error {
	return nil
}

func (adminOpenAIRouteAuditRepoStub) CreateOpenAIRouteShadowDecision(context.Context, *service.OpenAIRouteShadowDecisionRecord) error {
	return nil
}

func (adminOpenAIRouteAuditRepoStub) ListOpenAIRouteShadowDecisions(context.Context, *service.OpenAIRouteShadowDecisionFilter) (*service.OpenAIRouteShadowDecisionList, error) {
	return &service.OpenAIRouteShadowDecisionList{
		Decisions: []*service.OpenAIRouteShadowDecisionRecord{{DecisionID: "shadow:test", GroupID: 7, Model: "gpt-5.6-sol"}},
		Total:     1,
		Page:      1,
		PageSize:  20,
	}, nil
}

func (adminOpenAIRouteAuditRepoStub) GetOpenAIRouteShadowDecisionStats(context.Context, *service.OpenAIRouteShadowDecisionFilter) (*service.OpenAIRouteShadowDecisionStats, error) {
	return &service.OpenAIRouteShadowDecisionStats{Total: 1, Evaluated: 1}, nil
}

func TestParseOpenAIRouteShadowDecisionFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?time_range=1h&group_id=7&model=gpt-5.6-sol&policy_version=3&evaluated=true&diverged=false&emergency=true&page=2&page_size=500", nil)

	filter, err := parseOpenAIRouteShadowDecisionFilter(c, true)
	require.NoError(t, err)
	require.NotNil(t, filter.GroupID)
	require.Equal(t, int64(7), *filter.GroupID)
	require.NotNil(t, filter.PolicyVersion)
	require.Equal(t, 3, *filter.PolicyVersion)
	require.Equal(t, "gpt-5.6-sol", filter.Model)
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

	for _, path := range []string{"/decisions?time_range=1h", "/stats?time_range=1h", "/health"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, path)
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope), path)
		require.Equal(t, float64(0), envelope["code"], path)
	}
}
