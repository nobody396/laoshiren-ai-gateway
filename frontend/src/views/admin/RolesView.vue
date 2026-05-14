<template>
  <AppLayout>
    <div>
      <!-- Page Header -->
      <div class="mb-6 flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t("admin.rbac.roles", "角色管理") }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t("admin.rbac.rolesDesc", "管理系统角色及其权限分配") }}
          </p>
        </div>
        <button class="btn btn-primary" @click="openCreateModal">
          <span class="mr-1">+</span>
          {{ t("admin.rbac.createRole", "创建角色") }}
        </button>
      </div>

      <!-- Role List -->
      <div class="card">
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div
            class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"
          ></div>
        </div>
        <div
          v-else-if="roles.length === 0"
          class="py-12 text-center text-gray-500 dark:text-gray-400"
        >
          {{ t("admin.rbac.noRoles", "暂无角色数据") }}
        </div>
        <div v-else class="table-wrapper">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t("admin.rbac.roleName", "角色名称") }}</th>
                <th>{{ t("admin.rbac.description", "描述") }}</th>
                <th>{{ t("admin.rbac.type", "类型") }}</th>
                <th>{{ t("admin.rbac.status", "状态") }}</th>
                <th>{{ t("admin.rbac.createdAt", "创建时间") }}</th>
                <th class="text-right">
                  {{ t("admin.rbac.actions", "操作") }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="role in roles" :key="role.id">
                <td class="font-medium text-gray-900 dark:text-white">
                  {{ role.name }}
                </td>
                <td class="text-gray-500 dark:text-gray-400">
                  {{ role.description || "-" }}
                </td>
                <td>
                  <span
                    v-if="role.is_super_admin"
                    class="badge badge-warning"
                    >{{ t("admin.rbac.superAdmin", "超级管理员") }}</span
                  >
                  <span v-else class="badge badge-primary">{{
                    t("admin.rbac.normalRole", "普通角色")
                  }}</span>
                </td>
                <td>
                  <span
                    :class="
                      role.status === 'active'
                        ? 'badge badge-success'
                        : 'badge badge-gray'
                    "
                  >
                    {{
                      role.status === "active"
                        ? t("common.enabled", "启用")
                        : t("common.disabled", "禁用")
                    }}
                  </span>
                </td>
                <td class="text-sm text-gray-500 dark:text-gray-400">
                  {{ formatDate(role.created_at) }}
                </td>
                <td class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      class="btn btn-secondary text-xs"
                      @click="openPermissionModal(role)"
                    >
                      {{ t("admin.rbac.assignPermissions", "分配权限") }}
                    </button>
                    <button
                      class="btn btn-secondary text-xs"
                      @click="openEditModal(role)"
                    >
                      {{ t("common.edit", "编辑") }}
                    </button>
                    <button
                      v-if="!role.is_super_admin"
                      class="btn btn-danger text-xs"
                      @click="confirmDelete(role)"
                    >
                      {{ t("common.delete", "删除") }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Create/Edit Role Modal -->
      <div
        v-if="showRoleModal"
        class="modal-overlay"
        @click.self="showRoleModal = false"
      >
        <div class="modal-content" style="max-width: 560px">
          <div class="modal-header">
            <h3 class="modal-title">
              {{
                editingRole
                  ? t("common.edit", "编辑角色")
                  : t("admin.rbac.createRole", "创建角色")
              }}
            </h3>
            <button
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              @click="showRoleModal = false"
            >
              &times;
            </button>
          </div>
          <div class="modal-body">
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.roleName", "角色名称") }}</label
              >
              <input
                v-model="roleForm.name"
                class="input"
                :placeholder="
                  t('admin.rbac.roleNamePlaceholder', '请输入角色名称')
                "
              />
            </div>
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.description", "描述") }}</label
              >
              <textarea
                v-model="roleForm.description"
                class="input"
                rows="3"
                :placeholder="t('admin.rbac.descPlaceholder', '请输入角色描述')"
              ></textarea>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showRoleModal = false">
              {{ t("common.cancel", "取消") }}
            </button>
            <button
              class="btn btn-primary"
              :disabled="saving"
              @click="saveRole"
            >
              {{
                saving
                  ? t("common.saving", "保存中...")
                  : t("common.save", "保存")
              }}
            </button>
          </div>
        </div>
      </div>

      <!-- Permission Assignment Modal (双 Tab) -->
      <div
        v-if="showPermModal"
        class="modal-overlay"
        @click.self="showPermModal = false"
      >
        <div class="modal-content" style="max-width: 860px">
          <div class="modal-header">
            <h3 class="modal-title">
              {{ t("admin.rbac.assignPermissions", "分配权限") }} -
              {{ permTargetRole?.name }}
            </h3>
            <button
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              @click="showPermModal = false"
            >
              &times;
            </button>
          </div>

          <!-- Tab Header -->
          <div class="border-b border-gray-200 dark:border-dark-700">
            <nav class="-mb-px flex gap-4 px-6">
              <button
                type="button"
                class="border-b-2 px-1 py-3 text-sm font-medium transition-colors"
                :class="
                  activeTab === 'menu'
                    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                "
                @click="activeTab = 'menu'"
              >
                {{ t("admin.rbac.roleMenu", "角色菜单") }}
                <span class="ml-1 text-xs text-gray-400"
                  >({{ checkedMenuIds.length }})</span
                >
              </button>
              <button
                type="button"
                class="border-b-2 px-1 py-3 text-sm font-medium transition-colors"
                :class="
                  activeTab === 'api'
                    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                "
                @click="activeTab = 'api'"
              >
                {{ t("admin.rbac.roleApi", "角色 API") }}
                <span class="ml-1 text-xs text-gray-400"
                  >({{ checkedApiIds.length }})</span
                >
              </button>
            </nav>
          </div>

          <div class="modal-body" style="max-height: 60vh; overflow-y: auto">
            <!-- Loading -->
            <div
              v-if="permLoading"
              class="flex items-center justify-center py-8"
            >
              <div
                class="h-6 w-6 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"
              ></div>
            </div>

            <!-- Tab: Menu -->
            <div v-else-if="activeTab === 'menu'">
              <div class="mb-3 flex items-center gap-2">
                <input
                  v-model="menuKeyword"
                  class="input flex-1"
                  :placeholder="
                    t('admin.rbac.filterMenuPlaceholder', '筛选菜单名称/标识')
                  "
                />
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="selectAllFilteredMenus"
                >
                  {{ t("admin.rbac.selectAll", "全选") }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="clearMenuSelection"
                >
                  {{ t("admin.rbac.clear", "清空") }}
                </button>
              </div>
              <div
                v-if="filteredMenus.length === 0"
                class="py-6 text-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t("admin.rbac.noMenus", "暂无菜单数据") }}
              </div>
              <div
                v-else
                class="divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700/50 dark:border-dark-700"
              >
                <label
                  v-for="m in filteredMenus"
                  :key="m.id"
                  class="flex items-center gap-3 px-4 py-2 hover:bg-gray-50 dark:hover:bg-dark-700/30"
                >
                  <input
                    type="checkbox"
                    :checked="checkedMenuIds.includes(m.id)"
                    @change="toggleMenu(m.id)"
                  />
                  <span class="flex-1 text-sm text-gray-800 dark:text-gray-100">
                    {{ m.name }}
                  </span>
                  <span
                    class="font-mono text-xs text-gray-500 dark:text-gray-400"
                  >
                    {{ m.path || "-" }}
                  </span>
                  <span
                    class="font-mono text-xs text-gray-400 dark:text-gray-500"
                  >
                    {{ m.permission_key }}
                  </span>
                </label>
              </div>
            </div>

            <!-- Tab: API -->
            <div v-else>
              <div class="mb-3 flex items-center gap-2">
                <input
                  v-model="apiKeyword"
                  class="input flex-1"
                  :placeholder="
                    t('admin.rbac.filterApiPlaceholder', '筛选路径/描述')
                  "
                />
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="selectAllFilteredApis"
                >
                  {{ t("admin.rbac.selectAll", "全选") }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="invertFilteredApis"
                >
                  {{ t("admin.rbac.invertSelection", "反选") }}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="clearApiSelection"
                >
                  {{ t("admin.rbac.clear", "清空") }}
                </button>
              </div>

              <div class="space-y-2">
                <div
                  v-for="group in filteredApiGroups"
                  :key="group.name"
                  class="rounded-lg border border-gray-200 dark:border-dark-700"
                >
                  <div
                    class="flex items-center justify-between bg-gray-50 px-3 py-2 dark:bg-dark-800"
                  >
                    <button
                      type="button"
                      class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200"
                      @click="toggleApiGroup(group.name)"
                    >
                      <svg
                        class="h-3.5 w-3.5 transition-transform"
                        :class="
                          collapsedGroups.has(group.name) ? '' : 'rotate-90'
                        "
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2.5"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M9 5l7 7-7 7"
                        />
                      </svg>
                      <span>{{ group.name || "(未分组)" }}</span>
                      <span class="text-xs text-gray-400"
                        >{{ checkedInGroup(group) }} /
                        {{ group.apis.length }}</span
                      >
                    </button>
                    <div class="flex gap-1">
                      <button
                        type="button"
                        class="btn btn-secondary text-xs"
                        @click.stop="toggleGroupAll(group, true)"
                      >
                        {{ t("admin.rbac.selectGroup", "全选") }}
                      </button>
                      <button
                        type="button"
                        class="btn btn-secondary text-xs"
                        @click.stop="toggleGroupAll(group, false)"
                      >
                        {{ t("admin.rbac.clearGroup", "清空") }}
                      </button>
                    </div>
                  </div>
                  <div
                    v-if="!collapsedGroups.has(group.name)"
                    class="divide-y divide-gray-100 dark:divide-dark-700/50"
                  >
                    <label
                      v-for="api in group.apis"
                      :key="api.id"
                      class="flex items-center gap-3 px-4 py-2 hover:bg-gray-50 dark:hover:bg-dark-700/30"
                    >
                      <input
                        type="checkbox"
                        :checked="checkedApiIds.includes(api.id)"
                        @change="toggleApi(api.id)"
                      />
                      <span :class="methodBadgeClass(api.method)">{{
                        api.method
                      }}</span>
                      <span class="flex-1 font-mono text-sm">{{
                        api.path
                      }}</span>
                      <span class="text-xs text-gray-500 dark:text-gray-400">{{
                        api.description
                      }}</span>
                    </label>
                  </div>
                </div>
                <div
                  v-if="filteredApiGroups.length === 0"
                  class="py-6 text-center text-sm text-gray-500 dark:text-gray-400"
                >
                  {{ t("admin.rbac.noApis", "暂无 API 数据") }}
                </div>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showPermModal = false">
              {{ t("common.cancel", "取消") }}
            </button>
            <button
              class="btn btn-primary"
              :disabled="saving"
              @click="savePermissions"
            >
              {{
                saving
                  ? t("common.saving", "保存中...")
                  : t("common.save", "保存")
              }}
            </button>
          </div>
        </div>
      </div>

      <!-- Delete Confirmation Modal -->
      <div
        v-if="showDeleteModal"
        class="modal-overlay"
        @click.self="showDeleteModal = false"
      >
        <div class="modal-content" style="max-width: 420px">
          <div class="modal-header">
            <h3 class="modal-title">
              {{ t("admin.rbac.confirmDelete", "确认删除") }}
            </h3>
            <button
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              @click="showDeleteModal = false"
            >
              &times;
            </button>
          </div>
          <div class="modal-body">
            <p class="text-gray-600 dark:text-gray-300">
              {{
                t("admin.rbac.deleteConfirmMsg", {
                  name: deletingRole?.name ?? "",
                }) ||
                `确定要删除角色 "${deletingRole?.name}" 吗? 此操作不可撤销。`
              }}
            </p>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showDeleteModal = false">
              {{ t("common.cancel", "取消") }}
            </button>
            <button
              class="btn btn-danger"
              :disabled="saving"
              @click="doDeleteRole"
            >
              {{ t("common.delete", "删除") }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import rbacAPI from "@/api/admin/rbac";
import type { AdminAPI, AdminMenu, AdminRole } from "@/api/admin/rbac";
import AppLayout from "@/components/layout/AppLayout.vue";

const { t } = useI18n();

const roles = ref<AdminRole[]>([]);
const loading = ref(false);
const saving = ref(false);

// Role modal
const showRoleModal = ref(false);
const editingRole = ref<AdminRole | null>(null);
const roleForm = ref({ name: "", description: "" });

// Permission modal (Tab)
const showPermModal = ref(false);
const permTargetRole = ref<AdminRole | null>(null);
const permLoading = ref(false);
const activeTab = ref<"menu" | "api">("menu");

const menus = ref<AdminMenu[]>([]);
const checkedMenuIds = ref<number[]>([]);
const menuKeyword = ref("");

const allApis = ref<AdminAPI[]>([]);
const checkedApiIds = ref<number[]>([]);
const apiKeyword = ref("");
const collapsedGroups = ref<Set<string>>(new Set());

// Delete modal
const showDeleteModal = ref(false);
const deletingRole = ref<AdminRole | null>(null);

interface ApiGroup {
  name: string;
  apis: AdminAPI[];
}

// ===== Filtered Views =====

const filteredMenus = computed<AdminMenu[]>(() => {
  const kw = menuKeyword.value.trim().toLowerCase();
  const list = [...menus.value].sort((a, b) => a.sort_order - b.sort_order);
  if (!kw) return list;
  return list.filter(
    (m) =>
      m.name.toLowerCase().includes(kw) ||
      (m.path ?? "").toLowerCase().includes(kw) ||
      m.permission_key.toLowerCase().includes(kw),
  );
});

const filteredApiGroups = computed<ApiGroup[]>(() => {
  const kw = apiKeyword.value.trim().toLowerCase();
  const bucket = new Map<string, AdminAPI[]>();
  for (const api of allApis.value) {
    if (
      kw &&
      !api.path.toLowerCase().includes(kw) &&
      !(api.description ?? "").toLowerCase().includes(kw) &&
      !(api.group ?? "").toLowerCase().includes(kw)
    ) {
      continue;
    }
    const name = api.group || "(未分组)";
    if (!bucket.has(name)) bucket.set(name, []);
    bucket.get(name)!.push(api);
  }
  const arr: ApiGroup[] = [];
  for (const [name, apis] of bucket.entries()) {
    apis.sort((a, b) => a.path.localeCompare(b.path));
    arr.push({ name, apis });
  }
  arr.sort((a, b) => a.name.localeCompare(b.name));
  return arr;
});

onMounted(() => {
  loadRoles();
});

async function loadRoles() {
  loading.value = true;
  try {
    roles.value = await rbacAPI.listRoles();
  } catch {
    // handled by axios interceptor
  } finally {
    loading.value = false;
  }
}

function openCreateModal() {
  editingRole.value = null;
  roleForm.value = { name: "", description: "" };
  showRoleModal.value = true;
}

function openEditModal(role: AdminRole) {
  editingRole.value = role;
  roleForm.value = { name: role.name, description: role.description };
  showRoleModal.value = true;
}

async function saveRole() {
  if (!roleForm.value.name.trim()) return;
  saving.value = true;
  try {
    if (editingRole.value) {
      await rbacAPI.updateRole(editingRole.value.id, roleForm.value);
    } else {
      await rbacAPI.createRole(roleForm.value);
    }
    showRoleModal.value = false;
    await loadRoles();
  } catch {
    // interceptor
  } finally {
    saving.value = false;
  }
}

function confirmDelete(role: AdminRole) {
  deletingRole.value = role;
  showDeleteModal.value = true;
}

async function doDeleteRole() {
  if (!deletingRole.value) return;
  saving.value = true;
  try {
    await rbacAPI.deleteRole(deletingRole.value.id);
    showDeleteModal.value = false;
    await loadRoles();
  } catch {
    // interceptor
  } finally {
    saving.value = false;
  }
}

async function openPermissionModal(role: AdminRole) {
  permTargetRole.value = role;
  showPermModal.value = true;
  activeTab.value = "menu";
  menuKeyword.value = "";
  apiKeyword.value = "";
  collapsedGroups.value = new Set();
  permLoading.value = true;
  try {
    const [menuList, apis, roleMenus, roleApis] = await Promise.all([
      rbacAPI.listMenus(),
      rbacAPI.listAllApis(),
      rbacAPI.getRoleMenus(role.id),
      rbacAPI.getRoleApis(role.id),
    ]);
    menus.value = menuList;
    allApis.value = apis;
    checkedMenuIds.value = roleMenus;
    checkedApiIds.value = roleApis;
  } catch {
    // interceptor
  } finally {
    permLoading.value = false;
  }
}

function toggleMenu(id: number) {
  const idx = checkedMenuIds.value.indexOf(id);
  if (idx >= 0) {
    checkedMenuIds.value.splice(idx, 1);
  } else {
    checkedMenuIds.value.push(id);
  }
}

function selectAllFilteredMenus() {
  const set = new Set(checkedMenuIds.value);
  for (const m of filteredMenus.value) set.add(m.id);
  checkedMenuIds.value = Array.from(set);
}

function clearMenuSelection() {
  checkedMenuIds.value = [];
}

function toggleApi(id: number) {
  const idx = checkedApiIds.value.indexOf(id);
  if (idx >= 0) {
    checkedApiIds.value.splice(idx, 1);
  } else {
    checkedApiIds.value.push(id);
  }
}

function toggleApiGroup(name: string) {
  const next = new Set(collapsedGroups.value);
  if (next.has(name)) {
    next.delete(name);
  } else {
    next.add(name);
  }
  collapsedGroups.value = next;
}

function checkedInGroup(group: ApiGroup): number {
  return group.apis.filter((a) => checkedApiIds.value.includes(a.id)).length;
}

function toggleGroupAll(group: ApiGroup, select: boolean) {
  const set = new Set(checkedApiIds.value);
  for (const api of group.apis) {
    if (select) set.add(api.id);
    else set.delete(api.id);
  }
  checkedApiIds.value = Array.from(set);
}

function selectAllFilteredApis() {
  const set = new Set(checkedApiIds.value);
  for (const g of filteredApiGroups.value) {
    for (const api of g.apis) set.add(api.id);
  }
  checkedApiIds.value = Array.from(set);
}

function invertFilteredApis() {
  const set = new Set(checkedApiIds.value);
  for (const g of filteredApiGroups.value) {
    for (const api of g.apis) {
      if (set.has(api.id)) set.delete(api.id);
      else set.add(api.id);
    }
  }
  checkedApiIds.value = Array.from(set);
}

function clearApiSelection() {
  checkedApiIds.value = [];
}

async function savePermissions() {
  if (!permTargetRole.value) return;
  saving.value = true;
  try {
    await Promise.all([
      rbacAPI.setRoleMenus(permTargetRole.value.id, {
        menu_ids: checkedMenuIds.value,
      }),
      rbacAPI.setRoleApis(permTargetRole.value.id, {
        api_ids: checkedApiIds.value,
      }),
    ]);
    showPermModal.value = false;
  } catch {
    // interceptor
  } finally {
    saving.value = false;
  }
}

function methodBadgeClass(method: string): string {
  switch (method.toUpperCase()) {
    case "GET":
      return "badge badge-success";
    case "POST":
      return "badge badge-primary";
    case "PUT":
      return "badge badge-warning";
    case "DELETE":
      return "badge badge-danger";
    case "PATCH":
      return "badge badge-purple";
    default:
      return "badge badge-gray";
  }
}

function formatDate(dateStr: string): string {
  if (!dateStr) return "-";
  return new Date(dateStr).toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
</script>
