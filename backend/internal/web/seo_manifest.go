//go:build embed

package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"io/fs"
	"regexp"
	"strings"
	"time"
)

type SEOManifest struct {
	SiteName     string     `json:"siteName"`
	SiteOrigin   string     `json:"siteOrigin"`
	OGImage      string     `json:"ogImage"`
	LastModified string     `json:"lastModified"`
	Routes       []SEORoute `json:"routes"`
}

type SEORoute struct {
	Path         string   `json:"path"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Priority     float64  `json:"priority"`
	Changefreq   string   `json:"changefreq"`
	OGType       string   `json:"ogType"`
	SchemaType   string   `json:"schemaType"`
	DateModified string   `json:"dateModified"`
	StaticHTML   string   `json:"staticHtml"`
	FAQ          []SEOFAQ `json:"faq"`
}

type SEOFAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

var (
	titleTagPattern        = regexp.MustCompile(`(?is)<title>.*?</title>`)
	descriptionMetaPattern = regexp.MustCompile(`(?is)<meta\s+name=["']description["'][^>]*>`)
	robotsMetaPattern      = regexp.MustCompile(`(?is)<meta\s+name=["']robots["'][^>]*>`)
	canonicalLinkPattern   = regexp.MustCompile(`(?is)<link\s+rel=["']canonical["'][^>]*>`)
	ogTypePattern          = regexp.MustCompile(`(?is)<meta\s+property=["']og:type["'][^>]*>`)
	ogTitlePattern         = regexp.MustCompile(`(?is)<meta\s+property=["']og:title["'][^>]*>`)
	ogDescriptionPattern   = regexp.MustCompile(`(?is)<meta\s+property=["']og:description["'][^>]*>`)
	ogURLPattern           = regexp.MustCompile(`(?is)<meta\s+property=["']og:url["'][^>]*>`)
	twitterTitlePattern    = regexp.MustCompile(`(?is)<meta\s+name=["']twitter:title["'][^>]*>`)
	twitterDescPattern     = regexp.MustCompile(`(?is)<meta\s+name=["']twitter:description["'][^>]*>`)
	serverSchemaPattern    = regexp.MustCompile(`(?is)<script[^>]*data-seo=["']server-structured-data["'][^>]*>.*?</script>`)
	appMountPattern        = regexp.MustCompile(`(?is)<div\s+id=["']app["']\s*></div>`)
)

func loadSEOManifest(distFS fs.FS) *SEOManifest {
	raw, err := fs.ReadFile(distFS, "seo-manifest.json")
	if err != nil {
		return nil
	}
	var manifest SEOManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil
	}
	return &manifest
}

func (m *SEOManifest) renderHTML(base []byte, requestPath string) []byte {
	if m == nil {
		return base
	}
	route := m.routeForPath(requestPath)
	if route == nil {
		return base
	}

	canonicalURL := strings.TrimRight(m.SiteOrigin, "/") + route.Path
	ogType := strings.TrimSpace(route.OGType)
	if ogType == "" {
		ogType = "website"
	}

	out := base
	out = replaceOrInsertHead(out, titleTagPattern, `<title>`+escapeText(route.Title)+`</title>`)
	out = replaceOrInsertHead(out, descriptionMetaPattern, `<meta name="description" content="`+escapeAttr(route.Description)+`" />`)
	out = replaceOrInsertHead(out, robotsMetaPattern, `<meta name="robots" content="index,follow" />`)
	out = replaceOrInsertHead(out, canonicalLinkPattern, `<link rel="canonical" href="`+escapeAttr(canonicalURL)+`" />`)
	out = replaceOrInsertHead(out, ogTypePattern, `<meta property="og:type" content="`+escapeAttr(ogType)+`" />`)
	out = replaceOrInsertHead(out, ogTitlePattern, `<meta property="og:title" content="`+escapeAttr(route.Title)+`" />`)
	out = replaceOrInsertHead(out, ogDescriptionPattern, `<meta property="og:description" content="`+escapeAttr(route.Description)+`" />`)
	out = replaceOrInsertHead(out, ogURLPattern, `<meta property="og:url" content="`+escapeAttr(canonicalURL)+`" />`)
	out = replaceOrInsertHead(out, twitterTitlePattern, `<meta name="twitter:title" content="`+escapeAttr(route.Title)+`" />`)
	out = replaceOrInsertHead(out, twitterDescPattern, `<meta name="twitter:description" content="`+escapeAttr(route.Description)+`" />`)

	if schema := m.structuredData(route, canonicalURL); len(schema) > 0 {
		if data, err := json.Marshal(schema); err == nil {
			script := `<script type="application/ld+json" data-seo="server-structured-data" data-canonical="` + escapeAttr(canonicalURL) + `" nonce="` + NonceHTMLPlaceholder + `">` + string(data) + `</script>`
			out = replaceOrInsertHead(out, serverSchemaPattern, script)
		}
	}

	if strings.TrimSpace(route.StaticHTML) != "" {
		out = injectStaticHTML(out, route.StaticHTML)
	}

	return out
}

func (m *SEOManifest) renderNotFoundHTML(base []byte, requestPath string) []byte {
	canonicalURL := strings.TrimRight(m.SiteOrigin, "/") + normalizeSEOPath(requestPath)
	out := base
	out = replaceOrInsertHead(out, titleTagPattern, `<title>页面未找到 - 老实人AI</title>`)
	out = replaceOrInsertHead(out, descriptionMetaPattern, `<meta name="description" content="这个页面不存在，请返回老实人AI文档中心或首页查找 Claude Code、Codex 和 AI API 网关相关指南。" />`)
	out = replaceOrInsertHead(out, robotsMetaPattern, `<meta name="robots" content="noindex,nofollow" />`)
	out = replaceOrInsertHead(out, canonicalLinkPattern, `<link rel="canonical" href="`+escapeAttr(canonicalURL)+`" />`)
	out = replaceOrInsertHead(out, ogTypePattern, `<meta property="og:type" content="website" />`)
	out = replaceOrInsertHead(out, ogTitlePattern, `<meta property="og:title" content="页面未找到 - 老实人AI" />`)
	out = replaceOrInsertHead(out, ogDescriptionPattern, `<meta property="og:description" content="这个页面不存在，请返回老实人AI文档中心或首页查找 Claude Code、Codex 和 AI API 网关相关指南。" />`)
	out = replaceOrInsertHead(out, ogURLPattern, `<meta property="og:url" content="`+escapeAttr(canonicalURL)+`" />`)
	out = replaceOrInsertHead(out, twitterTitlePattern, `<meta name="twitter:title" content="页面未找到 - 老实人AI" />`)
	out = replaceOrInsertHead(out, twitterDescPattern, `<meta name="twitter:description" content="这个页面不存在，请返回老实人AI文档中心或首页查找 Claude Code、Codex 和 AI API 网关相关指南。" />`)
	return injectStaticHTML(out, `<main class="seo-static-content"><h1>页面未找到</h1><p>这个页面不存在。你可以返回 <a href="/">老实人AI首页</a> 或 <a href="/docs">文档中心</a>，查看 Claude Code、Codex、API Key 和 Base URL 配置指南。</p></main>`)
}

func (m *SEOManifest) renderNoindexHTML(base []byte, requestPath string) []byte {
	if m == nil {
		return base
	}
	canonicalURL := strings.TrimRight(m.SiteOrigin, "/") + normalizeSEOPath(requestPath)
	out := replaceOrInsertHead(base, robotsMetaPattern, `<meta name="robots" content="noindex,nofollow" />`)
	out = replaceOrInsertHead(out, canonicalLinkPattern, `<link rel="canonical" href="`+escapeAttr(canonicalURL)+`" />`)
	return serverSchemaPattern.ReplaceAll(out, nil)
}

func (m *SEOManifest) renderChangelogHTML(
	base []byte,
	requestPath string,
	page *ChangelogPage,
) []byte {
	if page == nil {
		return base
	}
	siteName := "老实人AI"
	siteOrigin := "https://laoshirenai.com"
	if m != nil {
		if strings.TrimSpace(m.SiteName) != "" {
			siteName = strings.TrimSpace(m.SiteName)
		}
		if strings.TrimSpace(m.SiteOrigin) != "" {
			siteOrigin = strings.TrimRight(m.SiteOrigin, "/")
		}
	}

	title := strings.TrimSpace(page.Title) + " - 更新日志 - " + siteName
	description := strings.TrimSpace(page.Summary)
	canonicalURL := siteOrigin + normalizeSEOPath(requestPath)
	out := base
	out = replaceOrInsertHead(out, titleTagPattern, `<title>`+escapeText(title)+`</title>`)
	out = replaceOrInsertHead(out, descriptionMetaPattern, `<meta name="description" content="`+escapeAttr(description)+`" />`)
	out = replaceOrInsertHead(out, robotsMetaPattern, `<meta name="robots" content="index,follow" />`)
	out = replaceOrInsertHead(out, canonicalLinkPattern, `<link rel="canonical" href="`+escapeAttr(canonicalURL)+`" />`)
	out = replaceOrInsertHead(out, ogTypePattern, `<meta property="og:type" content="article" />`)
	out = replaceOrInsertHead(out, ogTitlePattern, `<meta property="og:title" content="`+escapeAttr(title)+`" />`)
	out = replaceOrInsertHead(out, ogDescriptionPattern, `<meta property="og:description" content="`+escapeAttr(description)+`" />`)
	out = replaceOrInsertHead(out, ogURLPattern, `<meta property="og:url" content="`+escapeAttr(canonicalURL)+`" />`)
	out = replaceOrInsertHead(out, twitterTitlePattern, `<meta name="twitter:title" content="`+escapeAttr(title)+`" />`)
	out = replaceOrInsertHead(out, twitterDescPattern, `<meta name="twitter:description" content="`+escapeAttr(description)+`" />`)

	article := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Article",
		"headline":    strings.TrimSpace(page.Title),
		"description": description,
		"url":         canonicalURL,
		"inLanguage":  "zh-CN",
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  siteName,
			"url":   siteOrigin,
		},
	}
	if page.PublishedAt != nil {
		article["datePublished"] = page.PublishedAt.UTC().Format(time.RFC3339)
	}
	if !page.UpdatedAt.IsZero() {
		article["dateModified"] = page.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if data, err := json.Marshal(article); err == nil {
		script := `<script type="application/ld+json" data-seo="server-structured-data" data-canonical="` +
			escapeAttr(canonicalURL) + `" nonce="` + NonceHTMLPlaceholder + `">` + string(data) + `</script>`
		out = replaceOrInsertHead(out, serverSchemaPattern, script)
	}
	return injectStaticHTML(
		out,
		`<main class="seo-static-content"><h1>`+escapeText(page.Title)+`</h1><p>`+
			escapeText(description)+`</p></main>`,
	)
}

func (m *SEOManifest) routeForPath(path string) *SEORoute {
	normalized := normalizeSEOPath(path)
	for i := range m.Routes {
		if normalizeSEOPath(m.Routes[i].Path) == normalized {
			return &m.Routes[i]
		}
	}
	return nil
}

func (m *SEOManifest) structuredData(route *SEORoute, canonicalURL string) map[string]any {
	if route == nil {
		return nil
	}

	siteName := strings.TrimSpace(m.SiteName)
	if siteName == "" {
		siteName = "老实人AI"
	}
	siteOrigin := strings.TrimRight(m.SiteOrigin, "/")
	orgID := siteOrigin + "/#organization"
	org := map[string]any{
		"@type":         "Organization",
		"@id":           orgID,
		"name":          siteName,
		"alternateName": []string{"老实人 AI", "老实人ai", "Laoshiren AI", "laoshirenai"},
		"url":           siteOrigin,
		"logo":          siteOrigin + m.OGImage,
	}

	schemaType := strings.TrimSpace(route.SchemaType)
	if schemaType == "" {
		schemaType = "WebPage"
	}

	page := map[string]any{
		"@type":       schemaType,
		"@id":         canonicalURL + "#webpage",
		"url":         canonicalURL,
		"name":        route.Title,
		"description": route.Description,
		"publisher":   map[string]any{"@id": orgID},
		"inLanguage":  "zh-CN",
	}
	if route.DateModified != "" && (schemaType == "TechArticle" || schemaType == "FAQPage") {
		page["dateModified"] = route.DateModified
		page["headline"] = route.Title
		page["mainEntityOfPage"] = canonicalURL
		page["author"] = map[string]any{"@id": orgID}
	}

	graph := []any{org, page}
	if len(route.FAQ) > 0 {
		entities := make([]any, 0, len(route.FAQ))
		for _, faq := range route.FAQ {
			question := strings.TrimSpace(faq.Question)
			answer := strings.TrimSpace(faq.Answer)
			if question == "" || answer == "" {
				continue
			}
			entities = append(entities, map[string]any{
				"@type": "Question",
				"name":  question,
				"acceptedAnswer": map[string]any{
					"@type": "Answer",
					"text":  answer,
				},
			})
		}
		if len(entities) > 0 {
			graph = append(graph, map[string]any{
				"@type":      "FAQPage",
				"@id":        canonicalURL + "#faq",
				"mainEntity": entities,
				"inLanguage": "zh-CN",
			})
		}
	}

	return map[string]any{
		"@context": "https://schema.org",
		"@graph":   graph,
	}
}

func (m *SEOManifest) shouldServeNotFound(path string) bool {
	normalized := normalizeSEOPath(path)
	if normalized == "/" || m.routeForPath(normalized) != nil {
		return false
	}
	if normalized == "/home" || normalized == "/legal" {
		return false
	}

	if strings.HasPrefix(normalized, "/docs/") || strings.HasPrefix(normalized, "/legal/") {
		return true
	}

	if isKnownSPARoute(normalized) {
		return false
	}

	return true
}

func (m *SEOManifest) shouldServeNoindex(path string) bool {
	return isKnownSPARoute(normalizeSEOPath(path))
}

func isKnownSPARoute(path string) bool {
	prefixes := []string{
		"/admin", "/agent", "/auth", "/custom", "/dashboard", "/email-verify",
		"/feedbacks", "/forgot-password", "/get-subscription", "/invoice", "/key-usage",
		"/keys", "/login", "/pricing", "/profile", "/purchase", "/redeem", "/register",
		"/resources", "/reset-password", "/settings", "/setup", "/subscriptions", "/topup", "/usage",
		"/usage-receipt",
		"/users",
	}
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func normalizeSEOPath(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" || clean == "/" {
		return "/"
	}
	clean = strings.Split(clean, "?")[0]
	clean = strings.Split(clean, "#")[0]
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if len(clean) > 1 {
		clean = strings.TrimRight(clean, "/")
	}
	return clean
}

func replaceOrInsertHead(content []byte, pattern *regexp.Regexp, replacement string) []byte {
	replacementBytes := []byte(replacement)
	if pattern.Match(content) {
		return pattern.ReplaceAll(content, replacementBytes)
	}
	headClose := []byte("</head>")
	return bytes.Replace(content, headClose, append(replacementBytes, headClose...), 1)
}

func injectStaticHTML(content []byte, html string) []byte {
	if strings.TrimSpace(html) == "" || !appMountPattern.Match(content) {
		return content
	}
	replacement := `<div id="app">` + html + `</div>`
	return appMountPattern.ReplaceAllLiteral(content, []byte(replacement))
}

func routeAwareETag(baseETag string, path string) string {
	cleanBase := strings.Trim(baseETag, `"`)
	routeHash := sha256.Sum256([]byte(normalizeSEOPath(path)))
	return `"` + cleanBase + "-" + hex.EncodeToString(routeHash[:4]) + `"`
}

func escapeText(value string) string {
	return html.EscapeString(value)
}

func escapeAttr(value string) string {
	return html.EscapeString(value)
}
