package handler

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

type publicStatsTotalsStub struct {
	totals service.DashboardLifetimeTotals
	err    error
}

func (s *publicStatsTotalsStub) LifetimeTotals(_ context.Context) (service.DashboardLifetimeTotals, error) {
	return s.totals, s.err
}

type publicStatsGiftValueStub struct {
	value float64
	err   error
}

func (s *publicStatsGiftValueStub) SumGiftedRedeemValue(_ context.Context) (float64, error) {
	return s.value, s.err
}

type publicStatsCompensationValueStub struct {
	value float64
	err   error
}

func (s *publicStatsCompensationValueStub) SumPublicCompensationCNY(_ context.Context) (float64, error) {
	return s.value, s.err
}

func TestPublicStatsHandlerReturnsEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewPublicStatsService(
		&publicStatsTotalsStub{totals: service.DashboardLifetimeTotals{TotalRequests: 123456, TotalTokens: 219638600}},
		&publicStatsGiftValueStub{value: 1557},
		&publicStatsCompensationValueStub{value: 830.8612566},
		nil,
	)
	h := NewPublicStatsHandler(svc)

	router := gin.New()
	router.GET("/api/v1/public/stats", h.GetPublicStats)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/stats", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TokensTotal     int64   `json:"tokens_total"`
			RequestsTotal   int64   `json:"requests_total"`
			CompensationCNY float64 `json:"compensation_cny"`
			UpdatedAt       string  `json:"updated_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, "success", body.Message)
	require.Equal(t, int64(2196386000), body.Data.TokensTotal)
	require.Equal(t, int64(1234560), body.Data.RequestsTotal)
	require.Equal(t, 23878.61, body.Data.CompensationCNY)
	require.NotEmpty(t, body.Data.UpdatedAt)
}

func TestPublicStatsHandlerUnavailableWhenRepoNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPublicStatsHandler(service.NewPublicStatsService(nil, nil, nil, nil))

	router := gin.New()
	router.GET("/api/v1/public/stats", h.GetPublicStats)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/public/stats", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}
