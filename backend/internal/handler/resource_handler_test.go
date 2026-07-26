package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
	handler := NewResourceHandler(downloads)
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
