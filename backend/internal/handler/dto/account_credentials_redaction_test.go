package dto

import (
	"encoding/json"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountFromServiceRedactsCredentialSecrets(t *testing.T) {
	account := &service.Account{
		ID:       42,
		Name:     "redaction-target",
		Platform: service.PlatformGemini,
		Type:     service.AccountTypeOAuth,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"access_token":              "C2_ACCESS_TOKEN_SENTINEL",
			"refresh_token":             "C2_REFRESH_TOKEN_SENTINEL",
			"api_key":                   "C2_API_KEY_SENTINEL",
			"authorization":             "Bearer C2_AUTHORIZATION_SENTINEL",
			"password":                  "C2_PASSWORD_SENTINEL",
			"client_secret":             "C2_CLIENT_SECRET_SENTINEL",
			"aws_secret_access_key":     "C2_AWS_SECRET_SENTINEL",
			"aws_session_token":         "C2_AWS_SESSION_SENTINEL",
			"base_url":                  "https://safe-upstream.example.com",
			"model_mapping":             map[string]any{"gemini-2.5-pro": "gemini-2.5-pro"},
			"tier_id":                   "google_ai_pro",
			"plan_type":                 "pro",
			"expires_at":                "1893456000",
			"subscription_expires_at":   "1893456000",
			"intercept_warmup_requests": true,
			"pool_mode":                 true,
			"pool_mode_retry_count":     float64(2),
		},
	}

	out := AccountFromService(account)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	body := string(raw)

	for _, sentinel := range []string{
		"C2_ACCESS_TOKEN_SENTINEL",
		"C2_REFRESH_TOKEN_SENTINEL",
		"C2_API_KEY_SENTINEL",
		"C2_AUTHORIZATION_SENTINEL",
		"C2_PASSWORD_SENTINEL",
		"C2_CLIENT_SECRET_SENTINEL",
		"C2_AWS_SECRET_SENTINEL",
		"C2_AWS_SESSION_SENTINEL",
	} {
		require.NotContains(t, body, sentinel)
	}

	require.Equal(t, "https://safe-upstream.example.com", out.Credentials["base_url"])
	require.Equal(t, "google_ai_pro", out.Credentials["tier_id"])
	require.Equal(t, "pro", out.Credentials["plan_type"])
	require.Equal(t, "1893456000", out.Credentials["expires_at"])
	require.Equal(t, "1893456000", out.Credentials["subscription_expires_at"])
	require.Equal(t, true, out.Credentials["intercept_warmup_requests"])
	require.Equal(t, true, out.Credentials["pool_mode"])
	require.Equal(t, float64(2), out.Credentials["pool_mode_retry_count"])
	require.Contains(t, out.Credentials, "model_mapping")

	require.Equal(t, "C2_ACCESS_TOKEN_SENTINEL", account.Credentials["access_token"])
	require.Equal(t, "C2_API_KEY_SENTINEL", account.Credentials["api_key"])
}
