package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type downloadResourceGitHubStub struct {
	release   *GitHubRelease
	releases  map[string]*GitHubRelease
	files     map[string][]byte
	downloads []string
}

func (s *downloadResourceGitHubStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	if release := s.releases[repo]; release != nil {
		return release, nil
	}
	return s.release, nil
}

func (s *downloadResourceGitHubStub) DownloadFile(_ context.Context, url, dest string, _ int64) error {
	s.downloads = append(s.downloads, url)
	data, ok := s.files[url]
	if !ok {
		return fmt.Errorf("fixture download not found: %s", url)
	}
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

func TestDownloadResourceServiceSyncCCSwitchVerifiesOfficialDigest(t *testing.T) {
	dir := t.TempDir()
	content := []byte("verified-windows-installer")
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName: "v3.18.0",
			Name:    "CC Switch v3.18.0",
			Assets: []GitHubAsset{{
				Name:               "CC-Switch-v3.18.0-Windows.msi",
				BrowserDownloadURL: "https://example.test/windows",
				Size:               int64(len(content)),
				Digest:             digest,
			}},
		},
		files: map[string][]byte{"https://example.test/windows": content},
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
	require.Equal(t, digest[len("sha256:"):], manifest.Assets[0].SHA256)
}

func TestDownloadResourceServiceSyncCCSwitchRejectsDigestMismatch(t *testing.T) {
	dir := t.TempDir()
	content := []byte("tampered-windows-installer")
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{
			TagName: "v3.18.0",
			Name:    "CC Switch v3.18.0",
			Assets: []GitHubAsset{{
				Name:               "CC-Switch-v3.18.0-Windows.msi",
				BrowserDownloadURL: "https://example.test/windows",
				Size:               int64(len(content)),
				Digest:             "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			}},
		},
		files: map[string][]byte{"https://example.test/windows": content},
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
	require.ErrorContains(t, err, "checksum mismatch")
	require.NoFileExists(t, filepath.Join(dir, ccSwitchToolID, "v3.18.0", "CC-Switch-v3.18.0-Windows.msi"))
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
					{Name: "OpenAI.Codex_26.707.3748.0_arm64__2p2nqsd0c76g0.Msix", BrowserDownloadURL: "https://example.test/codex-msix-arm64", Size: int64(len("codex-msix-arm64"))},
					{Name: "Codex-mac-arm64.dmg", BrowserDownloadURL: "https://example.test/skip-mirror-mac", Size: int64(len("skip-mirror-mac"))},
				},
			},
		},
		files: map[string][]byte{
			"https://example.test/codex-mac":        []byte("codex-mac"),
			"https://example.test/codex-app":        []byte("codex-app"),
			"https://example.test/codex-msix":       []byte("codex-msix"),
			"https://example.test/codex-msix-arm64": []byte("codex-msix-arm64"),
			"https://example.test/chatgpt.dmg":      []byte("official-chatgpt-mac"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:                true,
			CacheDir:               dir,
			UpdateIntervalHours:    1,
			CodexRepo:              "openai/codex",
			CodexWindowsMirrorRepo: "Wangnov/codex-app-mirror",
			CodexMacOfficialURL:    "https://example.test/chatgpt.dmg",
			MaxAssetBytes:          1024,
		},
	}, stub)

	err := svc.SyncCodex(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListTool(context.Background(), codexToolID)
	require.NoError(t, err)
	require.Equal(t, "codex-app-26.707.31428", manifest.Version)
	require.Len(t, manifest.Assets, 4)
	require.Equal(t, "macos", manifest.Assets[0].Platform)
	require.Equal(t, "windows", manifest.Assets[1].Platform)
	require.Equal(t, "x64", manifest.Assets[1].Arch)
	require.Equal(t, "arm64", manifest.Assets[2].Arch)
	require.Equal(t, "macos", manifest.Assets[3].Platform)
	require.Equal(t, "universal", manifest.Assets[3].Arch)
	require.Equal(t, "ChatGPT.dmg", manifest.Assets[3].Name)
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
			"https://example.test/claude.dmg":                                 []byte("macos"),
			"https://downloads.claude.ai/releases/win32/x64/1.0.0/Claude.exe": []byte("windows-x64"),
			"https://example.test/win-arm64":                                  []byte("windows-arm64"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:                      true,
			CacheDir:                     dir,
			UpdateIntervalHours:          1,
			ClaudeDesktopMacURL:          "https://example.test/claude.dmg",
			ClaudeDesktopWindowsX64URL:   "https://downloads.claude.ai/releases/win32/x64/1.0.0/Claude.exe",
			ClaudeDesktopWindowsARM64URL: "https://example.test/win-arm64",
			MaxAssetBytes:                1024,
		},
	}, stub)

	err := svc.SyncClaudeDesktop(context.Background())
	require.NoError(t, err)
	require.NoError(t, svc.SyncClaudeDesktop(context.Background()))
	require.Len(t, stub.downloads, 3, "immutable Claude packages must not be downloaded again")

	manifest, err := svc.ListTool(context.Background(), claudeDesktopToolID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", manifest.Version)
	require.Len(t, manifest.Assets, 3)
	require.Equal(t, "macos", manifest.Assets[0].Platform)
	require.Equal(t, "windows", manifest.Assets[1].Platform)
	require.Equal(t, "x64", manifest.Assets[1].Arch)

	windowsAsset, err := svc.GetClaudeDesktopWindowsX64Asset(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Claude-Setup-x64.exe", windowsAsset.Asset.Name)
	require.Equal(t, "windows", windowsAsset.Asset.Platform)
	require.Equal(t, "x64", windowsAsset.Asset.Arch)
}

func TestDownloadResourceServiceImmutableAssetSurvivesCurrentManifestAdvance(t *testing.T) {
	dir := t.TempDir()
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir,
	}}, &downloadResourceGitHubStub{})

	writeVersion := func(version, name, content string) CachedDownloadAsset {
		versionDir := filepath.Join(dir, ccSwitchToolID, sanitizePathSegment(version))
		require.NoError(t, os.MkdirAll(versionDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(versionDir, name), []byte(content), 0644))
		asset := CachedDownloadAsset{
			ID: makeAssetID(name), Name: name, Size: int64(len(content)),
			SHA256:   fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
			Platform: "windows", Arch: "x64",
		}
		require.NoError(t, svc.writeManifest(CachedDownloadManifest{
			Tool: ccSwitchToolID, Version: version, Assets: []CachedDownloadAsset{asset},
		}))
		return asset
	}

	oldAsset := writeVersion("v3.18.0", "CC-Switch-v3.18.0-Windows.msi", "old-version")
	_ = writeVersion("v3.19.0", "CC-Switch-v3.19.0-Windows.msi", "new-version")

	current, err := svc.ListTool(context.Background(), ccSwitchToolID)
	require.NoError(t, err)
	require.Equal(t, "v3.19.0", current.Version)

	oldFile, err := svc.GetImmutableToolAsset(
		context.Background(), ccSwitchToolID, "v3.18.0", oldAsset.ID,
	)
	require.NoError(t, err)
	content, err := os.ReadFile(oldFile.Path)
	require.NoError(t, err)
	require.Equal(t, "old-version", string(content))
	require.FileExists(t, filepath.Join(dir, ccSwitchToolID, "v3.18.0", versionManifestName))
}

func TestDownloadResourceServiceImmutableAssetRejectsTraversalAndUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir,
	}}, &downloadResourceGitHubStub{})

	for _, tc := range []struct {
		version string
		asset   string
	}{
		{version: "../v1", asset: "installer.exe"},
		{version: "v1", asset: "../installer.exe"},
		{version: "missing", asset: "installer.exe"},
	} {
		_, err := svc.GetImmutableToolAsset(context.Background(), ccSwitchToolID, tc.version, tc.asset)
		require.Error(t, err)
	}
}

func TestDownloadResourceServiceSyncFailsClosedWithoutDownloadClient(t *testing.T) {
	dir := t.TempDir()
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir,
	}}, nil)

	for _, syncFn := range []func(context.Context) error{
		svc.SyncCCSwitch,
		svc.SyncCodex,
		svc.SyncCodexPlusPlus,
		svc.SyncClaudeDesktop,
		svc.SyncGitForWindows,
		svc.SyncGrokBuild,
	} {
		require.ErrorContains(t, syncFn(context.Background()), "download client is not configured")
	}
}

func TestClaudeDesktopVersionFromSourcesUsesConcreteWindowsVersion(t *testing.T) {
	require.Equal(t, "1.25927.0", claudeDesktopVersionFromSources([]staticDownloadSource{{
		Platform: "windows",
		URL:      "https://downloads.claude.ai/releases/win32/x64/1.25927.0/Claude.exe",
	}}))
}

func TestDownloadResourceServiceSyncGrokBuildUsesVerifiedOfficialVersion(t *testing.T) {
	dir := t.TempDir()
	primary := "https://primary.example.test/cli"
	fallback := "https://fallback.example.test/cli"
	version := "1.0.3"
	files := map[string][]byte{
		primary + "/stable":  []byte(version + "\n"),
		fallback + "/stable": []byte(version + "\n"),
	}
	for _, arch := range []string{"x86_64", "aarch64"} {
		urlPath := fmt.Sprintf("/grok-%s-windows-%s.exe", version, arch)
		payload := []byte("verified-" + arch)
		files[primary+urlPath] = payload
		files[fallback+urlPath] = payload
	}
	stub := &downloadResourceGitHubStub{files: files}
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir, MaxAssetBytes: 1024,
		GrokBuildPrimaryBaseURL: primary, GrokBuildFallbackBaseURL: fallback,
	}}, stub)

	require.NoError(t, svc.SyncGrokBuild(context.Background()))
	manifest, err := svc.ListTool(context.Background(), grokBuildToolID)
	require.NoError(t, err)
	require.Equal(t, version, manifest.Version)
	require.Len(t, manifest.Assets, 2)
	require.Equal(t, "x64", manifest.Assets[0].Arch)
	require.Equal(t, "arm64", manifest.Assets[1].Arch)
	for _, asset := range manifest.Assets {
		require.FileExists(t, asset.Path)
		require.NotEmpty(t, asset.SHA256)
	}
}

func TestDownloadResourceServiceSyncGrokBuildRejectsVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	primary := "https://primary.example.test/cli"
	fallback := "https://fallback.example.test/cli"
	version := "1.0.3"
	stub := &downloadResourceGitHubStub{files: map[string][]byte{
		primary + "/stable":  []byte(version),
		fallback + "/stable": []byte("1.0.4"),
	}}
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir, MaxAssetBytes: 1024,
		GrokBuildPrimaryBaseURL: primary, GrokBuildFallbackBaseURL: fallback,
	}}, stub)

	err := svc.SyncGrokBuild(context.Background())
	require.ErrorContains(t, err, "disagree on stable version")
	_, manifestErr := svc.ListTool(context.Background(), grokBuildToolID)
	require.ErrorIs(t, manifestErr, ErrDownloadManifestNotReady)
}

func TestDownloadResourceServiceSyncGitForWindowsSelectsInstallers(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{
		release: &GitHubRelease{TagName: "v2.51.0.windows.1", Assets: []GitHubAsset{
			{Name: "Git-2.51.0-64-bit.exe", BrowserDownloadURL: "https://example.test/x64", Size: 3},
			{Name: "Git-2.51.0-arm64.exe", BrowserDownloadURL: "https://example.test/arm64", Size: 4},
			{Name: "PortableGit-2.51.0-64-bit.7z.exe", BrowserDownloadURL: "https://example.test/portable", Size: 8},
		}},
		files: map[string][]byte{
			"https://example.test/x64":   []byte("x64"),
			"https://example.test/arm64": []byte("arm5"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir, GitForWindowsRepo: defaultGitForWindowsRepo, MaxAssetBytes: 1024,
	}}, stub)

	require.NoError(t, svc.SyncGitForWindows(context.Background()))
	manifest, err := svc.ListTool(context.Background(), gitForWindowsToolID)
	require.NoError(t, err)
	require.Len(t, manifest.Assets, 2)
	require.Equal(t, "x64", manifest.Assets[0].Arch)
	require.Equal(t, "arm64", manifest.Assets[1].Arch)
}

func TestDownloadResourceServiceListVersionStatusDistinguishesCacheModes(t *testing.T) {
	dir := t.TempDir()
	stub := &downloadResourceGitHubStub{releases: map[string]*GitHubRelease{
		defaultCodexRepo:         {TagName: "rust-v0.62.0", PublishedAt: "2026-08-01T00:00:00Z"},
		defaultCodexPPRepo:       {TagName: "v1.2.4", PublishedAt: "2026-08-01T00:00:00Z"},
		defaultClaudeCodeRepo:    {TagName: "v1.0.80", PublishedAt: "2026-08-01T00:00:00Z"},
		defaultGitForWindowsRepo: {TagName: "v2.53.0.windows.2", PublishedAt: "2026-08-01T00:00:00Z"},
		defaultCCSwitchRepo:      {TagName: "v3.18.0", PublishedAt: "2026-08-01T00:00:00Z"},
	}}
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{CacheDir: dir}}, stub)
	require.NoError(t, svc.writeManifest(CachedDownloadManifest{
		Tool: ccSwitchToolID, Version: "3.18.0", UpdatedAt: "2026-08-02T00:00:00Z",
	}))
	require.NoError(t, svc.writeManifest(CachedDownloadManifest{
		Tool: codexToolID, Version: "v0.12.0", UpdatedAt: "2026-08-02T00:00:00Z",
	}))
	require.NoError(t, svc.writeManifest(CachedDownloadManifest{
		Tool: codexPlusPlusToolID, Version: "v1.2.4", UpdatedAt: "2026-08-02T00:00:00Z",
	}))
	require.NoError(t, svc.writeManifest(CachedDownloadManifest{
		Tool: claudeDesktopToolID, Version: "1.25927.0", UpdatedAt: "2026-08-02T00:00:00Z",
	}))
	require.NoError(t, svc.writeManifest(CachedDownloadManifest{
		Tool: grokBuildToolID, Version: "1.0.3", UpdatedAt: "2026-08-02T00:00:00Z",
	}))
	require.NoError(t, svc.writeManifest(CachedDownloadManifest{
		Tool: gitForWindowsToolID, Version: "v2.53.0.windows.2", UpdatedAt: "2026-08-02T00:00:00Z",
	}))

	items := svc.ListVersionStatus(context.Background())
	require.Len(t, items, 7)
	require.Equal(t, "cached", items[0].State)
	require.Equal(t, "current", items[1].State)
	require.Equal(t, "cached", items[2].State)
	require.Equal(t, "npm-mirror", items[3].State)
	require.Empty(t, items[3].CachedVersion)
	require.Equal(t, "cached", items[4].State)
	require.Equal(t, "current", items[5].State)
	require.Equal(t, "current", items[6].State)
}
