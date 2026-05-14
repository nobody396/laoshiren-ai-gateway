<template>
  <AppLayout>
    <div>
      <!-- Page Header -->
      <div class="mb-6 flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t("admin.rbac.menus", "菜单管理") }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t("admin.rbac.menusDesc", "维护后台导航菜单 (扁平单层)") }}
          </p>
        </div>
        <button class="btn btn-primary" @click="openCreateModal">
          <span class="mr-1">+</span>
          {{ t("admin.rbac.createMenu", "创建菜单") }}
        </button>
      </div>

      <!-- Keyword -->
      <div class="mb-3">
        <input
          v-model="keyword"
          class="input max-w-sm"
          :placeholder="
            t('admin.rbac.filterMenuPlaceholder', '筛选菜单名称 / 路径 / 标识')
          "
        />
      </div>

      <!-- Menu Table -->
      <div class="card">
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div
            class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"
          ></div>
        </div>
        <div
          v-else-if="filteredMenus.length === 0"
          class="py-12 text-center text-gray-500 dark:text-gray-400"
        >
          {{ t("admin.rbac.noMenus", "暂无菜单数据") }}
        </div>
        <div v-else class="table-wrapper">
          <table class="table">
            <thead>
              <tr>
                <th class="w-16">{{ t("admin.rbac.sortOrder", "排序") }}</th>
                <th>{{ t("admin.rbac.permName", "菜单名称") }}</th>
                <th>{{ t("admin.rbac.englishMenuName", "英文名称") }}</th>
                <th>{{ t("admin.rbac.routePath", "路由路径") }}</th>
                <th>{{ t("admin.rbac.icon", "图标") }}</th>
                <th>{{ t("admin.rbac.permKey", "权限标识") }}</th>
                <th>{{ t("admin.rbac.status", "状态") }}</th>
                <th class="text-right">
                  {{ t("admin.rbac.actions", "操作") }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in filteredMenus" :key="m.id">
                <td class="text-gray-500 dark:text-gray-400">
                  {{ m.sort_order }}
                </td>
                <td class="font-medium text-gray-900 dark:text-white">
                  {{ m.name }}
                </td>
                <td class="text-gray-600 dark:text-gray-300">
                  {{ m.name_en || "-" }}
                </td>
                <td class="font-mono text-sm text-gray-600 dark:text-gray-300">
                  {{ m.path || "-" }}
                </td>
                <td class="text-sm text-gray-500 dark:text-gray-400">
                  {{ m.icon || "-" }}
                </td>
                <td class="font-mono text-xs text-gray-500 dark:text-gray-400">
                  {{ m.permission_key }}
                </td>
                <td>
                  <span
                    :class="
                      m.status === 'active'
                        ? 'badge badge-success'
                        : 'badge badge-gray'
                    "
                  >
                    {{
                      m.status === "active"
                        ? t("common.enabled", "启用")
                        : t("common.disabled", "禁用")
                    }}
                  </span>
                </td>
                <td class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      class="btn btn-secondary text-xs"
                      @click="openEditModal(m)"
                    >
                      {{ t("common.edit", "编辑") }}
                    </button>
                    <button
                      class="btn btn-danger text-xs"
                      @click="confirmDelete(m)"
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

      <!-- Create / Edit Modal -->
      <div
        v-if="showModal"
        class="modal-overlay"
        @click.self="showModal = false"
      >
        <div class="modal-content" style="max-width: 560px">
          <div class="modal-header">
            <h3 class="modal-title">
              {{
                editingMenu
                  ? t("common.edit", "编辑菜单")
                  : t("admin.rbac.createMenu", "创建菜单")
              }}
            </h3>
            <button
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              @click="showModal = false"
            >
              &times;
            </button>
          </div>
          <div class="modal-body">
            <!-- Name -->
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.permName", "菜单名称") }} *</label
              >
              <input
                v-model="form.name"
                class="input"
                :placeholder="
                  t('admin.rbac.menuNamePlaceholder', '如: 用户管理')
                "
              />
            </div>
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.englishMenuName", "英文名称") }}</label
              >
              <input
                v-model="form.name_en"
                class="input"
                :placeholder="
                  t(
                    'admin.rbac.englishMenuNamePlaceholder',
                    'e.g. User Management',
                  )
                "
              />
            </div>
            <!-- Permission Key -->
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.permKey", "权限标识") }} *</label
              >
              <input
                v-model="form.permission_key"
                class="input"
                :readonly="!!editingMenu"
                :disabled="!!editingMenu"
                :placeholder="
                  t('admin.rbac.permKeyPlaceholder', '如: admin:users')
                "
              />
              <p
                v-if="editingMenu"
                class="mt-1 text-xs text-amber-600 dark:text-amber-400"
              >
                {{
                  t(
                    "admin.rbac.permKeyLockedHint",
                    "权限标识与前端侧边栏 / 路由守卫硬编码绑定，创建后不可修改；如需变更请删除后重建。",
                  )
                }}
              </p>
            </div>
            <!-- Path -->
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.routePath", "路由路径") }}</label
              >
              <input
                v-model="form.path"
                class="input"
                :readonly="!!editingMenu"
                :disabled="!!editingMenu"
                :placeholder="
                  t('admin.rbac.routePathPlaceholder', '如: /admin/users')
                "
              />
              <p
                v-if="editingMenu"
                class="mt-1 text-xs text-amber-600 dark:text-amber-400"
              >
                {{
                  t(
                    "admin.rbac.routePathLockedHint",
                    "路由路径由前端代码注册，仅作记录用；编辑此字段不会改变实际跳转地址。",
                  )
                }}
              </p>
            </div>
            <!-- Component -->
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.component", "组件路径") }}</label
              >
              <input
                v-model="form.component"
                class="input"
                placeholder="如: admin/UsersView"
              />
            </div>
            <!-- Icon -->
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.icon", "图标") }}</label
              >
              <input
                v-model="form.icon"
                class="input"
                placeholder="如: users, settings, chart"
              />
            </div>
            <!-- Sort Order -->
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.sortOrder", "排序") }}</label
              >
              <input
                v-model.number="form.sort_order"
                type="number"
                class="input"
              />
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showModal = false">
              {{ t("common.cancel", "取消") }}
            </button>
            <button
              class="btn btn-primary"
              :disabled="saving"
              @click="saveMenu"
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
                t("admin.rbac.deleteMenuMsg", {
                  name: deletingMenu?.name ?? "",
                }) || `确定要删除菜单 "${deletingMenu?.name}" 吗?`
              }}
            </p>
          </div>
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="showDeleteModal = false">
              {{ t("common.cancel", "取消") }}
            </button>
            <button class="btn btn-danger" :disabled="saving" @click="doDelete">
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
import type { AdminMenu } from "@/api/admin/rbac";
import AppLayout from "@/components/layout/AppLayout.vue";
import { usePermissionStore } from "@/stores/permission";

const { t } = useI18n();
const permissionStore = usePermissionStore();

const menus = ref<AdminMenu[]>([]);
const loading = ref(false);
const saving = ref(false);
const keyword = ref("");

// Modal state
const showModal = ref(false);
const editingMenu = ref<AdminMenu | null>(null);
const form = ref({
  name: "",
  name_en: "",
  permission_key: "",
  path: "",
  component: "",
  icon: "",
  sort_order: 0,
});

// Delete
const showDeleteModal = ref(false);
const deletingMenu = ref<AdminMenu | null>(null);

const filteredMenus = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  const list = [...menus.value].sort((a, b) => a.sort_order - b.sort_order);
  if (!kw) return list;
  return list.filter(
    (m) =>
      m.name.toLowerCase().includes(kw) ||
      (m.name_en ?? "").toLowerCase().includes(kw) ||
      (m.path ?? "").toLowerCase().includes(kw) ||
      m.permission_key.toLowerCase().includes(kw),
  );
});

onMounted(() => {
  loadData();
});

async function loadData() {
  loading.value = true;
  try {
    menus.value = await rbacAPI.listMenus();
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false;
  }
}

function openCreateModal() {
  editingMenu.value = null;
  form.value = {
    name: "",
    name_en: "",
    permission_key: "",
    path: "",
    component: "",
    icon: "",
    sort_order: 0,
  };
  showModal.value = true;
}

function openEditModal(menu: AdminMenu) {
  editingMenu.value = menu;
  form.value = {
    name: menu.name,
    name_en: menu.name_en ?? "",
    permission_key: menu.permission_key,
    path: menu.path,
    component: menu.component,
    icon: menu.icon,
    sort_order: menu.sort_order,
  };
  showModal.value = true;
}

async function saveMenu() {
  if (!form.value.name.trim() || !form.value.permission_key.trim()) return;
  saving.value = true;
  try {
    if (editingMenu.value) {
      await rbacAPI.updateMenu(editingMenu.value.id, form.value);
    } else {
      await rbacAPI.createMenu(form.value);
    }
    showModal.value = false;
    await loadData();
    await permissionStore.fetchMenuTree();
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false;
  }
}

function confirmDelete(menu: AdminMenu) {
  deletingMenu.value = menu;
  showDeleteModal.value = true;
}

async function doDelete() {
  if (!deletingMenu.value) return;
  saving.value = true;
  try {
    await rbacAPI.deleteMenu(deletingMenu.value.id);
    showDeleteModal.value = false;
    await loadData();
    await permissionStore.fetchMenuTree();
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false;
  }
}
</script>
