package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexImagePreviewServesInlineImageAndRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newCodexResponsesTestHandler(t, nil, nil)
	preview, err := handler.gatewayService.StoreCodexImagePreviewBase64(codexNativeImageBridgeTestPNG)
	require.NoError(t, err)
	wantBody, err := base64.StdEncoding.DecodeString(codexNativeImageBridgeTestPNG)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/v1/codex-image/preview", handler.CodexImagePreview)
	router.HEAD("/v1/codex-image/preview", handler.CodexImagePreview)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/codex-image/preview?token="+preview.Token, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "inline")
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Equal(t, wantBody, recorder.Body.Bytes())

	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/v1/codex-image/preview?token="+preview.Token, nil))
	require.Equal(t, http.StatusOK, head.Code)
	require.Equal(t, "image/png", head.Header().Get("Content-Type"))
	require.Empty(t, head.Body.Bytes())

	for _, rawQuery := range []string{
		"",
		"token=" + strings.Repeat("g", 64),
		"token=" + strings.Repeat("a", 65),
		"token=" + strings.Repeat("a", 64) + "&token=" + strings.Repeat("b", 64),
		"token=%ZZ",
		"token=" + strings.Repeat("a", 64) + "&extra=1",
	} {
		bad := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/codex-image/preview", nil)
		request.URL.RawQuery = rawQuery
		router.ServeHTTP(bad, request)
		require.Equal(t, http.StatusNotFound, bad.Code, "raw_query=%q", rawQuery)
		require.Equal(t, "private, no-store", bad.Header().Get("Cache-Control"))
		require.Empty(t, bad.Body.String())
	}
}

func TestCodexImagePreviewTokenQueryGateFailsClosed(t *testing.T) {
	valid := strings.Repeat("a", 64)
	tests := []struct {
		name     string
		rawQuery string
		wantOK   bool
	}{
		{name: "exact token", rawQuery: "token=" + valid, wantOK: true},
		{name: "missing", rawQuery: ""},
		{name: "empty", rawQuery: "token="},
		{name: "repeated", rawQuery: "token=" + valid + "&token=" + valid},
		{name: "overlong", rawQuery: "token=" + valid + "a"},
		{name: "wrong alphabet", rawQuery: "token=" + strings.Repeat("g", 64)},
		{name: "uppercase", rawQuery: "token=" + strings.Repeat("A", 64)},
		{name: "encoded", rawQuery: "token=%61" + strings.Repeat("a", 63)},
		{name: "invalid encoding", rawQuery: "token=%ZZ"},
		{name: "extra field", rawQuery: "token=" + valid + "&x=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/codex-image/preview", nil)
			c.Request.URL.RawQuery = tt.rawQuery

			got, ok := codexImagePreviewToken(c)

			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, valid, got)
			} else {
				require.Empty(t, got)
			}
		})
	}
}
