<template>
  <AppLayout>
    <div>
      <!-- Page Header -->
      <div class="mb-6 flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t("admin.rbac.apis", "API 管理") }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{
              t("admin.rbac.apisDesc", "维护后台接口资源清单, 供角色分配使用")
            }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-primary" @click="openCreateModal">
            <span class="mr-1">+</span>
            {{ t("admin.rbac.createApi", "新增") }}
          </button>
          <button
            class="btn btn-danger"
            :disabled="selectedIds.length === 0"
            @click="confirmBatchDelete"
          >
            {{ t("admin.rbac.batchDelete", "批量删除") }}
            <span v-if="selectedIds.length > 0" class="ml-1"
              >({{ selectedIds.length }})</span
            >
          </button>
          <button class="btn btn-secondary" :disabled="syncing" @click="doSync">
            <span v-if="syncing" class="mr-1">...</span>
            {{ t("admin.rbac.syncApis", "同步 API") }}
          </button>
          <button class="btn btn-secondary" @click="loadData">
            {{ t("common.refresh", "刷新") }}
          </button>
        </div>
      </div>

      <!-- Filter -->
      <div class="mb-4">
        <div class="flex flex-wrap items-center gap-3">
          <div class="w-full sm:w-64">
            <SearchInput
              v-model="filter.keyword"
              :placeholder="
                t('admin.rbac.apiKeywordPlaceholder', '搜索路径/描述')
              "
              @search="onSearch"
            />
          </div>
          <div class="w-full sm:w-40">
            <Select
              v-model="filter.group"
              :options="groupSelectOptions"
              :placeholder="t('admin.rbac.anyGroup', '全部分组')"
              @change="onSearch"
            />
          </div>
          <div class="w-full sm:w-36">
            <Select
              v-model="filter.method"
              :options="methodSelectOptions"
              :placeholder="t('admin.rbac.anyMethod', '全部方法')"
              @change="onSearch"
            />
          </div>
          <button class="btn btn-secondary" @click="onReset">
            {{ t("common.reset", "重置") }}
          </button>
        </div>
      </div>

      <!-- Table -->
      <div class="card">
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div
            class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"
          ></div>
        </div>
        <div
          v-else-if="apis.length === 0"
          class="py-12 text-center text-gray-500 dark:text-gray-400"
        >
          {{ t("admin.rbac.noApis", "暂无 API 数据") }}
        </div>
        <div v-else class="table-wrapper">
          <table class="table">
            <thead>
              <tr>
                <th class="w-10">
                  <input
                    type="checkbox"
                    :checked="allSelected"
                    :indeterminate.prop="partiallySelected"
                    @change="toggleSelectAll"
                  />
                </th>
                <th>ID</th>
                <th>{{ t("admin.rbac.apiPath", "API 路径") }}</th>
                <th>{{ t("admin.rbac.apiGroup", "API 分组") }}</th>
                <th>{{ t("admin.rbac.apiDescription", "API 简介") }}</th>
                <th>{{ t("admin.rbac.apiMethod", "请求方法") }}</th>
                <th class="text-right">
                  {{ t("admin.rbac.actions", "操作") }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="api in apis" :key="api.id">
                <td>
                  <input
                    type="checkbox"
                    :checked="selectedIds.includes(api.id)"
                    @change="toggleSelectOne(api.id)"
                  />
                </td>
                <td class="text-gray-500 dark:text-gray-400">{{ api.id }}</td>
                <td class="font-mono text-sm">{{ api.path }}</td>
                <td>
                  <span class="badge badge-primary">{{
                    api.group || "-"
                  }}</span>
                </td>
                <td class="text-gray-600 dark:text-gray-300">
                  {{ api.description || "-" }}
                </td>
                <td>
                  <span :class="methodBadgeClass(api.method)">{{
                    api.method
                  }}</span>
                </td>
                <td class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      class="btn btn-secondary text-xs"
                      @click="openEditModal(api)"
                    >
                      {{ t("common.edit", "编辑") }}
                    </button>
                    <button
                      class="btn btn-danger text-xs"
                      @click="confirmDelete(api)"
                    >
                      {{ t("common.delete", "删除") }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <Pagination
          v-if="total > 0"
          class="mt-4"
          :page="page"
          :total="total"
          :page-size="pageSize"
          :page-size-options="[20, 50, 100, 200]"
          @update:page="gotoPage"
          @update:pageSize="handlePageSizeChange"
        />
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
                editingApi
                  ? t("common.edit", "编辑 API")
                  : t("admin.rbac.createApi", "新增 API")
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
            <div class="mb-4 grid grid-cols-2 gap-4">
              <div>
                <label
                  class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t("admin.rbac.apiMethod", "请求方法") }} *</label
                >
                <select v-model="form.method" class="input">
                  <option v-for="m in methodOptions" :key="m" :value="m">
                    {{ m }}
                  </option>
                </select>
              </div>
              <div>
                <label
                  class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t("admin.rbac.apiGroup", "API 分组") }}</label
                >
                <input
                  v-model="form.group"
                  class="input"
                  :placeholder="
                    t('admin.rbac.apiGroupPlaceholder', '如: users')
                  "
                />
              </div>
            </div>
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.apiPath", "API 路径") }} *</label
              >
              <input
                v-model="form.path"
                class="input"
                :placeholder="
                  t('admin.rbac.apiPathPlaceholder', '如: /admin/users/:id')
                "
              />
            </div>
            <div class="mb-4">
              <label
                class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
                >{{ t("admin.rbac.apiDescription", "API 简介") }}</label
              >
              <input
                v-model="form.description"
                class="input"
                :placeholder="
                  t('admin.rbac.apiDescPlaceholder', '简要描述此接口用途')
                "
              />
            </div>
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
            <button class="btn btn-primary" :disabled="saving" @click="saveApi">
              {{
                saving
                  ? t("common.saving", "保存中...")
                  : t("common.save", "保存")
              }}
            </button>
          </div>
        </div>
      </div>

      <!-- Delete Confirmation -->
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
            <p v-if="deletingApi" class="text-gray-600 dark:text-gray-300">
              {{
                t("admin.rbac.deleteApiMsg", {
                  method: deletingApi.method,
                  path: deletingApi.path,
                }) || `确定要删除 ${deletingApi.method} ${deletingApi.path} 吗?`
              }}
            </p>
            <p v-else class="text-gray-600 dark:text-gray-300">
              {{
                t("admin.rbac.deleteApisMsg", {
                  count: selectedIds.length,
                }) || `确定要删除已选中的 ${selectedIds.length} 条 API 吗?`
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
import type { AdminAPI } from "@/api/admin/rbac";
import AppLayout from "@/components/layout/AppLayout.vue";
import SearchInput from "@/components/common/SearchInput.vue";
import Select from "@/components/common/Select.vue";
import Pagination from "@/components/common/Pagination.vue";

const { t } = useI18n();

const methodOptions = ["GET", "POST", "PUT", "DELETE", "PATCH"];

const apis = ref<AdminAPI[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const saving = ref(false);
const syncing = ref(false);

const filter = ref({
  keyword: "",
  group: "",
  method: "",
});
const groupOptions = ref<string[]>([]);

const selectedIds = ref<number[]>([]);

// Modal
const showModal = ref(false);
const editingApi = ref<AdminAPI | null>(null);
const form = ref({
  method: "GET",
  path: "",
  group: "",
  description: "",
  sort_order: 0,
});

const showDeleteModal = ref(false);
const deletingApi = ref<AdminAPI | null>(null); // null => 批量删除

const totalPages = computed(() =>
  Math.max(1, Math.ceil(total.value / pageSize.value)),
);
const groupSelectOptions = computed(() => [
  { value: "", label: t("admin.rbac.anyGroup", "全部分组") },
  ...groupOptions.value.map((g) => ({ value: g, label: g })),
]);
const methodSelectOptions = computed(() => [
  { value: "", label: t("admin.rbac.anyMethod", "全部方法") },
  ...methodOptions.map((m) => ({ value: m, label: m })),
]);
const allSelected = computed(
  () => apis.value.length > 0 && selectedIds.value.length === apis.value.length,
);
const partiallySelected = computed(
  () =>
    selectedIds.value.length > 0 &&
    selectedIds.value.length < apis.value.length,
);

onMounted(async () => {
  await loadGroups();
  await loadData();
});

async function loadGroups() {
  try {
    groupOptions.value = await rbacAPI.getApiGroups();
  } catch {
    // ignore
  }
}

async function loadData() {
  loading.value = true;
  try {
    const res = await rbacAPI.listApis({
      keyword: filter.value.keyword || undefined,
      group: filter.value.group || undefined,
      method: filter.value.method || undefined,
      page: page.value,
      page_size: pageSize.value,
    });
    apis.value = res.list ?? [];
    total.value = res.total ?? 0;
    selectedIds.value = [];
  } catch {
    // interceptor
  } finally {
    loading.value = false;
  }
}

function onSearch() {
  page.value = 1;
  loadData();
}

function onReset() {
  filter.value = { keyword: "", group: "", method: "" };
  page.value = 1;
  loadData();
}

function handlePageSizeChange(size: number) {
  pageSize.value = size;
  page.value = 1;
  loadData();
}

function gotoPage(next: number) {
  if (next < 1 || next > totalPages.value) return;
  page.value = next;
  loadData();
}

function toggleSelectAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked;
  selectedIds.value = checked ? apis.value.map((a) => a.id) : [];
}

function toggleSelectOne(id: number) {
  const idx = selectedIds.value.indexOf(id);
  if (idx >= 0) {
    selectedIds.value.splice(idx, 1);
  } else {
    selectedIds.value.push(id);
  }
}

function openCreateModal() {
  editingApi.value = null;
  form.value = {
    method: "GET",
    path: "",
    group: "",
    description: "",
    sort_order: 0,
  };
  showModal.value = true;
}

function openEditModal(api: AdminAPI) {
  editingApi.value = api;
  form.value = {
    method: api.method,
    path: api.path,
    group: api.group,
    description: api.description,
    sort_order: api.sort_order,
  };
  showModal.value = true;
}

async function saveApi() {
  if (!form.value.path.trim() || !form.value.method.trim()) return;
  saving.value = true;
  try {
    if (editingApi.value) {
      await rbacAPI.updateApi(editingApi.value.id, form.value);
    } else {
      await rbacAPI.createApi(form.value);
    }
    showModal.value = false;
    await loadData();
    await loadGroups();
  } catch {
    // interceptor
  } finally {
    saving.value = false;
  }
}

function confirmDelete(api: AdminAPI) {
  deletingApi.value = api;
  showDeleteModal.value = true;
}

function confirmBatchDelete() {
  if (selectedIds.value.length === 0) return;
  deletingApi.value = null;
  showDeleteModal.value = true;
}

async function doDelete() {
  saving.value = true;
  try {
    if (deletingApi.value) {
      await rbacAPI.deleteApi(deletingApi.value.id);
    } else {
      await rbacAPI.deleteApis(selectedIds.value);
    }
    showDeleteModal.value = false;
    await loadData();
    await loadGroups();
  } catch {
    // interceptor
  } finally {
    saving.value = false;
  }
}

async function doSync() {
  syncing.value = true;
  try {
    await rbacAPI.syncApis();
    await loadData();
    await loadGroups();
  } catch {
    // interceptor
  } finally {
    syncing.value = false;
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
</script>
