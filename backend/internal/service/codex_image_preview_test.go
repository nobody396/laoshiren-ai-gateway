package service

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexImagePreviewStoreOpenAndExpire(t *testing.T) {
	dir := t.TempDir()
	svc := newCodexImagePreviewTestService(dir, 2, 1024*1024)

	preview, err := svc.StoreCodexImagePreviewBase64(codexBridgeTestPNG)
	require.NoError(t, err)
	require.Regexp(t, `^[a-f0-9]{64}$`, preview.Token)
	require.Equal(t, "image/png", preview.ContentType)
	require.NotContains(t, preview.Token, "user")

	body, contentType, size, remainingTTL, err := svc.OpenCodexImagePreview(preview.Token)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.NoError(t, body.Close())
	require.Equal(t, "image/png", contentType)
	require.Equal(t, int64(len(data)), size)
	require.Greater(t, remainingTTL, time.Duration(0))
	require.Equal(t, codexBridgeTestPNG, base64.StdEncoding.EncodeToString(data))

	expiredAt := time.Now().Add(-3 * time.Second)
	require.NoError(t, os.Chtimes(filepath.Join(dir, preview.Token), expiredAt, expiredAt))
	_, _, _, _, err = svc.OpenCodexImagePreview(preview.Token)
	require.ErrorIs(t, err, ErrCodexImagePreviewNotFound)
	require.NoFileExists(t, filepath.Join(dir, preview.Token))
}

func TestCodexImagePreviewRejectsTraversalWrongMIMEAndOversize(t *testing.T) {
	svc := newCodexImagePreviewTestService(t.TempDir(), 3600, 1024*1024)
	for _, token := range []string{"../secret", strings.Repeat("a", 63), strings.Repeat("g", 64), strings.Repeat("a", 65)} {
		_, _, _, _, err := svc.OpenCodexImagePreview(token)
		require.ErrorIs(t, err, ErrCodexImagePreviewNotFound)
	}

	_, err := svc.StoreCodexImagePreviewBase64(base64.StdEncoding.EncodeToString([]byte("not an image")))
	require.ErrorContains(t, err, "unsupported content type")

	tinyLimit := newCodexImagePreviewTestService(t.TempDir(), 3600, 16)
	_, err = tinyLimit.StoreCodexImagePreviewBase64(codexBridgeTestPNG)
	require.ErrorIs(t, err, ErrCodexImagePreviewTooLarge)
}

func TestCodexImagePreviewBatchDoesNotLeavePartialMultiImageResult(t *testing.T) {
	dir := t.TempDir()
	svc := newCodexImagePreviewTestService(dir, 3600, 1024*1024)
	invalid := base64.StdEncoding.EncodeToString([]byte("not an image"))

	previews, err := svc.StoreCodexImagePreviewsBase64([]string{codexBridgeTestPNG, invalid})
	require.Error(t, err)
	require.Nil(t, previews)
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	require.Empty(t, entries, "a failed multi-image set must remove earlier stored members")

	previews, err = svc.StoreCodexImagePreviewsBase64([]string{codexBridgeTestPNG, codexBridgeTestPNG})
	require.NoError(t, err)
	require.Len(t, previews, 2)
	require.NotEqual(t, previews[0].Token, previews[1].Token)
}

func TestCodexMarkdownImageTextSupportsMultipleImagesAndFallbackLinks(t *testing.T) {
	text := codexMarkdownImageText([]string{
		"https://api.example/v1/codex-image/previews/" + strings.Repeat("a", 64),
		"https://api.example/v1/codex-image/previews/" + strings.Repeat("b", 64),
	})
	require.Equal(t, 2, strings.Count(text, "![生成的图片"))
	require.Equal(t, 2, strings.Count(text, "点击这里打开原图"))
	require.Contains(t, text, "生成的图片 1")
	require.Contains(t, text, "生成的图片 2")
}

func TestCodexImagePreviewURLUsesSafeAbsoluteRequestOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://api.example/v1/responses", nil)
	c.Request.Host = "api.example"
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	token := strings.Repeat("a", 64)

	previewURL, err := codexImagePreviewURL(c, token)
	require.NoError(t, err)
	require.Equal(t, "https://api.example/v1/codex-image/previews/"+token, previewURL)

	c.Request.Host = "api.example@evil.example"
	_, err = codexImagePreviewURL(c, token)
	require.ErrorContains(t, err, "invalid Codex image preview host")
}

func TestCodexImagePreviewFileErrorDoesNotLeakCapabilityToken(t *testing.T) {
	token := strings.Repeat("a", 64)
	err := codexImagePreviewFileError(&os.PathError{
		Op:   "open",
		Path: filepath.Join("/data/codex-image-previews", token),
		Err:  os.ErrPermission,
	})
	require.ErrorIs(t, err, os.ErrPermission)
	require.NotContains(t, err.Error(), token)
}

func newCodexImagePreviewTestService(dir string, ttlSeconds int, maxBytes int64) *OpenAIGatewayService {
	return &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		CodexImagePreview: config.CodexImagePreviewConfig{
			Enabled: true, DataDir: dir, TTLSeconds: ttlSeconds, MaxImageBytes: maxBytes,
		},
	}}}
}
