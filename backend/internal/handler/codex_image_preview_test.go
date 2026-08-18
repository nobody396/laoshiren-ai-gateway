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
	router.GET("/v1/codex-image/previews/:token", handler.CodexImagePreview)
	router.HEAD("/v1/codex-image/previews/:token", handler.CodexImagePreview)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/codex-image/previews/"+preview.Token, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "inline")
	require.Contains(t, recorder.Header().Get("Cache-Control"), "private")
	require.Equal(t, wantBody, recorder.Body.Bytes())

	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/v1/codex-image/previews/"+preview.Token, nil))
	require.Equal(t, http.StatusOK, head.Code)
	require.Equal(t, "image/png", head.Header().Get("Content-Type"))
	require.Empty(t, head.Body.Bytes())

	bad := httptest.NewRecorder()
	router.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/v1/codex-image/previews/"+strings.Repeat("g", 64), nil))
	require.Equal(t, http.StatusNotFound, bad.Code)
	require.Equal(t, "no-store", bad.Header().Get("Cache-Control"))
}
