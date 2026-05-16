package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const apiNoStoreCacheControl = "no-store, no-cache, must-revalidate, max-age=0, private"

// NoStoreAPIResponses prevents browser and edge caches from storing API responses.
func NoStoreAPIResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isNoStoreAPIPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		applyNoStoreAPIHeaders(c.Writer.Header())
		c.Next()
		applyNoStoreAPIHeaders(c.Writer.Header())
	}
}

func isNoStoreAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

func applyNoStoreAPIHeaders(header http.Header) {
	header.Set("Cache-Control", apiNoStoreCacheControl)
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
	appendVaryHeader(header, "Authorization", "Cookie", "Origin")
}

func appendVaryHeader(header http.Header, values ...string) {
	existing := map[string]struct{}{}
	for _, raw := range header.Values("Vary") {
		for _, part := range strings.Split(raw, ",") {
			key := strings.ToLower(strings.TrimSpace(part))
			if key != "" {
				existing[key] = struct{}{}
			}
		}
	}
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := existing[key]; ok {
			continue
		}
		header.Add("Vary", strings.TrimSpace(value))
		existing[key] = struct{}{}
	}
}
