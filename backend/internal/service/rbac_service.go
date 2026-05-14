package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// RBAC 错误定义
var (
	ErrRoleNotFound           = errors.New("role not found")
	ErrMenuNotFound           = errors.New("menu not found")
	ErrAPINotFound            = errors.New("api not found")
	ErrCannotDeleteSuperAdmin = errors.New("cannot delete super admin role")
	ErrRoleNameRequired       = errors.New("role name is required")
	ErrMenuNameRequired       = errors.New("menu name is required")
	ErrMenuKeyRequired        = errors.New("menu permission_key is required")
	ErrInvalidMenuType        = errors.New("invalid menu type, must be 'menu'")
	ErrAPIPathRequired        = errors.New("api path is required")
	ErrAPIMethodRequired      = errors.New("api method is required")
	// ErrUserNotAdmin 目标用户不是后台管理员 (users.role != admin)，不允许分配 RBAC 角色.
	ErrUserNotAdmin = errors.New("rbac roles can only be assigned to admin users")
)

// MenuTreeNode 菜单节点（20260505 扁平化后不再存在父子层级）。
// 保留类型名以最小化调用端改动，Children / ParentID 字段已移除。
type MenuTreeNode struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	NameEn        string `json:"name_en,omitempty"`
	Type          string `json:"type"`
	Path          string `json:"path,omitempty"`
	Component     string `json:"component,omitempty"`
	Icon          string `json:"icon,omitempty"`
	PermissionKey string `json:"permission_key,omitempty"`
	SortOrder     int    `json:"sort_order"`
}

// validMenuTypes 合法的菜单类型 (扁平化后仅保留 menu).
var validMenuTypes = map[string]bool{
	MenuTypeMenu: true,
}

// RBACService RBAC 业务逻辑层
type RBACService struct {
	repo  RBACRepository
	cache RBACCache

	// mu 保护并发写缓存
	mu sync.Mutex
}

// NewRBACService 创建 RBAC 服务实例
func NewRBACService(repo RBACRepository, cache RBACCache) *RBACService {
	return &RBACService{
		repo:  repo,
		cache: cache,
	}
}

// ---------------------------------------------------------------------------
// 菜单树 & 权限查询
// ---------------------------------------------------------------------------

// GetUserMenuTree 获取用户的动态菜单列表 (用于侧边栏渲染).
//
// 20260505 扁平化后 admin_menus 不再存在父子层级与多种类型，因此直接
// 按 sort_order 返回整体授权列表，函数名保留原称以减少调用点变更。
func (s *RBACService) GetUserMenuTree(ctx context.Context, userID int64) ([]MenuTreeNode, error) {
	// 1. 尝试从缓存读取
	cached, err := s.cache.GetUserMenuTree(ctx, userID)
	if err == nil && cached != nil {
		var tree []MenuTreeNode
		if jsonErr := json.Unmarshal(cached, &tree); jsonErr == nil {
			return tree, nil
		}
	}

	// 2. 获取用户角色
	roles, err := s.repo.GetRolesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}

	isSuperAdmin := false
	for _, r := range roles {
		if r.IsSuperAdmin {
			isSuperAdmin = true
			break
		}
	}

	var menus []*AdminMenu
	if isSuperAdmin {
		menus, err = s.repo.ListMenus(ctx)
		if err != nil {
			return nil, fmt.Errorf("list menus: %w", err)
		}
	} else {
		roleIDs := make([]int64, 0, len(roles))
		for _, r := range roles {
			roleIDs = append(roleIDs, r.ID)
		}
		menus, err = s.repo.GetMenusByRoleIDs(ctx, roleIDs)
		if err != nil {
			return nil, fmt.Errorf("get role menus: %w", err)
		}
	}

	tree := buildMenuList(menus)

	// 3. 写入缓存
	if data, jsonErr := json.Marshal(tree); jsonErr == nil {
		_ = s.cache.SetUserMenuTree(ctx, userID, data)
	}

	return tree, nil
}

// GetUserPermissionKeys 获取用户的 permission_key 集合.
// 返回值为菜单.permission_key 与 API ("api:<METHOD>:<path>") 的并集;
// 超级管理员返回 ["*"].
func (s *RBACService) GetUserPermissionKeys(ctx context.Context, userID int64) ([]string, error) {
	cached, err := s.cache.GetUserPermissionKeys(ctx, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	roles, err := s.repo.GetRolesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user roles: %w", err)
	}

	for _, r := range roles {
		if r.IsSuperAdmin {
			keys := []string{"*"}
			_ = s.cache.SetUserPermissionKeys(ctx, userID, keys)
			return keys, nil
		}
	}

	roleIDs := make([]int64, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.ID)
	}

	menus, err := s.repo.GetMenusByRoleIDs(ctx, roleIDs)
	if err != nil {
		return nil, fmt.Errorf("get role menus: %w", err)
	}
	apis, err := s.repo.GetAPIsByRoleIDs(ctx, roleIDs)
	if err != nil {
		return nil, fmt.Errorf("get role apis: %w", err)
	}

	keys := make([]string, 0, len(menus)+len(apis))
	seen := make(map[string]struct{}, len(menus)+len(apis))
	add := func(k string) {
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for _, m := range menus {
		add(m.PermissionKey)
	}
	for _, a := range apis {
		add(APIPermissionKey(a.Method, a.Path))
	}

	if keys == nil {
		keys = []string{}
	}

	_ = s.cache.SetUserPermissionKeys(ctx, userID, keys)
	return keys, nil
}

// APIPermissionKey 根据 method + path 组装权限 key.
func APIPermissionKey(method, path string) string {
	return fmt.Sprintf("api:%s:%s", strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(path))
}

// IsSuperAdmin 检查用户是否超级管理员.
func (s *RBACService) IsSuperAdmin(ctx context.Context, userID int64) (bool, error) {
	roles, err := s.repo.GetRolesByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, r := range roles {
		if r.IsSuperAdmin {
			return true, nil
		}
	}
	return false, nil
}

// CheckPermission 检查用户是否具备指定权限 key.
func (s *RBACService) CheckPermission(ctx context.Context, userID int64, permissionKey string) (bool, error) {
	keys, err := s.GetUserPermissionKeys(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, k := range keys {
		if k == "*" || k == permissionKey {
			return true, nil
		}
	}
	return false, nil
}

// ---------------------------------------------------------------------------
// 角色管理
// ---------------------------------------------------------------------------

// ListRoles 获取所有角色.
func (s *RBACService) ListRoles(ctx context.Context) ([]*AdminRole, error) {
	return s.repo.GetAllRoles(ctx)
}

// GetRole 获取单个角色.
func (s *RBACService) GetRole(ctx context.Context, id int64) (*AdminRole, error) {
	roles, err := s.repo.GetAllRoles(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, ErrRoleNotFound
}

// CreateRole 创建角色.
func (s *RBACService) CreateRole(ctx context.Context, role *AdminRole) error {
	if role.Name == "" {
		return ErrRoleNameRequired
	}
	if role.Status == "" {
		role.Status = ResourceStatusActive
	}
	return s.repo.CreateRole(ctx, role)
}

// UpdateRole 更新角色.
func (s *RBACService) UpdateRole(ctx context.Context, role *AdminRole) error {
	if role.Name == "" {
		return ErrRoleNameRequired
	}
	return s.repo.UpdateRole(ctx, role)
}

// DeleteRole 删除角色 (不允许删除超级管理员角色).
func (s *RBACService) DeleteRole(ctx context.Context, id int64) error {
	role, err := s.GetRole(ctx, id)
	if err != nil {
		return err
	}
	if role.IsSuperAdmin {
		return ErrCannotDeleteSuperAdmin
	}
	if err := s.repo.DeleteRole(ctx, id); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// ---------------------------------------------------------------------------
// 菜单管理
// ---------------------------------------------------------------------------

// ListMenus 扁平列出所有菜单.
func (s *RBACService) ListMenus(ctx context.Context) ([]*AdminMenu, error) {
	return s.repo.ListMenus(ctx)
}

// GetMenuTree 完整菜单列表（扁平化后不再构树，函数名保留以兼容调用点）。
func (s *RBACService) GetMenuTree(ctx context.Context) ([]MenuTreeNode, error) {
	all, err := s.repo.ListMenus(ctx)
	if err != nil {
		return nil, err
	}
	return buildMenuList(all), nil
}

// GetMenu 获取单个菜单.
func (s *RBACService) GetMenu(ctx context.Context, id int64) (*AdminMenu, error) {
	m, err := s.repo.GetMenu(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMenuNotFound
	}
	return m, nil
}

// CreateMenu 创建菜单节点.
func (s *RBACService) CreateMenu(ctx context.Context, m *AdminMenu) error {
	if m.Name == "" {
		return ErrMenuNameRequired
	}
	if m.PermissionKey == "" {
		return ErrMenuKeyRequired
	}
	if m.Type == "" {
		m.Type = MenuTypeMenu
	}
	if !validMenuTypes[m.Type] {
		return ErrInvalidMenuType
	}
	if m.Status == "" {
		m.Status = ResourceStatusActive
	}
	if err := s.repo.CreateMenu(ctx, m); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// UpdateMenu 更新菜单节点.
func (s *RBACService) UpdateMenu(ctx context.Context, m *AdminMenu) error {
	if m.PermissionKey == "" {
		return ErrMenuKeyRequired
	}
	if m.Type != "" && !validMenuTypes[m.Type] {
		return ErrInvalidMenuType
	}
	if err := s.repo.UpdateMenu(ctx, m); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// DeleteMenu 删除菜单节点.
func (s *RBACService) DeleteMenu(ctx context.Context, id int64) error {
	if err := s.repo.DeleteMenu(ctx, id); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// ---------------------------------------------------------------------------
// API 资源管理
// ---------------------------------------------------------------------------

// ListAPIs 分页+筛选列出 API.
func (s *RBACService) ListAPIs(ctx context.Context, filter APIListFilter) ([]*AdminAPI, int, error) {
	return s.repo.ListAPIs(ctx, filter)
}

// ListAllAPIs 全量列出 API (常用于角色分配界面分组展示).
func (s *RBACService) ListAllAPIs(ctx context.Context) ([]*AdminAPI, error) {
	return s.repo.ListAllAPIs(ctx)
}

// GetAPIGroups 所有 API 的 group 去重集合.
func (s *RBACService) GetAPIGroups(ctx context.Context) ([]string, error) {
	return s.repo.GetAPIGroups(ctx)
}

// GetAPI 获取单个 API.
func (s *RBACService) GetAPI(ctx context.Context, id int64) (*AdminAPI, error) {
	a, err := s.repo.GetAPI(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrAPINotFound
	}
	return a, nil
}

// CreateAPI 创建 API.
func (s *RBACService) CreateAPI(ctx context.Context, a *AdminAPI) error {
	if a.Path == "" {
		return ErrAPIPathRequired
	}
	if a.Method == "" {
		return ErrAPIMethodRequired
	}
	a.Method = strings.ToUpper(a.Method)
	if a.Status == "" {
		a.Status = ResourceStatusActive
	}
	if err := s.repo.CreateAPI(ctx, a); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// UpdateAPI 更新 API.
func (s *RBACService) UpdateAPI(ctx context.Context, a *AdminAPI) error {
	if a.Path == "" {
		return ErrAPIPathRequired
	}
	if a.Method == "" {
		return ErrAPIMethodRequired
	}
	a.Method = strings.ToUpper(a.Method)
	if err := s.repo.UpdateAPI(ctx, a); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// DeleteAPI 删除单个 API.
func (s *RBACService) DeleteAPI(ctx context.Context, id int64) error {
	if err := s.repo.DeleteAPI(ctx, id); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// DeleteAPIs 批量删除 API.
func (s *RBACService) DeleteAPIs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.repo.DeleteAPIsByIDs(ctx, ids); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// SyncAPIsOptions 控制 SyncAPIs 的行为开关.
type SyncAPIsOptions struct {
	// Prune 为 true 时, 将删除 DB 里存在但本次 items 未出现的记录 (按 method+path 做差集).
	// 关联的 admin_role_apis 会由外键 ON DELETE CASCADE 自动清理.
	// 默认 false: 保守策略, 保护管理员手工新增的 API 条目.
	Prune bool
}

// SyncAPIsResult 汇报 SyncAPIs 一次同步的结构化结果.
type SyncAPIsResult struct {
	// Synced 本次传入的 items 总数 (去重前, 与路由扫描值一致).
	Synced int `json:"synced"`
	// Created 本次新增的 API 条数.
	Created int `json:"created"`
	// Updated 本次被覆盖 group/description 的条数 (key 已存在).
	Updated int `json:"updated"`
	// Pruned 本次被差集删除的孤儿条数 (仅 Prune=true 时可能非零).
	Pruned int `json:"pruned"`
	// PrunedIDs 被删除的孤儿 ID 列表, 升序排列, 供审计/回放/测试断言.
	PrunedIDs []int64 `json:"pruned_ids,omitempty"`
}

// SyncAPIs 用路由扫描结果同步 API 资源表 (纯 upsert, 不 prune).
// 兼容旧调用方; 如需结构化结果或孤儿清理, 请使用 SyncAPIsWithOptions.
func (s *RBACService) SyncAPIs(ctx context.Context, items []*AdminAPI) error {
	_, err := s.SyncAPIsWithOptions(ctx, items, SyncAPIsOptions{})
	return err
}

// SyncAPIsWithOptions 幂等同步 API 资源表并返回结构化结果, 可选开启 prune.
//
// 语义:
//  1. (method, path) 已存在 → 仅在新值非空时覆盖 group/description (保留管理员手动编辑的 sort_order 等).
//  2. (method, path) 不存在 → 新增一条.
//  3. opts.Prune=true → 额外删除 DB 里存在但本次 items 未出现的记录;
//     admin_role_apis 会随外键 ON DELETE CASCADE 自动清理, 已绑定的角色会失去对应授权.
//
// 注意: 当 items 为空且 Prune=true 时视为空操作, 避免一次配置错误清空整个 admin_apis 表.
func (s *RBACService) SyncAPIsWithOptions(ctx context.Context, items []*AdminAPI, opts SyncAPIsOptions) (*SyncAPIsResult, error) {
	res := &SyncAPIsResult{Synced: len(items)}
	if len(items) == 0 {
		// 保护性短路: items 为空时即使 Prune=true 也不动表, 防止扫描器异常导致全表清空.
		return res, nil
	}
	for _, a := range items {
		a.Method = strings.ToUpper(strings.TrimSpace(a.Method))
		if a.Status == "" {
			a.Status = ResourceStatusActive
		}
	}

	// 读取 DB 全量快照, 用于 diff 统计 (created/updated) 并在 Prune=true 时算差集.
	existing, err := s.repo.ListAllAPIs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list existing apis: %w", err)
	}
	type apiKey struct{ Method, Path string }
	existingByKey := make(map[apiKey]*AdminAPI, len(existing))
	for _, e := range existing {
		existingByKey[apiKey{strings.ToUpper(e.Method), e.Path}] = e
	}
	incomingKeys := make(map[apiKey]struct{}, len(items))
	for _, a := range items {
		k := apiKey{a.Method, a.Path}
		if _, dup := incomingKeys[k]; dup {
			continue
		}
		incomingKeys[k] = struct{}{}
		if _, ok := existingByKey[k]; ok {
			res.Updated++
		} else {
			res.Created++
		}
	}

	if err := s.repo.UpsertAPIs(ctx, items); err != nil {
		return nil, err
	}

	if opts.Prune {
		orphanIDs := make([]int64, 0)
		for k, e := range existingByKey {
			if _, ok := incomingKeys[k]; !ok {
				orphanIDs = append(orphanIDs, e.ID)
			}
		}
		if len(orphanIDs) > 0 {
			sort.Slice(orphanIDs, func(i, j int) bool { return orphanIDs[i] < orphanIDs[j] })
			if err := s.repo.DeleteAPIsByIDs(ctx, orphanIDs); err != nil {
				return nil, fmt.Errorf("prune orphan apis: %w", err)
			}
			res.Pruned = len(orphanIDs)
			res.PrunedIDs = orphanIDs
		}
	}

	_ = s.cache.InvalidateAllPermissions(ctx)
	return res, nil
}

// ---------------------------------------------------------------------------
// 角色 - 菜单 / API 关联
// ---------------------------------------------------------------------------

// GetRoleMenuIDs 获取角色当前绑定的菜单 ID 列表.
func (s *RBACService) GetRoleMenuIDs(ctx context.Context, roleID int64) ([]int64, error) {
	return s.repo.GetRoleMenuIDs(ctx, roleID)
}

// AssignRoleMenus 全量替换角色的菜单绑定.
func (s *RBACService) AssignRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	if err := s.repo.AssignRoleMenus(ctx, roleID, menuIDs); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// GetRoleAPIIDs 获取角色当前绑定的 API ID 列表.
func (s *RBACService) GetRoleAPIIDs(ctx context.Context, roleID int64) ([]int64, error) {
	return s.repo.GetRoleAPIIDs(ctx, roleID)
}

// AssignRoleAPIs 全量替换角色的 API 绑定.
func (s *RBACService) AssignRoleAPIs(ctx context.Context, roleID int64, apiIDs []int64) error {
	if err := s.repo.AssignRoleAPIs(ctx, roleID, apiIDs); err != nil {
		return err
	}
	_ = s.cache.InvalidateAllPermissions(ctx)
	return nil
}

// ---------------------------------------------------------------------------
// 用户 - 角色关联
// ---------------------------------------------------------------------------

// AssignUserRoles 全量替换用户的角色绑定.
//
// 业务约束: 在 roleIDs 非空时, 目标用户必须是后台管理员 (users.role == admin).
// roleIDs 为空 (清空绑定) 时不做此约束, 允许用于「降级前清理」流程.
func (s *RBACService) AssignUserRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	if len(roleIDs) > 0 {
		role, err := s.repo.GetUserRole(ctx, userID)
		if err != nil {
			return err
		}
		if role != RoleAdmin {
			return ErrUserNotAdmin
		}
	}
	if err := s.repo.AssignUserRoles(ctx, userID, roleIDs); err != nil {
		return err
	}
	_ = s.cache.InvalidateUserPermissions(ctx, userID)
	return nil
}

// GetUserRoles 获取用户角色.
func (s *RBACService) GetUserRoles(ctx context.Context, userID int64) ([]*AdminRole, error) {
	return s.repo.GetUserRoles(ctx, userID)
}

// ---------------------------------------------------------------------------
// 扁平列表构建辅助函数
// ---------------------------------------------------------------------------

// buildMenuList 将 AdminMenu 转换为 MenuTreeNode 并按 sort_order 升序返回.
//
// 20260505 扁平化后菜单不再有父子层级，本函数仅执行 DTO 转换 + 排序。
func buildMenuList(menus []*AdminMenu) []MenuTreeNode {
	result := make([]MenuTreeNode, 0, len(menus))
	for _, m := range menus {
		result = append(result, MenuTreeNode{
			ID:            m.ID,
			Name:          m.Name,
			NameEn:        m.NameEn,
			Type:          m.Type,
			Path:          m.Path,
			Component:     m.Component,
			Icon:          m.Icon,
			PermissionKey: m.PermissionKey,
			SortOrder:     m.SortOrder,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SortOrder < result[j].SortOrder
	})
	return result
}
