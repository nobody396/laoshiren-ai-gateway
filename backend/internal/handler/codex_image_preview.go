package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CodexImagePreview serves a short-lived, unguessable generated-image URL.
// The token is the only locator; it contains no user, account or prompt data.
func (h *OpenAIGatewayHandler) CodexImagePreview(c *gin.Context) {
	body, contentType, size, remainingTTL, err := h.gatewayService.OpenCodexImagePreview(c.Param("token"))
	if err != nil {
		c.Header("Cache-Control", "no-store")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer func() { _ = body.Close() }()

	maxAge := int64(remainingTTL / time.Second)
	if maxAge > 3600 {
		maxAge = 3600
	}
	if maxAge < 0 {
		maxAge = 0
	}
	extension := codexImagePreviewExtension(contentType)
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="generated-image%s"`, extension))
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Status(http.StatusOK)
	if c.Request != nil && c.Request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(c.Writer, body)
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
