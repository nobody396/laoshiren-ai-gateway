package handler

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
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
var publicDownloadPathUnsafeChars = regexp.MustCompile(`[^a-z0-9._-]+`)

type ResourceHandler struct {
	downloads *service.DownloadResourceService
	setup     *service.ClientSetupService
}

type clientSetupTicketRequest struct {
	Target   string `json:"target"`
	APIKeyID *int64 `json:"api_key_id"`
}

type clientSetupExchangeRequest struct {
	Ticket string `json:"ticket" binding:"required"`
}

type publicDownloadManifest struct {
	Tool        string                `json:"tool"`
	Version     string                `json:"version"`
	ReleaseName string                `json:"release_name"`
	PublishedAt string                `json:"published_at"`
	UpdatedAt   string                `json:"updated_at"`
	Assets      []publicDownloadAsset `json:"assets"`
}

type publicDownloadAsset struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Size                   int64  `json:"size"`
	SHA256                 string `json:"sha256"`
	Platform               string `json:"platform"`
	Arch                   string `json:"arch"`
	Role                   string `json:"role,omitempty"`
	ComponentVersion       string `json:"component_version,omitempty"`
	UpstreamSHA256         string `json:"upstream_sha256,omitempty"`
	UpstreamCompressedSize int64  `json:"upstream_compressed_size,omitempty"`
	DownloadURL            string `json:"download_url"`
}

func NewResourceHandler(downloads *service.DownloadResourceService, setup *service.ClientSetupService) *ResourceHandler {
	return &ResourceHandler{downloads: downloads, setup: setup}
}

func buildPublicDownloadManifest(manifest *service.CachedDownloadManifest, tool string) publicDownloadManifest {
	result := publicDownloadManifest{
		Tool:        manifest.Tool,
		Version:     manifest.Version,
		ReleaseName: manifest.ReleaseName,
		PublishedAt: manifest.PublishedAt,
		UpdatedAt:   manifest.UpdatedAt,
		Assets:      make([]publicDownloadAsset, 0, len(manifest.Assets)),
	}
	for _, asset := range manifest.Assets {
		result.Assets = append(result.Assets, publicDownloadAsset{
			ID:                     asset.ID,
			Name:                   asset.Name,
			Size:                   asset.Size,
			SHA256:                 asset.SHA256,
			Platform:               asset.Platform,
			Arch:                   asset.Arch,
			Role:                   asset.Role,
			ComponentVersion:       asset.ComponentVersion,
			UpstreamSHA256:         asset.UpstreamSHA256,
			UpstreamCompressedSize: asset.UpstreamCompressed,
			DownloadURL:            buildImmutableDownloadURL(tool, manifest, asset),
		})
	}
	return result
}

func buildImmutableDownloadURL(tool string, manifest *service.CachedDownloadManifest, asset service.CachedDownloadAsset) string {
	version := asset.ID
	if manifest != nil && strings.TrimSpace(manifest.Version) != "" {
		version = sanitizePublicDownloadPathSegment(manifest.Version)
	}
	return fmt.Sprintf(
		"https://laoshirenai.com/downloads/%s/%s/%s/%s",
		sanitizePublicDownloadPathSegment(tool),
		version,
		strings.ToLower(strings.TrimSpace(asset.SHA256)),
		asset.ID,
	)
}

func sanitizePublicDownloadPathSegment(value string) string {
	clean := strings.Trim(publicDownloadPathUnsafeChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
	if clean == "" || clean == "." || clean == ".." {
		return "unknown"
	}
	return clean
}

func serveImmutableDownload(c *gin.Context, file *service.DownloadAssetFile, requestedSHA256, requestedFilename string) {
	if file == nil ||
		!strings.EqualFold(strings.TrimSpace(requestedSHA256), strings.TrimSpace(file.Asset.SHA256)) ||
		requestedFilename != file.Asset.ID {
		response.NotFound(c, "安装包不存在")
		return
	}

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Accept-Ranges", "bytes")
	c.FileAttachment(file.Path, file.Asset.Name)
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

func (h *ResourceHandler) ListVersionStatus(c *gin.Context) {
	c.Header("Cache-Control", "private, max-age=300")
	response.Success(c, h.downloads.ListVersionStatus(c.Request.Context()))
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

// DownloadClaudeDesktopWindowsX64 serves the locally verified installer from a
// content-addressed, non-API URL. Keeping the checksum in the URL makes the
// response immutable and allows the site CDN to cache the large executable at
// edge locations instead of proxying every download to the application host.
func (h *ResourceHandler) DownloadClaudeDesktopWindowsX64(c *gin.Context) {
	file, err := h.downloads.GetImmutableToolAssetBySHA(
		c.Request.Context(),
		"claude-desktop",
		c.Param("sha256"),
		"",
	)
	if err != nil {
		if errors.Is(err, service.ErrDownloadManifestNotReady) {
			response.Error(c, http.StatusServiceUnavailable, "Claude Desktop 安装包正在同步，请稍后再试")
			return
		}
		if errors.Is(err, service.ErrDownloadAssetNotFound) {
			response.NotFound(c, "Claude Desktop 安装包不存在")
			return
		}
		response.InternalError(c, "读取 Claude Desktop 安装包失败")
		return
	}
	if file.Asset.Platform != "windows" || file.Asset.Arch != "x64" ||
		!strings.EqualFold(filepath.Ext(file.Asset.Name), ".exe") ||
		!strings.EqualFold(strings.TrimSpace(c.Param("sha256")), strings.TrimSpace(file.Asset.SHA256)) {
		response.NotFound(c, "Claude Desktop 安装包不存在")
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Accept-Ranges", "bytes")
	c.FileAttachment(file.Path, "Claude-Setup.exe")
}

func (h *ResourceHandler) DownloadCCSwitchImmutablePackage(c *gin.Context) {
	h.downloadImmutableToolPackage(c, "cc-switch")
}

func (h *ResourceHandler) DownloadCodexImmutablePackage(c *gin.Context) {
	h.downloadImmutableToolPackage(c, "codex")
}

func (h *ResourceHandler) DownloadCodexWindowsImmutablePackage(c *gin.Context) {
	file, err := h.downloads.GetImmutableToolAsset(
		c.Request.Context(),
		"codex",
		c.Param("version"),
		c.Param("filename"),
	)
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	if file.Asset.Platform != "windows" || file.Asset.Arch != "x64" || !strings.EqualFold(filepath.Ext(file.Asset.Name), ".msix") {
		response.NotFound(c, "安装包不存在")
		return
	}
	serveImmutableDownload(c, file, c.Param("sha256"), c.Param("filename"))
}

func (h *ResourceHandler) DownloadGitForWindowsImmutablePackage(c *gin.Context) {
	h.downloadImmutableToolPackage(c, "git-for-windows")
}

func (h *ResourceHandler) DownloadGrokBuildImmutablePackage(c *gin.Context) {
	h.downloadImmutableToolPackage(c, "grok-build")
}

func (h *ResourceHandler) DownloadCodexPlusPlusImmutablePackage(c *gin.Context) {
	h.downloadImmutableToolPackage(c, "codex-plus-plus")
}

func (h *ResourceHandler) DownloadClaudeDesktopImmutablePackage(c *gin.Context) {
	h.downloadImmutableToolPackage(c, "claude-desktop")
}

func (h *ResourceHandler) downloadImmutableToolPackage(c *gin.Context, tool string) {
	file, err := h.downloads.GetImmutableToolAsset(
		c.Request.Context(),
		tool,
		c.Param("version"),
		c.Param("filename"),
	)
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	serveImmutableDownload(c, file, c.Param("sha256"), c.Param("filename"))
}

func (h *ResourceHandler) CreateSetupTicket(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req clientSetupTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请选择 API 密钥或一键安装目标")
		return
	}
	var (
		ticket *service.ClientSetupTicket
		err    error
	)
	if req.APIKeyID != nil {
		ticket, err = h.setup.IssueTicketForAPIKey(c.Request.Context(), subject.UserID, *req.APIKeyID)
	} else {
		ticket, err = h.setup.IssueTicket(c.Request.Context(), subject.UserID, req.Target)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{
		"ticket":     ticket.Ticket,
		"expires_in": ticket.ExpiresIn,
		"target":     ticket.Target,
		"key_name":   ticket.KeyName,
		"group_name": ticket.GroupName,
	})
}

func (h *ResourceHandler) ExchangeSetupTicket(c *gin.Context) {
	var req clientSetupExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "一键安装凭证不能为空")
		return
	}
	credential, err := h.setup.ExchangeTicket(c.Request.Context(), req.Ticket)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{
		"target":   credential.Target,
		"api_key":  credential.APIKey,
		"base_url": credential.BaseURL,
	})
}

func (h *ResourceHandler) DownloadCodexWindowsLatest(c *gin.Context) {
	file, err := h.downloads.GetCodexWindowsDesktopAsset(c.Request.Context())
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	manifest, err := h.downloads.ListTool(c.Request.Context(), "codex")
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	// Keep the mutable `latest.msix` endpoint tiny and uncached. The client follows
	// this redirect to the SHA-addressed package, which EdgeOne can safely cache
	// without ever serving a stale installer under the same URL.
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Redirect(http.StatusTemporaryRedirect, buildCodexWindowsImmutableURL(manifest.Version, file.Asset))
}

func (h *ResourceHandler) CodexWindowsLatestManifest(c *gin.Context) {
	file, err := h.downloads.GetCodexWindowsDesktopAsset(c.Request.Context())
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	manifest, err := h.downloads.ListTool(c.Request.Context(), "codex")
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, publicDownloadManifest{
		Tool:        manifest.Tool,
		Version:     manifest.Version,
		ReleaseName: manifest.ReleaseName,
		PublishedAt: manifest.PublishedAt,
		UpdatedAt:   manifest.UpdatedAt,
		Assets: []publicDownloadAsset{{
			ID:          file.Asset.ID,
			Name:        file.Asset.Name,
			Size:        file.Asset.Size,
			SHA256:      file.Asset.SHA256,
			Platform:    file.Asset.Platform,
			Arch:        file.Asset.Arch,
			DownloadURL: buildCodexWindowsImmutableURL(manifest.Version, file.Asset),
		}},
	})
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

func (h *ResourceHandler) CodexLatestManifest(c *gin.Context) {
	manifest, err := h.downloads.ListTool(c.Request.Context(), "codex")
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, buildPublicDownloadManifest(manifest, "codex"))
}

func (h *ResourceHandler) GitForWindowsLatestManifest(c *gin.Context) {
	h.latestImmutableToolManifest(c, "git-for-windows")
}

func (h *ResourceHandler) GrokBuildLatestManifest(c *gin.Context) {
	h.latestImmutableToolManifest(c, "grok-build")
}

func (h *ResourceHandler) ClaudeDesktopLatestManifest(c *gin.Context) {
	h.latestImmutableToolManifest(c, "claude-desktop")
}

func (h *ResourceHandler) latestImmutableToolManifest(c *gin.Context, tool string) {
	manifest, err := h.downloads.ListTool(c.Request.Context(), tool)
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, buildPublicDownloadManifest(manifest, tool))
}

func (h *ResourceHandler) DownloadCodexPackage(c *gin.Context) {
	file, err := h.downloads.GetToolAsset(c.Request.Context(), "codex", c.Param("assetID"))
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	switch strings.ToLower(filepath.Ext(file.Asset.Name)) {
	case ".msix":
		c.Header("Content-Type", "application/msix")
	case ".dmg":
		c.Header("Content-Type", "application/x-apple-diskimage")
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.FileAttachment(file.Path, file.Asset.Name)
}

func (h *ResourceHandler) CodexWindowsAppInstaller(c *gin.Context) {
	file, err := h.downloads.GetCodexWindowsDesktopAsset(c.Request.Context())
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	manifest, err := h.downloads.ListTool(c.Request.Context(), "codex")
	if err != nil {
		handlePublicDownloadError(c, err)
		return
	}
	xml, err := buildCodexWindowsAppInstaller(manifest.Version, file.Asset)
	if err != nil {
		response.InternalError(c, "安装包版本格式无效")
		return
	}
	c.Header("Content-Type", "application/appinstaller")
	c.Header("Content-Disposition", `attachment; filename="Codex-Windows-x64.appinstaller"`)
	c.Header("Cache-Control", "no-cache, must-revalidate")
	c.String(http.StatusOK, xml)
}

func buildCodexWindowsAppInstaller(manifestVersion string, asset service.CachedDownloadAsset) (string, error) {
	matches := codexWindowsMSIXVersion.FindStringSubmatch(asset.Name)
	if len(matches) != 2 {
		return "", errors.New("invalid Codex Windows MSIX filename")
	}
	version := matches[1]
	packageURL := buildCodexWindowsImmutableURL(manifestVersion, asset)
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

func buildCodexWindowsImmutableURL(manifestVersion string, asset service.CachedDownloadAsset) string {
	return fmt.Sprintf(
		"https://laoshirenai.com/downloads/codex/windows-x64/%s/%s/%s",
		sanitizePublicDownloadPathSegment(manifestVersion),
		strings.ToLower(strings.TrimSpace(asset.SHA256)),
		asset.ID,
	)
}

func handlePublicDownloadError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrDownloadManifestNotReady) {
		response.Error(c, http.StatusServiceUnavailable, "安装包正在同步，请稍后再试")
		return
	}
	if errors.Is(err, service.ErrDownloadAssetNotFound) || errors.Is(err, service.ErrDownloadToolNotFound) {
		response.NotFound(c, "安装包不存在")
		return
	}
	response.InternalError(c, "读取安装包失败")
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

func (h *ResourceHandler) CCSwitchLatestManifest(c *gin.Context) {
	manifest, err := h.downloads.ListCCSwitch(c.Request.Context())
	if err != nil {
		handleCCSwitchPublicDownloadError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, buildPublicDownloadManifest(manifest, "cc-switch"))
}

func (h *ResourceHandler) DownloadCCSwitchPackage(c *gin.Context) {
	file, err := h.downloads.GetCCSwitchAsset(c.Request.Context(), c.Param("assetID"))
	if err != nil {
		handleCCSwitchPublicDownloadError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.FileAttachment(file.Path, file.Asset.Name)
}

func handleCCSwitchPublicDownloadError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrDownloadManifestNotReady) {
		response.Error(c, http.StatusServiceUnavailable, "CC Switch 安装包正在同步，请稍后再试")
		return
	}
	if errors.Is(err, service.ErrDownloadToolNotFound) || errors.Is(err, service.ErrDownloadAssetNotFound) {
		response.NotFound(c, "CC Switch 安装包不存在")
		return
	}
	response.InternalError(c, "读取 CC Switch 安装包失败")
}
