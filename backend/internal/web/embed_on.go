//go:build embed

package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

const (
	// NonceHTMLPlaceholder is the placeholder for nonce in HTML script tags
	NonceHTMLPlaceholder = "__CSP_NONCE_VALUE__"
)

//go:embed all:dist
var frontendFS embed.FS

// PublicSettingsProvider is an interface to fetch public settings
type PublicSettingsProvider interface {
	GetPublicSettingsForInjection(ctx context.Context) (any, error)
}

// FrontendServer serves the embedded frontend with settings injection
type FrontendServer struct {
	distFS     fs.FS
	fileServer http.Handler
	baseHTML   []byte
	cache      *HTMLCache
	settings   PublicSettingsProvider
	seo        *SEOManifest
	changelog  ChangelogPageResolver
}

// NewFrontendServer creates a new frontend server with settings injection
func NewFrontendServer(
	settingsProvider PublicSettingsProvider,
	changelogResolvers ...ChangelogPageResolver,
) (*FrontendServer, error) {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return nil, err
	}

	// Read base HTML once
	file, err := distFS.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	baseHTML, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	cache := NewHTMLCache()
	cache.SetBaseHTML(baseHTML)

	server := &FrontendServer{
		distFS:     distFS,
		fileServer: http.FileServer(http.FS(distFS)),
		baseHTML:   baseHTML,
		cache:      cache,
		settings:   settingsProvider,
		seo:        loadSEOManifest(distFS),
	}
	if len(changelogResolvers) > 0 {
		server.changelog = changelogResolvers[0]
	}
	return server, nil
}

// InvalidateCache invalidates the HTML cache (call when settings change)
func (s *FrontendServer) InvalidateCache() {
	if s != nil && s.cache != nil {
		s.cache.Invalidate()
	}
}

// Middleware returns the Gin middleware handler
func (s *FrontendServer) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes
		if shouldBypassEmbeddedFrontend(path) {
			c.Next()
			return
		}

		if target, ok := legacyFrontendRedirect(path); ok {
			c.Redirect(http.StatusMovedPermanently, target)
			c.Abort()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// For index.html or SPA routes, serve with injected settings
		if cleanPath == "index.html" || !s.fileExists(cleanPath) {
			if slug, ok := changelogSlugFromPath(path); ok && s.changelog != nil {
				ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				page, err := s.changelog.ResolvePublishedChangelogPage(ctx, slug)
				cancel()
				if err != nil {
					log.Printf("Warning: Failed to resolve changelog page %q: %v", slug, err)
					s.serveIndexHTMLWithStatus(c, http.StatusServiceUnavailable)
					return
				}
				if page == nil {
					s.serveIndexHTMLWithStatus(c, http.StatusNotFound)
					return
				}
				s.serveChangelogHTML(c, page)
				return
			}
			if s.seo != nil && s.seo.shouldServeNotFound(path) {
				s.serveIndexHTMLWithStatus(c, http.StatusNotFound)
				return
			}
			s.serveIndexHTML(c)
			return
		}

		setSearchCriticalAssetCacheHeaders(c, cleanPath)

		// Serve static files normally
		s.fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func (s *FrontendServer) fileExists(path string) bool {
	file, err := s.distFS.Open(path)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

func changelogSlugFromPath(path string) (string, bool) {
	normalized := strings.TrimSpace(strings.Split(strings.Split(path, "?")[0], "#")[0])
	if normalized == "" {
		return "", false
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	normalized = strings.TrimRight(normalized, "/")
	const prefix = "/changelog/"
	if !strings.HasPrefix(normalized, prefix) {
		return "", false
	}
	slug := strings.TrimSpace(strings.TrimPrefix(normalized, prefix))
	if slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return slug, true
}

func legacyFrontendRedirect(path string) (string, bool) {
	switch normalizeSEOPath(path) {
	case "/docs/backend/ai/claude-code":
		return "/docs/integration-claude-code", true
	case "/docs/claude-code-quickstart", "/docs/claude-code-troubleshooting", "/docs/claude-code-china-guide":
		return "/docs/integration-claude-code", true
	case "/docs/codex-quickstart", "/docs/codex-troubleshooting", "/docs/codex-china-guide", "/docs/codex-custom-api-guide":
		return "/docs/integration-codex", true
	case "/docs/base-url-guide", "/docs/common-api-errors":
		return "/docs/api-overview", true
	case "/docs/api-key-group-guide":
		return "/docs/models", true
	case "/docs/gpt-image-quickstart":
		return "/docs/api-images", true
	default:
		return "", false
	}
}

func (s *FrontendServer) serveIndexHTML(c *gin.Context) {
	s.serveIndexHTMLWithStatusAndChangelog(c, http.StatusOK, nil)
}

func (s *FrontendServer) serveIndexHTMLWithStatus(c *gin.Context, status int) {
	s.serveIndexHTMLWithStatusAndChangelog(c, status, nil)
}

func (s *FrontendServer) serveChangelogHTML(c *gin.Context, page *ChangelogPage) {
	s.serveIndexHTMLWithStatusAndChangelog(c, http.StatusOK, page)
}

func (s *FrontendServer) serveIndexHTMLWithStatusAndChangelog(
	c *gin.Context,
	status int,
	changelogPage *ChangelogPage,
) {
	// Get nonce from context (generated by SecurityHeaders middleware)
	nonce := middleware.GetNonceFromContext(c)

	requestPath := c.Request.URL.Path

	// Check cache first
	cached := s.cache.Get()
	if cached != nil {
		etag := routeAwareETag(cached.ETag, requestPath)
		rendered := s.seo.renderHTML(cached.Content, requestPath)
		if changelogPage != nil {
			rendered = s.seo.renderChangelogHTML(cached.Content, requestPath, changelogPage)
		} else if status == http.StatusNotFound {
			rendered = s.seo.renderNotFoundHTML(cached.Content, requestPath)
		} else if s.seo.shouldServeNoindex(requestPath) {
			rendered = s.seo.renderNoindexHTML(rendered, requestPath)
			c.Header("X-Robots-Tag", "noindex, nofollow")
		}
		// Replace nonce placeholder with actual nonce before serving
		content := replaceNoncePlaceholder(rendered, nonce)

		c.Header("ETag", etag)
		setNonceHTMLNoStore(c)
		c.Data(status, "text/html; charset=utf-8", content)
		c.Abort()
		return
	}

	// Cache miss - fetch settings and render
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err != nil {
		// Fallback: serve without injection
		rendered := s.seo.renderHTML(s.baseHTML, requestPath)
		if changelogPage != nil {
			rendered = s.seo.renderChangelogHTML(s.baseHTML, requestPath, changelogPage)
		} else if status == http.StatusNotFound {
			rendered = s.seo.renderNotFoundHTML(s.baseHTML, requestPath)
		} else if s.seo.shouldServeNoindex(requestPath) {
			rendered = s.seo.renderNoindexHTML(rendered, requestPath)
			c.Header("X-Robots-Tag", "noindex, nofollow")
		}
		content := replaceNoncePlaceholder(rendered, nonce)
		setNonceHTMLNoStore(c)
		c.Data(status, "text/html; charset=utf-8", content)
		c.Abort()
		return
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		// Fallback: serve without injection
		rendered := s.seo.renderHTML(s.baseHTML, requestPath)
		if changelogPage != nil {
			rendered = s.seo.renderChangelogHTML(s.baseHTML, requestPath, changelogPage)
		} else if status == http.StatusNotFound {
			rendered = s.seo.renderNotFoundHTML(s.baseHTML, requestPath)
		} else if s.seo.shouldServeNoindex(requestPath) {
			rendered = s.seo.renderNoindexHTML(rendered, requestPath)
			c.Header("X-Robots-Tag", "noindex, nofollow")
		}
		content := replaceNoncePlaceholder(rendered, nonce)
		setNonceHTMLNoStore(c)
		c.Data(status, "text/html; charset=utf-8", content)
		c.Abort()
		return
	}

	rendered := s.injectSettings(settingsJSON)
	s.cache.Set(rendered, settingsJSON)

	// Replace nonce placeholder with actual nonce before serving
	rendered = s.seo.renderHTML(rendered, requestPath)
	if changelogPage != nil {
		rendered = s.seo.renderChangelogHTML(rendered, requestPath, changelogPage)
	} else if status == http.StatusNotFound {
		rendered = s.seo.renderNotFoundHTML(rendered, requestPath)
	} else if s.seo.shouldServeNoindex(requestPath) {
		rendered = s.seo.renderNoindexHTML(rendered, requestPath)
		c.Header("X-Robots-Tag", "noindex, nofollow")
	}
	content := replaceNoncePlaceholder(rendered, nonce)

	cached = s.cache.Get()
	if cached != nil {
		c.Header("ETag", routeAwareETag(cached.ETag, requestPath))
	}
	setNonceHTMLNoStore(c)
	c.Data(status, "text/html; charset=utf-8", content)
	c.Abort()
}

// A CSP nonce is unique to each response. Returning 304 would make the browser
// reuse HTML containing the previous nonce while applying the new response's
// CSP header, so every inline bootstrap script would be rejected. Always send
// fresh nonce-bearing HTML and prevent browser storage of that document.
func setNonceHTMLNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

func (s *FrontendServer) injectSettings(settingsJSON []byte) []byte {
	// Create the script tag to inject with nonce placeholder
	// The placeholder will be replaced with actual nonce at request time
	script := []byte(`<script nonce="` + NonceHTMLPlaceholder + `">window.__APP_CONFIG__=` + string(settingsJSON) + `;</script>`)

	// Inject before </head>
	headClose := []byte("</head>")
	return bytes.Replace(s.baseHTML, headClose, append(script, headClose...), 1)
}

// replaceNoncePlaceholder replaces the nonce placeholder with actual nonce value
func replaceNoncePlaceholder(html []byte, nonce string) []byte {
	return bytes.ReplaceAll(html, []byte(NonceHTMLPlaceholder), []byte(nonce))
}

// ServeEmbeddedFrontend returns a middleware for serving embedded frontend
// This is the legacy function for backward compatibility when no settings provider is available
func ServeEmbeddedFrontend() gin.HandlerFunc {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("failed to get dist subdirectory: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(distFS))

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if shouldBypassEmbeddedFrontend(path) {
			c.Next()
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		if file, err := distFS.Open(cleanPath); err == nil {
			_ = file.Close()
			setSearchCriticalAssetCacheHeaders(c, cleanPath)
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		serveIndexHTML(c, distFS)
	}
}

func shouldBypassEmbeddedFrontend(path string) bool {
	trimmed := strings.TrimSpace(path)
	return strings.HasPrefix(trimmed, "/api/") ||
		strings.HasPrefix(trimmed, "/downloads/") ||
		strings.HasPrefix(trimmed, "/v1/") ||
		strings.HasPrefix(trimmed, "/v1beta/") ||
		strings.HasPrefix(trimmed, "/antigravity/") ||
		strings.HasPrefix(trimmed, "/gpt-image/") ||
		strings.HasPrefix(trimmed, "/setup/") ||
		trimmed == "/health" ||
		trimmed == "/responses" ||
		strings.HasPrefix(trimmed, "/responses/")
}

func setSearchCriticalAssetCacheHeaders(c *gin.Context, cleanPath string) {
	if !isSearchCriticalAsset(cleanPath) {
		return
	}

	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func isSearchCriticalAsset(cleanPath string) bool {
	switch strings.TrimPrefix(cleanPath, "/") {
	case "sitemap.xml", "robots.txt", "llms.txt", "seo-manifest.json":
		return true
	default:
		return false
	}
}

func serveIndexHTML(c *gin.Context, fsys fs.FS) {
	file, err := fsys.Open("index.html")
	if err != nil {
		c.String(http.StatusNotFound, "Frontend not found")
		c.Abort()
		return
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read index.html")
		c.Abort()
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	c.Abort()
}

func HasEmbeddedFrontend() bool {
	_, err := frontendFS.ReadFile("dist/index.html")
	return err == nil
}
