package responseheaders

import (
	"net/http"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
)

func TestFilterHeadersDisabledUsesDefaultAllowlist(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("Via", "1.1 Caddy")
	src.Add("Request-Id", "req-official")
	src.Add("X-Request-Id", "req-123")
	src.Add("Anthropic-Organization-Id", "org_123")
	src.Add("Anthropic-Ratelimit-Requests-Limit", "100")
	src.Add("X-Upstream", "api.anthropic.com")
	src.Add("X-Kna-Upstream", "api.anthropic.com")
	src.Add("X-Passthrough", "true")
	src.Add("Body-Sha256", "abc123")
	src.Add("X-Body-Sha256", "def456")
	src.Add("X-Test", "ok")
	src.Add("Connection", "keep-alive")
	src.Add("Content-Length", "123")
	src.Add("Set-Cookie", "secret=1")

	cfg := config.ResponseHeaderConfig{
		Enabled:     false,
		ForceRemove: []string{"x-request-id"},
	}

	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type passthrough, got %q", filtered.Get("Content-Type"))
	}
	if filtered.Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected X-Request-Id allowed, got %q", filtered.Get("X-Request-Id"))
	}
	if filtered.Get("Via") != "1.1 Caddy" {
		t.Fatalf("expected Via allowed, got %q", filtered.Get("Via"))
	}
	if filtered.Get("Request-Id") != "req-official" {
		t.Fatalf("expected Request-Id allowed, got %q", filtered.Get("Request-Id"))
	}
	if filtered.Get("Anthropic-Organization-Id") != "org_123" {
		t.Fatalf("expected Anthropic-Organization-Id allowed, got %q", filtered.Get("Anthropic-Organization-Id"))
	}
	if filtered.Get("Anthropic-Ratelimit-Requests-Limit") != "100" {
		t.Fatalf("expected Anthropic-Ratelimit-* allowed, got %q", filtered.Get("Anthropic-Ratelimit-Requests-Limit"))
	}
	if filtered.Get("X-Upstream") != "api.anthropic.com" {
		t.Fatalf("expected X-Upstream allowed, got %q", filtered.Get("X-Upstream"))
	}
	if filtered.Get("X-Kna-Upstream") != "api.anthropic.com" {
		t.Fatalf("expected X-Kna-Upstream allowed, got %q", filtered.Get("X-Kna-Upstream"))
	}
	if filtered.Get("X-Passthrough") != "true" {
		t.Fatalf("expected X-Passthrough allowed, got %q", filtered.Get("X-Passthrough"))
	}
	if filtered.Get("Body-Sha256") != "abc123" {
		t.Fatalf("expected Body-Sha256 allowed, got %q", filtered.Get("Body-Sha256"))
	}
	if filtered.Get("X-Body-Sha256") != "def456" {
		t.Fatalf("expected X-Body-Sha256 allowed, got %q", filtered.Get("X-Body-Sha256"))
	}
	if filtered.Get("X-Test") != "" {
		t.Fatalf("expected X-Test removed, got %q", filtered.Get("X-Test"))
	}
	if filtered.Get("Set-Cookie") != "" {
		t.Fatalf("expected Set-Cookie removed, got %q", filtered.Get("Set-Cookie"))
	}
	if filtered.Get("Connection") != "" {
		t.Fatalf("expected Connection to be removed, got %q", filtered.Get("Connection"))
	}
	if filtered.Get("Content-Length") != "" {
		t.Fatalf("expected Content-Length to be removed, got %q", filtered.Get("Content-Length"))
	}
}

func TestFilterHeadersEnabledUsesAllowlist(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("Anthropic-Organization-Id", "org_123")
	src.Add("X-Extra", "ok")
	src.Add("X-Remove", "nope")
	src.Add("X-Blocked", "nope")

	cfg := config.ResponseHeaderConfig{
		Enabled:           true,
		AdditionalAllowed: []string{"x-extra"},
		ForceRemove:       []string{"x-remove", "anthropic-organization-id"},
	}

	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type allowed, got %q", filtered.Get("Content-Type"))
	}
	if filtered.Get("X-Extra") != "ok" {
		t.Fatalf("expected X-Extra allowed, got %q", filtered.Get("X-Extra"))
	}
	if filtered.Get("X-Remove") != "" {
		t.Fatalf("expected X-Remove removed, got %q", filtered.Get("X-Remove"))
	}
	if filtered.Get("Anthropic-Organization-Id") != "" {
		t.Fatalf("expected Anthropic-Organization-Id force removed, got %q", filtered.Get("Anthropic-Organization-Id"))
	}
	if filtered.Get("X-Blocked") != "" {
		t.Fatalf("expected X-Blocked removed, got %q", filtered.Get("X-Blocked"))
	}
}
