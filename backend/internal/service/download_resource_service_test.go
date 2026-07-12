package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type downloadResourceGitHubStub struct {
	release  *GitHubRelease
	releases map[string]*GitHubRelease
	files    map[string][]byte
}

func (s *downloadResourceGitHubStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	if release := s.releases[repo]; release != nil {
		return release, nil
	}
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

func TestDownloadResourceServiceDownloadTokenReturnsAsset(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName: "v3.16.0",
			Name:    "CC Switch v3.16.0",
			Assets: []GitHubAsset{
				{Name: "CC-Switch-v3.16.0-Windows.msi", BrowserDownloadURL: "https://example.test/windows", Size: int64(len("windows"))},
			},
		},
		files: map[string][]byte{
			"https://example.test/windows": []byte("windows"),
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
	require.NoError(t, svc.SyncCCSwitch(context.Background()))

	manifest, err := svc.ListCCSwitch(context.Background())
	require.NoError(t, err)

	token, expiresAt, err := svc.CreateToolAssetDownloadToken(context.Background(), ccSwitchToolID, manifest.Assets[0].ID, time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.True(t, expiresAt.After(time.Now()))

	file, err := svc.GetToolAssetByDownloadToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, manifest.Assets[0].ID, file.Asset.ID)
	require.FileExists(t, file.Path)
}

func TestDownloadResourceServiceDownloadTokenExpires(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName: "v3.16.0",
			Name:    "CC Switch v3.16.0",
			Assets: []GitHubAsset{
				{Name: "CC-Switch-v3.16.0-Windows.msi", BrowserDownloadURL: "https://example.test/windows", Size: int64(len("windows"))},
			},
		},
		files: map[string][]byte{
			"https://example.test/windows": []byte("windows"),
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
	require.NoError(t, svc.SyncCCSwitch(context.Background()))

	manifest, err := svc.ListCCSwitch(context.Background())
	require.NoError(t, err)
	token, _, err := svc.CreateToolAssetDownloadToken(context.Background(), ccSwitchToolID, manifest.Assets[0].ID, time.Nanosecond)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)
	_, err = svc.GetToolAssetByDownloadToken(context.Background(), token)
	require.ErrorIs(t, err, ErrDownloadTokenInvalid)
}

func TestDownloadResourceServiceSyncCodexCachesSelectedAssets(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		releases: map[string]*GitHubRelease{
			"openai/codex": {
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
			"Wangnov/codex-app-mirror": {
				TagName:     "codex-app-26.707.31428",
				Name:        "Codex App Mirror 26.707.31428",
				PublishedAt: "2026-07-10T03:42:05Z",
				Assets: []GitHubAsset{
					{Name: "OpenAI.Codex_26.707.3748.0_x64__2p2nqsd0c76g0.Msix", BrowserDownloadURL: "https://example.test/codex-msix", Size: int64(len("codex-msix"))},
					{Name: "OpenAI.Codex_26.707.3748.0_arm64__2p2nqsd0c76g0.Msix", BrowserDownloadURL: "https://example.test/skip-arm64", Size: int64(len("skip"))},
				},
			},
		},
		files: map[string][]byte{
			"https://example.test/codex-mac":  []byte("codex-mac"),
			"https://example.test/codex-app":  []byte("codex-app"),
			"https://example.test/codex-msix": []byte("codex-msix"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:                true,
			CacheDir:               dir,
			UpdateIntervalHours:    1,
			CodexRepo:              "openai/codex",
			CodexWindowsMirrorRepo: "Wangnov/codex-app-mirror",
			MaxAssetBytes:          1024,
		},
	}, stub)

	err := svc.SyncCodex(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListTool(context.Background(), codexToolID)
	require.NoError(t, err)
	require.Equal(t, "codex-app-26.707.31428", manifest.Version)
	require.Len(t, manifest.Assets, 2)
	require.Equal(t, "macos", manifest.Assets[0].Platform)
	require.Equal(t, "windows", manifest.Assets[1].Platform)
	require.Equal(t, "x64", manifest.Assets[1].Arch)
	require.NotEmpty(t, manifest.Assets[0].SHA256)
}

func TestDownloadResourceServiceSyncCodexPlusPlusCachesInstallAssets(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName:     "v1.2.4",
			Name:        "v1.2.4",
			PublishedAt: "2026-06-08T03:27:55Z",
			Assets: []GitHubAsset{
				{Name: "CodexPlusPlus-1.2.4-windows-x64-setup.exe", BrowserDownloadURL: "https://example.test/cpp-win", Size: int64(len("cpp-win"))},
				{Name: "CodexPlusPlus-1.2.4-macos-arm64.dmg", BrowserDownloadURL: "https://example.test/cpp-arm64", Size: int64(len("cpp-arm64"))},
				{Name: "CodexPlusPlus-1.2.4-macos-x64.dmg", BrowserDownloadURL: "https://example.test/cpp-x64", Size: int64(len("cpp-x64"))},
				{Name: "latest.json", BrowserDownloadURL: "https://example.test/latest", Size: int64(len("skip"))},
			},
		},
		files: map[string][]byte{
			"https://example.test/cpp-win":   []byte("cpp-win"),
			"https://example.test/cpp-arm64": []byte("cpp-arm64"),
			"https://example.test/cpp-x64":   []byte("cpp-x64"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:             true,
			CacheDir:            dir,
			UpdateIntervalHours: 1,
			CodexPlusPlusRepo:   "BigPizzaV3/CodexPlusPlus",
			MaxAssetBytes:       1024,
		},
	}, stub)

	err := svc.SyncCodexPlusPlus(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListTool(context.Background(), codexPlusPlusToolID)
	require.NoError(t, err)
	require.Equal(t, "v1.2.4", manifest.Version)
	require.Len(t, manifest.Assets, 3)
	require.Equal(t, "windows", manifest.Assets[0].Platform)
	require.Equal(t, "macos", manifest.Assets[1].Platform)
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
