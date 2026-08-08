package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountCredentialsRedactionRouter(adminSvc *stubAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts", handler.List)
	router.GET("/api/v1/admin/accounts/:id", handler.GetByID)
	return router
}

func TestAccountHandlerListAndDetailRedactCredentialSecrets(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.accounts = []service.Account{
		{
			ID:       77,
			Name:     "c2-redaction",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeAPIKey,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"api_key":                   "C2_HANDLER_API_KEY_SENTINEL",
				"authorization":             "Bearer C2_HANDLER_AUTHORIZATION_SENTINEL",
				"password":                  "C2_HANDLER_PASSWORD_SENTINEL",
				"secret":                    "C2_HANDLER_SECRET_SENTINEL",
				"base_url":                  "https://api.openai.example.com",
				"model_mapping":             map[string]any{"gpt-5": "gpt-5.1"},
				"intercept_warmup_requests": true,
			},
		},
	}
	router := setupAccountCredentialsRedactionRouter(adminSvc)

	for _, target := range []string{"/api/v1/admin/accounts", "/api/v1/admin/accounts/77"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		for _, sentinel := range []string{
			"C2_HANDLER_API_KEY_SENTINEL",
			"C2_HANDLER_AUTHORIZATION_SENTINEL",
			"C2_HANDLER_PASSWORD_SENTINEL",
			"C2_HANDLER_SECRET_SENTINEL",
		} {
			require.NotContains(t, body, sentinel, "target %s leaked %s", target, sentinel)
		}
		require.Contains(t, body, "https://api.openai.example.com")
		require.Contains(t, body, "intercept_warmup_requests")

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed))
		require.Equal(t, float64(0), parsed["code"])
	}
}
