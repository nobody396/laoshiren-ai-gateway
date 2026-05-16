package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RBACHandler 处理 RBAC 相关的 HTTP 请求.
//
// 扁平化后 RBAC 资源分为两类:
//   - Menu: 前端菜单/路由（一级扁平列表，type 固定为 menu）
//   - API (后端接口): 由启动时路由扫描 + 管理员手动补充
//
// 角色可独立绑定菜单集合与 API 集合.
type RBACHandler struct {
	rbacService *service.RBACService
}

// NewRBACHandler creates a new RBACHandler.
func NewRBACHandler(rbacService *service.RBACService) *RBACHandler {
	return &RBACHandler{rbacService: rbacService}
}

// getCurrentUserID extracts the current user ID from the gin context.
func getCurrentUserID(c *gin.Context) (int64, bool) {
	user, exists := c.Get(string(middleware.ContextKeyUser))
	if !exists {
		return 0, false
	}
	authSubject, ok := user.(middleware.AuthSubject)
	if !ok {
		return 0, false
	}
	return authSubject.UserID, true
}

// ---------------------------------------------------------------------------
// 当前用户: 菜单 & 权限
// ---------------------------------------------------------------------------

// GetCurrentUserMenu 返回当前用户可见的菜单树.
// GET /admin/rbac/menu
func (h *RBACHandler) GetCurrentUserMenu(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	tree, err := h.rbacService.GetUserMenuTree(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tree)
}

// GetCurrentUserPermissions 返回当前用户的 permission_key 集合 (menu key ∪ api key).
// GET /admin/rbac/me/permissions
func (h *RBACHandler) GetCurrentUserPermissions(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	keys, err := h.rbacService.GetUserPermissionKeys(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, keys)
}

// ---------------------------------------------------------------------------
// 菜单管理
// ---------------------------------------------------------------------------

// ListMenus 扁平列出所有菜单.
// GET /admin/rbac/menus
func (h *RBACHandler) ListMenus(c *gin.Context) {
	list, err := h.rbacService.ListMenus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}

// GetMenuTree 返回完整菜单树.
// GET /admin/rbac/menus/tree
func (h *RBACHandler) GetMenuTree(c *gin.Context) {
	tree, err := h.rbacService.GetMenuTree(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tree)
}

// CreateMenuRequest 创建菜单的请求体。
// 扁平化后 type 固定为 menu，前端不再传递。
type CreateMenuRequest struct {
	Name          string `json:"name" binding:"required"`
	NameEn        string `json:"name_en"`
	Path          string `json:"path"`
	Component     string `json:"component"`
	Icon          string `json:"icon"`
	PermissionKey string `json:"permission_key" binding:"required"`
	SortOrder     int    `json:"sort_order"`
}

// CreateMenu 创建菜单.
// POST /admin/rbac/menus
func (h *RBACHandler) CreateMenu(c *gin.Context) {
	var req CreateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	m := &service.AdminMenu{
		Name:          req.Name,
		NameEn:        req.NameEn,
		Type:          service.MenuTypeMenu,
		Path:          req.Path,
		Component:     req.Component,
		Icon:          req.Icon,
		PermissionKey: req.PermissionKey,
		SortOrder:     req.SortOrder,
	}
	if err := h.rbacService.CreateMenu(c.Request.Context(), m); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, response.Response{Code: 0, Message: "success", Data: m})
}

// UpdateMenuRequest 更新菜单的请求体, 字段均为可选。
type UpdateMenuRequest struct {
	Name          *string `json:"name"`
	NameEn        *string `json:"name_en"`
	Path          *string `json:"path"`
	Component     *string `json:"component"`
	Icon          *string `json:"icon"`
	PermissionKey *string `json:"permission_key"`
	SortOrder     *int    `json:"sort_order"`
	Status        *string `json:"status"`
}

// UpdateMenu 更新菜单.
// PUT /admin/rbac/menus/:id
func (h *RBACHandler) UpdateMenu(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid menu ID")
		return
	}
	var req UpdateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	// 先读取现有菜单作为基准，再用 request 字段 patch，
	// 避免未传字段被零值覆盖触发 ent NotEmpty 校验失败。
	m, err := h.rbacService.GetMenu(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	m.Type = service.MenuTypeMenu
	if req.Name != nil {
		m.Name = *req.Name
	}
	if req.NameEn != nil {
		m.NameEn = *req.NameEn
	}
	if req.Path != nil {
		m.Path = *req.Path
	}
	if req.Component != nil {
		m.Component = *req.Component
	}
	if req.Icon != nil {
		m.Icon = *req.Icon
	}
	if req.PermissionKey != nil {
		m.PermissionKey = *req.PermissionKey
	}
	if req.SortOrder != nil {
		m.SortOrder = *req.SortOrder
	}
	if req.Status != nil {
		m.Status = *req.Status
	}
	if err := h.rbacService.UpdateMenu(c.Request.Context(), m); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, m)
}

// DeleteMenu 删除菜单.
// DELETE /admin/rbac/menus/:id
func (h *RBACHandler) DeleteMenu(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid menu ID")
		return
	}
	if err := h.rbacService.DeleteMenu(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// API 管理
// ---------------------------------------------------------------------------

// ListAPIs 分页列出 API, 支持 group/method/keyword 过滤.
// GET /admin/rbac/apis?group=&method=&keyword=&page=&page_size=
func (h *RBACHandler) ListAPIs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	filter := service.APIListFilter{
		Group:   strings.TrimSpace(c.Query("group")),
		Method:  strings.ToUpper(strings.TrimSpace(c.Query("method"))),
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Offset:  (page - 1) * pageSize,
		Limit:   pageSize,
	}
	list, total, err := h.rbacService.ListAPIs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListAllAPIs 返回全量 API, 供角色分配界面按 group 分组展示.
// GET /admin/rbac/apis/all
func (h *RBACHandler) ListAllAPIs(c *gin.Context) {
	list, err := h.rbacService.ListAllAPIs(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}

// GetAPIGroups 返回所有 API 的 group 去重列表, 供前端筛选下拉.
// GET /admin/rbac/apis/groups
func (h *RBACHandler) GetAPIGroups(c *gin.Context) {
	groups, err := h.rbacService.GetAPIGroups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, groups)
}

// CreateAPIRequest 创建 API 的请求体.
type CreateAPIRequest struct {
	Group       string `json:"group"`
	Path        string `json:"path" binding:"required"`
	Method      string `json:"method" binding:"required"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

// CreateAPI 手动创建 API.
// POST /admin/rbac/apis
func (h *RBACHandler) CreateAPI(c *gin.Context) {
	var req CreateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	a := &service.AdminAPI{
		Group:       req.Group,
		Path:        req.Path,
		Method:      req.Method,
		Description: req.Description,
		SortOrder:   req.SortOrder,
	}
	if err := h.rbacService.CreateAPI(c.Request.Context(), a); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, response.Response{Code: 0, Message: "success", Data: a})
}

// UpdateAPIRequest 更新 API 的请求体, 字段均为可选.
type UpdateAPIRequest struct {
	Group       *string `json:"group"`
	Path        *string `json:"path"`
	Method      *string `json:"method"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
	Status      *string `json:"status"`
}

// UpdateAPI 更新 API.
// PUT /admin/rbac/apis/:id
func (h *RBACHandler) UpdateAPI(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API ID")
		return
	}
	var req UpdateAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	a := &service.AdminAPI{ID: id}
	if req.Group != nil {
		a.Group = *req.Group
	}
	if req.Path != nil {
		a.Path = *req.Path
	}
	if req.Method != nil {
		a.Method = *req.Method
	}
	if req.Description != nil {
		a.Description = *req.Description
	}
	if req.SortOrder != nil {
		a.SortOrder = *req.SortOrder
	}
	if req.Status != nil {
		a.Status = *req.Status
	}
	if err := h.rbacService.UpdateAPI(c.Request.Context(), a); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, a)
}

// DeleteAPI 删除单个 API.
// DELETE /admin/rbac/apis/:id
func (h *RBACHandler) DeleteAPI(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API ID")
		return
	}
	if err := h.rbacService.DeleteAPI(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteAPIsRequest 批量删除 API 的请求体.
type DeleteAPIsRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

// DeleteAPIs 批量删除 API.
// DELETE /admin/rbac/apis
func (h *RBACHandler) DeleteAPIs(c *gin.Context) {
	var req DeleteAPIsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.rbacService.DeleteAPIs(c.Request.Context(), req.IDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// SyncAPIs 从当前 Gin 路由表重新扫描同步 API 资源.
//
// 注意: 本端点复用启动时的路由扫描结果 (由 server 层通过 SetRouteSnapshot 注入),
// 管理员可手动触发以在不重启的情况下同步新加的路由.
//
// Query 参数:
//
//	prune=true|1  同步完成后额外删除 DB 中本次快照未出现的孤儿 API 记录
//	              (admin_role_apis 会随外键 CASCADE 自动清理). 默认 false.
//
// POST /admin/rbac/apis/sync[?prune=true]
func (h *RBACHandler) SyncAPIs(c *gin.Context) {
	snapshot := getRouteSnapshot()
	prune := parseBoolQuery(c, "prune")
	if len(snapshot) == 0 {
		// 快照为空: 不论是否开启 prune 都不动表 (与 service 层空输入保护策略一致).
		response.Success(c, gin.H{"synced": 0, "note": "no route snapshot available"})
		return
	}
	res, err := h.rbacService.SyncAPIsWithOptions(c.Request.Context(), snapshot, service.SyncAPIsOptions{Prune: prune})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"synced":     res.Synced,
		"created":    res.Created,
		"updated":    res.Updated,
		"pruned":     res.Pruned,
		"pruned_ids": res.PrunedIDs,
		"prune":      prune,
	})
}

// parseBoolQuery 解析常见的 truthy query 值 ("true"/"1"/"yes", 大小写不敏感).
func parseBoolQuery(c *gin.Context, key string) bool {
	switch strings.ToLower(strings.TrimSpace(c.Query(key))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// 角色管理
// ---------------------------------------------------------------------------

// ListRoles 返回所有角色.
// GET /admin/rbac/roles
func (h *RBACHandler) ListRoles(c *gin.Context) {
	list, err := h.rbacService.ListRoles(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}

// CreateRoleRequest 创建角色的请求体.
type CreateRoleRequest struct {
	Name         string `json:"name" binding:"required"`
	Description  string `json:"description"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

// CreateRole 创建角色.
// POST /admin/rbac/roles
func (h *RBACHandler) CreateRole(c *gin.Context) {
	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	role := &service.AdminRole{
		Name:         req.Name,
		Description:  req.Description,
		IsSuperAdmin: req.IsSuperAdmin,
	}
	if err := h.rbacService.CreateRole(c.Request.Context(), role); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusCreated, response.Response{Code: 0, Message: "success", Data: role})
}

// UpdateRoleRequest 更新角色的请求体.
type UpdateRoleRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	IsSuperAdmin *bool   `json:"is_super_admin"`
}

// UpdateRole 更新角色.
// PUT /admin/rbac/roles/:id
func (h *RBACHandler) UpdateRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid role ID")
		return
	}
	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	role, err := h.rbacService.GetRole(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = *req.Description
	}
	if req.IsSuperAdmin != nil {
		role.IsSuperAdmin = *req.IsSuperAdmin
	}
	if err := h.rbacService.UpdateRole(c.Request.Context(), role); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, role)
}

// DeleteRole 删除角色.
// DELETE /admin/rbac/roles/:id
func (h *RBACHandler) DeleteRole(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid role ID")
		return
	}
	if err := h.rbacService.DeleteRole(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// 角色-菜单 / 角色-API 关联
// ---------------------------------------------------------------------------

// GetRoleMenus 返回角色当前绑定的菜单 ID 列表.
// GET /admin/rbac/roles/:id/menus
func (h *RBACHandler) GetRoleMenus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid role ID")
		return
	}
	ids, err := h.rbacService.GetRoleMenuIDs(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ids)
}

// SetRoleMenusRequest 设置角色菜单的请求体.
type SetRoleMenusRequest struct {
	MenuIDs []int64 `json:"menu_ids" binding:"required"`
}

// SetRoleMenus 全量替换角色菜单.
// PUT /admin/rbac/roles/:id/menus
func (h *RBACHandler) SetRoleMenus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid role ID")
		return
	}
	var req SetRoleMenusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.rbacService.AssignRoleMenus(c.Request.Context(), id, req.MenuIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// GetRoleAPIs 返回角色当前绑定的 API ID 列表.
// GET /admin/rbac/roles/:id/apis
func (h *RBACHandler) GetRoleAPIs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid role ID")
		return
	}
	ids, err := h.rbacService.GetRoleAPIIDs(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ids)
}

// SetRoleAPIsRequest 设置角色 API 的请求体.
type SetRoleAPIsRequest struct {
	APIIDs []int64 `json:"api_ids" binding:"required"`
}

// SetRoleAPIs 全量替换角色 API.
// PUT /admin/rbac/roles/:id/apis
func (h *RBACHandler) SetRoleAPIs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid role ID")
		return
	}
	var req SetRoleAPIsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.rbacService.AssignRoleAPIs(c.Request.Context(), id, req.APIIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// 用户-角色 关联
// ---------------------------------------------------------------------------

// GetUserRoles 返回用户绑定的角色.
// GET /admin/rbac/users/:id/roles
func (h *RBACHandler) GetUserRoles(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	list, err := h.rbacService.GetUserRoles(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}

// SetUserRolesRequest 设置用户角色的请求体.
type SetUserRolesRequest struct {
	RoleIDs []int64 `json:"role_ids" binding:"required"`
}

// SetUserRoles 全量替换用户角色.
// PUT /admin/rbac/users/:id/roles
func (h *RBACHandler) SetUserRoles(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	var req SetUserRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.rbacService.AssignUserRoles(c.Request.Context(), id, req.RoleIDs); err != nil {
		if errors.Is(err, service.ErrUserNotAdmin) {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// ---------------------------------------------------------------------------
// 路由快照: 供 SyncAPIs handler 使用
// ---------------------------------------------------------------------------

// routeSnapshot 由 server 层在启动路由注册完成后通过 SetRouteSnapshot 注入.
// 管理员手动触发 /admin/rbac/apis/sync 时复用此快照.
//
// 注意: 未使用互斥锁, 因为只在启动阶段赋值一次, 之后仅读取.
var routeSnapshot []*service.AdminAPI

// SetRouteSnapshot 由 server 层调用, 注入路由快照供 manual sync 使用.
func SetRouteSnapshot(items []*service.AdminAPI) {
	routeSnapshot = items
}

func getRouteSnapshot() []*service.AdminAPI {
	return routeSnapshot
}
