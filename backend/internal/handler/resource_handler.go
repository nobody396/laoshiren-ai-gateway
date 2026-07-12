package handler

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const resourceDownloadTokenTTL = 5 * time.Minute

const (
	codexWindowsPublicBase = "https://laoshirenai.com/api/v1/public-downloads/codex/windows-x64"
	codexPackageName       = "OpenAI.Codex"
	codexPackagePublisher  = "CN=50BDFD77-8903-4850-9FFE-6E8522F64D5B"
)

var codexWindowsMSIXVersion = regexp.MustCompile(`(?i)^OpenAI\.Codex_([0-9]+(?:\.[0-9]+){3})_x64__.*\.msix$`)

type ResourceHandler struct {
	downloads *service.DownloadResourceService
}

func NewResourceHandler(downloads *service.DownloadResourceService) *ResourceHandler {
	return &ResourceHandler{downloads: downloads}
}

func (h *ResourceHandler) ListTool(c *gin.Context) {
	manifest, err := h.downloads.ListTool(c.Request.Context(), c.Param("tool"))
	if err != nil {
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "安装包正在同步，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrDownloadToolNotFound) {
			response.NotFound(c, "下载资源不存在")
			return
		}
		response.InternalError(c, "加载下载资源失败")
		return
	}
	response.Success(c, manifest)
}

func (h *ResourceHandler) DownloadTool(c *gin.Context) {
	file, err := h.downloads.GetToolAsset(c.Request.Context(), c.Param("tool"), c.Param("assetID"))
	if err != nil {
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "安装包正在同步，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrDownloadToolNotFound) || errors.Is(err, service.ErrDownloadAssetNotFound) {
			response.NotFound(c, "安装包不存在")
			return
		}
		response.InternalError(c, "读取安装包失败")
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.FileAttachment(file.Path, file.Asset.Name)
}

func (h *ResourceHandler) CreateDownloadURL(c *gin.Context) {
	token, expiresAt, err := h.downloads.CreateToolAssetDownloadToken(
		c.Request.Context(),
		c.Param("tool"),
		c.Param("assetID"),
		resourceDownloadTokenTTL,
	)
	if err != nil {
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "安装包正在同步，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrDownloadToolNotFound) || errors.Is(err, service.ErrDownloadAssetNotFound) {
			response.NotFound(c, "安装包不存在")
			return
		}
		response.InternalError(c, "创建下载链接失败")
		return
	}
	response.Success(c, gin.H{
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func (h *ResourceHandler) DownloadWithToken(c *gin.Context) {
	file, err := h.downloads.GetToolAssetByDownloadToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		if errors.Is(err, service.ErrDownloadTokenInvalid) {
			response.Error(c, http.StatusGone, "下载链接已过期，请重新点击下载")
			return
		}
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "安装包正在同步，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrDownloadToolNotFound) || errors.Is(err, service.ErrDownloadAssetNotFound) {
			response.NotFound(c, "安装包不存在")
			return
		}
		response.InternalError(c, "读取安装包失败")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(file.Path, file.Asset.Name)
}

func (h *ResourceHandler) DownloadCodexWindowsLatest(c *gin.Context) {
	file, err := h.downloads.GetCodexWindowsDesktopAsset(c.Request.Context())
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	c.Header("Content-Type", "application/msix")
	c.Header("Cache-Control", "public, max-age=300")
	c.FileAttachment(file.Path, file.Asset.Name)
}

func (h *ResourceHandler) DownloadCodexWindowsPackage(c *gin.Context) {
	file, err := h.downloads.GetCodexWindowsDesktopAsset(c.Request.Context())
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	if c.Param("assetID") != file.Asset.ID {
		response.NotFound(c, "安装包不存在")
		return
	}
	c.Header("Content-Type", "application/msix")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(file.Path)
}

func (h *ResourceHandler) CodexWindowsAppInstaller(c *gin.Context) {
	file, err := h.downloads.GetCodexWindowsDesktopAsset(c.Request.Context())
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	xml, err := buildCodexWindowsAppInstaller(file.Asset)
	if err != nil {
		response.InternalError(c, "安装包版本格式无效")
		return
	}
	c.Header("Content-Type", "application/appinstaller")
	c.Header("Content-Disposition", `attachment; filename="Codex-Windows-x64.appinstaller"`)
	c.Header("Cache-Control", "no-cache, must-revalidate")
	c.String(http.StatusOK, xml)
}

func buildCodexWindowsAppInstaller(asset service.CachedDownloadAsset) (string, error) {
	matches := codexWindowsMSIXVersion.FindStringSubmatch(asset.Name)
	if len(matches) != 2 {
		return "", errors.New("invalid Codex Windows MSIX filename")
	}
	version := matches[1]
	packageURL := fmt.Sprintf("%s/packages/%s", codexWindowsPublicBase, asset.ID)
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<AppInstaller xmlns="http://schemas.microsoft.com/appx/appinstaller/2021"
  Uri="%s/latest.appinstaller"
  Version="%s">
  <MainPackage
    Name="%s"
    Publisher="%s"
    Version="%s"
    ProcessorArchitecture="x64"
    Uri="%s" />
  <UpdateSettings>
    <OnLaunch HoursBetweenUpdateChecks="24" ShowPrompt="false" UpdateBlocksActivation="false" />
    <AutomaticBackgroundTask />
  </UpdateSettings>
</AppInstaller>
`, codexWindowsPublicBase, version, codexPackageName, codexPackagePublisher, version, packageURL)
	return strings.TrimSpace(xml) + "\n", nil
}

func handlePublicDownloadError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrDownloadManifestNotReady) {
		response.Error(c, http.StatusServiceUnavailable, "Codex Windows 安装包正在同步，请稍后再试")
		return
	}
	if errors.Is(err, service.ErrDownloadAssetNotFound) {
		response.NotFound(c, "Codex Windows 安装包不存在")
		return
	}
	response.InternalError(c, "读取 Codex Windows 安装包失败")
}

func (h *ResourceHandler) ListCCSwitch(c *gin.Context) {
	manifest, err := h.downloads.ListCCSwitch(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "CC Switch 安装包正在同步，请稍后再试")
			return
		}
		response.InternalError(c, "加载下载资源失败")
		return
	}
	response.Success(c, manifest)
}

func (h *ResourceHandler) DownloadCCSwitch(c *gin.Context) {
	assetID := c.Param("assetID")
	file, err := h.downloads.GetCCSwitchAsset(c.Request.Context(), assetID)
	if err != nil {
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "CC Switch 安装包正在同步，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrDownloadAssetNotFound) {
			response.NotFound(c, "安装包不存在")
			return
		}
		response.InternalError(c, "读取安装包失败")
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.FileAttachment(file.Path, file.Asset.Name)
}
