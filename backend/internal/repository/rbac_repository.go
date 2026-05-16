package repository

import (
	"context"
	"fmt"
	"strings"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/adminapi"
	"github.com/bozhouDev/DragonCode-sub2api/ent/adminmenu"
	"github.com/bozhouDev/DragonCode-sub2api/ent/adminrole"
	"github.com/bozhouDev/DragonCode-sub2api/ent/adminroleapi"
	"github.com/bozhouDev/DragonCode-sub2api/ent/adminrolemenu"
	"github.com/bozhouDev/DragonCode-sub2api/ent/adminuserrole"
	dbuser "github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// rbacRepository 实现 service.RBACRepository 接口.
type rbacRepository struct {
	client *dbent.Client
}

// NewRBACRepository 创建 RBAC 仓库实例.
func NewRBACRepository(client *dbent.Client) service.RBACRepository {
	return &rbacRepository{client: client}
}

// ---------------------------------------------------------------------------
// 角色
// ---------------------------------------------------------------------------

func (r *rbacRepository) GetAllRoles(ctx context.Context) ([]*service.AdminRole, error) {
	client := clientFromContext(ctx, r.client)
	items, err := client.AdminRole.Query().
		Order(dbent.Asc(adminrole.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return adminRoleEntitiesToService(items), nil
}

func (r *rbacRepository) CreateRole(ctx context.Context, role *service.AdminRole) error {
	client := clientFromContext(ctx, r.client)
	builder := client.AdminRole.Create().
		SetName(role.Name).
		SetDescription(role.Description).
		SetIsSuperAdmin(role.IsSuperAdmin)
	if role.Status != "" {
		builder = builder.SetStatus(role.Status)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	role.ID = created.ID
	role.CreatedAt = created.CreatedAt
	role.UpdatedAt = created.UpdatedAt
	role.Status = created.Status
	return nil
}

func (r *rbacRepository) UpdateRole(ctx context.Context, role *service.AdminRole) error {
	client := clientFromContext(ctx, r.client)
	updated, err := client.AdminRole.UpdateOneID(role.ID).
		SetName(role.Name).
		SetDescription(role.Description).
		SetIsSuperAdmin(role.IsSuperAdmin).
		SetStatus(role.Status).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	role.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *rbacRepository) DeleteRole(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.AdminRole.Delete().Where(adminrole.IDEQ(id)).Exec(ctx)
	return err
}

func (r *rbacRepository) GetRolesByUserID(ctx context.Context, userID int64) ([]*service.AdminRole, error) {
	return r.getRolesByUserID(ctx, userID, true)
}

func (r *rbacRepository) getRolesByUserID(ctx context.Context, userID int64, activeOnly bool) ([]*service.AdminRole, error) {
	client := clientFromContext(ctx, r.client)

	userRoles, err := client.AdminUserRole.Query().
		Where(adminuserrole.UserIDEQ(userID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(userRoles) == 0 {
		return nil, nil
	}

	roleIDs := make([]int64, 0, len(userRoles))
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleID)
	}

	query := client.AdminRole.Query().
		Where(adminrole.IDIn(roleIDs...))
	if activeOnly {
		query = query.Where(adminrole.StatusEQ(service.ResourceStatusActive))
	}
	roles, err := query.Order(dbent.Asc(adminrole.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return adminRoleEntitiesToService(roles), nil
}

// ---------------------------------------------------------------------------
// 菜单
// ---------------------------------------------------------------------------

func (r *rbacRepository) ListMenus(ctx context.Context) ([]*service.AdminMenu, error) {
	client := clientFromContext(ctx, r.client)
	items, err := client.AdminMenu.Query().
		Where(adminmenu.StatusEQ(service.ResourceStatusActive)).
		Order(dbent.Asc(adminmenu.FieldSortOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return adminMenuEntitiesToService(items), nil
}

func (r *rbacRepository) GetMenu(ctx context.Context, id int64) (*service.AdminMenu, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.AdminMenu.Query().Where(adminmenu.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return adminMenuEntityToService(m), nil
}

func (r *rbacRepository) CreateMenu(ctx context.Context, m *service.AdminMenu) error {
	client := clientFromContext(ctx, r.client)
	builder := client.AdminMenu.Create().
		SetName(m.Name).
		SetNameEn(m.NameEn).
		SetType(m.Type).
		SetPath(m.Path).
		SetComponent(m.Component).
		SetIcon(m.Icon).
		SetPermissionKey(m.PermissionKey).
		SetSortOrder(m.SortOrder)
	if m.Status != "" {
		builder = builder.SetStatus(m.Status)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	m.ID = created.ID
	m.CreatedAt = created.CreatedAt
	m.UpdatedAt = created.UpdatedAt
	if m.Status == "" {
		m.Status = created.Status
	}
	return nil
}

func (r *rbacRepository) UpdateMenu(ctx context.Context, m *service.AdminMenu) error {
	client := clientFromContext(ctx, r.client)
	builder := client.AdminMenu.UpdateOneID(m.ID).
		SetName(m.Name).
		SetNameEn(m.NameEn).
		SetType(m.Type).
		SetPath(m.Path).
		SetComponent(m.Component).
		SetIcon(m.Icon).
		SetPermissionKey(m.PermissionKey).
		SetSortOrder(m.SortOrder).
		SetStatus(m.Status)
	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	m.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *rbacRepository) DeleteMenu(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.AdminMenu.Delete().Where(adminmenu.IDEQ(id)).Exec(ctx)
	return err
}

func (r *rbacRepository) GetMenusByRoleIDs(ctx context.Context, roleIDs []int64) ([]*service.AdminMenu, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	client := clientFromContext(ctx, r.client)

	links, err := client.AdminRoleMenu.Query().
		Where(adminrolemenu.RoleIDIn(roleIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, nil
	}

	seen := make(map[int64]struct{}, len(links))
	ids := make([]int64, 0, len(links))
	for _, l := range links {
		if _, ok := seen[l.MenuID]; !ok {
			seen[l.MenuID] = struct{}{}
			ids = append(ids, l.MenuID)
		}
	}

	items, err := client.AdminMenu.Query().
		Where(
			adminmenu.IDIn(ids...),
			adminmenu.StatusEQ(service.ResourceStatusActive),
		).
		Order(dbent.Asc(adminmenu.FieldSortOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return adminMenuEntitiesToService(items), nil
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

func (r *rbacRepository) ListAPIs(ctx context.Context, filter service.APIListFilter) ([]*service.AdminAPI, int, error) {
	client := clientFromContext(ctx, r.client)
	q := client.AdminAPI.Query()
	if filter.Group != "" {
		q = q.Where(adminapi.GroupEQ(filter.Group))
	}
	if filter.Method != "" {
		q = q.Where(adminapi.MethodEQ(strings.ToUpper(filter.Method)))
	}
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		q = q.Where(
			adminapi.Or(
				adminapi.PathContainsFold(kw),
				adminapi.DescriptionContainsFold(kw),
			),
		)
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	q = q.Order(dbent.Asc(adminapi.FieldGroup), dbent.Asc(adminapi.FieldSortOrder), dbent.Asc(adminapi.FieldID))
	if filter.Limit > 0 {
		q = q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}

	items, err := q.All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return adminAPIEntitiesToService(items), total, nil
}

func (r *rbacRepository) ListAllAPIs(ctx context.Context) ([]*service.AdminAPI, error) {
	client := clientFromContext(ctx, r.client)
	items, err := client.AdminAPI.Query().
		Order(dbent.Asc(adminapi.FieldGroup), dbent.Asc(adminapi.FieldSortOrder), dbent.Asc(adminapi.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return adminAPIEntitiesToService(items), nil
}

func (r *rbacRepository) GetAPI(ctx context.Context, id int64) (*service.AdminAPI, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.AdminAPI.Query().Where(adminapi.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return adminAPIEntityToService(m), nil
}

func (r *rbacRepository) CreateAPI(ctx context.Context, a *service.AdminAPI) error {
	client := clientFromContext(ctx, r.client)
	builder := client.AdminAPI.Create().
		SetGroup(a.Group).
		SetPath(a.Path).
		SetMethod(strings.ToUpper(a.Method)).
		SetDescription(a.Description).
		SetSortOrder(a.SortOrder)
	if a.Status != "" {
		builder = builder.SetStatus(a.Status)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	a.ID = created.ID
	a.CreatedAt = created.CreatedAt
	a.UpdatedAt = created.UpdatedAt
	if a.Status == "" {
		a.Status = created.Status
	}
	return nil
}

func (r *rbacRepository) UpdateAPI(ctx context.Context, a *service.AdminAPI) error {
	client := clientFromContext(ctx, r.client)
	updated, err := client.AdminAPI.UpdateOneID(a.ID).
		SetGroup(a.Group).
		SetPath(a.Path).
		SetMethod(strings.ToUpper(a.Method)).
		SetDescription(a.Description).
		SetSortOrder(a.SortOrder).
		SetStatus(a.Status).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, nil)
	}
	a.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *rbacRepository) DeleteAPI(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.AdminAPI.Delete().Where(adminapi.IDEQ(id)).Exec(ctx)
	return err
}

func (r *rbacRepository) DeleteAPIsByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	_, err := client.AdminAPI.Delete().Where(adminapi.IDIn(ids...)).Exec(ctx)
	return err
}

// UpsertAPIs 按 (method, path) 幂等写入:
// 不存在则插入, 存在则只更新 group / description (避免覆盖管理员调整过的 sort_order 等).
func (r *rbacRepository) UpsertAPIs(ctx context.Context, items []*service.AdminAPI) error {
	if len(items) == 0 {
		return nil
	}
	client := clientFromContext(ctx, r.client)

	// 先查询已存在的 (method, path) 组合, 区分 insert / update 两拨
	methods := make([]string, 0, len(items))
	paths := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, a := range items {
		key := strings.ToUpper(a.Method) + " " + a.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		methods = append(methods, strings.ToUpper(a.Method))
		paths = append(paths, a.Path)
	}

	existing, err := client.AdminAPI.Query().
		Where(
			adminapi.MethodIn(methods...),
			adminapi.PathIn(paths...),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query existing apis: %w", err)
	}
	existMap := make(map[string]*dbent.AdminAPI, len(existing))
	for _, e := range existing {
		existMap[strings.ToUpper(e.Method)+" "+e.Path] = e
	}

	// 准备 create 批
	creates := make([]*dbent.AdminAPICreate, 0, len(items))
	for _, a := range items {
		key := strings.ToUpper(a.Method) + " " + a.Path
		if _, ok := existMap[key]; ok {
			continue
		}
		b := client.AdminAPI.Create().
			SetGroup(a.Group).
			SetPath(a.Path).
			SetMethod(strings.ToUpper(a.Method)).
			SetDescription(a.Description).
			SetSortOrder(a.SortOrder)
		if a.Status != "" {
			b = b.SetStatus(a.Status)
		}
		creates = append(creates, b)
	}
	if len(creates) > 0 {
		if _, err := client.AdminAPI.CreateBulk(creates...).Save(ctx); err != nil {
			return fmt.Errorf("bulk create apis: %w", err)
		}
	}

	// 对已存在的记录, 仅在新值非空时覆盖 group / description
	for _, a := range items {
		key := strings.ToUpper(a.Method) + " " + a.Path
		e, ok := existMap[key]
		if !ok {
			continue
		}
		b := client.AdminAPI.UpdateOneID(e.ID)
		changed := false
		if a.Group != "" && a.Group != e.Group {
			b = b.SetGroup(a.Group)
			changed = true
		}
		if a.Description != "" && a.Description != e.Description {
			b = b.SetDescription(a.Description)
			changed = true
		}
		if changed {
			if _, err := b.Save(ctx); err != nil {
				return fmt.Errorf("update api %d: %w", e.ID, err)
			}
		}
	}
	return nil
}

func (r *rbacRepository) GetAPIsByRoleIDs(ctx context.Context, roleIDs []int64) ([]*service.AdminAPI, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	client := clientFromContext(ctx, r.client)

	links, err := client.AdminRoleAPI.Query().
		Where(adminroleapi.RoleIDIn(roleIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, nil
	}

	seen := make(map[int64]struct{}, len(links))
	ids := make([]int64, 0, len(links))
	for _, l := range links {
		if _, ok := seen[l.APIID]; !ok {
			seen[l.APIID] = struct{}{}
			ids = append(ids, l.APIID)
		}
	}

	items, err := client.AdminAPI.Query().
		Where(
			adminapi.IDIn(ids...),
			adminapi.StatusEQ(service.ResourceStatusActive),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return adminAPIEntitiesToService(items), nil
}

func (r *rbacRepository) GetAPIGroups(ctx context.Context) ([]string, error) {
	client := clientFromContext(ctx, r.client)
	items, err := client.AdminAPI.Query().
		GroupBy(adminapi.FieldGroup).
		Strings(ctx)
	if err != nil {
		return nil, err
	}
	// 去空组
	out := make([]string, 0, len(items))
	for _, g := range items {
		if g != "" {
			out = append(out, g)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 角色-菜单关联
// ---------------------------------------------------------------------------

func (r *rbacRepository) AssignRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	client := clientFromContext(ctx, r.client)

	if _, err := client.AdminRoleMenu.Delete().
		Where(adminrolemenu.RoleIDEQ(roleID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete existing role menus: %w", err)
	}

	if len(menuIDs) == 0 {
		return nil
	}
	builders := make([]*dbent.AdminRoleMenuCreate, 0, len(menuIDs))
	for _, mid := range menuIDs {
		builders = append(builders, client.AdminRoleMenu.Create().
			SetRoleID(roleID).
			SetMenuID(mid))
	}
	if _, err := client.AdminRoleMenu.CreateBulk(builders...).Save(ctx); err != nil {
		return fmt.Errorf("bulk insert role menus: %w", err)
	}
	return nil
}

func (r *rbacRepository) GetRoleMenuIDs(ctx context.Context, roleID int64) ([]int64, error) {
	client := clientFromContext(ctx, r.client)
	items, err := client.AdminRoleMenu.Query().
		Where(adminrolemenu.RoleIDEQ(roleID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.MenuID)
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// 角色-API 关联
// ---------------------------------------------------------------------------

func (r *rbacRepository) AssignRoleAPIs(ctx context.Context, roleID int64, apiIDs []int64) error {
	client := clientFromContext(ctx, r.client)

	if _, err := client.AdminRoleAPI.Delete().
		Where(adminroleapi.RoleIDEQ(roleID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete existing role apis: %w", err)
	}

	if len(apiIDs) == 0 {
		return nil
	}
	builders := make([]*dbent.AdminRoleAPICreate, 0, len(apiIDs))
	for _, aid := range apiIDs {
		builders = append(builders, client.AdminRoleAPI.Create().
			SetRoleID(roleID).
			SetAPIID(aid))
	}
	if _, err := client.AdminRoleAPI.CreateBulk(builders...).Save(ctx); err != nil {
		return fmt.Errorf("bulk insert role apis: %w", err)
	}
	return nil
}

func (r *rbacRepository) GetRoleAPIIDs(ctx context.Context, roleID int64) ([]int64, error) {
	client := clientFromContext(ctx, r.client)
	items, err := client.AdminRoleAPI.Query().
		Where(adminroleapi.RoleIDEQ(roleID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.APIID)
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// 用户-角色关联
// ---------------------------------------------------------------------------

func (r *rbacRepository) AssignUserRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	client := clientFromContext(ctx, r.client)

	if _, err := client.AdminUserRole.Delete().
		Where(adminuserrole.UserIDEQ(userID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete existing user roles: %w", err)
	}

	if len(roleIDs) == 0 {
		return nil
	}
	builders := make([]*dbent.AdminUserRoleCreate, 0, len(roleIDs))
	for _, rid := range roleIDs {
		builders = append(builders, client.AdminUserRole.Create().
			SetUserID(userID).
			SetRoleID(rid))
	}
	if _, err := client.AdminUserRole.CreateBulk(builders...).Save(ctx); err != nil {
		return fmt.Errorf("bulk insert user roles: %w", err)
	}
	return nil
}

func (r *rbacRepository) GetUserRoles(ctx context.Context, userID int64) ([]*service.AdminRole, error) {
	return r.getRolesByUserID(ctx, userID, false)
}

// GetUserRole 查询用户基础身份字段 users.role.
// 用于校验“只能向 admin 身份用户分配 RBAC 角色”等业务约束.
// 用户不存在时返回 ("", nil)，方便调用方统一按 "身份不是 admin" 处理.
func (r *rbacRepository) GetUserRole(ctx context.Context, userID int64) (string, error) {
	client := clientFromContext(ctx, r.client)
	u, err := client.User.Query().
		Where(dbuser.IDEQ(userID)).
		Select(dbuser.FieldRole).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("query user role: %w", err)
	}
	return u.Role, nil
}

// ---------------------------------------------------------------------------
// Ent entity -> service model 转换
// ---------------------------------------------------------------------------

func adminRoleEntityToService(m *dbent.AdminRole) *service.AdminRole {
	if m == nil {
		return nil
	}
	return &service.AdminRole{
		ID:           m.ID,
		Name:         m.Name,
		Description:  m.Description,
		IsSuperAdmin: m.IsSuperAdmin,
		Status:       m.Status,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func adminRoleEntitiesToService(models []*dbent.AdminRole) []*service.AdminRole {
	out := make([]*service.AdminRole, 0, len(models))
	for i := range models {
		if s := adminRoleEntityToService(models[i]); s != nil {
			out = append(out, s)
		}
	}
	return out
}

func adminMenuEntityToService(m *dbent.AdminMenu) *service.AdminMenu {
	if m == nil {
		return nil
	}
	return &service.AdminMenu{
		ID:            m.ID,
		Name:          m.Name,
		NameEn:        m.NameEn,
		Type:          m.Type,
		Path:          m.Path,
		Component:     m.Component,
		Icon:          m.Icon,
		PermissionKey: m.PermissionKey,
		SortOrder:     m.SortOrder,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func adminMenuEntitiesToService(models []*dbent.AdminMenu) []*service.AdminMenu {
	out := make([]*service.AdminMenu, 0, len(models))
	for i := range models {
		if s := adminMenuEntityToService(models[i]); s != nil {
			out = append(out, s)
		}
	}
	return out
}

func adminAPIEntityToService(m *dbent.AdminAPI) *service.AdminAPI {
	if m == nil {
		return nil
	}
	return &service.AdminAPI{
		ID:          m.ID,
		Group:       m.Group,
		Path:        m.Path,
		Method:      m.Method,
		Description: m.Description,
		SortOrder:   m.SortOrder,
		Status:      m.Status,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func adminAPIEntitiesToService(models []*dbent.AdminAPI) []*service.AdminAPI {
	out := make([]*service.AdminAPI, 0, len(models))
	for i := range models {
		if s := adminAPIEntityToService(models[i]); s != nil {
			out = append(out, s)
		}
	}
	return out
}
