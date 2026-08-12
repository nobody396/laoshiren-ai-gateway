//go:build embed

package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestReplaceNoncePlaceholder(t *testing.T) {
	t.Run("replaces_single_placeholder", func(t *testing.T) {
		html := []byte(`<script nonce="__CSP_NONCE_VALUE__">console.log('test');</script>`)
		nonce := "abc123xyz"

		result := replaceNoncePlaceholder(html, nonce)

		expected := `<script nonce="abc123xyz">console.log('test');</script>`
		assert.Equal(t, expected, string(result))
	})

	t.Run("replaces_multiple_placeholders", func(t *testing.T) {
		html := []byte(`<script nonce="__CSP_NONCE_VALUE__">a</script><script nonce="__CSP_NONCE_VALUE__">b</script>`)
		nonce := "nonce123"

		result := replaceNoncePlaceholder(html, nonce)

		assert.Equal(t, 2, strings.Count(string(result), `nonce="nonce123"`))
		assert.NotContains(t, string(result), NonceHTMLPlaceholder)
	})

	t.Run("handles_empty_nonce", func(t *testing.T) {
		html := []byte(`<script nonce="__CSP_NONCE_VALUE__">test</script>`)
		nonce := ""

		result := replaceNoncePlaceholder(html, nonce)

		assert.Equal(t, `<script nonce="">test</script>`, string(result))
	})

	t.Run("no_placeholder_returns_unchanged", func(t *testing.T) {
		html := []byte(`<script>console.log('test');</script>`)
		nonce := "abc123"

		result := replaceNoncePlaceholder(html, nonce)

		assert.Equal(t, string(html), string(result))
	})

	t.Run("handles_empty_html", func(t *testing.T) {
		html := []byte(``)
		nonce := "abc123"

		result := replaceNoncePlaceholder(html, nonce)

		assert.Empty(t, result)
	})
}

func TestNonceHTMLPlaceholder(t *testing.T) {
	t.Run("constant_value", func(t *testing.T) {
		assert.Equal(t, "__CSP_NONCE_VALUE__", NonceHTMLPlaceholder)
	})
}

func TestSEOManifest_RenderHTML(t *testing.T) {
	manifest := &SEOManifest{
		SiteName:   "老实人AI",
		SiteOrigin: "https://laoshirenai.com",
		OGImage:    "/og-image.png",
		Routes: []SEORoute{
			{
				Path:        "/docs/base-url-guide",
				Title:       "Base URL 填写总指南 - 文档 - 老实人AI",
				Description: "区分 Claude Code、Codex 和 OpenAI SDK 的 Base URL 填写方式。",
				OGType:      "article",
				SchemaType:  "TechArticle",
				StaticHTML:  `<main><h1>Base URL 填写总指南</h1><p>静态正文</p></main>`,
				FAQ: []SEOFAQ{
					{Question: "Base URL 要不要加 /v1？", Answer: "按客户端协议判断。"},
				},
			},
		},
	}
	base := []byte(`<!doctype html><html><head><title>老实人AI - AI 编码中转</title><meta name="description" content="home" /><meta name="robots" content="index,follow" /><link rel="canonical" href="https://laoshirenai.com/" /><meta property="og:type" content="website" /><meta property="og:title" content="home" /><meta property="og:description" content="home" /><meta property="og:url" content="https://laoshirenai.com/" /><meta name="twitter:title" content="home" /><meta name="twitter:description" content="home" /></head><body><div id="app"></div></body></html>`)

	rendered := manifest.renderHTML(base, "/docs/base-url-guide")
	body := string(rendered)

	assert.Contains(t, body, "<title>Base URL 填写总指南 - 文档 - 老实人AI</title>")
	assert.Contains(t, body, `content="区分 Claude Code、Codex 和 OpenAI SDK 的 Base URL 填写方式。"`)
	assert.Contains(t, body, `href="https://laoshirenai.com/docs/base-url-guide"`)
	assert.Contains(t, body, `property="og:type" content="article"`)
	assert.Contains(t, body, `data-seo="server-structured-data"`)
	assert.Contains(t, body, NonceHTMLPlaceholder)
	assert.Contains(t, body, `<div id="app"><main><h1>Base URL 填写总指南</h1><p>静态正文</p></main></div>`)
	assert.Contains(t, body, `"@type":"FAQPage"`)
	assert.Contains(t, body, `Base URL 要不要加 /v1？`)
}

func TestSEOManifest_NotFound(t *testing.T) {
	manifest := &SEOManifest{
		SiteOrigin: "https://laoshirenai.com",
		Routes: []SEORoute{
			{Path: "/docs/base-url-guide"},
			{Path: "/legal/terms"},
		},
	}

	assert.True(t, manifest.shouldServeNotFound("/docs/missing"))
	assert.True(t, manifest.shouldServeNotFound("/legal/missing"))
	assert.True(t, manifest.shouldServeNotFound("/missing"))
	assert.True(t, manifest.shouldServeNotFound("/missing/nested"))
	assert.False(t, manifest.shouldServeNotFound("/docs/base-url-guide"))
	assert.False(t, manifest.shouldServeNotFound("/dashboard"))
	assert.False(t, manifest.shouldServeNotFound("/legal"))

	base := []byte(`<!doctype html><html><head><title>home</title><meta name="description" content="home" /><meta name="robots" content="index,follow" /><link rel="canonical" href="https://laoshirenai.com/" /></head><body><div id="app"></div></body></html>`)
	body := string(manifest.renderNotFoundHTML(base, "/docs/missing"))

	assert.Contains(t, body, `<title>页面未找到 - 老实人AI</title>`)
	assert.Contains(t, body, `content="noindex,nofollow"`)
	assert.Contains(t, body, `<h1>页面未找到</h1>`)
}

func TestSEOManifest_NoindexSPA(t *testing.T) {
	manifest := &SEOManifest{SiteOrigin: "https://laoshirenai.com"}
	base := []byte(`<!doctype html><html><head><meta name="robots" content="index,follow" /><link rel="canonical" href="https://laoshirenai.com/" /><script type="application/ld+json" data-seo="server-structured-data">{}</script></head><body><div id="app"></div></body></html>`)

	body := string(manifest.renderNoindexHTML(base, "/dashboard"))

	assert.Contains(t, body, `<meta name="robots" content="noindex,nofollow" />`)
	assert.Contains(t, body, `<link rel="canonical" href="https://laoshirenai.com/dashboard" />`)
	assert.NotContains(t, body, `data-seo="server-structured-data"`)
	assert.True(t, manifest.shouldServeNoindex("/dashboard"))
	assert.False(t, manifest.shouldServeNoindex("/docs/claude-code-china-guide"))
}

func TestRouteAwareETag(t *testing.T) {
	assert.NotEqual(t, routeAwareETag(`"base"`, "/"), routeAwareETag(`"base"`, "/docs/base-url-guide"))
	assert.Equal(t, routeAwareETag(`"base"`, "/docs/base-url-guide"), routeAwareETag(`"base"`, "/docs/base-url-guide?utm=1"))
}

// mockSettingsProvider implements PublicSettingsProvider for testing
type mockSettingsProvider struct {
	settings any
	err      error
	called   int
}

type mockChangelogPageResolver struct {
	pages map[string]*ChangelogPage
	err   error
}

func (m *mockChangelogPageResolver) ResolvePublishedChangelogPage(
	_ context.Context,
	slug string,
) (*ChangelogPage, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.pages[slug], nil
}

func (m *mockSettingsProvider) GetPublicSettingsForInjection(ctx context.Context) (any, error) {
	m.called++
	return m.settings, m.err
}

func TestFrontendServer_InjectSettings(t *testing.T) {
	t.Run("injects_settings_with_nonce_placeholder", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"key": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		settingsJSON := []byte(`{"test":"data"}`)
		result := server.injectSettings(settingsJSON)

		// Should contain the script with nonce placeholder
		assert.Contains(t, string(result), `<script nonce="__CSP_NONCE_VALUE__">`)
		assert.Contains(t, string(result), `window.__APP_CONFIG__={"test":"data"};`)
		assert.Contains(t, string(result), `</script></head>`)
	})

	t.Run("injects_before_head_close", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"key": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		settingsJSON := []byte(`{}`)
		result := server.injectSettings(settingsJSON)

		// Script should be injected before </head>
		headCloseIndex := bytes.Index(result, []byte("</head>"))
		scriptIndex := bytes.Index(result, []byte(`<script nonce="`))

		assert.True(t, scriptIndex < headCloseIndex, "script should be before </head>")
	})

	t.Run("handles_complex_settings", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]any{
				"nested": map[string]any{
					"array": []int{1, 2, 3},
				},
			},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		settingsJSON := []byte(`{"nested":{"array":[1,2,3]},"special":"<>&"}`)
		result := server.injectSettings(settingsJSON)

		assert.Contains(t, string(result), `window.__APP_CONFIG__={"nested":{"array":[1,2,3]},"special":"<>&"};`)
	})
}

func TestFrontendServer_ServeIndexHTML(t *testing.T) {
	t.Run("serves_html_with_nonce", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		// Create a gin context with nonce
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		// Set nonce in context (simulating SecurityHeaders middleware)
		testNonce := "test-nonce-12345"
		c.Set(middleware.CSPNonceKey, testNonce)

		server.serveIndexHTML(c)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")

		body := w.Body.String()
		// Nonce placeholder should be replaced
		assert.NotContains(t, body, NonceHTMLPlaceholder)
		assert.Contains(t, body, `nonce="`+testNonce+`"`)
	})

	t.Run("caches_html_content", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		// First request
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c1.Set(middleware.CSPNonceKey, "nonce1")

		server.serveIndexHTML(c1)
		assert.Equal(t, 1, provider.called)

		// Second request - should use cache
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c2.Set(middleware.CSPNonceKey, "nonce2")

		server.serveIndexHTML(c2)
		// Settings provider should not be called again
		assert.Equal(t, 1, provider.called)

		// But nonce should be different
		assert.Contains(t, w2.Body.String(), `nonce="nonce2"`)
	})

	t.Run("sets_etag_header", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set(middleware.CSPNonceKey, "nonce123")

		server.serveIndexHTML(c)

		etag := w.Header().Get("ETag")
		assert.NotEmpty(t, etag)
		assert.True(t, strings.HasPrefix(etag, `"`))
		assert.True(t, strings.HasSuffix(etag, `"`))
	})

	t.Run("returns_fresh_html_for_matching_etag", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		// Use a real router for proper 304 handling
		router := gin.New()
		requestCount := 0
		router.Use(func(c *gin.Context) {
			requestCount++
			c.Set(middleware.CSPNonceKey, fmt.Sprintf("test-nonce-%d", requestCount))
			c.Next()
		})
		router.Use(server.Middleware())

		// First request to populate cache and get ETag
		w1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		router.ServeHTTP(w1, req1)
		etag := w1.Header().Get("ETag")
		require.NotEmpty(t, etag)

		// A matching ETag must not produce 304: the cached document contains the
		// previous response's CSP nonce and would be blocked by the new header.
		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.Header.Set("If-None-Match", etag)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Contains(t, w2.Body.String(), `nonce="test-nonce-2"`)
		assert.NotContains(t, w2.Body.String(), `nonce="test-nonce-1"`)
		assert.Equal(t, "no-store", w2.Header().Get("Cache-Control"))
	})

	t.Run("sets_cache_control_header", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set(middleware.CSPNonceKey, "nonce123")

		server.serveIndexHTML(c)

		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	})

	t.Run("fallback_on_settings_error", func(t *testing.T) {
		provider := &mockSettingsProvider{
			err: context.DeadlineExceeded,
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		// Invalidate cache to force settings fetch
		server.InvalidateCache()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set(middleware.CSPNonceKey, "nonce123")

		server.serveIndexHTML(c)

		// Should still return 200 with base HTML
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	})
}

func TestFrontendServer_InvalidateCache(t *testing.T) {
	t.Run("invalidates_cache", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		// First request to populate cache
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c1.Set(middleware.CSPNonceKey, "nonce1")

		server.serveIndexHTML(c1)
		assert.Equal(t, 1, provider.called)

		// Invalidate cache
		server.InvalidateCache()

		// Update settings
		provider.settings = map[string]string{"test": "new_value"}

		// Second request should fetch new settings
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c2.Set(middleware.CSPNonceKey, "nonce2")

		server.serveIndexHTML(c2)
		assert.Equal(t, 2, provider.called)
	})

	t.Run("handles_nil_server", func(t *testing.T) {
		var server *FrontendServer
		// Should not panic
		assert.NotPanics(t, func() {
			server.InvalidateCache()
		})
	})

	t.Run("handles_nil_cache", func(t *testing.T) {
		server := &FrontendServer{}
		// Should not panic
		assert.NotPanics(t, func() {
			server.InvalidateCache()
		})
	})
}

func TestFrontendServer_Middleware(t *testing.T) {
	t.Run("skips_api_routes", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		apiPaths := []string{
			"/api/v1/users",
			"/v1/models",
			"/v1beta/chat",
			"/antigravity/test",
			"/gpt-image/v1/models",
			"/gpt-image/media/task_123/0",
			"/setup/init",
			"/health",
			"/responses",
			"/responses/compact",
		}

		for _, path := range apiPaths {
			t.Run(path, func(t *testing.T) {
				router := gin.New()
				router.Use(server.Middleware())
				nextCalled := false
				router.GET(path, func(c *gin.Context) {
					nextCalled = true
					c.String(http.StatusOK, "ok")
				})

				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				router.ServeHTTP(w, req)

				assert.True(t, nextCalled, "next handler should be called for API route")
			})
		}
	})

	t.Run("skips_responses_compact_post_routes", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())
		nextCalled := false
		router.POST("/responses/compact", func(c *gin.Context) {
			nextCalled = true
			c.String(http.StatusOK, `{"ok":true}`)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/responses/compact", strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.True(t, nextCalled, "next handler should be called for compact API route")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"ok":true}`, w.Body.String())
	})

	t.Run("serves_index_for_spa_routes", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(middleware.CSPNonceKey, "test-nonce")
			c.Next()
		})
		router.Use(server.Middleware())

		spaPaths := []string{
			"/",
			"/dashboard",
			"/users/123",
			"/settings/profile",
		}

		for _, path := range spaPaths {
			t.Run(path, func(t *testing.T) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
			})
		}
	})

	t.Run("serves_private_spa_routes_with_noindex", func(t *testing.T) {
		provider := &mockSettingsProvider{settings: map[string]string{"test": "value"}}
		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "noindex, nofollow", w.Header().Get("X-Robots-Tag"))
		assert.Contains(t, w.Body.String(), `<meta name="robots" content="noindex,nofollow"`)
		assert.Contains(t, w.Body.String(), `href="https://laoshirenai.com/dashboard"`)
	})

	t.Run("redirects_legacy_claude_code_doc", func(t *testing.T) {
		provider := &mockSettingsProvider{settings: map[string]string{"test": "value"}}
		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/docs/backend/ai/claude-code", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "/docs/claude-code-china-guide", w.Header().Get("Location"))
	})

	t.Run("serves_static_seo_body_for_public_docs", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(middleware.CSPNonceKey, "test-nonce")
			c.Next()
		})
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/docs/claude-code-china-guide", nil)
		router.ServeHTTP(w, req)

		body := w.Body.String()
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, body, "Claude Code 国内使用完整指南")
		assert.Contains(t, body, `<main class="seo-static-content">`)
		assert.Contains(t, body, `data-seo="server-structured-data"`)
		assert.Contains(t, body, `"@type":"FAQPage"`)
	})

	t.Run("serves_noindex_404_for_missing_public_routes", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/docs/__missing__", nil)
		router.ServeHTTP(w, req)

		body := w.Body.String()
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, body, `<meta name="robots" content="noindex,nofollow"`)
		assert.Contains(t, body, "页面未找到")
	})

	t.Run("serves_published_changelog_detail_with_200_and_metadata", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}
		publishedAt := time.Date(2026, 7, 27, 2, 15, 17, 0, time.UTC)
		resolver := &mockChangelogPageResolver{pages: map[string]*ChangelogPage{
			"visual-refresh": {
				Slug:        "visual-refresh",
				Title:       "全站视觉体验焕新",
				Summary:     "首页与控制台使用一致的视觉语言。",
				PublishedAt: &publishedAt,
				UpdatedAt:   publishedAt,
			},
		}}

		server, err := NewFrontendServer(provider, resolver)
		require.NoError(t, err)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(middleware.CSPNonceKey, "test-nonce")
			c.Next()
		})
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/changelog/visual-refresh", nil)
		router.ServeHTTP(w, req)

		body := w.Body.String()
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, body, "<title>全站视觉体验焕新 - 更新日志 - 老实人AI</title>")
		assert.Contains(t, body, `href="https://laoshirenai.com/changelog/visual-refresh"`)
		assert.Contains(t, body, `property="og:type" content="article"`)
		assert.Contains(t, body, `"@type":"Article"`)
		assert.Contains(t, body, `<h1>全站视觉体验焕新</h1>`)
		assert.NotContains(t, body, "noindex,nofollow")
	})

	t.Run("serves_404_for_missing_changelog_detail", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}
		server, err := NewFrontendServer(provider, &mockChangelogPageResolver{
			pages: map[string]*ChangelogPage{},
		})
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/changelog/not-published", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), `<meta name="robots" content="noindex,nofollow"`)
	})

	t.Run("serves_503_when_changelog_resolution_fails", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}
		server, err := NewFrontendServer(provider, &mockChangelogPageResolver{
			err: errors.New("database unavailable"),
		})
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/changelog/visual-refresh", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("serves_static_files", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		// Request for existing static file
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "image/png")
	})

	t.Run("sets_no_store_headers_for_search_assets", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		router := gin.New()
		router.Use(server.Middleware())

		paths := []string{
			"/sitemap.xml",
			"/robots.txt",
			"/llms.txt",
			"/seo-manifest.json",
		}

		for _, path := range paths {
			t.Run(path, func(t *testing.T) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "no-store, no-cache, must-revalidate, max-age=0", w.Header().Get("Cache-Control"))
				assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
				assert.Equal(t, "0", w.Header().Get("Expires"))
			})
		}
	})
}

func TestChangelogSlugFromPath(t *testing.T) {
	tests := []struct {
		path string
		slug string
		ok   bool
	}{
		{path: "/changelog/visual-refresh", slug: "visual-refresh", ok: true},
		{path: "/changelog/visual-refresh/", slug: "visual-refresh", ok: true},
		{path: "/changelog", ok: false},
		{path: "/changelog/", ok: false},
		{path: "/changelog/nested/value", ok: false},
		{path: "/docs/visual-refresh", ok: false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			slug, ok := changelogSlugFromPath(test.path)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.slug, slug)
		})
	}
}

func TestNewFrontendServer(t *testing.T) {
	t.Run("creates_server_successfully", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)

		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.NotNil(t, server.distFS)
		assert.NotNil(t, server.fileServer)
		assert.NotNil(t, server.baseHTML)
		assert.NotNil(t, server.cache)
		assert.Equal(t, provider, server.settings)
	})

	t.Run("reads_base_html", func(t *testing.T) {
		provider := &mockSettingsProvider{
			settings: map[string]string{"test": "value"},
		}

		server, err := NewFrontendServer(provider)
		require.NoError(t, err)

		assert.NotEmpty(t, server.baseHTML)
		assert.Contains(t, string(server.baseHTML), "<!doctype html>")
	})
}

func TestHasEmbeddedFrontend(t *testing.T) {
	t.Run("returns_true_when_frontend_embedded", func(t *testing.T) {
		result := HasEmbeddedFrontend()
		assert.True(t, result)
	})
}

// Tests for legacy ServeEmbeddedFrontend function
func TestServeEmbeddedFrontend(t *testing.T) {
	t.Run("serves_static_files", func(t *testing.T) {
		middleware := ServeEmbeddedFrontend()

		router := gin.New()
		router.Use(middleware)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "image/png")
	})

	t.Run("serves_index_html_for_root", func(t *testing.T) {
		middleware := ServeEmbeddedFrontend()

		router := gin.New()
		router.Use(middleware)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, w.Body.String(), "<!doctype html>")
	})

	t.Run("serves_index_html_for_spa_routes", func(t *testing.T) {
		middleware := ServeEmbeddedFrontend()

		router := gin.New()
		router.Use(middleware)

		spaPaths := []string{"/dashboard", "/users/123", "/settings"}

		for _, path := range spaPaths {
			t.Run(path, func(t *testing.T) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
				assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
			})
		}
	})

	t.Run("skips_api_routes", func(t *testing.T) {
		middleware := ServeEmbeddedFrontend()

		apiPaths := []string{
			"/api/users",
			"/v1/models",
			"/v1beta/chat",
			"/antigravity/test",
			"/gpt-image/v1/models",
			"/gpt-image/media/task_123/0",
			"/setup/init",
			"/health",
			"/responses",
			"/responses/compact",
		}

		for _, path := range apiPaths {
			t.Run(path, func(t *testing.T) {
				nextCalled := false
				router := gin.New()
				router.Use(middleware)
				router.GET(path, func(c *gin.Context) {
					nextCalled = true
					c.String(http.StatusOK, "ok")
				})

				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, path, nil)
				router.ServeHTTP(w, req)

				assert.True(t, nextCalled, "next handler should be called for API route")
			})
		}
	})
}

func TestEmbeddedFrontendBypassesImmutableDownloadRoutes(t *testing.T) {
	require.True(t, shouldBypassEmbeddedFrontend("/downloads/claude-desktop/windows-x64/sha/Claude-Setup.exe"))
}

// Tests for HTMLCache
func TestHTMLCache(t *testing.T) {
	t.Run("new_cache_returns_nil", func(t *testing.T) {
		cache := NewHTMLCache()
		assert.Nil(t, cache.Get())
	})

	t.Run("set_and_get", func(t *testing.T) {
		cache := NewHTMLCache()
		cache.SetBaseHTML([]byte("<html></html>"))

		html := []byte("<html><body>test</body></html>")
		settings := []byte(`{"key":"value"}`)
		cache.Set(html, settings)

		result := cache.Get()
		require.NotNil(t, result)
		assert.Equal(t, html, result.Content)
		assert.NotEmpty(t, result.ETag)
	})

	t.Run("invalidate_clears_cache", func(t *testing.T) {
		cache := NewHTMLCache()
		cache.SetBaseHTML([]byte("<html></html>"))

		html := []byte("<html><body>test</body></html>")
		settings := []byte(`{"key":"value"}`)
		cache.Set(html, settings)

		require.NotNil(t, cache.Get())

		cache.Invalidate()

		assert.Nil(t, cache.Get())
	})

	t.Run("etag_changes_with_settings", func(t *testing.T) {
		cache := NewHTMLCache()
		cache.SetBaseHTML([]byte("<html></html>"))

		html := []byte("<html><body>test</body></html>")

		cache.Set(html, []byte(`{"v":1}`))
		etag1 := cache.Get().ETag

		cache.Invalidate()
		cache.Set(html, []byte(`{"v":2}`))
		etag2 := cache.Get().ETag

		assert.NotEqual(t, etag1, etag2)
	})

	t.Run("etag_format", func(t *testing.T) {
		cache := NewHTMLCache()
		cache.SetBaseHTML([]byte("<html></html>"))

		cache.Set([]byte("<html></html>"), []byte(`{}`))
		result := cache.Get()

		// ETag should be quoted
		assert.True(t, strings.HasPrefix(result.ETag, `"`))
		assert.True(t, strings.HasSuffix(result.ETag, `"`))
		// Should contain dash separator
		assert.Contains(t, result.ETag[1:len(result.ETag)-1], "-")
	})
}

// Benchmark tests
func BenchmarkReplaceNoncePlaceholder(b *testing.B) {
	html := []byte(`<!DOCTYPE html><html><head><script nonce="__CSP_NONCE_VALUE__">window.__APP_CONFIG__={"test":"data"};</script></head><body></body></html>`)
	nonce := "abcdefghijklmnop123456=="

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replaceNoncePlaceholder(html, nonce)
	}
}

func BenchmarkFrontendServerServeIndexHTML(b *testing.B) {
	provider := &mockSettingsProvider{
		settings: map[string]string{"test": "value"},
	}

	server, _ := NewFrontendServer(provider)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set(middleware.CSPNonceKey, "test-nonce")

		server.serveIndexHTML(c)
	}
}
