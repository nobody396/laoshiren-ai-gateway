<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <SearchInput v-model="search" :placeholder="t('feedback.admin.search')" @search="applyFilters" class="w-full sm:w-72" />
          <Select v-model="category" :options="categoryFilterOptions" class="w-40" @change="applyFilters" />
          <Select v-model="status" :options="statusFilterOptions" class="w-40" @change="applyFilters" />
          <Select v-model="priority" :options="priorityFilterOptions" class="w-40" @change="applyFilters" />
          <DateRangePicker
            :start-date="startDate"
            :end-date="endDate"
            @update:startDate="startDate = $event"
            @update:endDate="endDate = $event"
            @change="applyFilters"
          />
        </div>
      </template>

      <template #actions>
        <div class="flex flex-wrap justify-end gap-3">
		  <button class="btn btn-secondary" @click="toggleRewards">{{ t('feedback.admin.rewardLedger') }}</button>
          <Select v-model="batchStatus" :options="batchStatusOptions" class="w-40" />
          <button class="btn btn-secondary" :disabled="selectedIds.length === 0 || !batchStatus" @click="submitBatchStatus">
            {{ t('feedback.admin.batchUpdate') }}
          </button>
          <button class="btn btn-danger" :disabled="selectedIds.length === 0" @click="showBatchDeleteConfirm = true">
            {{ t('feedback.admin.batchDelete') }}
          </button>
          <button class="btn btn-secondary" :disabled="loading" @click="loadFeedbacks">{{ t('common.refresh') }}</button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading" row-key="id">
          <template #header-select>
            <input type="checkbox" :checked="allChecked" @change="toggleAll($event)" />
          </template>

          <template #cell-select="{ row }">
            <input type="checkbox" :checked="selectedIds.includes(row.id)" @change="toggleRow(row.id, $event)" />
          </template>

          <template #cell-category="{ value }">
            <StatusBadge :label="t(`feedback.category.${value}`)" tone="gray" />
          </template>

          <template #cell-status="{ value }">
            <StatusBadge :label="t(`feedback.status.${value}`)" :tone="feedbackStatusTone(value)" />
          </template>

          <template #cell-priority="{ value }">
            <StatusBadge :label="t(`feedback.priority.${value}`)" :tone="feedbackPriorityTone(value)" />
          </template>

          <template #cell-title="{ row }">
            <div class="space-y-1">
              <div class="font-medium text-gray-900 dark:text-white">{{ row.title }}</div>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                {{ row.user?.username || row.user?.email || '-' }}
              </div>
            </div>
          </template>

          <template #cell-last_reply_at="{ row }">
            <div class="space-y-1 text-sm">
              <div class="text-gray-500 dark:text-dark-400">{{ row.last_reply_at ? formatDateTime(row.last_reply_at) : '-' }}</div>
              <div v-if="row.last_reply_role" class="text-xs text-gray-400 dark:text-dark-400">{{ t(`feedback.reply.${row.last_reply_role}`) }}</div>
            </div>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <RouterLink :to="`/admin/feedbacks/${row.id}`" class="btn btn-secondary btn-sm">
                {{ t('feedback.detail.view') }}
              </RouterLink>
              <button class="btn btn-danger btn-sm" @click="confirmDeleteSingle(row.id)">
                {{ t('common.delete') }}
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState :title="t('feedback.empty.title')" :description="t('feedback.empty.description')" />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

	<BaseDialog :show="showRewards" :title="t('feedback.admin.rewardLedger')" width="extra-wide" @close="showRewards = false">
	  <div v-if="rewardsLoading" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
	  <div v-else class="overflow-x-auto">
		<table class="min-w-full text-sm">
		  <thead class="text-left text-gray-500"><tr><th class="p-2">{{ t('feedback.admin.rewardColumns.feedback') }}</th><th class="p-2">{{ t('feedback.admin.rewardColumns.user') }}</th><th class="p-2">{{ t('feedback.admin.rewardColumns.amount') }}</th><th class="p-2">{{ t('feedback.admin.rewardColumns.batch') }}</th><th class="p-2">{{ t('feedback.admin.rewardColumns.ledger') }}</th><th class="p-2">{{ t('feedback.admin.rewardColumns.time') }}</th></tr></thead>
		  <tbody><tr v-for="reward in rewards" :key="reward.id" class="border-t border-gray-100 dark:border-dark-700"><td class="p-2"><RouterLink class="text-primary-600 hover:underline" :to="`/admin/feedbacks/${reward.feedback_id}`">#{{ reward.feedback_id }}</RouterLink></td><td class="p-2"><div class="font-medium text-gray-900 dark:text-white">{{ reward.user?.email || reward.user?.username || '-' }}</div><div v-if="reward.user?.email && reward.user?.username" class="text-xs text-gray-500">{{ reward.user.username }}</div></td><td class="p-2 font-semibold text-emerald-600">+{{ reward.amount.toFixed(2) }}</td><td class="p-2"><code>{{ reward.batch_id }}</code></td><td class="p-2">{{ reward.account_change_record_id || '-' }}</td><td class="p-2">{{ formatDateTime(reward.granted_at) }}</td></tr></tbody>
		</table>
		<p v-if="rewards.length === 0" class="py-8 text-center text-gray-500">{{ t('feedback.admin.noRewards') }}</p>
	  </div>
	</BaseDialog>

    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('common.delete')"
      :message="t('feedback.admin.confirmDelete')"
      :danger="true"
      @confirm="handleDeleteSingle"
      @cancel="showDeleteConfirm = false"
    />

    <ConfirmDialog
      :show="showBatchDeleteConfirm"
      :title="t('feedback.admin.batchDelete')"
      :message="t('feedback.admin.confirmBatchDelete', { count: selectedIds.length })"
      :danger="true"
      @confirm="handleBatchDelete"
      @cancel="showBatchDeleteConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Column } from '@/components/common/types'
import type { FeedbackItem, FeedbackReward, FeedbackStatus } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import adminFeedbacksAPI from '@/api/admin/feedbacks'
import { formatDateTime } from '@/utils/format'
import { feedbackCategoryOptions, feedbackPriorityOptions, feedbackPriorityTone, feedbackStatusOptions, feedbackStatusTone, toDayEndRFC3339, toDayStartRFC3339 } from '@/utils/feedback'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const items = ref<FeedbackItem[]>([])
const search = ref('')
const category = ref('')
const status = ref('')
const priority = ref('')
const startDate = ref('')
const endDate = ref('')
const batchStatus = ref<FeedbackStatus | ''>('')
const selectedIds = ref<number[]>([])
const showDeleteConfirm = ref(false)
const showBatchDeleteConfirm = ref(false)
const deleteTargetId = ref<number | null>(null)
const showRewards = ref(false)
const rewardsLoading = ref(false)
const rewards = ref<FeedbackReward[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1,
})

const categoryFilterOptions = computed(() => [
  { value: '', label: t('common.all') },
  ...feedbackCategoryOptions.map((item) => ({ value: item, label: t(`feedback.category.${item}`) })),
])

const statusFilterOptions = computed(() => feedbackStatusOptions.map((item) => ({
  value: item,
  label: item ? t(`feedback.status.${item}`) : t('common.all'),
})))

const priorityFilterOptions = computed(() => feedbackPriorityOptions.map((item) => ({
  value: item,
  label: item ? t(`feedback.priority.${item}`) : t('common.all'),
})))

const batchStatusOptions = computed(() =>
  feedbackStatusOptions
    .filter((item): item is FeedbackStatus => item !== '')
    .map((item) => ({ value: item, label: t(`feedback.status.${item}`) }))
)

const allChecked = computed(() => items.value.length > 0 && selectedIds.value.length === items.value.length)

const columns = computed<Column[]>(() => [
  { key: 'select', label: '' },
  { key: 'category', label: t('feedback.columns.category') },
  { key: 'title', label: t('feedback.columns.title') },
  { key: 'status', label: t('feedback.columns.status') },
  { key: 'priority', label: t('feedback.columns.priority') },
  { key: 'reply_count', label: t('feedback.columns.replyCount') },
  { key: 'last_reply_at', label: t('feedback.columns.lastReplyAt') },
  { key: 'created_at', label: t('feedback.columns.createdAt') },
  { key: 'actions', label: t('common.actions') },
])

async function loadFeedbacks() {
  loading.value = true
  try {
    const response = await adminFeedbacksAPI.list({
      page: pagination.value.page,
      pageSize: pagination.value.page_size,
      category: category.value || undefined,
      status: status.value || undefined,
      priority: priority.value || undefined,
      search: search.value || undefined,
      start_time: startDate.value ? toDayStartRFC3339(startDate.value) : undefined,
      end_time: endDate.value ? toDayEndRFC3339(endDate.value) : undefined,
    })
    items.value = response.items
    selectedIds.value = selectedIds.value.filter((id) => response.items.some((item) => item.id === id))
    pagination.value = {
      page: response.page,
      page_size: response.page_size,
      total: response.total,
      pages: response.pages,
    }
  } catch {
    appStore.showError(t('feedback.message.loadFailed'))
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  pagination.value.page = 1
  loadFeedbacks()
}

function toggleAll(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedIds.value = checked ? items.value.map((item) => item.id) : []
}

function toggleRow(id: number, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedIds.value = checked ? [...selectedIds.value, id] : selectedIds.value.filter((item) => item !== id)
}

async function submitBatchStatus() {
  if (!batchStatus.value || selectedIds.value.length === 0) {
    return
  }
  try {
    await adminFeedbacksAPI.batchUpdateStatus(selectedIds.value, batchStatus.value)
    appStore.showSuccess(t('feedback.message.batchUpdated'))
    selectedIds.value = []
    await loadFeedbacks()
  } catch {
    appStore.showError(t('feedback.message.batchUpdateFailed'))
  }
}

function confirmDeleteSingle(id: number) {
  deleteTargetId.value = id
  showDeleteConfirm.value = true
}

async function handleDeleteSingle() {
  showDeleteConfirm.value = false
  if (deleteTargetId.value === null) return
  try {
    await adminFeedbacksAPI.delete(deleteTargetId.value)
    appStore.showSuccess(t('feedback.message.deleted'))
    await loadFeedbacks()
  } catch {
    appStore.showError(t('feedback.message.deleteFailed'))
  } finally {
    deleteTargetId.value = null
  }
}

async function handleBatchDelete() {
  showBatchDeleteConfirm.value = false
  if (selectedIds.value.length === 0) return
  try {
    const result = await adminFeedbacksAPI.batchDelete(selectedIds.value)
    appStore.showSuccess(t('feedback.message.batchDeleted', { count: result.deleted }))
    selectedIds.value = []
    await loadFeedbacks()
  } catch {
    appStore.showError(t('feedback.message.batchDeleteFailed'))
  }
}

function handlePageChange(page: number) {
  pagination.value.page = page
  loadFeedbacks()
}

function handlePageSizeChange(pageSize: number) {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadFeedbacks()
}

onMounted(loadFeedbacks)

async function toggleRewards() {
  showRewards.value = !showRewards.value
  if (!showRewards.value) return
  rewardsLoading.value = true
  try { rewards.value = (await adminFeedbacksAPI.listRewards({ pageSize: 100 })).items }
  catch { appStore.showError(t('feedback.message.loadFailed')) }
  finally { rewardsLoading.value = false }
}
</script>
