package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ip"
	"github.com/gin-gonic/gin"
)

func TestConfigureClientIP(t *testing.T) {
	for _, tt := range []struct {
		name    string
		proxies []string
		headers []string
		peer    string
		eo      string
		xff     string
		want    string
	}{
		{name: "trusted EdgeOne header ignores forged XFF", proxies: []string{"10.0.1.4/32"}, headers: []string{"EO-Connecting-IP"}, peer: "10.0.1.4:1234", eo: "198.51.100.8", xff: "192.0.2.99", want: "198.51.100.8"},
		{name: "untrusted socket ignores all headers", proxies: []string{"10.0.1.4/32"}, headers: []string{"EO-Connecting-IP"}, peer: "203.0.113.7:1234", eo: "198.51.100.8", xff: "192.0.2.99", want: "203.0.113.7"},
		{name: "other Docker peer not trusted", proxies: []string{"10.0.1.4/32"}, headers: []string{"EO-Connecting-IP"}, peer: "10.0.1.5:1234", eo: "198.51.100.8", want: "10.0.1.5"},
		{name: "missing EO does not fall back to forged XFF", proxies: []string{"10.0.1.4/32"}, headers: []string{"EO-Connecting-IP"}, peer: "10.0.1.4:1234", xff: "192.0.2.99", want: "10.0.1.4"},
		{name: "invalid EO falls back to socket", proxies: []string{"10.0.1.4/32"}, headers: []string{"EO-Connecting-IP"}, peer: "10.0.1.4:1234", eo: "not-an-ip", want: "10.0.1.4"},
		{name: "default retains XFF", proxies: []string{"10.0.1.4/32"}, peer: "10.0.1.4:1234", xff: "198.51.100.8", want: "198.51.100.8"},
		{name: "no trusted proxies", headers: []string{"EO-Connecting-IP"}, peer: "203.0.113.7:1234", eo: "198.51.100.8", want: "203.0.113.7"},
		{name: "invalid trusted proxy fails closed", proxies: []string{"invalid"}, headers: []string{"EO-Connecting-IP"}, peer: "203.0.113.7:1234", eo: "198.51.100.8", want: "203.0.113.7"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			configureClientIP(r, config.ServerConfig{TrustedProxies: tt.proxies, RemoteIPHeaders: tt.headers})
			r.GET("/", func(c *gin.Context) {
				if ip.GetTrustedClientIP(c) != c.ClientIP() {
					t.Fatal("security IP helper differs from trusted engine identity")
				}
				c.String(http.StatusOK, c.ClientIP())
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.peer
			req.Header.Set("EO-Connecting-IP", tt.eo)
			req.Header.Set("X-Forwarded-For", tt.xff)
			req.Header.Set("X-Real-IP", "192.0.2.123")
			req.Header.Set("CF-Connecting-IP", "192.0.2.124")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Body.String() != tt.want {
				t.Fatalf("client IP = %q, want %q", w.Body.String(), tt.want)
			}
		})
	}
}
