package service

import (
	"context"
	"time"
)

// ---------------------------------------------------------------------------
// 菜单 / API 资源类型与状态常量
// ---------------------------------------------------------------------------

const (
	ResourceStatusActive   = "active"
	ResourceStatusInactive = "inactive"

	// MenuTypeMenu 为当前 admin_menus 唯一支持的类型，保留常量以兼容旧调用点。
	MenuTypeMenu = "menu"
)

// ---------------------------------------------------------------------------
// 领域模型
// ---------------------------------------------------------------------------

// AdminRole 角色.
type AdminRole struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	IsSuperAdmin bool      `json:"is_super_admin"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AdminMenu 菜单资源（扁平一级，type 固定为 menu）。
type AdminMenu struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	NameEn        string    `json:"name_en"`
	Type          string    `json:"type"`
	Path          string    `json:"path"`
	Component     string    `json:"component"`
	Icon          string    `json:"icon"`
	PermissionKey string    `json:"permission_key"`
	SortOrder     int       `json:"sort_order"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AdminAPI 后端 API 资源.
type AdminAPI struct {
	ID          int64     `json:"id"`
	Group       string    `json:"group"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// APIListFilter API 列表筛选条件.
type APIListFilter struct {
	Group   string
	Method  string
	Keyword string // 按 path / description 模糊
	Offset  int
	Limit   int
}

// ---------------------------------------------------------------------------
// Repository 接口
// ---------------------------------------------------------------------------

// RBACRepository 提供 RBAC 数据库操作.
type RBACRepository interface {
	// ---- 角色 ----
	GetAllRoles(ctx context.Context) ([]*AdminRole, error)
	CreateRole(ctx context.Context, role *AdminRole) error
	UpdateRole(ctx context.Context, role *AdminRole) error
	DeleteRole(ctx context.Context, id int64) error
	GetRolesByUserID(ctx context.Context, userID int64) ([]*AdminRole, error)

	// ---- 菜单 ----
	ListMenus(ctx context.Context) ([]*AdminMenu, error)
	GetMenu(ctx context.Context, id int64) (*AdminMenu, error)
	CreateMenu(ctx context.Context, m *AdminMenu) error
	UpdateMenu(ctx context.Context, m *AdminMenu) error
	DeleteMenu(ctx context.Context, id int64) error
	GetMenusByRoleIDs(ctx context.Context, roleIDs []int64) ([]*AdminMenu, error)

	// ---- API ----
	ListAPIs(ctx context.Context, filter APIListFilter) ([]*AdminAPI, int, error)
	ListAllAPIs(ctx context.Context) ([]*AdminAPI, error)
	GetAPI(ctx context.Context, id int64) (*AdminAPI, error)
	CreateAPI(ctx context.Context, a *AdminAPI) error
	UpdateAPI(ctx context.Context, a *AdminAPI) error
	DeleteAPI(ctx context.Context, id int64) error
	DeleteAPIsByIDs(ctx context.Context, ids []int64) error
	UpsertAPIs(ctx context.Context, items []*AdminAPI) error
	GetAPIsByRoleIDs(ctx context.Context, roleIDs []int64) ([]*AdminAPI, error)
	GetAPIGroups(ctx context.Context) ([]string, error)

	// ---- 角色-菜单 ----
	AssignRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error
	GetRoleMenuIDs(ctx context.Context, roleID int64) ([]int64, error)

	// ---- 角色-API ----
	AssignRoleAPIs(ctx context.Context, roleID int64, apiIDs []int64) error
	GetRoleAPIIDs(ctx context.Context, roleID int64) ([]int64, error)

	// ---- 用户-角色 ----
	AssignUserRoles(ctx context.Context, userID int64, roleIDs []int64) error
	GetUserRoles(ctx context.Context, userID int64) ([]*AdminRole, error)
	// GetUserRole 查询 users.role 基础身份 (admin/user/agent)，用户不存在时返回 ("", nil).
	GetUserRole(ctx context.Context, userID int64) (string, error)
}

// ---------------------------------------------------------------------------
// Cache 接口
// ---------------------------------------------------------------------------

// RBACCache 提供 RBAC 数据的 Redis 缓存操作.
type RBACCache interface {
	GetUserPermissionKeys(ctx context.Context, userID int64) ([]string, error)
	SetUserPermissionKeys(ctx context.Context, userID int64, keys []string) error
	InvalidateUserPermissions(ctx context.Context, userID int64) error
	InvalidateAllPermissions(ctx context.Context) error
	GetUserMenuTree(ctx context.Context, userID int64) ([]byte, error)
	SetUserMenuTree(ctx context.Context, userID int64, data []byte) error
}
