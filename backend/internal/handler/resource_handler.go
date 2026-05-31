package handler

import (
	"errors"
	"net/http"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

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
