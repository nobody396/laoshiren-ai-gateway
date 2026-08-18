package handler

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var codexImagePreviewRawQueryPattern = regexp.MustCompile(`^token=([a-f0-9]{64})$`)

// CodexImagePreview serves a short-lived, unguessable generated-image URL.
// The token is the only locator; it contains no user, account or prompt data.
func (h *OpenAIGatewayHandler) CodexImagePreview(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	token, ok := codexImagePreviewToken(c)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	body, contentType, size, _, err := h.gatewayService.OpenCodexImagePreview(token)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer func() { _ = body.Close() }()

	extension := codexImagePreviewExtension(contentType)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="generated-image%s"`, extension))
	c.Status(http.StatusOK)
	if c.Request != nil && c.Request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(c.Writer, body)
}

func codexImagePreviewToken(c *gin.Context) (string, bool) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "", false
	}
	matches := codexImagePreviewRawQueryPattern.FindStringSubmatch(c.Request.URL.RawQuery)
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

func codexImagePreviewExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}
