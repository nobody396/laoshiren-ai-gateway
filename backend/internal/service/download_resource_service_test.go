package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/klauspost/compress/zstd"
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

func claudeDesktopTestFixture(
	t *testing.T,
	version, commit, codeVersion string,
	x64Compressed, arm64Compressed []byte,
) []byte {
	t.Helper()
	checksum := func(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
	pin := claudeCodePin{
		Version: codeVersion,
		Manifest: claudeCodeManifest{
			Version: codeVersion,
			Platforms: map[string]claudeCodePlatform{
				"win32-x64": {
					Binary: "claude.exe.zst", Checksum: checksum(x64Compressed), Size: int64(len(x64Compressed)),
				},
				"win32-arm64": {
					Binary: "claude.exe.zst", Checksum: checksum(arm64Compressed), Size: int64(len(arm64Compressed)),
				},
			},
		},
		BaseURL: claudeCodeOfficialBase,
	}
	buildRaw, err := json.Marshal(claudeDesktopBuildInfo{CommitHash: commit, AppVersion: version})
	require.NoError(t, err)
	pinRaw, err := json.Marshal(pin)
	require.NoError(t, err)
	// Production app.asar embeds raw JSON inside a JavaScript template literal;
	// its JSON quotes are not backslash-escaped.
	asar := []byte("function build(){return JSON.parse(`" + string(buildRaw) + "`)};" +
		"function pin(){return JSON.parse(`" + string(pinRaw) + "`)}")

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(claudeDesktopASARPath)
	require.NoError(t, err)
	_, err = entry.Write(asar)
	require.NoError(t, err)
	signature, err := writer.Create("AppxSignature.p7x")
	require.NoError(t, err)
	_, err = signature.Write([]byte("signed-fixture"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func zstdTestPayload(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer, err := zstd.NewWriter(&buffer)
	require.NoError(t, err)
	_, err = writer.Write(raw)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
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
			Enabled:       true,
			CacheDir:      dir,
			CCSwitchRepo:  "farion1231/cc-switch",
			MaxAssetBytes: 1024,
		},
	}, stub)
	staleDir := filepath.Join(dir, ccSwitchToolID, "v3.15.0")
	require.NoError(t, os.MkdirAll(staleDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(staleDir, "old.msi"), []byte("old"), 0644))

	err := svc.SyncCCSwitch(context.Background())
	require.NoError(t, err)
	require.NoDirExists(t, staleDir)

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
			Enabled:       true,
			CacheDir:      dir,
			CCSwitchRepo:  "farion1231/cc-switch",
			MaxAssetBytes: 1024,
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
			Enabled:       true,
			CacheDir:      dir,
			CCSwitchRepo:  "farion1231/cc-switch",
			MaxAssetBytes: 1024,
		},
	}, stub)

	err := svc.SyncCCSwitch(context.Background())
	require.ErrorContains(t, err, "checksum mismatch")
	require.NoFileExists(t, filepath.Join(dir, ccSwitchToolID, "v3.18.0", "CC-Switch-v3.18.0-Windows.msi"))
}

func TestDownloadResourceServiceGetCCSwitchAssetNotReady(t *testing.T) {
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:       true,
			CacheDir:      t.TempDir(),
			CCSwitchRepo:  "farion1231/cc-switch",
			MaxAssetBytes: 1024,
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
			Enabled:       true,
			CacheDir:      dir,
			CCSwitchRepo:  "farion1231/cc-switch",
			MaxAssetBytes: 1024,
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
			Enabled:       true,
			CacheDir:      dir,
			CCSwitchRepo:  "farion1231/cc-switch",
			MaxAssetBytes: 1024,
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
					{Name: "codex-aarch64-apple-darwin.dmg", BrowserDownloadURL: "https://example.test/codex-dmg-arm64", Size: int64(len("codex-dmg-arm64"))},
					{Name: "codex-x86_64-apple-darwin.dmg", BrowserDownloadURL: "https://example.test/codex-dmg-x64", Size: int64(len("codex-dmg-x64"))},
					{Name: "codex-aarch64-pc-windows-msvc.exe.zip", BrowserDownloadURL: "https://example.test/codex-cli-arm64", Size: int64(len("codex-cli-arm64"))},
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
			"https://example.test/codex-dmg-arm64":  []byte("codex-dmg-arm64"),
			"https://example.test/codex-dmg-x64":    []byte("codex-dmg-x64"),
			"https://example.test/codex-cli-arm64":  []byte("codex-cli-arm64"),
			"https://example.test/codex-app":        []byte("codex-app"),
			"https://example.test/codex-msix":       []byte("codex-msix"),
			"https://example.test/codex-msix-arm64": []byte("codex-msix-arm64"),
		},
	}
	svc := NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:                true,
			CacheDir:               dir,
			CodexRepo:              "openai/codex",
			CodexWindowsMirrorRepo: "Wangnov/codex-app-mirror",
			MaxAssetBytes:          1024,
		},
	}, stub)

	err := svc.SyncCodex(context.Background())
	require.NoError(t, err)

	manifest, err := svc.ListTool(context.Background(), codexToolID)
	require.NoError(t, err)
	require.Equal(t, "codex-app-26.707.31428__rust-v0.135.0", manifest.Version)
	require.Equal(t, "openai/codex, Wangnov/codex-app-mirror", manifest.Repo)
	require.Len(t, manifest.Assets, 6)
	require.Equal(t, "macos", manifest.Assets[0].Platform)
	require.Equal(t, "arm64", manifest.Assets[1].Arch)
	require.Equal(t, "x64", manifest.Assets[2].Arch)
	require.Equal(t, "windows", manifest.Assets[3].Platform)
	require.Equal(t, "arm64", manifest.Assets[3].Arch)
	require.Equal(t, "x64", manifest.Assets[4].Arch)
	require.Equal(t, "arm64", manifest.Assets[5].Arch)
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
			Enabled:           true,
			CacheDir:          dir,
			CodexPlusPlusRepo: "BigPizzaV3/CodexPlusPlus",
			MaxAssetBytes:     1024,
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

func TestDownloadResourceServiceSyncClaudeDesktopPublishesVerifiedPairs(t *testing.T) {
	dir := t.TempDir()
	base := "https://downloads.example.test/releases/win32"
	version := "1.30096.1"
	commit := strings.Repeat("1", 40)
	codeVersion := "2.1.229"
	x64Compressed := zstdTestPayload(t, []byte("MZ-x64-code-component"))
	arm64Compressed := zstdTestPayload(t, []byte("MZ-arm64-code-component"))
	msix := claudeDesktopTestFixture(t, version, commit, codeVersion, x64Compressed, arm64Compressed)
	latest, err := json.Marshal(claudeDesktopLatest{Version: version, Hash: commit})
	require.NoError(t, err)
	stub := &downloadResourceGitHubStub{files: map[string][]byte{
		base + "/x64/.latest":   latest,
		base + "/arm64/.latest": latest,
		base + "/x64/" + version + "/Claude-" + commit + ".msix":                   msix,
		base + "/arm64/" + version + "/Claude-" + commit + ".msix":                 msix,
		claudeCodeOfficialBase + "/" + codeVersion + "/win32-x64/claude.exe.zst":   x64Compressed,
		claudeCodeOfficialBase + "/" + codeVersion + "/win32-arm64/claude.exe.zst": arm64Compressed,
		"https://example.test/claude.dmg":                                          []byte("macos"),
	}}
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir, MaxAssetBytes: 1024 * 1024,
		ClaudeDesktopLatestBaseURL: base,
		ClaudeDesktopMacURL:        "https://example.test/claude.dmg",
	}}, stub)

	require.NoError(t, svc.SyncClaudeDesktop(context.Background()))
	largeDownloadCount := len(stub.downloads)
	staleDir := filepath.Join(dir, claudeDesktopToolID, "1.30095.0-aaaaaaaaaaaa")
	require.NoError(t, os.MkdirAll(staleDir, 0755))
	require.NoError(t, svc.SyncClaudeDesktop(context.Background()))
	require.Equal(t, largeDownloadCount+2, len(stub.downloads), "current release should only re-read two tiny .latest files")
	require.NoDirExists(t, staleDir, "no-op sync should still enforce latest-only retention")

	manifest, err := svc.ListTool(context.Background(), claudeDesktopToolID)
	require.NoError(t, err)
	require.Equal(t, version+"-"+commit[:12], manifest.Version)
	require.Len(t, manifest.Assets, 5)

	roles := map[string]CachedDownloadAsset{}
	for _, asset := range manifest.Assets {
		roles[asset.Arch+":"+asset.Role] = asset
	}
	require.Equal(t, codeVersion, roles["x64:"+claudeCodeAssetRole].ComponentVersion)
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(x64Compressed)), roles["x64:"+claudeCodeAssetRole].UpstreamSHA256)
	require.Equal(t, int64(len(x64Compressed)), roles["x64:"+claudeCodeAssetRole].UpstreamCompressed)
	require.Equal(t, claudeInstallerRole, roles["x64:"+claudeInstallerRole].Role)
	require.FileExists(t, filepath.Join(dir, claudeDesktopToolID, manifest.Version, roles["x64:"+claudeCodeAssetRole].Name))

	windowsAsset, err := svc.GetClaudeDesktopWindowsX64Asset(context.Background())
	require.NoError(t, err)
	require.Equal(t, claudeInstallerRole, windowsAsset.Asset.Role)
	require.Equal(t, ".msix", strings.ToLower(filepath.Ext(windowsAsset.Asset.Name)))
}

func TestDownloadResourceServiceSyncClaudeDesktopKeepsPreviousManifestOnPairFailure(t *testing.T) {
	dir := t.TempDir()
	base := "https://downloads.example.test/releases/win32"
	codePayload := zstdTestPayload(t, []byte("MZ-component"))
	files := map[string][]byte{"https://example.test/claude.dmg": []byte("macos")}
	addRelease := func(version string, commitByte byte, codeVersion string, includeComponents bool) claudeDesktopLatest {
		commit := strings.Repeat(string(commitByte), 40)
		latest := claudeDesktopLatest{Version: version, Hash: commit}
		raw, marshalErr := json.Marshal(latest)
		require.NoError(t, marshalErr)
		files[base+"/x64/.latest"] = raw
		files[base+"/arm64/.latest"] = raw
		msix := claudeDesktopTestFixture(t, version, commit, codeVersion, codePayload, codePayload)
		files[base+"/x64/"+version+"/Claude-"+commit+".msix"] = msix
		files[base+"/arm64/"+version+"/Claude-"+commit+".msix"] = msix
		if includeComponents {
			files[claudeCodeOfficialBase+"/"+codeVersion+"/win32-x64/claude.exe.zst"] = codePayload
			files[claudeCodeOfficialBase+"/"+codeVersion+"/win32-arm64/claude.exe.zst"] = codePayload
		}
		return latest
	}
	first := addRelease("1.0.0", 'a', "2.0.0", true)
	stub := &downloadResourceGitHubStub{files: files}
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir, MaxAssetBytes: 1024 * 1024,
		ClaudeDesktopLatestBaseURL: base,
		ClaudeDesktopMacURL:        "https://example.test/claude.dmg",
	}}, stub)
	require.NoError(t, svc.SyncClaudeDesktop(context.Background()))

	_ = addRelease("1.0.1", 'b', "2.0.1", false)
	require.ErrorContains(t, svc.SyncClaudeDesktop(context.Background()), "download Claude Desktop Code")
	manifest, err := svc.ListTool(context.Background(), claudeDesktopToolID)
	require.NoError(t, err)
	require.Equal(t, claudeDesktopReleaseID(first), manifest.Version)
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

func TestClaudeDesktopReleaseIDIncludesCommitIdentity(t *testing.T) {
	require.Equal(t, "1.30096.1-1234567890ab", claudeDesktopReleaseID(claudeDesktopLatest{
		Version: "1.30096.1",
		Hash:    "1234567890abcdef1234567890abcdef12345678",
	}))
}

func TestDownloadRetentionKeepsOnlyCurrentVersionAndPreservedDirectories(t *testing.T) {
	dir := t.TempDir()
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: dir,
	}}, &downloadResourceGitHubStub{})

	versions := []string{"1.0.0-aaaaaaaaaaaa", "1.0.1-bbbbbbbbbbbb", "1.0.2-cccccccccccc", "1.0.3-dddddddddddd"}
	for _, version := range versions {
		require.NoError(t, svc.writeManifest(CachedDownloadManifest{Tool: claudeDesktopToolID, Version: version}))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, claudeDesktopToolID, "macos-static"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, claudeDesktopToolID, "incomplete-version"), 0755))

	require.NoError(t, svc.cleanupHistoricalToolVersions(claudeDesktopToolID, versions[3], "macos-static"))
	require.NoDirExists(t, filepath.Join(dir, claudeDesktopToolID, versions[0]))
	require.NoDirExists(t, filepath.Join(dir, claudeDesktopToolID, versions[1]))
	require.NoDirExists(t, filepath.Join(dir, claudeDesktopToolID, versions[2]))
	require.NoDirExists(t, filepath.Join(dir, claudeDesktopToolID, "incomplete-version"))
	require.DirExists(t, filepath.Join(dir, claudeDesktopToolID, versions[3]))
	require.DirExists(t, filepath.Join(dir, claudeDesktopToolID, "macos-static"))
}

func TestDownloadRetentionRejectsUnsafeScope(t *testing.T) {
	svc := NewDownloadResourceService(&config.Config{Downloads: config.DownloadsConfig{
		Enabled: true, CacheDir: t.TempDir(),
	}}, &downloadResourceGitHubStub{})

	require.ErrorIs(t, svc.cleanupHistoricalToolVersions("unknown-tool", "v1"), ErrDownloadToolNotFound)
	require.ErrorContains(t, svc.cleanupHistoricalToolVersions(codexToolID, ""), "current download version is empty")
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
