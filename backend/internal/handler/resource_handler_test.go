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

func TestExchangeSetupTicketRejectsInvalidTicketWithoutExposingCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewResourceHandler(nil, nil)
	router := gin.New()
	router.POST("/setup/exchange", handler.ExchangeSetupTicket)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/setup/exchange", strings.NewReader(`{"ticket":"existing-ticket"}`)))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"reason":"INVALID_CLIENT_SETUP_TICKET"`)
	require.NotContains(t, recorder.Body.String(), `"api_key"`)
}

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
	xml, err := buildCodexWindowsAppInstaller("codex-app-26.707.3748", service.CachedDownloadAsset{
		ID:     "openai.codex_26.707.3748.0_x64_2p2nqsd0c76g0.msix",
		Name:   "OpenAI.Codex_26.707.3748.0_x64__2p2nqsd0c76g0.Msix",
		SHA256: strings.Repeat("a", 64),
	})
	require.NoError(t, err)
	require.Contains(t, xml, `Name="OpenAI.Codex"`)
	require.Contains(t, xml, `Publisher="CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B"`)
	require.Contains(t, xml, `Version="26.707.3748.0"`)
	require.Contains(t, xml, `HoursBetweenUpdateChecks="24"`)
	require.Contains(t, xml, `ShowPrompt="false" UpdateBlocksActivation="false"`)
	require.Contains(t, xml, `<AutomaticBackgroundTask />`)
	require.Contains(t, xml, `/downloads/codex/windows-x64/codex-app-26.707.3748/`+strings.Repeat("a", 64)+`/openai.codex_26.707.3748.0_x64_2p2nqsd0c76g0.msix`)
}

func TestBuildCodexWindowsAppInstallerRejectsUnexpectedFilename(t *testing.T) {
	_, err := buildCodexWindowsAppInstaller("codex-app-26.707.3748", service.CachedDownloadAsset{ID: "bad", Name: "bad.msix"})
	require.Error(t, err)
}

func TestCodexWindowsLatestRedirectsToContentAddressedPackage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheDir := t.TempDir()
	version := "codex-app-26.803.81509"
	versionDir := filepath.Join(cacheDir, "codex", version)
	require.NoError(t, os.MkdirAll(versionDir, 0755))
	payload := []byte("verified-codex-windows-msix")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	assetName := "OpenAI.Codex_26.803.81509.0_x64__2p2nqsd0c76g0.Msix"
	assetID := "openai.codex_26.803.81509.0_x64_2p2nqsd0c76g0.msix"
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, assetName), payload, 0644))
	rawManifest, err := json.Marshal(service.CachedDownloadManifest{
		Tool: "codex", Version: version, Assets: []service.CachedDownloadAsset{{
			ID: assetID, Name: assetName, Size: int64(len(payload)), SHA256: digest,
			Platform: "windows", Arch: "x64",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "codex", "manifest.json"), rawManifest, 0644))

	downloads := service.NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{Enabled: true, CacheDir: cacheDir},
	}, nil)
	handler := NewResourceHandler(downloads, nil)
	router := gin.New()
	router.GET("/latest.msix", handler.DownloadCodexWindowsLatest)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/latest.msix", nil))
	require.Equal(t, http.StatusTemporaryRedirect, recorder.Code)
	require.Equal(t,
		"https://laoshirenai.com/downloads/codex/windows-x64/"+version+"/"+digest+"/"+assetID,
		recorder.Header().Get("Location"),
	)
	require.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")
	require.NotEqual(t, string(payload), recorder.Body.String())
}

func TestBuildPublicDownloadManifestUsesSameSiteImmutablePackageURLs(t *testing.T) {
	manifest := buildPublicDownloadManifest(&service.CachedDownloadManifest{
		Tool:        "cc-switch",
		Version:     "v3.18.0",
		ReleaseName: "CC Switch v3.18.0",
		PublishedAt: "2026-07-21T15:34:53Z",
		UpdatedAt:   "2026-07-25T19:24:37Z",
		Assets: []service.CachedDownloadAsset{{
			ID:                 "cc-switch-v3.18.0-windows.msi",
			Name:               "CC-Switch-v3.18.0-Windows.msi",
			Size:               12849152,
			SHA256:             "c4a6eaf763269396f90a81377381e91c8341538b51376912c81bab73e844612d",
			Platform:           "windows",
			Arch:               "universal",
			Role:               "claude-desktop-code",
			ComponentVersion:   "2.1.229",
			UpstreamSHA256:     strings.Repeat("b", 64),
			UpstreamCompressed: 71120125,
		}},
	}, "cc-switch")

	require.Equal(t, "v3.18.0", manifest.Version)
	require.Len(t, manifest.Assets, 1)
	require.Equal(t,
		"https://laoshirenai.com/downloads/cc-switch/v3.18.0/c4a6eaf763269396f90a81377381e91c8341538b51376912c81bab73e844612d/cc-switch-v3.18.0-windows.msi",
		manifest.Assets[0].DownloadURL,
	)
	require.Equal(t, "c4a6eaf763269396f90a81377381e91c8341538b51376912c81bab73e844612d", manifest.Assets[0].SHA256)
	require.Equal(t, "claude-desktop-code", manifest.Assets[0].Role)
	require.Equal(t, "2.1.229", manifest.Assets[0].ComponentVersion)
	require.Equal(t, strings.Repeat("b", 64), manifest.Assets[0].UpstreamSHA256)
	require.Equal(t, int64(71120125), manifest.Assets[0].UpstreamCompressedSize)
}

func TestBuildPublicDownloadManifestRejectsUnsafeVersionSegments(t *testing.T) {
	manifest := buildPublicDownloadManifest(&service.CachedDownloadManifest{
		Tool: "grok-build", Version: "../", Assets: []service.CachedDownloadAsset{{
			ID: "grok.exe", Name: "grok.exe", SHA256: strings.Repeat("a", 64),
		}},
	}, "grok-build")

	require.Len(t, manifest.Assets, 1)
	require.Equal(t,
		"https://laoshirenai.com/downloads/grok-build/unknown/"+strings.Repeat("a", 64)+"/grok.exe",
		manifest.Assets[0].DownloadURL,
	)
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
	router.GET("/downloads/cc-switch/:version/:sha256/:filename", handler.DownloadCCSwitchImmutablePackage)

	manifestRecorder := httptest.NewRecorder()
	router.ServeHTTP(manifestRecorder, httptest.NewRequest(http.MethodGet, "/latest.json", nil))
	require.Equal(t, http.StatusOK, manifestRecorder.Code)
	require.Contains(t, manifestRecorder.Header().Get("Cache-Control"), "max-age=300")
	require.Contains(t, manifestRecorder.Body.String(), "/downloads/cc-switch/v3.18.0/d7173d29e8ebf3a524f799178551856f9f28c4efcf77ab2885bd180c09b13f75/"+assetID)

	packageRecorder := httptest.NewRecorder()
	router.ServeHTTP(packageRecorder, httptest.NewRequest(http.MethodGet, "/packages/"+assetID, nil))
	require.Equal(t, http.StatusOK, packageRecorder.Code)
	require.Contains(t, packageRecorder.Header().Get("Cache-Control"), "immutable")
	require.Equal(t, "verified-msi", packageRecorder.Body.String())

	immutableRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/downloads/cc-switch/v3.18.0/d7173d29e8ebf3a524f799178551856f9f28c4efcf77ab2885bd180c09b13f75/"+assetID, nil)
	request.Header.Set("Range", "bytes=0-7")
	router.ServeHTTP(immutableRecorder, request)
	require.Equal(t, http.StatusPartialContent, immutableRecorder.Code)
	require.Equal(t, "verified", immutableRecorder.Body.String())
	require.Equal(t, "bytes", immutableRecorder.Header().Get("Accept-Ranges"))
	require.Contains(t, immutableRecorder.Header().Get("Cache-Control"), "immutable")
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
	router.GET("/downloads/codex/:version/:sha256/:filename", handler.DownloadCodexImmutablePackage)

	manifestRecorder := httptest.NewRecorder()
	router.ServeHTTP(manifestRecorder, httptest.NewRequest(http.MethodGet, "/latest.json", nil))
	require.Equal(t, http.StatusOK, manifestRecorder.Code)
	require.Contains(t, manifestRecorder.Body.String(), "/downloads/codex/codex-app-26.721.4979/2f135f56ac6277ae11f773cd5ffb3756ca59ab4d7799a0eb344fe34da0df7ea2/"+assetID)

	packageRecorder := httptest.NewRecorder()
	router.ServeHTTP(packageRecorder, httptest.NewRequest(http.MethodGet, "/packages/"+assetID, nil))
	require.Equal(t, http.StatusOK, packageRecorder.Code)
	require.Equal(t, "application/x-apple-diskimage", packageRecorder.Header().Get("Content-Type"))
	require.Equal(t, "verified-dmg", packageRecorder.Body.String())

	immutableRecorder := httptest.NewRecorder()
	router.ServeHTTP(immutableRecorder, httptest.NewRequest(
		http.MethodGet,
		"/downloads/codex/codex-app-26.721.4979/2f135f56ac6277ae11f773cd5ffb3756ca59ab4d7799a0eb344fe34da0df7ea2/"+assetID,
		nil,
	))
	require.Equal(t, http.StatusOK, immutableRecorder.Code)
	require.Contains(t, immutableRecorder.Header().Get("Cache-Control"), "immutable")
	require.Equal(t, "verified-dmg", immutableRecorder.Body.String())
}

func TestGrokBuildPublicEndpointsServeContentAddressedWindowsBinary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheDir := t.TempDir()
	versionDir := filepath.Join(cacheDir, "grok-build", "1.0.3")
	require.NoError(t, os.MkdirAll(versionDir, 0755))
	payload := []byte("verified-grok-build-windows")
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	assetName := "grok-1.0.3-windows-x86_64.exe"
	assetID := "grok-1.0.3-windows-x86_64.exe"
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, assetName), payload, 0644))
	rawManifest, err := json.Marshal(service.CachedDownloadManifest{
		Tool: "grok-build", Version: "1.0.3", Assets: []service.CachedDownloadAsset{{
			ID: assetID, Name: assetName, Size: int64(len(payload)), SHA256: digest,
			Platform: "windows", Arch: "x64",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "grok-build", "manifest.json"), rawManifest, 0644))

	downloads := service.NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{Enabled: true, CacheDir: cacheDir},
	}, nil)
	handler := NewResourceHandler(downloads, nil)
	router := gin.New()
	router.GET("/latest.json", handler.GrokBuildLatestManifest)
	router.GET("/downloads/grok-build/:version/:sha256/:filename", handler.DownloadGrokBuildImmutablePackage)

	manifestRecorder := httptest.NewRecorder()
	router.ServeHTTP(manifestRecorder, httptest.NewRequest(http.MethodGet, "/latest.json", nil))
	require.Equal(t, http.StatusOK, manifestRecorder.Code)
	require.Contains(t, manifestRecorder.Body.String(), "/downloads/grok-build/1.0.3/"+digest+"/"+assetID)

	packageRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/downloads/grok-build/1.0.3/"+digest+"/"+assetID, nil)
	request.Header.Set("Range", "bytes=0-7")
	router.ServeHTTP(packageRecorder, request)
	require.Equal(t, http.StatusPartialContent, packageRecorder.Code)
	require.Equal(t, "verified", packageRecorder.Body.String())
	require.Contains(t, packageRecorder.Header().Get("Cache-Control"), "immutable")
}

func TestImmutableEndpointContinuesServingPreviousVersionAfterManifestAdvance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cacheDir := t.TempDir()
	toolDir := filepath.Join(cacheDir, "cc-switch")
	oldVersionDir := filepath.Join(toolDir, "v3.18.0")
	newVersionDir := filepath.Join(toolDir, "v3.19.0")
	require.NoError(t, os.MkdirAll(oldVersionDir, 0755))
	require.NoError(t, os.MkdirAll(newVersionDir, 0755))

	oldPayload := []byte("old-verified-installer")
	oldDigest := fmt.Sprintf("%x", sha256.Sum256(oldPayload))
	oldName := "CC-Switch-v3.18.0-Windows.msi"
	oldID := "cc-switch-v3.18.0-windows.msi"
	require.NoError(t, os.WriteFile(filepath.Join(oldVersionDir, oldName), oldPayload, 0644))
	oldManifest := service.CachedDownloadManifest{
		Tool: "cc-switch", Version: "v3.18.0", Assets: []service.CachedDownloadAsset{{
			ID: oldID, Name: oldName, Size: int64(len(oldPayload)), SHA256: oldDigest,
			Platform: "windows", Arch: "x64",
		}},
	}
	oldRaw, err := json.Marshal(oldManifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(oldVersionDir, ".manifest.json"), oldRaw, 0644))

	newPayload := []byte("new-installer")
	newName := "CC-Switch-v3.19.0-Windows.msi"
	require.NoError(t, os.WriteFile(filepath.Join(newVersionDir, newName), newPayload, 0644))
	newRaw, err := json.Marshal(service.CachedDownloadManifest{
		Tool: "cc-switch", Version: "v3.19.0", Assets: []service.CachedDownloadAsset{{
			ID: "cc-switch-v3.19.0-windows.msi", Name: newName,
			Size: int64(len(newPayload)), SHA256: fmt.Sprintf("%x", sha256.Sum256(newPayload)),
			Platform: "windows", Arch: "x64",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(toolDir, "manifest.json"), newRaw, 0644))

	downloads := service.NewDownloadResourceService(&config.Config{
		Downloads: config.DownloadsConfig{Enabled: true, CacheDir: cacheDir},
	}, nil)
	handler := NewResourceHandler(downloads, nil)
	router := gin.New()
	router.GET("/downloads/cc-switch/:version/:sha256/:filename", handler.DownloadCCSwitchImmutablePackage)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodGet,
		"/downloads/cc-switch/v3.18.0/"+oldDigest+"/"+oldID,
		nil,
	))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, string(oldPayload), recorder.Body.String())
	require.Contains(t, recorder.Header().Get("Cache-Control"), "immutable")
}
