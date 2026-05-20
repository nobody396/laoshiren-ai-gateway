package service

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSMTPMessageAddsDeliverabilityHeaders(t *testing.T) {
	msg := string(buildSMTPMessage(
		"no-reply@laoshirenai.com",
		"老实人AI",
		"jacob_fu@163.com",
		"[老实人AI] Email Verification Code",
		"<p>hello</p>",
	))

	parts := strings.SplitN(msg, "\r\n\r\n", 2)
	require.Len(t, parts, 2)
	headers := parts[0]

	require.Contains(t, headers, "To: jacob_fu@163.com")
	require.Regexp(t, regexp.MustCompile(`(?m)^From: =\?.+\?= <no-reply@laoshirenai\.com>\r?$`), headers)
	require.Regexp(t, regexp.MustCompile(`(?m)^Subject: =\?.+\?=\r?$`), headers)
	require.Regexp(t, regexp.MustCompile(`(?m)^Date: .+ [+-][0-9]{4}\r?$`), headers)
	require.Regexp(t, regexp.MustCompile(`(?m)^Message-ID: <[0-9]+\.[0-9a-f]+@laoshirenai\.com>\r?$`), headers)
	require.Contains(t, headers, "MIME-Version: 1.0")
	require.Contains(t, headers, "Content-Type: text/html; charset=UTF-8")
	require.Contains(t, headers, "Content-Transfer-Encoding: 8bit")
	require.NotContains(t, headers, "老实人AI")
	require.Equal(t, "<p>hello</p>", parts[1])
}

func TestMessageIDDomainSanitizesFromEmailDomain(t *testing.T) {
	require.Equal(t, "laoshirenai.com", messageIDDomain("No-Reply@LAOSHIRENAI.COM"))
	require.Equal(t, "example.com", messageIDDomain("x@exa\r\nmple.com"))
	require.Equal(t, "localhost.localdomain", messageIDDomain("not-an-email"))
}
