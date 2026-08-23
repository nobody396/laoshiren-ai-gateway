package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type statusHandlerSettingStub struct{ service.SettingRepository }

type enabledStatusHandlerSettingStub struct{ service.SettingRepository }

func (statusHandlerSettingStub) GetValue(context.Context, string) (string, error) {
	return "false", nil
}

func (enabledStatusHandlerSettingStub) GetValue(context.Context, string) (string, error) {
	return "true", nil
}

func TestPublicStatusHandlerReturnsDisabledSnapshotWithoutDatabaseQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewStatusHandler(service.NewStatusControlService(nil, statusHandlerSettingStub{}, nil))
	router.GET("/api/v1/service-status", handler.GetPublicStatus)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/service-status", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"enabled":false`)
}

func TestPublicStatusEndpointNeverExposesInternalSelectorsOrRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Now()
	mock.ExpectQuery("SELECT f.code").WillReturnRows(sqlmock.NewRows([]string{
		"family_code", "family_name", "product_code", "product_name", "critical", "status", "reason", "evidence_at", "computed_at",
		"component_code", "component_name", "model_pattern", "access_mode", "component_status", "component_reason", "component_evidence_at", "component_computed_at",
	}).AddRow("openai-codex", "OpenAI / Codex", "openai-codex-api", "OpenAI / Codex API", true, "operational", "fresh_success", now, now,
		"openai-codex-api-http", "OpenAI / Codex API · HTTP", "gpt-*", "http", "operational", "fresh_success", now, now))
	router := gin.New()
	handler := NewStatusHandler(service.NewStatusControlService(db, enabledStatusHandlerSettingStub{}, nil))
	router.GET("/api/v1/service-status", handler.GetPublicStatus)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/service-status", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `"status":"operational"`)
	for _, forbidden := range []string{"model_pattern", "group_id", "group_name", "route_fingerprint", "account_id", "base_url", "supplier", "gpt-*"} {
		require.NotContains(t, body, forbidden)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
