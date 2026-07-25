<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-72">
            <input
              v-model="searchQuery"
              type="search"
              class="input"
              :placeholder="t('admin.changelog.search')"
              @input="scheduleSearch"
            />
          </div>
          <Select v-model="filters.status" :options="statusOptions" class="w-40" @change="resetAndLoad" />
          <Select v-model="filters.category" :options="categoryOptions" class="w-44" @change="resetAndLoad" />
          <div class="flex flex-1 items-center justify-end gap-2">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadEntries">
              <Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" />
            </button>
            <button type="button" class="btn btn-primary" @click="openCreate">
              <Icon name="plus" size="md" />
              {{ t('admin.changelog.createEntry') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="entries" :loading="loading">
          <template #cell-title="{ row }">
            <div class="min-w-0">
              <p class="truncate font-medium text-gray-900 dark:text-white">{{ row.title }}</p>
              <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ row.summary }}</p>
            </div>
          </template>

          <template #cell-category="{ row }">
            <span class="badge badge-gray">{{ categoryLabel(row.category) }}</span>
          </template>

          <template #cell-status="{ row }">
            <span :class="['badge', statusClass(row)]">{{ statusLabel(row) }}</span>
          </template>

          <template #cell-publishedAt="{ row }">
            <span class="text-sm text-gray-600 dark:text-gray-300">
              {{ row.published_at ? formatDateTime(row.published_at) : '—' }}
            </span>
          </template>

          <template #cell-publicLink="{ row }">
            <button
              v-if="row.status === 'published'"
              type="button"
              class="inline-flex items-center gap-1 text-xs text-primary-600 hover:underline dark:text-primary-400"
              @click="copyPublicLink(row)"
            >
              <Icon name="copy" size="xs" />
              {{ t('admin.changelog.copyLink') }}
            </button>
            <span v-else class="text-gray-400">—</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <a
                v-if="row.status === 'published'"
                :href="`/changelog/${row.slug}`"
                target="_blank"
                rel="noopener noreferrer"
                class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:hover:bg-dark-700 dark:hover:text-white"
                :title="t('common.preview')"
              >
                <Icon name="eye" size="sm" />
              </a>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:hover:bg-dark-700 dark:hover:text-white"
                :title="t('common.edit')"
                @click="openEdit(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                v-if="row.status === 'published'"
                type="button"
                class="rounded-lg p-1.5 text-gray-500 hover:bg-amber-50 hover:text-amber-700 dark:hover:bg-amber-900/20 dark:hover:text-amber-300"
                :title="t('admin.changelog.archive')"
                @click="archiveEntry(row)"
              >
                <Icon name="archive" size="sm" />
              </button>
              <button
                v-else
                type="button"
                class="rounded-lg p-1.5 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('common.delete')"
                @click="requestDelete(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.changelog.emptyTitle')"
              :description="t('admin.changelog.emptyDescription')"
              :action-text="t('admin.changelog.createEntry')"
              @action="openCreate"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="changePage"
          @update:pageSize="changePageSize"
        />
      </template>
    </TablePageLayout>

    <ChangelogEditorDrawer
      :show="editorOpen"
      :entry="editingEntry"
      :saving="saving"
      @close="closeEditor"
      @save="saveEntry"
    />

    <ConfirmDialog
      :show="deleteDialogOpen"
      :title="t('admin.changelog.deleteTitle')"
      :message="t('admin.changelog.deleteConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="deleteDialogOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import ChangelogEditorDrawer from '@/components/admin/changelog/ChangelogEditorDrawer.vue'
import type {
  AdminChangelogEntry,
  ChangelogCategory,
  CreateChangelogRequest
} from '@/types'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const entries = ref<AdminChangelogEntry[]>([])
const loading = ref(false)
const saving = ref(false)
const editorOpen = ref(false)
const editingEntry = ref<AdminChangelogEntry | null>(null)
const deleteDialogOpen = ref(false)
const deletingEntry = ref<AdminChangelogEntry | null>(null)
const searchQuery = ref('')
const filters = reactive({ status: '', category: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0, pages: 0 })
let searchTimer: number | null = null
let requestID = 0

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('admin.changelog.columns.title') },
  { key: 'category', label: t('admin.changelog.columns.category') },
  { key: 'status', label: t('admin.changelog.columns.status') },
  { key: 'publishedAt', label: t('admin.changelog.columns.publishedAt') },
  { key: 'publicLink', label: t('admin.changelog.columns.publicLink') },
  { key: 'actions', label: t('admin.changelog.columns.actions') }
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.changelog.allStatuses') },
  { value: 'draft', label: t('admin.changelog.statuses.draft') },
  { value: 'published', label: t('admin.changelog.statuses.published') },
  { value: 'archived', label: t('admin.changelog.statuses.archived') }
])
const categoryOptions = computed(() => [
  { value: '', label: t('admin.changelog.allCategories') },
  { value: 'feature', label: t('changelog.categories.feature') },
  { value: 'model_config', label: t('changelog.categories.modelConfig') },
  { value: 'improvement', label: t('changelog.categories.improvement') },
  { value: 'fix', label: t('changelog.categories.fix') }
])

function categoryLabel(value: ChangelogCategory) {
  return categoryOptions.value.find((item) => item.value === value)?.label ?? value
}
function isScheduled(entry: AdminChangelogEntry) {
  return entry.status === 'published' &&
    !!entry.published_at &&
    new Date(entry.published_at).getTime() > Date.now()
}
function statusLabel(entry: AdminChangelogEntry) {
  return isScheduled(entry)
    ? t('admin.changelog.statuses.scheduled')
    : t(`admin.changelog.statuses.${entry.status}`)
}
function statusClass(entry: AdminChangelogEntry) {
  if (isScheduled(entry)) return 'badge-warning'
  if (entry.status === 'published') return 'badge-success'
  if (entry.status === 'archived') return 'badge-warning'
  return 'badge-gray'
}

async function loadEntries() {
  const currentRequest = ++requestID
  loading.value = true
  try {
    const result = await adminAPI.changelog.list(pagination.page, pagination.page_size, {
      status: filters.status || undefined,
      category: filters.category || undefined,
      search: searchQuery.value.trim() || undefined
    })
    if (currentRequest !== requestID) return
    entries.value = result.items
    Object.assign(pagination, {
      page: result.page,
      page_size: result.page_size,
      total: result.total,
      pages: result.pages
    })
  } catch (error: any) {
    if (currentRequest !== requestID) return
    appStore.showError(error?.message || t('admin.changelog.loadFailed'))
  } finally {
    if (currentRequest === requestID) loading.value = false
  }
}

function resetAndLoad() {
  pagination.page = 1
  void loadEntries()
}
function scheduleSearch() {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(resetAndLoad, 350)
}
function changePage(value: number) {
  pagination.page = value
  void loadEntries()
}
function changePageSize(value: number) {
  pagination.page_size = value
  resetAndLoad()
}
function openCreate() {
  editingEntry.value = null
  editorOpen.value = true
}
function openEdit(entry: AdminChangelogEntry) {
  editingEntry.value = entry
  editorOpen.value = true
}
function closeEditor() {
  if (saving.value) return
  editorOpen.value = false
}

async function saveEntry(payload: CreateChangelogRequest) {
  saving.value = true
  try {
    if (editingEntry.value) {
      await adminAPI.changelog.update(editingEntry.value.id, payload)
      appStore.showSuccess(t('admin.changelog.updated'))
    } else {
      await adminAPI.changelog.create(payload)
      appStore.showSuccess(payload.status === 'published'
        ? t('admin.changelog.published')
        : t('admin.changelog.draftSaved'))
    }
    editorOpen.value = false
    await loadEntries()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.changelog.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function archiveEntry(entry: AdminChangelogEntry) {
  try {
    await adminAPI.changelog.update(entry.id, { status: 'archived' })
    appStore.showSuccess(t('admin.changelog.archived'))
    await loadEntries()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.changelog.saveFailed'))
  }
}

function requestDelete(entry: AdminChangelogEntry) {
  deletingEntry.value = entry
  deleteDialogOpen.value = true
}
async function confirmDelete() {
  if (!deletingEntry.value) return
  try {
    await adminAPI.changelog.delete(deletingEntry.value.id)
    appStore.showSuccess(t('admin.changelog.deleted'))
    deleteDialogOpen.value = false
    deletingEntry.value = null
    await loadEntries()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.changelog.deleteFailed'))
  }
}

async function copyPublicLink(entry: AdminChangelogEntry) {
  const url = new URL(`/changelog/${entry.slug}`, window.location.origin).toString()
  await copyToClipboard(url, t('admin.changelog.linkCopied'))
}

onMounted(() => void loadEntries())
onBeforeUnmount(() => {
  requestID += 1
  if (searchTimer) window.clearTimeout(searchTimer)
})
</script>
