package service

import (
	"context"
	"os"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type downloadResourceGitHubStub struct {
	release *GitHubRelease
	files   map[string][]byte
}

func (s *downloadResourceGitHubStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *downloadResourceGitHubStub) DownloadFile(_ context.Context, url, dest string, _ int64) error {
	data := s.files[url]
	return os.WriteFile(dest, data, 0644)
}

func (s *downloadResourceGitHubStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, nil
}

func TestDownloadResourceServiceSyncCCSwitchCachesInstallAssets(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName:     "v3.16.0",
			Name:        "CC Switch v3.16.0",
			PublishedAt: "2026-05-31T00:00:00Z",
			Assets: []GitHubAsset{
				{Name: "CC-Switch-v3.16.0-Windows.msi", BrowserDownloadURL: "https://example.test/windows", Size: int64(len("windows"))},
				{Name: "CC-Switch-v3.16.0-macOS.dmg", BrowserDownloadURL: "https://example.test/macos", Size: int64(len("macos"))},
				{Name: "CC-Switch-v3.16.0-Windows.msi.sig", BrowserDownloadURL: "https://example.test/sig", Size: 3},
				{Name: "latest.json", BrowserDownloadURL: "https://example.test/latest", Size: 2},
			},
		},
		files: map[string][]byte{
			"https://example.test/windows": []byte("windows"),
			"https://example.test/macos":   []byte("macos"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:             true,
			CacheDir:            dir,
			UpdateIntervalHours: 1,
			CCSwitchRepo:        "farion1231/cc-switch",
			MaxAssetBytes:       1024,
		},
	}, stub)

	err := svc.SyncCCSwitch(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListCCSwitch(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v3.16.0", manifest.Version)
	require.Len(t, manifest.Assets, 2)
	require.Equal(t, "windows", manifest.Assets[0].Platform)
	require.NotEmpty(t, manifest.Assets[0].SHA256)

	file, err := svc.GetCCSwitchAsset(context.Background(), manifest.Assets[0].ID)
	require.NoError(t, err)
	require.FileExists(t, file.Path)
	content, err := os.ReadFile(file.Path)
	require.NoError(t, err)
	require.Equal(t, "windows", string(content))
}

func TestDownloadResourceServiceGetCCSwitchAssetNotReady(t *testing.T) {
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:             true,
			CacheDir:            t.TempDir(),
			UpdateIntervalHours: 1,
			CCSwitchRepo:        "farion1231/cc-switch",
			MaxAssetBytes:       1024,
		},
	}, &downloadResourceGitHubStub{})

	_, err := svc.GetCCSwitchAsset(context.Background(), "missing")
	require.ErrorIs(t, err, ErrDownloadManifestNotReady)
}

func TestDownloadResourceServiceSyncCodexCachesSelectedAssets(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName:     "rust-v0.135.0",
			Name:        "0.135.0",
			PublishedAt: "2026-05-31T00:00:00Z",
			Assets: []GitHubAsset{
				{Name: "codex-aarch64-apple-darwin.tar.gz", BrowserDownloadURL: "https://example.test/codex-mac", Size: int64(len("codex-mac"))},
				{Name: "codex-app-server-package-x86_64-pc-windows-msvc.tar.gz", BrowserDownloadURL: "https://example.test/codex-app", Size: int64(len("codex-app"))},
				{Name: "codex-app-server-x86_64-pc-windows-msvc.exe.zip", BrowserDownloadURL: "https://example.test/skip-server", Size: int64(len("skip"))},
				{Name: "codex-npm-0.135.0.tgz", BrowserDownloadURL: "https://example.test/skip-npm", Size: int64(len("skip"))},
			},
		},
		files: map[string][]byte{
			"https://example.test/codex-mac": []byte("codex-mac"),
			"https://example.test/codex-app": []byte("codex-app"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:             true,
			CacheDir:            dir,
			UpdateIntervalHours: 1,
			CodexRepo:           "openai/codex",
			MaxAssetBytes:       1024,
		},
	}, stub)

	err := svc.SyncCodex(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListTool(context.Background(), codexToolID)
	require.NoError(t, err)
	require.Equal(t, "rust-v0.135.0", manifest.Version)
	require.Len(t, manifest.Assets, 2)
	require.Equal(t, "macos", manifest.Assets[0].Platform)
	require.NotEmpty(t, manifest.Assets[0].SHA256)
}

func TestDownloadResourceServiceSyncClaudeDesktopCachesStaticAssets(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		files: map[string][]byte{
			"https://example.test/claude.dmg": []byte("macos"),
			"https://example.test/win-x64":    []byte("windows-x64"),
			"https://example.test/win-arm64":  []byte("windows-arm64"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:                      true,
			CacheDir:                     dir,
			UpdateIntervalHours:          1,
			ClaudeDesktopMacURL:          "https://example.test/claude.dmg",
			ClaudeDesktopWindowsX64URL:   "https://example.test/win-x64",
			ClaudeDesktopWindowsARM64URL: "https://example.test/win-arm64",
			MaxAssetBytes:                1024,
		},
	}, stub)

	err := svc.SyncClaudeDesktop(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListTool(context.Background(), claudeDesktopToolID)
	require.NoError(t, err)
	require.Equal(t, "latest", manifest.Version)
	require.Len(t, manifest.Assets, 3)
	require.Equal(t, "macos", manifest.Assets[0].Platform)
	require.Equal(t, "windows", manifest.Assets[1].Platform)
	require.Equal(t, "x64", manifest.Assets[1].Arch)
}
