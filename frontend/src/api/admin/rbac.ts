/**
 * Admin RBAC API endpoints
 *
 * 拆分后的两套资源:
 *   - 菜单 (单层, type 固定 menu): /admin/rbac/menus*
 *   - API:                         /admin/rbac/apis*
 * 角色分别绑定菜单集合与 API 集合.
 */

import { apiClient } from "../client";

// ==================== Types ====================

export interface AdminRole {
  id: number;
  name: string;
  description: string;
  is_super_admin: boolean;
  status: string;
  created_at: string;
  updated_at: string;
}

/** 菜单 (扁平单层, type 固定 'menu'). */
export interface AdminMenu {
  id: number;
  name: string;
  name_en: string;
  type: "menu";
  path: string;
  component: string;
  icon: string;
  permission_key: string;
  sort_order: number;
  status: string;
  created_at: string;
  updated_at: string;
}

/** 后端 API 资源. */
export interface AdminAPI {
  id: number;
  group: string;
  path: string;
  method: string;
  description: string;
  sort_order: number;
  status: string;
  created_at: string;
  updated_at: string;
}

/** 扁平菜单节点 (保留命名以兼容调用方). */
export interface MenuTreeNode {
  id: number;
  name: string;
  name_en?: string;
  type: string;
  path?: string;
  component?: string;
  icon?: string;
  permission_key?: string;
  sort_order: number;
}

export interface CreateRoleRequest {
  name: string;
  description?: string;
  is_super_admin?: boolean;
}

export interface UpdateRoleRequest {
  name?: string;
  description?: string;
  is_super_admin?: boolean;
}

export interface CreateMenuRequest {
  name: string;
  name_en?: string;
  path?: string;
  component?: string;
  icon?: string;
  permission_key: string;
  sort_order?: number;
}

export interface UpdateMenuRequest {
  name?: string;
  name_en?: string;
  path?: string;
  component?: string;
  icon?: string;
  permission_key?: string;
  sort_order?: number;
  status?: string;
}

export interface CreateAPIRequest {
  group?: string;
  path: string;
  method: string;
  description?: string;
  sort_order?: number;
}

export interface UpdateAPIRequest {
  group?: string;
  path?: string;
  method?: string;
  description?: string;
  sort_order?: number;
  status?: string;
}

export interface ListAPIsParams {
  group?: string;
  method?: string;
  keyword?: string;
  page?: number;
  page_size?: number;
}

export interface ListAPIsResult {
  list: AdminAPI[];
  total: number;
  page: number;
  page_size: number;
}

export interface SetRoleMenusRequest {
  menu_ids: number[];
}

export interface SetRoleAPIsRequest {
  api_ids: number[];
}

export interface SetUserRolesRequest {
  role_ids: number[];
}

export interface UserMenuVisibilitySettings {
  invoice_management_enabled: boolean;
  feedback_management_enabled: boolean;
}

// ==================== 当前用户: 菜单 & 权限 ====================

export async function getMyMenuTree(): Promise<MenuTreeNode[]> {
  const { data } = await apiClient.get<MenuTreeNode[]>("/admin/rbac/menu");
  return data;
}

export async function getMyPermissions(): Promise<string[]> {
  const { data } = await apiClient.get<string[]>("/admin/rbac/me/permissions");
  return data;
}

export async function getUserMenuVisibility(): Promise<UserMenuVisibilitySettings> {
  const { data } = await apiClient.get<UserMenuVisibilitySettings>(
    "/admin/rbac/user-menus/visibility",
  );
  return data;
}

export async function updateUserMenuVisibility(
  request: UserMenuVisibilitySettings,
): Promise<UserMenuVisibilitySettings> {
  const { data } = await apiClient.put<UserMenuVisibilitySettings>(
    "/admin/rbac/user-menus/visibility",
    request,
  );
  return data;
}

// ==================== 菜单管理 ====================

export async function listMenus(): Promise<AdminMenu[]> {
  const { data } = await apiClient.get<AdminMenu[]>("/admin/rbac/menus");
  return data;
}

export async function getMenuTree(): Promise<MenuTreeNode[]> {
  const { data } = await apiClient.get<MenuTreeNode[]>(
    "/admin/rbac/menus/tree",
  );
  return data;
}

export async function createMenu(
  request: CreateMenuRequest,
): Promise<AdminMenu> {
  const { data } = await apiClient.post<AdminMenu>(
    "/admin/rbac/menus",
    request,
  );
  return data;
}

export async function updateMenu(
  id: number,
  request: UpdateMenuRequest,
): Promise<AdminMenu> {
  const { data } = await apiClient.put<AdminMenu>(
    `/admin/rbac/menus/${id}`,
    request,
  );
  return data;
}

export async function deleteMenu(id: number): Promise<void> {
  await apiClient.delete(`/admin/rbac/menus/${id}`);
}

// ==================== API 管理 ====================

export async function listApis(
  params: ListAPIsParams = {},
): Promise<ListAPIsResult> {
  const { data } = await apiClient.get<ListAPIsResult>("/admin/rbac/apis", {
    params,
  });
  return data;
}

export async function listAllApis(): Promise<AdminAPI[]> {
  const { data } = await apiClient.get<AdminAPI[]>("/admin/rbac/apis/all");
  return data;
}

export async function getApiGroups(): Promise<string[]> {
  const { data } = await apiClient.get<string[]>("/admin/rbac/apis/groups");
  return data;
}

export async function createApi(request: CreateAPIRequest): Promise<AdminAPI> {
  const { data } = await apiClient.post<AdminAPI>("/admin/rbac/apis", request);
  return data;
}

export async function updateApi(
  id: number,
  request: UpdateAPIRequest,
): Promise<AdminAPI> {
  const { data } = await apiClient.put<AdminAPI>(
    `/admin/rbac/apis/${id}`,
    request,
  );
  return data;
}

export async function deleteApi(id: number): Promise<void> {
  await apiClient.delete(`/admin/rbac/apis/${id}`);
}

export async function deleteApis(ids: number[]): Promise<void> {
  await apiClient.delete("/admin/rbac/apis", { data: { ids } });
}

export async function syncApis(): Promise<{ synced: number; note?: string }> {
  const { data } = await apiClient.post<{ synced: number; note?: string }>(
    "/admin/rbac/apis/sync",
  );
  return data;
}

// ==================== 角色管理 ====================

export async function listRoles(): Promise<AdminRole[]> {
  const { data } = await apiClient.get<AdminRole[]>("/admin/rbac/roles");
  return data;
}

export async function createRole(
  request: CreateRoleRequest,
): Promise<AdminRole> {
  const { data } = await apiClient.post<AdminRole>(
    "/admin/rbac/roles",
    request,
  );
  return data;
}

export async function updateRole(
  id: number,
  request: UpdateRoleRequest,
): Promise<AdminRole> {
  const { data } = await apiClient.put<AdminRole>(
    `/admin/rbac/roles/${id}`,
    request,
  );
  return data;
}

export async function deleteRole(id: number): Promise<void> {
  await apiClient.delete(`/admin/rbac/roles/${id}`);
}

// ==================== 角色-菜单 / 角色-API ====================

export async function getRoleMenus(roleId: number): Promise<number[]> {
  const { data } = await apiClient.get<number[]>(
    `/admin/rbac/roles/${roleId}/menus`,
  );
  return data;
}

export async function setRoleMenus(
  roleId: number,
  request: SetRoleMenusRequest,
): Promise<void> {
  await apiClient.put(`/admin/rbac/roles/${roleId}/menus`, request);
}

export async function getRoleApis(roleId: number): Promise<number[]> {
  const { data } = await apiClient.get<number[]>(
    `/admin/rbac/roles/${roleId}/apis`,
  );
  return data;
}

export async function setRoleApis(
  roleId: number,
  request: SetRoleAPIsRequest,
): Promise<void> {
  await apiClient.put(`/admin/rbac/roles/${roleId}/apis`, request);
}

// ==================== User Roles ====================

export async function getUserRoles(userId: number): Promise<AdminRole[]> {
  const { data } = await apiClient.get<AdminRole[]>(
    `/admin/rbac/users/${userId}/roles`,
  );
  return data;
}

export async function setUserRoles(
  userId: number,
  request: SetUserRolesRequest,
): Promise<void> {
  await apiClient.put(`/admin/rbac/users/${userId}/roles`, request);
}

const rbacAPI = {
  // 当前用户
  getMyMenuTree,
  getMyPermissions,
  getUserMenuVisibility,
  updateUserMenuVisibility,
  // 菜单
  listMenus,
  getMenuTree,
  createMenu,
  updateMenu,
  deleteMenu,
  // API
  listApis,
  listAllApis,
  getApiGroups,
  createApi,
  updateApi,
  deleteApi,
  deleteApis,
  syncApis,
  // 角色
  listRoles,
  createRole,
  updateRole,
  deleteRole,
  // 角色 <-> 资源
  getRoleMenus,
  setRoleMenus,
  getRoleApis,
  setRoleApis,
  // 用户 <-> 角色
  getUserRoles,
  setUserRoles,
};

export default rbacAPI;
