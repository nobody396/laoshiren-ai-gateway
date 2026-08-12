package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClaudeDesktopWindowsDownloadIsContentAddressedImmutableAndRangeCapable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheDir := t.TempDir()
	versionDir := filepath.Join(cacheDir, "claude-desktop", "latest")
	require.NoError(t, os.MkdirAll(versionDir, 0755))
	payload := []byte("verified-claude-desktop-installer")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	assetName := "Claude-Setup-x64.exe"
	assetID := "claude-setup-x64.exe"
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, assetName), payload, 0644))
	rawManifest, err := json.Marshal(service.CachedDownloadManifest{
		Tool:    "claude-desktop",
		Version: "latest",
		Assets: []service.CachedDownloadAsset{{
			ID:       assetID,
			Name:     assetName,
			Size:     int64(len(payload)),
			SHA256:   digest,
			Platform: "windows",
			Arch:     "x64",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "claude-desktop", "manifest.json"), rawManifest, 0644))

	downloads := service.NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{Enabled: true, CacheDir: cacheDir},
	}, nil)
	handler := NewResourceHandler(downloads, nil)
	router := gin.New()
	router.GET("/downloads/claude-desktop/windows-x64/:sha256/Claude-Setup.exe", handler.DownloadClaudeDesktopWindowsX64)

	request := httptest.NewRequest(http.MethodGet, "/downloads/claude-desktop/windows-x64/"+digest+"/Claude-Setup.exe", nil)
	request.Header.Set("Range", "bytes=0-7")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "bytes 0-7/33", recorder.Header().Get("Content-Range"))
	require.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
	require.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "verified", recorder.Body.String())
	require.Contains(t, recorder.Header().Get("Content-Disposition"), `filename="Claude-Setup.exe"`)

	wrongDigest := httptest.NewRecorder()
	router.ServeHTTP(wrongDigest, httptest.NewRequest(
		http.MethodGet,
		"/downloads/claude-desktop/windows-x64/"+strings.Repeat("0", 64)+"/Claude-Setup.exe",
		nil,
	))
	require.Equal(t, http.StatusNotFound, wrongDigest.Code)
}

func TestBuildCodexWindowsAppInstallerEnablesNonBlockingUpdates(t *testing.T) {
	xml, err := buildCodexWindowsAppInstaller(service.CachedDownloadAsset{
		ID:   "openai.codex_26.707.3748.0_x64_2p2nqsd0c76g0.msix",
		Name: "OpenAI.Codex_26.707.3748.0_x64__2p2nqsd0c76g0.Msix",
	})
	require.NoError(t, err)
	require.Contains(t, xml, `Name="OpenAI.Codex"`)
	require.Contains(t, xml, `Publisher="CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B"`)
	require.Contains(t, xml, `Version="26.707.3748.0"`)
	require.Contains(t, xml, `HoursBetweenUpdateChecks="24"`)
	require.Contains(t, xml, `ShowPrompt="false" UpdateBlocksActivation="false"`)
	require.Contains(t, xml, `<AutomaticBackgroundTask />`)
	require.Contains(t, xml, `/packages/openai.codex_26.707.3748.0_x64_2p2nqsd0c76g0.msix`)
}

func TestBuildCodexWindowsAppInstallerRejectsUnexpectedFilename(t *testing.T) {
	_, err := buildCodexWindowsAppInstaller(service.CachedDownloadAsset{ID: "bad", Name: "bad.msix"})
	require.Error(t, err)
}

func TestBuildPublicDownloadManifestUsesSameSiteImmutablePackageURLs(t *testing.T) {
	manifest := buildPublicDownloadManifest(&service.CachedDownloadManifest{
		Tool:        "cc-switch",
		Version:     "v3.18.0",
		ReleaseName: "CC Switch v3.18.0",
		PublishedAt: "2026-07-21T15:34:53Z",
		UpdatedAt:   "2026-07-25T19:24:37Z",
		Assets: []service.CachedDownloadAsset{{
			ID:       "cc-switch-v3.18.0-windows.msi",
			Name:     "CC-Switch-v3.18.0-Windows.msi",
			Size:     12849152,
			SHA256:   "c4a6eaf763269396f90a81377381e91c8341538b51376912c81bab73e844612d",
			Platform: "windows",
			Arch:     "universal",
		}},
	}, ccSwitchPublicBase)

	require.Equal(t, "v3.18.0", manifest.Version)
	require.Len(t, manifest.Assets, 1)
	require.Equal(t,
		"https://laoshirenai.com/api/v1/public-downloads/cc-switch/packages/cc-switch-v3.18.0-windows.msi",
		manifest.Assets[0].DownloadURL,
	)
	require.Equal(t, "c4a6eaf763269396f90a81377381e91c8341538b51376912c81bab73e844612d", manifest.Assets[0].SHA256)
}

func TestCCSwitchPublicEndpointsServeCachedManifestAndPackageWithoutAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheDir := t.TempDir()
	versionDir := filepath.Join(cacheDir, "cc-switch", "v3.18.0")
	require.NoError(t, os.MkdirAll(versionDir, 0755))
	assetName := "CC-Switch-v3.18.0-Windows.msi"
	assetID := "cc-switch-v3.18.0-windows.msi"
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, assetName), []byte("verified-msi"), 0644))
	rawManifest, err := json.Marshal(service.CachedDownloadManifest{
		Tool:        "cc-switch",
		Version:     "v3.18.0",
		ReleaseName: "CC Switch v3.18.0",
		Assets: []service.CachedDownloadAsset{{
			ID:       assetID,
			Name:     assetName,
			Size:     int64(len("verified-msi")),
			SHA256:   "d7173d29e8ebf3a524f799178551856f9f28c4efcf77ab2885bd180c09b13f75",
			Platform: "windows",
			Arch:     "universal",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "cc-switch", "manifest.json"), rawManifest, 0644))

	downloads := service.NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{
			Enabled:  true,
			CacheDir: cacheDir,
		},
	}, nil)
	handler := NewResourceHandler(downloads, nil)
	router := gin.New()
	router.GET("/latest.json", handler.CCSwitchLatestManifest)
	router.GET("/packages/:assetID", handler.DownloadCCSwitchPackage)

	manifestRecorder := httptest.NewRecorder()
	router.ServeHTTP(manifestRecorder, httptest.NewRequest(http.MethodGet, "/latest.json", nil))
	require.Equal(t, http.StatusOK, manifestRecorder.Code)
	require.Contains(t, manifestRecorder.Header().Get("Cache-Control"), "max-age=300")
	require.Contains(t, manifestRecorder.Body.String(), ccSwitchPublicBase+"/packages/"+assetID)

	packageRecorder := httptest.NewRecorder()
	router.ServeHTTP(packageRecorder, httptest.NewRequest(http.MethodGet, "/packages/"+assetID, nil))
	require.Equal(t, http.StatusOK, packageRecorder.Code)
	require.Contains(t, packageRecorder.Header().Get("Cache-Control"), "immutable")
	require.Equal(t, "verified-msi", packageRecorder.Body.String())
}

func TestCodexPublicEndpointsExposeCrossPlatformDesktopPackages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheDir := t.TempDir()
	versionDir := filepath.Join(cacheDir, "codex", "codex-app-26.721.4979")
	require.NoError(t, os.MkdirAll(versionDir, 0755))
	assetName := "Codex-mac-arm64.dmg"
	assetID := "codex-mac-arm64.dmg"
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, assetName), []byte("verified-dmg"), 0644))
	rawManifest, err := json.Marshal(service.CachedDownloadManifest{
		Tool:    "codex",
		Version: "codex-app-26.721.4979",
		Assets: []service.CachedDownloadAsset{{
			ID:       assetID,
			Name:     assetName,
			Size:     int64(len("verified-dmg")),
			SHA256:   "2f135f56ac6277ae11f773cd5ffb3756ca59ab4d7799a0eb344fe34da0df7ea2",
			Platform: "macos",
			Arch:     "arm64",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "codex", "manifest.json"), rawManifest, 0644))

	downloads := service.NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{Enabled: true, CacheDir: cacheDir},
	}, nil)
	handler := NewResourceHandler(downloads, nil)
	router := gin.New()
	router.GET("/latest.json", handler.CodexLatestManifest)
	router.GET("/packages/:assetID", handler.DownloadCodexPackage)

	manifestRecorder := httptest.NewRecorder()
	router.ServeHTTP(manifestRecorder, httptest.NewRequest(http.MethodGet, "/latest.json", nil))
	require.Equal(t, http.StatusOK, manifestRecorder.Code)
	require.Contains(t, manifestRecorder.Body.String(), codexPublicBase+"/packages/"+assetID)

	packageRecorder := httptest.NewRecorder()
	router.ServeHTTP(packageRecorder, httptest.NewRequest(http.MethodGet, "/packages/"+assetID, nil))
	require.Equal(t, http.StatusOK, packageRecorder.Code)
	require.Equal(t, "application/x-apple-diskimage", packageRecorder.Header().Get("Content-Type"))
	require.Equal(t, "verified-dmg", packageRecorder.Body.String())
}
