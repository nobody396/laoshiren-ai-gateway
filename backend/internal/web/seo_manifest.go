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
)

type SEOManifest struct {
	SiteName     string     `json:"siteName"`
	SiteOrigin   string     `json:"siteOrigin"`
	OGImage      string     `json:"ogImage"`
	LastModified string     `json:"lastModified"`
	Routes       []SEORoute `json:"routes"`
}

type SEORoute struct {
	Path         string  `json:"path"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Priority     float64 `json:"priority"`
	Changefreq   string  `json:"changefreq"`
	OGType       string  `json:"ogType"`
	SchemaType   string  `json:"schemaType"`
	DateModified string  `json:"dateModified"`
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
			script := `<script type="application/ld+json" data-seo="server-structured-data" nonce="` + NonceHTMLPlaceholder + `">` + string(data) + `</script>`
			out = replaceOrInsertHead(out, serverSchemaPattern, script)
		}
	}

	return out
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

	return map[string]any{
		"@context": "https://schema.org",
		"@graph":   []any{org, page},
	}
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
