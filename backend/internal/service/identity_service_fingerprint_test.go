package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fingerprintIdentityCacheStub struct {
	fingerprint *Fingerprint
	setCalls    int
	lastSet     *Fingerprint
}

func (s *fingerprintIdentityCacheStub) GetFingerprint(context.Context, int64) (*Fingerprint, error) {
	if s.fingerprint == nil {
		return nil, nil
	}
	clone := *s.fingerprint
	return &clone, nil
}

func (s *fingerprintIdentityCacheStub) SetFingerprint(_ context.Context, _ int64, fp *Fingerprint) error {
	s.setCalls++
	clone := *fp
	s.lastSet = &clone
	s.fingerprint = &clone
	return nil
}

func (s *fingerprintIdentityCacheStub) GetMaskedSessionID(context.Context, int64) (string, error) {
	return "", nil
}

func (s *fingerprintIdentityCacheStub) SetMaskedSessionID(context.Context, int64, string) error {
	return nil
}

func fingerprintHeaders(ua string) http.Header {
	header := http.Header{}
	if ua != "" {
		header.Set("User-Agent", ua)
	}
	return header
}

func TestIsAcceptableFingerprintUserAgent(t *testing.T) {
	for name, testCase := range map[string]struct {
		ua   string
		want bool
	}{
		"official":       {ua: "claude-cli/2.1.220 (external, cli)", want: true},
		"other product":  {ua: "some-sdk/1.2.3 (node)", want: true},
		"local suffix":   {ua: "claude-cli/999.0.0-local (undefined, cli)", want: false},
		"sentinel major": {ua: "claude-cli/999.0.0 (external, cli)", want: false},
		"browser":        {ua: "Mozilla/5.0 (Macintosh)", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, testCase.want, isAcceptableFingerprintUserAgent(testCase.ua))
		})
	}
	require.True(t, isAcceptableFingerprintUserAgent(defaultFingerprint.UserAgent))
}

func TestGetOrCreateFingerprintRejectsMalformedUserAgent(t *testing.T) {
	cache := &fingerprintIdentityCacheStub{}
	svc := NewIdentityService(cache, nil, nil, nil)

	fingerprint, err := svc.GetOrCreateFingerprint(context.Background(), 1, fingerprintHeaders("claude-cli/999.0.0-local (undefined, cli)"))

	require.NoError(t, err)
	require.Equal(t, defaultFingerprint.UserAgent, fingerprint.UserAgent)
	require.Equal(t, 1, cache.setCalls)
}

func TestGetOrCreateFingerprintDoesNotOverwriteValidCacheWithSentinel(t *testing.T) {
	cache := &fingerprintIdentityCacheStub{fingerprint: &Fingerprint{UserAgent: "claude-cli/2.1.90 (external, cli)", ClientID: "cid-1", UpdatedAt: time.Now().Unix()}}
	svc := NewIdentityService(cache, nil, nil, nil)

	fingerprint, err := svc.GetOrCreateFingerprint(context.Background(), 1, fingerprintHeaders("claude-cli/999.0.0-local (undefined, cli)"))

	require.NoError(t, err)
	require.Equal(t, "claude-cli/2.1.90 (external, cli)", fingerprint.UserAgent)
	require.Zero(t, cache.setCalls)
}

func TestGetOrCreateFingerprintHealsMalformedCache(t *testing.T) {
	cache := &fingerprintIdentityCacheStub{fingerprint: &Fingerprint{UserAgent: "claude-cli/999.0.0-local (undefined, cli)", ClientID: "cid-1", UpdatedAt: time.Now().Unix()}}
	svc := NewIdentityService(cache, nil, nil, nil)

	fingerprint, err := svc.GetOrCreateFingerprint(context.Background(), 1, fingerprintHeaders("claude-cli/2.1.91 (external, cli)"))

	require.NoError(t, err)
	require.Equal(t, "claude-cli/2.1.91 (external, cli)", fingerprint.UserAgent)
	require.Equal(t, "cid-1", fingerprint.ClientID)
	require.Equal(t, 1, cache.setCalls)
}
